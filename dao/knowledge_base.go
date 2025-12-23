package dao

import (
	"diabetes-agent-backend/model"
	"diabetes-agent-backend/request"
	"errors"

	"gorm.io/gorm"
)

func SaveKnowledgeMetadata(req request.UploadKnowledgeMetadataRequest, email string) error {
	fileMetadata := model.KnowledgeMetadata{
		UserEmail:  email,
		FileName:   req.FileName,
		FileType:   model.FileType(req.FileType),
		FileSize:   req.FileSize,
		ObjectName: req.ObjectName,
		Status:     model.StatusUploaded,
	}
	return DB.Create(&fileMetadata).Error
}

func GetKnowledgeMetadataByEmail(email string) ([]model.KnowledgeMetadata, error) {
	var fileMetadata []model.KnowledgeMetadata
	if err := DB.Where("user_email = ?", email).
		Order("created_at DESC").
		Find(&fileMetadata).Error; err != nil {
		return nil, err
	}
	return fileMetadata, nil
}

func GetKnowledgeMetadataByEmailAndFileName(email, fileName string) (*model.KnowledgeMetadata, error) {
	var fileMetadata model.KnowledgeMetadata
	if err := DB.Where("user_email = ? AND file_name = ?", email, fileName).
		First(&fileMetadata).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &fileMetadata, nil
}

func DeleteKnowledgeMetadataByEmailAndFileName(email, fileName string) error {
	return DB.Where("user_email = ? AND file_name = ?", email, fileName).
		Delete(&model.KnowledgeMetadata{}).Error
}

func UpdateKnowledgeMetadataStatus(email, fileName string, status model.Status) error {
	return DB.Model(&model.KnowledgeMetadata{}).
		Where("user_email = ? AND file_name = ?", email, fileName).
		Update("status", status).Error
}
