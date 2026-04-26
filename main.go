package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"telegram-bot/bot"

	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("Не удалось загрузить .env, использую переменные окружения")
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN не установлен")
	}

	ownerID, err := strconv.Atoi(os.Getenv("OWNER_ID"))
	if err != nil {
		log.Fatal("OWNER_ID не задан или некорректен")
	}

	ownerName := os.Getenv("OWNER_NAME")
	channelID, err := strconv.Atoi(os.Getenv("CHANNEL_ID"))
	if err != nil {
		log.Fatal("CHANNEL_ID не задан или некорректен")
	}

	postMessage := os.Getenv("POST_MESSAGE")

	// Параметры webhook
	webhookURL := os.Getenv("WEBHOOK_URL")
	if webhookURL == "" {
		log.Fatal("WEBHOOK_URL не установлен. Пример: https://bot.example.com")
	}

	webhookPort := os.Getenv("WEBHOOK_PORT")
	if webhookPort == "" {
		webhookPort = "8080"
	}
	webhookSecret := os.Getenv("WEBHOOK_SECRET_TOKEN")

	botInstance, err := bot.NewBot(
		token,
		int64(channelID), int64(ownerID),
		ownerName, postMessage,
		webhookURL, webhookPort, webhookSecret,
	)
	if err != nil {
		log.Fatalf("Ошибка создания бота: %v", err)
	}

	botInstance.Start()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	botInstance.Stop()
}
