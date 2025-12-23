package chat

import (
	"context"
	"diabetes-agent-backend/dao"
	"diabetes-agent-backend/model"
	"encoding/json"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/schema"
	"gorm.io/gorm"
)

const (
	tableName = "chat_message"
	limit     = 200
)

type MySQLChatMessageHistory struct {
	DB        *gorm.DB
	TableName string
	Session   string
	Limit     int

	// 每轮对话的Agent消息ID
	AgentMessageID uint

	// 每轮对话的用户消息ID
	UserMessageID uint
}

var _ schema.ChatMessageHistory = &MySQLChatMessageHistory{}

func NewMySQLChatMessageHistory(session string) *MySQLChatMessageHistory {
	return &MySQLChatMessageHistory{
		DB:        dao.DB,
		TableName: tableName,
		Session:   session,
		Limit:     limit,
	}
}

// Messages 加载记忆时优先选取消息摘要，若为空选取全量消息
func (h *MySQLChatMessageHistory) Messages(ctx context.Context) ([]llms.ChatMessage, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var messages []struct {
		Content string
		Summary string
		Role    string
	}

	result := h.DB.WithContext(ctx).
		Table(h.TableName).
		Select("content, summary, role").
		Where("session_id = ?", h.Session).
		Order("created_at ASC").
		Limit(h.Limit).
		Find(&messages)

	if result.Error != nil {
		return nil, result.Error
	}

	var msgs []llms.ChatMessage
	for _, msg := range messages {
		var content string
		if msg.Summary != "" {
			content = msg.Summary
		} else {
			content = msg.Content
		}

		switch msg.Role {
		case string(llms.ChatMessageTypeAI):
			msgs = append(msgs, llms.AIChatMessage{Content: content})
		case string(llms.ChatMessageTypeHuman):
			msgs = append(msgs, llms.HumanChatMessage{Content: content})
		case string(llms.ChatMessageTypeSystem):
			msgs = append(msgs, llms.SystemChatMessage{Content: content})
		}
	}

	return msgs, nil
}

func (h *MySQLChatMessageHistory) AddMessage(ctx context.Context, message llms.ChatMessage) error {
	return h.addMessage(ctx, message.GetContent(), message.GetType())
}

func (h *MySQLChatMessageHistory) AddAIMessage(ctx context.Context, text string) error {
	return h.addMessage(ctx, text, llms.ChatMessageTypeAI)
}

func (h *MySQLChatMessageHistory) AddUserMessage(ctx context.Context, text string) error {
	return h.addMessage(ctx, text, llms.ChatMessageTypeHuman)
}

func (h *MySQLChatMessageHistory) addMessage(ctx context.Context, text string, role llms.ChatMessageType) error {
	if ctx == nil {
		ctx = context.Background()
	}

	msg := model.Message{
		SessionID: h.Session,
		Role:      string(role),
		Content:   text,
	}

	result := h.DB.WithContext(ctx).
		Table(h.TableName).
		Create(&msg)

	if result.Error != nil {
		return result.Error
	}

	switch role {
	case llms.ChatMessageTypeAI:
		h.AgentMessageID = msg.ID
	case llms.ChatMessageTypeHuman:
		h.UserMessageID = msg.ID
	}

	return nil
}

func (h *MySQLChatMessageHistory) Clear(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	result := h.DB.WithContext(ctx).
		Table(h.TableName).
		Where("session_id = ?", h.Session).
		Delete(nil)

	return result.Error
}

func (h *MySQLChatMessageHistory) SetMessages(ctx context.Context, messages []llms.ChatMessage) error {
	if ctx == nil {
		ctx = context.Background()
	}

	return h.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.WithContext(ctx).
			Table(h.TableName).
			Where("session_id = ?", h.Session).
			Delete(nil).Error; err != nil {
			return err
		}

		var values []map[string]any
		for _, msg := range messages {
			values = append(values, map[string]any{
				"session_id": h.Session,
				"content":    msg.GetContent(),
				"role":       msg.GetType(),
			})
		}

		if len(values) > 0 {
			if err := tx.WithContext(ctx).
				Table(h.TableName).
				CreateInBatches(values, 100).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (h *MySQLChatMessageHistory) SetImmediateSteps(ctx context.Context, steps string) error {
	if ctx == nil {
		ctx = context.Background()
	}

	result := h.DB.WithContext(ctx).
		Table(h.TableName).
		Where("id = ?", h.AgentMessageID).
		Update("immediate_steps", steps)

	return result.Error
}

func (h *MySQLChatMessageHistory) SetToolCallResults(ctx context.Context, toolCallResults []model.ToolCallResult) error {
	if ctx == nil {
		ctx = context.Background()
	}

	toolCallResultsJSON, err := json.Marshal(toolCallResults)
	if err != nil {
		return err
	}

	result := h.DB.WithContext(ctx).
		Table(h.TableName).
		Where("id = ?", h.AgentMessageID).
		Update("tool_call_results", toolCallResultsJSON)

	return result.Error
}
