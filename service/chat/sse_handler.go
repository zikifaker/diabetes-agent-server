package chat

import (
	"context"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tmc/langchaingo/callbacks"
)

const (
	// Agent 输出缓冲区大小阈值
	prefixBufferMaxKeep = 10

	// 最终答案的前缀
	finalAnswerPrefix = "AI:"

	eventImmediateSteps = "immediate_steps"
	eventFinalAnswer    = "final_answer"
	eventToolCallResult = "tool_call_result"
)

// GinSSEHandler 基于 Gin 的回调处理器，使用 SSE 发送 Agent 的输出内容
type GinSSEHandler struct {
	callbacks.SimpleHandler

	Ctx     *gin.Context
	Session string

	// 缓冲区，用于跨 chunk 识别最终答案的前缀
	prefixBuffer *strings.Builder

	// Agent 输出中是否包含最终答案
	hasFinalAnswer bool

	// 存储 Agent 的思考步骤
	immediateStepsBuilder *strings.Builder
}

var _ callbacks.Handler = &GinSSEHandler{}

func NewGinSSEHandler(ctx *gin.Context, session string) *GinSSEHandler {
	return &GinSSEHandler{
		Ctx:                   ctx,
		Session:               session,
		prefixBuffer:          &strings.Builder{},
		hasFinalAnswer:        false,
		immediateStepsBuilder: &strings.Builder{},
	}
}

func (h *GinSSEHandler) HandleStreamingFunc(ctx context.Context, chunk []byte) {
	text := string(chunk)

	if h.hasFinalAnswer {
		h.Ctx.SSEvent(eventFinalAnswer, text)
		h.Ctx.Writer.Flush()
		return
	}

	h.prefixBuffer.WriteString(text)
	bufferStr := h.prefixBuffer.String()

	if idx := strings.Index(bufferStr, finalAnswerPrefix); idx != -1 {
		// 前缀前为思考内容
		before := bufferStr[:idx]
		if len(before) > 0 {
			h.immediateStepsBuilder.WriteString(before)
			h.Ctx.SSEvent(eventImmediateSteps, before)
		}

		// 前缀后为最终答案
		after := bufferStr[idx+len(finalAnswerPrefix):]
		if len(after) > 0 {
			h.Ctx.SSEvent(eventFinalAnswer, after)
		}

		h.hasFinalAnswer = true
		h.prefixBuffer.Reset()
	} else {
		// 保留最后 prefixBufferMaxKeep 个 rune，防止缓冲区过大
		if h.prefixBuffer.Len() > 0 {
			runes := []rune(bufferStr)
			if len(runes) > prefixBufferMaxKeep {
				flushRunes := runes[:len(runes)-prefixBufferMaxKeep]
				flushText := string(flushRunes)
				h.immediateStepsBuilder.WriteString(flushText)
				h.Ctx.SSEvent(eventImmediateSteps, flushText)

				remaining := string(runes[len(runes)-prefixBufferMaxKeep:])
				h.prefixBuffer.Reset()
				h.prefixBuffer.WriteString(remaining)
			}
		}
	}

	h.Ctx.Writer.Flush()
}

func (h *GinSSEHandler) HandleToolEnd(ctx context.Context, result string) {
	h.Ctx.SSEvent(eventToolCallResult, result)
	h.Ctx.Writer.Flush()
}

func (h *GinSSEHandler) GetImmediateSteps() string {
	return h.immediateStepsBuilder.String()
}
