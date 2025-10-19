package database

import (
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type Message struct {
	ID          uint `gorm:"primaryKey"`
	MessageID   int  `gorm:"not null"`
	MessageText string
	MediaType   string `gorm:"size:50"`
	MediaFileID string
	CreatedAt   time.Time
	Status      string `gorm:"default:'pending'"`
	ChannelID   int64
}

type Admin struct {
	ID       uint  `gorm:"primaryKey"`
	UserID   int64 `gorm:"uniqueIndex;not null"`
	UserName string
}

type Database struct {
	db *gorm.DB
}

func NewDatabase() (*Database, error) {
	db, err := gorm.Open(sqlite.Open("bot.db"), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	err = db.AutoMigrate(&Message{}, &Admin{})
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

func (d *Database) SaveMessage(msg *Message) error {
	return d.db.Create(msg).Error
}

func (d *Database) GetPendingMessages() ([]Message, error) {
	var messages []Message
	err := d.db.Where("status = ?", "pending").Order("created_at asc").Find(&messages).Error
	return messages, err
}

func (d *Database) UpdateMessageStatus(messageID int, status string) error {
	return d.db.Model(&Message{}).Where("message_id = ?", messageID).Update("status", status).Error
}

func (d *Database) DeleteMessage(messageID int) error {
	return d.db.Delete(&Message{}, &Message{MessageID: messageID}).Error
}

func (d *Database) GetMessageByID(messageID int) (Message, error) {
	var message Message
	err := d.db.First(&message, &Message{MessageID: messageID}).Error
	return message, err
}

func (d *Database) IsAdmin(userID int64) bool {
	err := d.db.First(&Admin{}, &Admin{UserID: userID}).Error
	return err == nil
}

func (d *Database) AddAdmin(userID int64, userName string) error {
	admin := Admin{
		UserID:   userID,
		UserName: userName,
	}
	return d.db.Create(&admin).Error
}

func (d *Database) RemoveAdmin(userID int64) error {
	return d.db.Where("user_id = ?", userID).Delete(&Admin{}).Error
}

func (d *Database) GetAdmins() ([]Admin, error) {
	var admins []Admin
	err := d.db.Find(&admins).Error
	return admins, err
}
