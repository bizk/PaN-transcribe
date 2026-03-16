package bot

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/override/pan-transcribe/internal/queue"
)

var supportedFormats = map[string]bool{
	".mp3":  true,
	".wav":  true,
	".m4a":  true,
	".ogg":  true,
	".flac": true,
	".oga":  true,
}

func (b *Bot) handleStart(msg *tgbotapi.Message) {
	text := `Hola! Soy tu bot de transcripción para clases de psicología.

Comandos disponibles:
/transcribe - Solo transcribir audio
/summarize - Transcribir y generar resumen
/status - Ver estado de trabajos pendientes
/setprompt <texto> - Personalizar prompt de resumen
/showprompt - Ver prompt actual
/cloud - Usar API de OpenAI (más rápido)
/local - Usar Whisper local (por defecto)

Envíame un archivo de audio (mp3, wav, m4a, ogg, flac) después de seleccionar el modo con /transcribe o /summarize.`

	b.reply(msg.Chat.ID, text)
}

func (b *Bot) handleTranscribe(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextWithSummary(msg.From.ID, false); err != nil {
		log.Printf("Error setting mode: %v", err)
	}
	b.reply(msg.Chat.ID, "Modo transcripción activado. Envíame un archivo de audio.")
}

func (b *Bot) handleSummarize(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextWithSummary(msg.From.ID, true); err != nil {
		log.Printf("Error setting mode: %v", err)
	}
	b.reply(msg.Chat.ID, "Modo transcripción + resumen activado. Envíame un archivo de audio.")
}

func (b *Bot) handleStatus(msg *tgbotapi.Message) {
	jobs, err := b.jobStore.GetJobsForUser(msg.Chat.ID)
	if err != nil {
		log.Printf("Error getting jobs: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener estado de trabajos.")
		return
	}

	if len(jobs) == 0 {
		b.reply(msg.Chat.ID, "No tienes trabajos pendientes o en proceso.")
		return
	}

	var sb strings.Builder
	sb.WriteString("Tus trabajos:\n\n")

	for _, job := range jobs {
		status := "Pendiente"
		if job.Status == queue.StatusProcessing {
			status = "Procesando"
		}
		sb.WriteString(fmt.Sprintf("- #%d: %s\n", job.ID, status))
	}

	b.reply(msg.Chat.ID, sb.String())
}

func (b *Bot) handleSetPrompt(msg *tgbotapi.Message) {
	prompt := strings.TrimPrefix(msg.Text, "/setprompt ")
	prompt = strings.TrimSpace(prompt)

	if prompt == "" || prompt == "/setprompt" {
		b.reply(msg.Chat.ID, "Uso: /setprompt <tu prompt personalizado>")
		return
	}

	if err := b.settingsStore.SetCustomPrompt(msg.From.ID, prompt); err != nil {
		log.Printf("Error setting prompt: %v", err)
		b.reply(msg.Chat.ID, "Error al guardar el prompt.")
		return
	}

	b.reply(msg.Chat.ID, "Prompt personalizado guardado.")
}

func (b *Bot) handleShowPrompt(msg *tgbotapi.Message) {
	prompt, err := b.settingsStore.GetCustomPrompt(msg.From.ID)
	if err != nil {
		log.Printf("Error getting prompt: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener el prompt.")
		return
	}

	if prompt == "" {
		prompt = b.config.Summary.DefaultPrompt
		b.reply(msg.Chat.ID, fmt.Sprintf("Usando prompt por defecto:\n\n%s", prompt))
	} else {
		b.reply(msg.Chat.ID, fmt.Sprintf("Tu prompt personalizado:\n\n%s", prompt))
	}
}

func (b *Bot) handleCloud(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextMode(msg.From.ID, "cloud"); err != nil {
		log.Printf("Error setting mode: %v", err)
		b.reply(msg.Chat.ID, "Error al configurar modo.")
		return
	}
	b.reply(msg.Chat.ID, "El próximo audio se procesará con la API de OpenAI (más rápido).")
}

func (b *Bot) handleLocal(msg *tgbotapi.Message) {
	if err := b.settingsStore.SetNextMode(msg.From.ID, "local"); err != nil {
		log.Printf("Error setting mode: %v", err)
		b.reply(msg.Chat.ID, "Error al configurar modo.")
		return
	}
	b.reply(msg.Chat.ID, "El próximo audio se procesará con Whisper local.")
}

func (b *Bot) handleAudioUpload(msg *tgbotapi.Message) {
	var fileID string
	var fileName string
	var fileSize int

	if msg.Audio != nil {
		fileID = msg.Audio.FileID
		fileName = msg.Audio.FileName
		fileSize = msg.Audio.FileSize
	} else if msg.Voice != nil {
		fileID = msg.Voice.FileID
		fileName = "voice.ogg"
		fileSize = msg.Voice.FileSize
	} else if msg.Document != nil {
		fileID = msg.Document.FileID
		fileName = msg.Document.FileName
		fileSize = msg.Document.FileSize
	}

	// Check file size
	maxSize := b.config.Processing.MaxFileSizeMB * 1024 * 1024
	if fileSize > maxSize {
		b.reply(msg.Chat.ID, fmt.Sprintf("Archivo muy grande. Máximo: %dMB", b.config.Processing.MaxFileSizeMB))
		return
	}

	// Check format
	ext := strings.ToLower(filepath.Ext(fileName))
	if !supportedFormats[ext] {
		b.reply(msg.Chat.ID, "Formato no soportado. Usa: mp3, wav, m4a, ogg, flac")
		return
	}

	// Download file
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Printf("Error getting file: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener el archivo.")
		return
	}

	// Create job first to get ID for filename
	mode, _ := b.settingsStore.GetAndClearNextMode(msg.From.ID)
	if mode == "" {
		mode = b.config.Processing.DefaultMode
	}

	// Get withSummary setting from user preferences (set by /transcribe or /summarize commands)
	withSummary, _ := b.settingsStore.GetAndClearNextWithSummary(msg.From.ID)

	// Download to temp file
	audioDir := filepath.Join(b.dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		log.Printf("Error creating audio dir: %v", err)
		b.reply(msg.Chat.ID, "Error interno al preparar directorio.")
		return
	}

	// Create job
	job := &queue.Job{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.MessageID,
		Mode:        mode,
		WithSummary: withSummary,
	}

	jobID, err := b.jobStore.Create(job)
	if err != nil {
		log.Printf("Error creating job: %v", err)
		b.reply(msg.Chat.ID, "Error al crear el trabajo.")
		return
	}

	// Download file
	audioPath := filepath.Join(audioDir, fmt.Sprintf("%d%s", jobID, ext))
	if err := b.downloadFile(file.Link(b.api.Token), audioPath); err != nil {
		log.Printf("Error downloading file: %v", err)
		b.jobStore.Fail(jobID, "Error al descargar archivo")
		b.reply(msg.Chat.ID, "Error al descargar el archivo.")
		return
	}

	// Update job with audio path
	if err := b.setJobAudioPath(jobID, audioPath); err != nil {
		log.Printf("Error setting audio path: %v", err)
		b.reply(msg.Chat.ID, "Error interno.")
		return
	}

	// Get queue position
	position, _ := b.jobStore.GetPendingBefore(jobID)
	position++ // 1-indexed for user

	// Estimate time (rough: 1 min audio = 2 min processing for local)
	estimatedMinutes := 5 // Base estimate
	if mode == "cloud" {
		estimatedMinutes = 2
	}

	b.reply(msg.Chat.ID, fmt.Sprintf(
		"Audio recibido. Posición en cola: #%d. Tiempo estimado: ~%d minutos. Usa /status para ver el progreso.",
		position, estimatedMinutes,
	))
}

func (b *Bot) downloadFile(url, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func (b *Bot) setJobAudioPath(jobID int64, audioPath string) error {
	return b.jobStore.SetAudioPath(jobID, audioPath)
}
