package handlers

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"telegram-bot/database"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

const welcomeText = `🤖 Добро пожаловать в анонимную предложку!

Просто отправьте сюда ваше предложение, идею или сообщение, и оно будет анонимно рассмотрено модераторами.

Ваша личность будет скрыта - модераторы увидят только содержание вашего сообщения.

❓ Что можно отправлять:
• Текстовые предложения
• Фотографии
• Документы
• Видео
• Кружочки (видеосообщения)
• Аудио и голосовые сообщения
• Стикеры
• Идеи и пожелания

Ваше предложение будет рассмотрено в ближайшее время!`

type ProposalsHandler struct {
	db          *database.Database
	media       *MediaHandler
	channelID   int64
	ownerID     int64
	botUsername string
}

func NewProposalsHandler(db *database.Database, media *MediaHandler, channelID, ownerID int64, botUsername string) *ProposalsHandler {
	return &ProposalsHandler{
		db:          db,
		media:       media,
		channelID:   channelID,
		ownerID:     ownerID,
		botUsername: botUsername,
	}
}

func (p *ProposalsHandler) HandleUserProposal(bot *telego.Bot, update telego.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID

	// 🔥 ПРОВЕРКА ВРЕМЕННОГО СОСТОЯНИЯ (для ответов и админ-действий)
	state, targetID, _ := p.db.GetUserState(userID)

	if state == "reply_mode" {
		p.handleReplyContent(bot, msg, userID, uint(targetID))
		return
	} else if state == "reason" {
		p.handleSendReason(bot, msg, userID, targetID)
		return
	} else if state == "ban_reason" {
		p.handleSendBanReason(bot, msg, userID, targetID)
		return
	}

	if p.db.IsBanned(userID) {
		bot.SendMessage(tu.Message(tu.ID(userID), "Вы заблокированы."))
		return
	}

	if msg.Chat.Type != "private" {
		return
	}

	if msg.Text != "" && msg.Text[0] == '/' {
		if strings.HasPrefix(msg.Text, "/reply") {
			p.handleReplyCommand(bot, msg)
		}
		return
	}

	if msg.Text == "" && msg.Photo == nil && msg.Document == nil &&
		msg.Video == nil && msg.VideoNote == nil && msg.Audio == nil &&
		msg.Voice == nil && msg.Sticker == nil {
		return
	}

	if p.db.MessageExists(chatID, msg.MessageID) {
		return
	}

	mediaType, mediaFileID := p.media.GetMediaInfo(msg)
	messageText := p.media.ExtractMessageText(msg)

	message := &database.Message{
		ChatID:            chatID,
		TelegramMessageID: msg.MessageID,
		SenderID:          uint(userID),
		MessageText:       messageText,
		MediaType:         mediaType,
		MediaFileID:       mediaFileID,
		CreatedAt:         time.Now(),
		Status:            "pending",
		ChannelID:         p.channelID,
	}

	if err := p.db.SaveMessage(message); err != nil {
		bot.SendMessage(tu.Message(tu.ID(chatID), "❌ Произошла ошибка при отправке предложения. Попробуйте позже."))
		return
	}

	bot.SendMessage(tu.Message(tu.ID(chatID), "✅ Ваше предложение принято! Оно будет рассмотрено модераторами анонимно."))
	p.notifyAdminsAboutNewProposal(bot, message)
}

func (p *ProposalsHandler) HandleStartCommand(bot *telego.Bot, update telego.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	userID := msg.From.ID
	chatID := msg.Chat.ID
	text := strings.TrimSpace(msg.Text)

	if strings.HasPrefix(text, "/start") {
		payload := strings.TrimSpace(strings.TrimPrefix(text, "/start"))
		if strings.HasPrefix(payload, "reply_") {
			p.handleDeepLinkReply(bot, msg, strings.TrimPrefix(payload, "reply_"))
			return
		}
	}

	if p.db.IsAdmin(userID) || userID == p.ownerID {
		var messageText string
		if userID == p.ownerID {
			messageText = "👑 Панель владельца\n\nДоступные команды:\n/addadmin <ID>\n/admins\n/banned\n/proposals\n/pardon <BAN-ID>"
		} else {
			messageText = "🛠️ Панель модератора\n\nДоступные команды:\n/proposals"
		}
		bot.SendMessage(tu.Message(tu.ID(chatID), messageText))
	} else {
		bot.SendMessage(tu.Message(tu.ID(chatID), welcomeText))
	}
}

func (p *ProposalsHandler) handleReplyCommand(bot *telego.Bot, msg *telego.Message) {
	args := strings.TrimSpace(strings.TrimPrefix(msg.Text, "/reply"))
	if args == "" {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "📝 Использование: /reply <ID_поста>"))
		return
	}
	parentID, err := strconv.Atoi(args)
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Неверный формат ID поста."))
		return
	}
	_, err = p.db.GetMessageByDBID(uint(parentID))
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Пост не найден."))
		return
	}

	_ = p.db.SetUserState(msg.From.ID, "reply_mode", int64(parentID))
	bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "✍️ Отправьте ваш ответ на пост."))
}

func (p *ProposalsHandler) handleDeepLinkReply(bot *telego.Bot, msg *telego.Message, parentIDStr string) {
	parentID, err := strconv.Atoi(parentIDStr)
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Неверная ссылка для ответа."))
		return
	}
	_, err = p.db.GetMessageByDBID(uint(parentID))
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Пост не найден."))
		return
	}

	_ = p.db.SetUserState(msg.From.ID, "reply_mode", int64(parentID))
	bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "✍️ Отправьте ваш ответ на пост."))
}

func (p *ProposalsHandler) handleReplyContent(bot *telego.Bot, msg *telego.Message, userID int64, parentID uint) {
	if p.db.MessageExists(msg.Chat.ID, msg.MessageID) {
		_ = p.db.ClearUserState(userID)
		return
	}

	mediaType, mediaFileID := p.media.GetMediaInfo(msg)
	messageText := p.media.ExtractMessageText(msg)

	message := &database.Message{
		ChatID:            msg.Chat.ID,
		TelegramMessageID: msg.MessageID,
		SenderID:          uint(userID),
		MessageText:       messageText,
		MediaType:         mediaType,
		MediaFileID:       mediaFileID,
		CreatedAt:         time.Now(),
		Status:            "pending",
		ChannelID:         p.channelID,
		ParentMessageID:   &parentID,
	}

	if err := p.db.SaveMessage(message); err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Ошибка при отправке ответа."))
		_ = p.db.ClearUserState(userID)
		return
	}

	_ = p.db.ClearUserState(userID)
	bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "✅ Ваш ответ принят!"))
	p.notifyAdminsAboutNewProposal(bot, message)
}

func (p *ProposalsHandler) handleSendReason(bot *telego.Bot, msg *telego.Message, adminID int64, targetUserID int64) {
	bot.SendMessage(tu.Message(tu.ID(targetUserID), fmt.Sprintf("Ваше сообщение отклонено по причине: %s", msg.Text)))
	_ = p.db.ClearUserState(adminID)
	bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "✅ Причина отправлена."))
}

func (p *ProposalsHandler) handleSendBanReason(bot *telego.Bot, msg *telego.Message, adminID int64, targetUserID int64) {
	banID, err := p.db.CreateBanRecord(targetUserID, msg.Text)
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Ошибка при блокировке."))
		_ = p.db.ClearUserState(adminID)
		return
	}
	_ = p.db.BanUser(targetUserID)
	_ = p.db.ClearUserState(adminID)

	bot.SendMessage(tu.Message(tu.ID(targetUserID), fmt.Sprintf("Вы заблокированы. Код обращения: %s", banID)))
	bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "✅ Пользователь заблокирован."))
}

func (p *ProposalsHandler) notifyAdminsAboutNewProposal(bot *telego.Bot, message *database.Message) {
	admins, err := p.db.GetAdmins()
	if err != nil {
		return
	}

	notification := fmt.Sprintf(
		"📨 Новое предложение!\n\nID: %d\n💬 %s\n📁 Тип: %s\n\n/proposals",
		message.ID, message.MessageText, message.MediaType,
	)
	for _, admin := range admins {
		_, _ = bot.SendMessage(tu.Message(tu.ID(admin.UserID), notification))
	}
}
