package controller

import (
	"context"
	"diabetes-agent-backend/request"
	"diabetes-agent-backend/service/chat"
	"diabetes-agent-backend/service/summarization"
	"diabetes-agent-backend/utils"
	"log/slog"

	"github.com/gin-gonic/gin"
)

func AgentChat(c *gin.Context) {
	utils.SetSSEHeaders(c)

	var req request.ChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error(ErrParseRequest.Error(), "err", err)
		utils.SendSSEMessage(c, utils.EventError, ErrParseRequest)
		utils.SendSSEMessage(c, utils.EventDone, "")
		return
	}

	agent, err := chat.NewAgent(c, req)
	if err != nil {
		slog.Error(ErrCreateAgent.Error(), "err", err)
		utils.SendSSEMessage(c, utils.EventError, ErrCreateAgent)
		utils.SendSSEMessage(c, utils.EventDone, "")
		return
	}
	defer agent.Close()

	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()

	// 监听客户端的取消信号
	go func() {
		<-c.Done()
		cancel()
	}()

	if err := agent.Call(ctx, req.Query, c); err != nil {
		slog.Error(ErrCallAgent.Error(), "err", err)
		utils.SendSSEMessage(c, utils.EventError, ErrCallAgent)
		utils.SendSSEMessage(c, utils.EventDone, "")
		return
	}

	utils.SendSSEMessage(c, utils.EventDone, "")

	summarization.SummarizerInstance.RegisterSummaryTask(summarization.SummaryTask{
		MessageIDs: []uint{
			agent.ChatHistory.UserMessageID,
			agent.ChatHistory.AgentMessageID,
		},
	})
}
