package dao

import (
	"diabetes-agent-backend/model"
)

func GetSessionsByEmail(email string) ([]model.Session, error) {
	var sessions []model.Session
	if err := DB.Where("user_email = ?", email).
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func DeleteSession(email, sessionID string) error {
	// 删除会话
	err := DB.Where("user_email = ? AND session_id = ?", email, sessionID).
		Delete(&model.Session{}).Error
	if err != nil {
		return err
	}

	// 删除会话内的对话记录
	err = DB.Where("session_id = ?", sessionID).
		Delete(&[]model.Message{}).Error
	if err != nil {
		return err
	}

	return nil
}

func GetMessagesBySessionID(sessionID string) ([]model.Message, error) {
	var messages []model.Message
	if err := DB.Where("session_id = ?", sessionID).
		Order("created_at ASC").
		Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func GetMessageByID(messageID uint) (*model.Message, error) {
	var message model.Message
	if err := DB.Where("id = ?", messageID).
		First(&message).Error; err != nil {
		return nil, err
	}
	return &message, nil
}

func UpdateSessionTitle(email, sessionID, title string) error {
	err := DB.Model(&model.Session{}).
		Where("user_email = ? AND session_id = ?", email, sessionID).
		Update("title", title).Error
	if err != nil {
		return err
	}
	return nil
}
