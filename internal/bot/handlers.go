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

	// Get file info from Telegram
	file, err := b.api.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		log.Printf("Error getting file: %v", err)
		b.reply(msg.Chat.ID, "Error al obtener el archivo.")
		return
	}

	// Notify user that download is starting
	b.reply(msg.Chat.ID, "Descargando audio...")

	// Prepare audio directory
	audioDir := filepath.Join(b.dataDir, "audio")
	if err := os.MkdirAll(audioDir, 0755); err != nil {
		log.Printf("Error creating audio dir: %v", err)
		b.reply(msg.Chat.ID, "Error interno al preparar directorio.")
		return
	}

	// Download to temp file first (use timestamp to avoid conflicts)
	tempPath := filepath.Join(audioDir, fmt.Sprintf("temp_%d_%d%s", msg.Chat.ID, msg.MessageID, ext))
	if err := b.downloadFile(file.Link(b.api.Token), tempPath); err != nil {
		log.Printf("Error downloading file: %v", err)
		b.reply(msg.Chat.ID, "Error al descargar el archivo.")
		return
	}

	// Get withSummary setting from user preferences (set by /transcribe or /summarize commands)
	withSummary, _ := b.settingsStore.GetAndClearNextWithSummary(msg.From.ID)

	// Create job with audio path already set
	job := &queue.Job{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.MessageID,
		AudioPath:   tempPath,
		WithSummary: withSummary,
	}

	jobID, err := b.jobStore.Create(job)
	if err != nil {
		log.Printf("Error creating job: %v", err)
		os.Remove(tempPath) // Clean up downloaded file
		b.reply(msg.Chat.ID, "Error al crear el trabajo.")
		return
	}

	// Rename temp file to final path with job ID
	finalPath := filepath.Join(audioDir, fmt.Sprintf("%d%s", jobID, ext))
	if err := os.Rename(tempPath, finalPath); err != nil {
		log.Printf("Error renaming file: %v", err)
		// File is still at tempPath, update the job to use that path
	} else {
		// Update job with final path
		if err := b.setJobAudioPath(jobID, finalPath); err != nil {
			log.Printf("Error updating audio path: %v", err)
		}
	}

	// Get queue position
	position, _ := b.jobStore.GetPendingBefore(jobID)
	position++ // 1-indexed for user

	b.reply(msg.Chat.ID, fmt.Sprintf(
		"Audio recibido. Posición en cola: #%d. Usa /status para ver el progreso.",
		position,
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
