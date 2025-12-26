package model

import "time"

type FileType string

const (
	FileTypePDF      FileType = "pdf"
	FileTypeMarkdown FileType = "md"
	FileTypeText     FileType = "txt"
)

type Status string

const (
	// 文件上传完成
	StatusUploaded Status = "UPLOADED"

	// 文件向量化处理完成
	StatusProcessed Status = "PROCESSED"

	// 文件向量化处理失败
	StatusProcessedFailed Status = "PROCESSED_FAILED"
)

// KnowledgeMetadata 存储知识文件元数据
// 建立联合索引 (user_email, created_at)，在 file_name 上建立全文索引
type KnowledgeMetadata struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `gorm:"not null;index:idx_email_created" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null" json:"updated_at"`
	UserEmail string    `gorm:"not null;index:idx_email_created" json:"user_email"`
	FileName  string    `gorm:"not null;index:idx_fulltext_file_name,class:FULLTEXT,option:WITH PARSER ngram" json:"file_name"`
	FileType  FileType  `gorm:"not null" json:"file_type"`
	FileSize  int64     `gorm:"not null" json:"file_size"`

	// 文件在OSS上的完整路径，不包含bucket名称
	ObjectName string `gorm:"not null" json:"object_name"`

	// 文件处理状态
	Status Status `gorm:"not null;default:UPLOADED" json:"status"`
}

func (KnowledgeMetadata) TableName() string {
	return "knowledge_metadata"
}
