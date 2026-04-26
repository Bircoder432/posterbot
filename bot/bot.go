package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"telegram-bot/database"
	"telegram-bot/handlers"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
)

type Bot struct {
	bot           *telego.Bot
	db            *database.Database
	botHandler    *th.BotHandler
	channelID     int64
	ownerID       int64
	ownerName     string
	postMessage   string
	webhookURL    string
	webhookPort   string
	webhookSecret string
	updatesChan   chan telego.Update
	httpServer    *http.Server
}

func NewBot(
	token string,
	channelID, ownerID int64,
	ownerName, postMessage,
	webhookURL, webhookPort, webhookSecret string,
) (*Bot, error) {
	bot, err := telego.NewBot(token)
	if err != nil {
		return nil, err
	}

	db, err := database.NewDatabase()
	if err != nil {
		return nil, err
	}

	botInstance := &Bot{
		bot:           bot,
		db:            db,
		channelID:     channelID,
		ownerID:       ownerID,
		ownerName:     ownerName,
		postMessage:   postMessage,
		webhookURL:    webhookURL,
		webhookPort:   webhookPort,
		webhookSecret: webhookSecret,
	}

	botInstance.initializeOwner()

	return botInstance, nil
}

func (b *Bot) initializeOwner() {
	if !b.db.IsAdmin(b.ownerID) {
		err := b.db.AddAdmin(b.ownerID, b.ownerName)
		if err != nil {
			log.Printf("Предупреждение: не удалось добавить владельца: %v", err)
		} else {
			log.Printf("✅ Владелец %d добавлен как администратор", b.ownerID)
		}
	}
}

func (b *Bot) Start() {
	// Устанавливаем webhook в Telegram
	webhookParams := &telego.SetWebhookParams{
		URL: fmt.Sprintf("%s/%s", b.webhookURL, b.bot.Token()),
	}
	if b.webhookSecret != "" {
		webhookParams.SecretToken = b.webhookSecret
	}

	if err := b.bot.SetWebhook(webhookParams); err != nil {
		log.Fatalf("Ошибка установки webhook: %v", err)
	}

	// Канал для обновлений из HTTP-обработчика
	b.updatesChan = make(chan telego.Update, 100)

	botHandler, err := th.NewBotHandler(b.bot, b.updatesChan)
	if err != nil {
		log.Fatalf("Ошибка создания обработчика: %v", err)
	}

	b.registerHandlers(botHandler)
	b.botHandler = botHandler
	go botHandler.Start()

	// HTTP-сервер для приёма webhook от Telegram
	mux := http.NewServeMux()
	mux.HandleFunc("/"+b.bot.Token(), b.handleWebhook)

	b.httpServer = &http.Server{
		Addr:    ":" + b.webhookPort,
		Handler: mux,
	}

	go func() {
		log.Printf("🌐 Webhook-сервер слушает порт %s", b.webhookPort)
		if err := b.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска сервера: %v", err)
		}
	}()

	log.Println("🤖 Бот-предложка запущен в режиме webhook!")
}

func (b *Bot) handleWebhook(w http.ResponseWriter, r *http.Request) {
	// Проверка секретного токена (если задан)
	if b.webhookSecret != "" {
		if r.Header.Get("X-Telegram-Bot-Api-Secret-Token") != b.webhookSecret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
	}

	var update telego.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		log.Printf("Ошибка декодирования webhook: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	b.updatesChan <- update
	w.WriteHeader(http.StatusOK)
}

func (b *Bot) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. Останавливаем HTTP-сервер (новые запросы не принимаем)
	if b.httpServer != nil {
		if err := b.httpServer.Shutdown(ctx); err != nil {
			log.Printf("Ошибка остановки сервера: %v", err)
		}
	}

	// 2. Останавливаем обработчик
	if b.botHandler != nil {
		b.botHandler.Stop()
	}

	// 3. Удаляем webhook в Telegram
	b.bot.DeleteWebhook(&telego.DeleteWebhookParams{DropPendingUpdates: true})

	// 4. Закрываем канал обновлений
	if b.updatesChan != nil {
		close(b.updatesChan)
	}

	log.Println("Бот остановлен")
}

func (b *Bot) registerHandlers(bh *th.BotHandler) {
	mediaHandler := handlers.NewMediaHandler(b.db, b.postMessage)
	proposalsHandler := handlers.NewProposalsHandler(b.db, mediaHandler, b.channelID, b.ownerID)
	moderationHandler := handlers.NewModerationHandler(b.db, mediaHandler, b.channelID, b.ownerID)
	adminHandler := handlers.NewAdminHandler(b.db, b.ownerID)

	bh.Handle(proposalsHandler.HandleStartCommand, th.CommandEqual("start"))
	bh.Handle(moderationHandler.HandleProposalsCommand, th.CommandEqual("proposals"))
	bh.Handle(adminHandler.HandleAddAdminCommand, th.CommandEqual("addadmin"))
	bh.Handle(adminHandler.HandleListAdminsCommand, th.CommandEqual("admins"))
	bh.Handle(moderationHandler.HandlePardonCommand, th.CommandEqual("pardon"))

	bh.Handle(moderationHandler.HandleCallback, th.AnyCallbackQuery())
	bh.Handle(proposalsHandler.HandleUserProposal, th.AnyMessage())
}
