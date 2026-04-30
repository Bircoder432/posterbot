package handlers

import (
	"fmt"
	"strings"

	"telegram-bot/database"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type ModerationHandler struct {
	db          *database.Database
	media       *MediaHandler
	channelID   int64
	ownerID     int64
	botUsername string
}

func NewModerationHandler(db *database.Database, media *MediaHandler, channelID, ownerID int64, botUsername string) *ModerationHandler {
	return &ModerationHandler{
		db:          db,
		media:       media,
		channelID:   channelID,
		ownerID:     ownerID,
		botUsername: botUsername,
	}
}

func (m *ModerationHandler) HandleProposalsCommand(bot *telego.Bot, update telego.Update) {
	msg := update.Message
	if msg == nil {
		return
	}
	m.ShowProposals(bot, msg.Chat.ID, msg.From.ID)
}

func (m *ModerationHandler) ShowProposals(bot *telego.Bot, chatID int64, userID int64) {
	if !m.db.IsAdmin(userID) && userID != m.ownerID {
		bot.SendMessage(tu.Message(tu.ID(chatID), "❌ Нет доступа."))
		return
	}

	messages, err := m.db.GetPendingMessages()
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(chatID), "❌ Ошибка: "+err.Error()))
		return
	}
	if len(messages) == 0 {
		bot.SendMessage(tu.Message(tu.ID(chatID), "✅ Нет новых предложений."))
		return
	}

	bot.SendMessage(tu.Message(tu.ID(chatID), fmt.Sprintf("📨 Найдено %d предложений:", len(messages))))
	m.SendMessageForModeration(bot, chatID, messages[0])
}

func (m *ModerationHandler) SendMessageForModeration(bot *telego.Bot, chatID int64, message database.Message) {
	if err := m.media.SendMediaForModeration(bot, chatID, message); err != nil {
		return
	}

	text := fmt.Sprintf("📨 Предложение #%d\n⏰ %s\n\nВыберите действие:", message.ID, message.CreatedAt.Format("02.01.2006 15:04"))
	kb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("✅ ОДОБРИТЬ").WithCallbackData(fmt.Sprintf("approve_%d", message.ID)),
			tu.InlineKeyboardButton("❌ ОТКЛОНИТЬ").WithCallbackData(fmt.Sprintf("reject_%d", message.ID)),
		),
	)
	bot.SendMessage(tu.Message(tu.ID(chatID), text).WithReplyMarkup(kb))
}

func (m *ModerationHandler) HandlePardonCommand(bot *telego.Bot, update telego.Update) {
	msg := update.Message
	if msg == nil {
		return
	}
	senderID := msg.From.ID

	if !m.db.IsAdmin(senderID) && senderID != m.ownerID {
		return
	}

	args := msg.Text
	if len(args) < 12 {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "📝 /pardon <BAN-ID>"))
		return
	}
	var banID string
	_, err := fmt.Sscanf(args, "/pardon %s", &banID)
	if err != nil || banID == "" {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Неверный формат."))
		return
	}
	record, err := m.db.GetBanRecordByBanID(banID)
	if err != nil {
		bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), "❌ Бан не найден или уже снят."))
		return
	}
	_ = m.db.PardonUser(record.UserID)
	_ = m.db.PardonByBanID(banID)
	_, _ = bot.SendMessage(tu.Message(tu.ID(record.UserID), "✅ Ваш доступ восстановлен."))
	bot.SendMessage(tu.Message(tu.ID(msg.Chat.ID), fmt.Sprintf("✅ Бан %s деактивирован.", banID)))
}

func (m *ModerationHandler) HandleCallback(bot *telego.Bot, update telego.Update) {
	callback := update.CallbackQuery
	if callback == nil {
		return
	}
	userID := callback.From.ID
	chatID := callback.Message.Chat.ID

	if !m.db.IsAdmin(userID) && userID != m.ownerID {
		bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("❌ Нет доступа."))
		return
	}

	data := callback.Data
	var messageID int
	var senderID int

	if n, _ := fmt.Sscanf(data, "approve_%d", &messageID); n == 1 {
		m.HandleApprove(bot, chatID, messageID, callback)
	} else if n, _ := fmt.Sscanf(data, "reject_%d", &messageID); n == 1 {
		m.HandleReject(bot, chatID, messageID, callback)
	} else if n, _ := fmt.Sscanf(data, "reason_%d", &senderID); n == 1 {
		m.HandleReason(bot, chatID, senderID, callback)
	} else if n, _ := fmt.Sscanf(data, "ban_reason_%d", &senderID); n == 1 {
		m.HandleBanReason(bot, chatID, senderID, callback)
	} else if strings.HasPrefix(data, "next") {
		m.HandleNext(bot, chatID, callback)
	}
}

func (m *ModerationHandler) HandleNext(bot *telego.Bot, chatID int64, callback *telego.CallbackQuery) {
	m.ShowProposals(bot, chatID, callback.From.ID)
	bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("✅ Далее"))
}

func (m *ModerationHandler) HandleReason(bot *telego.Bot, chatID int64, senderID int, callback *telego.CallbackQuery) {
	adminID := callback.From.ID
	bot.SendMessage(tu.Message(tu.ID(chatID), "Введите причину отказа"))
	_ = m.db.SetUserState(adminID, "reason", int64(senderID))
	bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("✅ Введите причину"))
}

func (m *ModerationHandler) HandleBanReason(bot *telego.Bot, chatID int64, senderID int, callback *telego.CallbackQuery) {
	adminID := callback.From.ID
	bot.SendMessage(tu.Message(tu.ID(chatID), "Введите причину блокировки"))
	_ = m.db.SetUserState(adminID, "ban_reason", int64(senderID))
	bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("✅ Введите причину"))
}

func (m *ModerationHandler) HandleApprove(bot *telego.Bot, chatID int64, messageID int, callback *telego.CallbackQuery) {
	message, err := m.db.GetMessageByID(uint(messageID))
	if err != nil {
		bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("❌ Не найдено"))
		return
	}

	var replyToMsgID int
	if message.ParentMessageID != nil {
		parent, err := m.db.GetMessageByDBID(*message.ParentMessageID)
		if err == nil && parent.ChannelMessageID != nil {
			replyToMsgID = *parent.ChannelMessageID
		}
	}

	sentMsg, err := m.media.PublishMedia(bot, m.channelID, message, m.botUsername, replyToMsgID)
	if err != nil {
		bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("❌ Ошибка публикации"))
		return
	}
	if sentMsg != nil {
		_ = m.db.UpdateMessageChannelID(message.ID, sentMsg.MessageID)
	}

	_ = m.db.UpdateMessageStatus(message.ID, "approved")
	bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("✅ Опубликовано!"))
	_ = bot.DeleteMessage(&telego.DeleteMessageParams{ChatID: tu.ID(chatID), MessageID: callback.Message.MessageID})
	m.ShowProposals(bot, chatID, callback.From.ID)
}

func (m *ModerationHandler) HandleReject(bot *telego.Bot, chatID int64, messageID int, callback *telego.CallbackQuery) {
	msg, err := m.db.GetMessageByID(uint(messageID))
	if err != nil {
		bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("❌ Не найдено"))
		return
	}
	senderID := msg.SenderID
	_ = m.db.UpdateMessageStatus(msg.ID, "rejected")
	_ = m.db.DeleteMessage(msg.ID)

	bot.AnswerCallbackQuery(tu.CallbackQuery(callback.ID).WithText("✅ Отклонено!"))
	_ = bot.DeleteMessage(&telego.DeleteMessageParams{ChatID: tu.ID(chatID), MessageID: callback.Message.MessageID})

	kb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(
			tu.InlineKeyboardButton("Причина").WithCallbackData(fmt.Sprintf("reason_%d", senderID)),
			tu.InlineKeyboardButton("Далее").WithCallbackData("next"),
			tu.InlineKeyboardButton("Бан").WithCallbackData(fmt.Sprintf("ban_reason_%d", senderID)),
		),
	)
	bot.SendMessage(tu.Message(tu.ID(chatID), "Выберите действие").WithReplyMarkup(kb))
}
