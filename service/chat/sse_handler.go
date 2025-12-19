package chat

import (
	"context"
	"diabetes-agent-backend/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tmc/langchaingo/callbacks"
)

const (
	// Agent 输出缓冲区大小阈值
	prefixBufferMaxKeep = 10

	// 最终答案的前缀
	finalAnswerPrefix = "AI:"
)

// GinSSEHandler 基于 Gin 的回调处理器，使用 SSE 发送 Agent 的输出内容
type GinSSEHandler struct {
	callbacks.SimpleHandler

	Ctx     *gin.Context
	Session string

	// 存储 Agent 的思考步骤
	ImmediateSteps *strings.Builder

	// 存储 Agent 的最终答案
	FinalAnswer *strings.Builder

	// 缓冲区，用于跨 chunk 识别最终答案的前缀
	prefixBuffer *strings.Builder

	hasFinalAnswer bool
}

var _ callbacks.Handler = &GinSSEHandler{}

func NewGinSSEHandler(ctx *gin.Context, session string) *GinSSEHandler {
	return &GinSSEHandler{
		Ctx:            ctx,
		Session:        session,
		ImmediateSteps: &strings.Builder{},
		FinalAnswer:    &strings.Builder{},
		prefixBuffer:   &strings.Builder{},
	}
}

func (h *GinSSEHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) {
	text := string(chunk)

	if h.hasFinalAnswer {
		h.FinalAnswer.WriteString(text)
		utils.SendSSEMessage(h.Ctx, utils.EventFinalAnswer, text)
		return
	}

	h.prefixBuffer.WriteString(text)
	bufferStr := h.prefixBuffer.String()

	if idx := strings.Index(bufferStr, finalAnswerPrefix); idx != -1 {
		// 前缀前为思考内容
		before := bufferStr[:idx]
		if len(before) > 0 {
			h.ImmediateSteps.WriteString(before)
			utils.SendSSEMessage(h.Ctx, utils.EventImmediateSteps, before)
		}

		// 前缀后为最终答案
		after := bufferStr[idx+len(finalAnswerPrefix):]
		if len(after) > 0 {
			h.FinalAnswer.WriteString(after)
			utils.SendSSEMessage(h.Ctx, utils.EventFinalAnswer, after)
		}

		h.prefixBuffer.Reset()
		h.hasFinalAnswer = true
	} else {
		// 保留最后 prefixBufferMaxKeep 个 rune，防止缓冲区过大
		if h.prefixBuffer.Len() > 0 {
			runes := []rune(bufferStr)
			if len(runes) > prefixBufferMaxKeep {
				flushRunes := runes[:len(runes)-prefixBufferMaxKeep]
				flushText := string(flushRunes)
				h.ImmediateSteps.WriteString(flushText)
				utils.SendSSEMessage(h.Ctx, utils.EventImmediateSteps, flushText)

				remaining := string(runes[len(runes)-prefixBufferMaxKeep:])
				h.prefixBuffer.Reset()
				h.prefixBuffer.WriteString(remaining)
			}
		}
	}
}

func (h *GinSSEHandler) HandleToolEnd(ctx context.Context, result string) {
	utils.SendSSEMessage(h.Ctx, utils.EventToolCallResult, result)
}
