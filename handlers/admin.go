package handlers

import (
	"fmt"
	"log"

	"telegram-bot/database"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

type AdminHandler struct {
	db      *database.Database
	ownerID int64
}

func NewAdminHandler(db *database.Database, ownerID int64) *AdminHandler {
	return &AdminHandler{
		db:      db,
		ownerID: ownerID,
	}
}

func (a *AdminHandler) IsOwner(userID int64) bool {
	return userID == a.ownerID
}

func (a *AdminHandler) HandleAddAdminCommand(bot *telego.Bot, update telego.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	if !a.IsOwner(msg.From.ID) {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"❌ Только владелец бота может добавлять администраторов.",
		))
		return
	}

	// Парсим команду: /addadmin <user_id>
	args := msg.Text
	if len(args) < 10 {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"📝 Использование: /addadmin <ID_пользователя>\n\n"+
				"Пример: /addadmin 123456789",
		))
		return
	}

	var targetUserID int64
	_, err := fmt.Sscanf(args, "/addadmin %d", &targetUserID)
	if err != nil || targetUserID == 0 {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"❌ Неверный формат ID. Используйте: /addadmin <ID_пользователя>\n\n"+
				"Пример: /addadmin 123456789",
		))
		return
	}

	if targetUserID == a.ownerID {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"❌ Вы уже являетесь владельцем бота.",
		))
		return
	}

	if a.db.IsAdmin(targetUserID) {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			fmt.Sprintf("❌ Пользователь с ID %d уже является администратором.", targetUserID),
		))
		return
	}

	var userName string
	user, err := bot.GetChat(&telego.GetChatParams{
		ChatID: tu.ID(targetUserID),
	})
	if err != nil {
		userName = fmt.Sprintf("user_%d", targetUserID)
		log.Printf("Не удалось получить информацию о пользователе %d: %v", targetUserID, err)
	} else {
		if user.Type == "private" {
			userName = user.FirstName
			if user.Username != "" {
				userName = user.Username
			}
		} else {
			userName = user.Title
		}
	}

	err = a.db.AddAdmin(targetUserID, userName)
	if err != nil {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"❌ Ошибка при добавлении администратора: "+err.Error(),
		))
		return
	}

	successMsg := fmt.Sprintf("✅ Пользователь %s (ID: %d) добавлен как администратор!", userName, targetUserID)
	bot.SendMessage(tu.Message(
		tu.ID(msg.Chat.ID),
		successMsg,
	))

	log.Printf("Добавлен новый администратор: %s (ID: %d)", userName, targetUserID)

	notificationMsg := "🎉 Вы были добавлены как модератор бота-предложки!\n\n" +
		"Используйте команду /start для доступа к панели модерации."

	_, err = bot.SendMessage(tu.Message(
		tu.ID(targetUserID),
		notificationMsg,
	))
	if err != nil {
		log.Printf("Не удалось отправить уведомление пользователю %d: %v", targetUserID, err)
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"⚠️ Администратор добавлен, но не удалось отправить ему уведомление.",
		))
	} else {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"✅ Уведомление отправлено новому администратору.",
		))
	}
}

func (a *AdminHandler) HandleListAdminsCommand(bot *telego.Bot, update telego.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	if !a.IsOwner(msg.From.ID) {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"❌ Только владелец бота может просматривать список администраторов.",
		))
		return
	}

	admins, err := a.db.GetAdmins()
	if err != nil {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"❌ Ошибка при получении списка администраторов: "+err.Error(),
		))
		return
	}

	if len(admins) == 0 {
		bot.SendMessage(tu.Message(
			tu.ID(msg.Chat.ID),
			"📋 Список модераторов пуст.",
		))
		return
	}

	adminList := "📋 Список модераторов:\n\n"
	adminList += fmt.Sprintf("👑 Владелец: ID %d\n", a.ownerID)

	for i, admin := range admins {
		adminList += fmt.Sprintf("%d. @%s (ID: %d)\n", i+1, admin.UserName, admin.UserID)
	}

	bot.SendMessage(tu.Message(
		tu.ID(msg.Chat.ID),
		adminList,
	))
}

// ⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣠⣤⣤⣤⣤⣤⣤⣤⣤⣄⡀⠀⠀⠀⠀⠀⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⠀⢀⣴⣿⡿⠛⠉⠙⠛⠛⠛⠛⠻⢿⣿⣷⣤⡀⠀⠀⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⠀⣼⣿⠋⠀⠀⠀⠀⠀⠀⠀⢀⣀⣀⠈⢻⣿⣿⡄⠀⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⣸⣿⡏⠀⠀⠀⣠⣶⣾⣿⣿⣿⠿⠿⠿⢿⣿⣿⣿⣄⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⣿⣿⠁⠀⠀⢰⣿⣿⣯⠁⠀⠀⠀⠀⠀⠀⠀⠈⠙⢿⣷⡄⠀
// ⠀⠀⣀⣤⣴⣶⣶⣿⡟⠀⠀⠀⢸⣿⣿⣿⣆⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⣷⠀
// ⠀⢰⣿⡟⠋⠉⣹⣿⡇⠀⠀⠀⠘⣿⣿⣿⣿⣷⣦⣤⣤⣤⣶⣶⣶⣶⣿⣿⣿⠀
// ⠀⢸⣿⡇⠀⠀⣿⣿⡇⠀⠀⠀⠀⠹⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⣿⡿⠃⠀
// ⠀⣸⣿⡇⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠉⠻⠿⣿⣿⣿⣿⡿⠿⠿⠛⢻⣿⡇⠀⠀
// ⠀⣿⣿⠁⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿⣧⠀⠀
// ⠀⣿⣿⠀⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿⣿⠀⠀
// ⠀⣿⣿⠀⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿⣿⠀⠀
// ⠀⢿⣿⡆⠀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⢸⣿⡇⠀⠀
// ⠀⠸⣿⣧⡀⠀⣿⣿⡇⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⣿⣿⠃⠀⠀
// ⠀⠀⠛⢿⣿⣿⣿⣿⣇⠀⠀⠀⠀⠀⣰⣿⣿⣷⣶⣶⣶⣶⠶⠀⢠⣿⣿⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⣿⣿⠀⠀⠀⠀⠀⣿⣿⡇⠀⣽⣿⡏⠁⠀⠀⢸⣿⡇⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⣿⣿⠀⠀⠀⠀⠀⣿⣿⡇⠀⢹⣿⡆⠀⠀⠀⣸⣿⠇⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⢿⣿⣦⣄⣀⣠⣴⣿⣿⠁⠀⠈⠻⣿⣿⣿⣿⡿⠏⠀⠀⠀⠀
// ⠀⠀⠀⠀⠀⠀⠀⠈⠛⠻⠿⠿⠿⠿⠋⠁⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀⠀
