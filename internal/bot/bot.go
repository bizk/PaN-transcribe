package bot

import (
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/override/pan-transcribe/internal/config"
	"github.com/override/pan-transcribe/internal/queue"
)

type Bot struct {
	api           *tgbotapi.BotAPI
	auth          *Authorizer
	jobStore      *queue.JobStore
	settingsStore *queue.SettingsStore
	config        *config.Config
	dataDir       string
}

func New(cfg *config.Config, jobStore *queue.JobStore, settingsStore *queue.SettingsStore, dataDir string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.Telegram.BotToken)
	if err != nil {
		return nil, fmt.Errorf("creating bot API: %w", err)
	}

	log.Printf("Authorized on account %s", api.Self.UserName)

	return &Bot{
		api:           api,
		auth:          NewAuthorizer(cfg.Telegram.AllowedUsers),
		jobStore:      jobStore,
		settingsStore: settingsStore,
		config:        cfg,
		dataDir:       dataDir,
	}, nil
}

func (b *Bot) Start() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		// Check authorization
		if !b.auth.IsAllowed(update.Message.From.ID) {
			log.Printf("Unauthorized access attempt from user %d", update.Message.From.ID)
			continue
		}

		go b.handleMessage(update.Message)
	}

	return nil
}

func (b *Bot) handleMessage(msg *tgbotapi.Message) {
	if msg.IsCommand() {
		b.handleCommand(msg)
		return
	}

	// Check for audio file
	if msg.Audio != nil || msg.Voice != nil || msg.Document != nil {
		b.handleAudioUpload(msg)
		return
	}
}

func (b *Bot) handleCommand(msg *tgbotapi.Message) {
	switch msg.Command() {
	case "start":
		b.handleStart(msg)
	case "transcribe":
		b.handleTranscribe(msg)
	case "summarize":
		b.handleSummarize(msg)
	case "status":
		b.handleStatus(msg)
	case "setprompt":
		b.handleSetPrompt(msg)
	case "showprompt":
		b.handleShowPrompt(msg)
	case "cloud":
		b.handleCloud(msg)
	case "local":
		b.handleLocal(msg)
	default:
		b.reply(msg.Chat.ID, "Comando desconocido. Usa /start para ver los comandos disponibles.")
	}
}

func (b *Bot) reply(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending message: %v", err)
	}
}

func (b *Bot) replyWithFile(chatID int64, filePath string, caption string) error {
	file := tgbotapi.FilePath(filePath)
	doc := tgbotapi.NewDocument(chatID, file)
	doc.Caption = caption

	_, err := b.api.Send(doc)
	return err
}

func (b *Bot) SendResult(chatID int64, outputPath, summaryPath string) error {
	// Send transcription file
	if err := b.replyWithFile(chatID, outputPath, "Transcripción completada"); err != nil {
		return fmt.Errorf("sending transcription: %w", err)
	}

	// Send summary file if exists
	if summaryPath != "" {
		if err := b.replyWithFile(chatID, summaryPath, "Resumen de la clase"); err != nil {
			return fmt.Errorf("sending summary: %w", err)
		}
	}

	return nil
}

func (b *Bot) SendError(chatID int64, errorMsg string) {
	b.reply(chatID, fmt.Sprintf("Error: %s", errorMsg))
}

func (b *Bot) SendProgress(chatID int64, percent int, estimatedMinutes int) {
	text := fmt.Sprintf("Procesando... %d%% completado. Tiempo restante estimado: ~%d min", percent, estimatedMinutes)
	b.reply(chatID, text)
}
