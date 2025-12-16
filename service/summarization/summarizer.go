package summarization

import (
	"bytes"
	"context"
	"diabetes-agent-backend/config"
	"diabetes-agent-backend/dao"
	"diabetes-agent-backend/model"
	"diabetes-agent-backend/service/chat"
	"diabetes-agent-backend/utils"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
	"gorm.io/gorm"
)

const (
	modelName       = "deepseek-v3"
	taskChanSize    = 100
	workerNum       = 10
	updateBatchSize = 1

	// 触发生成对话摘要的最小对话长度（字节数）
	minContentLengthForSummary = 2500
)

//go:embed prompts/summarization.txt
var summaryPrompt string

type SummaryTask struct {
	MessageIDs []uint
}

// Summarizer 负责生成对话摘要
type Summarizer struct {
	llm             llms.Model
	taskChan        chan SummaryTask
	workerNum       int
	updateBatchSize int
}

// SummarizerInstance Summarizer单例实例
var SummarizerInstance *Summarizer

func init() {
	var err error
	SummarizerInstance, err = newSummarizer()
	if err != nil {
		panic(fmt.Sprintf("Failed to create summarizer: %v", err))
	}
}

func newSummarizer() (*Summarizer, error) {
	httpClient := utils.DefaultHTTPClient()
	llm, err := openai.New(
		openai.WithModel(modelName),
		openai.WithToken(config.Cfg.Model.APIKey),
		openai.WithBaseURL(chat.BaseURL),
		openai.WithHTTPClient(httpClient),
	)
	if err != nil {
		return nil, err
	}

	return &Summarizer{
		llm:             llm,
		taskChan:        make(chan SummaryTask, taskChanSize),
		workerNum:       workerNum,
		updateBatchSize: updateBatchSize,
	}, nil
}

func (s *Summarizer) Run() {
	ctx := context.Background()
	for i := 1; i <= s.workerNum; i++ {
		go s.executeSummarization(ctx, i)
	}
}

func (s *Summarizer) RegisterSummaryTask(task SummaryTask) {
	s.taskChan <- task
}

func (s *Summarizer) executeSummarization(ctx context.Context, id int) {
	slog.Info("Starting summary worker", "worker_id", id)

	// 暂存等待批量更新的message
	updates := make([]*model.Message, 0, s.updateBatchSize)

	defer func() {
		slog.Info("Summary worker exit", "worker_id", id)
		s.flushBatchUpdates(updates)
	}()

	for task := range s.taskChan {
		select {
		case <-ctx.Done():
			slog.Info("Summary worker shutting down", "worker_id", id)
			return
		default:
			for _, msgID := range task.MessageIDs {
				msg, err := dao.GetMessageByID(msgID)
				if err != nil {
					slog.Error("Failed to get message",
						"msg_id", msgID,
						"err", err,
					)
					continue
				}

				if len(msg.Content) < minContentLengthForSummary || msg.Summary != "" {
					continue
				}

				res, err := s.summarizeMessage(ctx, msg.Role, msg.Content)
				if err != nil {
					slog.Error("Failed to summary message",
						"msg_id", msgID,
						"err", err,
					)
					continue
				}

				msg.Summary = res
				updates = append(updates, msg)

				if len(updates) >= s.updateBatchSize {
					updates, err = s.flushBatchUpdates(updates)
					if err != nil {
						slog.Error("Failed to flush batch updates", "err", err)
					}
				}

				if len(updates) > 0 {
					updates, err = s.flushBatchUpdates(updates)
					if err != nil {
						slog.Error("Failed to flush batch updates", "err", err)
					}
				}
			}
		}
	}
}

func (s *Summarizer) summarizeMessage(ctx context.Context, role, content string) (string, error) {
	tmpl, err := template.New("prompt").Parse(summaryPrompt)
	if err != nil {
		return "", fmt.Errorf("failed to parse prompt template: %v", err)
	}

	var buf bytes.Buffer
	data := struct {
		Role    string
		Content string
	}{
		Role:    role,
		Content: content,
	}

	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template: %v", err)
	}

	resp, err := llms.GenerateFromSinglePrompt(ctx, s.llm, buf.String())
	if err != nil {
		return "", fmt.Errorf("llm call error: %w", err)
	}

	return resp, nil
}

func (s *Summarizer) flushBatchUpdates(updates []*model.Message) ([]*model.Message, error) {
	if len(updates) == 0 {
		return updates, nil
	}

	err := dao.DB.Transaction(func(tx *gorm.DB) error {
		for _, msg := range updates {
			if err := tx.Model(&model.Message{}).
				Where("id = ?", msg.ID).
				Update("summary", msg.Summary).Error; err != nil {
				return fmt.Errorf("failed to update message %d: %v", msg.ID, err)
			}
		}
		return nil
	})
	if err != nil {
		return updates, fmt.Errorf("failed to update messages batch: %v", err)
	}

	return updates[:0], nil
}
