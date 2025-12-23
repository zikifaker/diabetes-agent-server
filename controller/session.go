package controller

import (
	"diabetes-agent-backend/dao"
	"diabetes-agent-backend/model"
	"diabetes-agent-backend/request"
	"diabetes-agent-backend/response"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func CreateSession(c *gin.Context) {
	email := c.GetString("email")
	session := model.Session{
		UserEmail: email,
		SessionID: uuid.New().String(),
		Title:     model.DefaultSessionTitle,
	}
	if err := dao.DB.Create(&session).Error; err != nil {
		slog.Error(ErrCreateSession.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrCreateSession.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response.Response{
		Data: response.SessionResponse{
			SessionID: session.SessionID,
			Title:     session.Title,
		},
	})
}

func GetSessions(c *gin.Context) {
	email := c.GetString("email")
	sessions, err := dao.GetSessionsByEmail(email)
	if err != nil {
		slog.Error(ErrGetSessions.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGetSessions.Error(),
		})
		return
	}

	var resp response.GetSessionsResponse
	for _, s := range sessions {
		resp.Sessions = append(resp.Sessions, response.SessionResponse{
			SessionID: s.SessionID,
			Title:     s.Title,
		})
	}

	c.JSON(http.StatusOK, response.Response{
		Data: resp,
	})
}

func DeleteSession(c *gin.Context) {
	email := c.GetString("email")
	sessionID := c.Param("id")
	if err := dao.DeleteSession(email, sessionID); err != nil {
		slog.Error(ErrDeleteSession.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrDeleteSession.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{})
}

func GetSessionMessages(c *gin.Context) {
	sessionID := c.Param("id")
	messages, err := dao.GetMessagesBySessionID(sessionID)
	if err != nil {
		slog.Error(ErrGetSessionMessages.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGetSessionMessages.Error(),
		})
		return
	}

	var resp response.GetSessionMessagesResponse
	for _, m := range messages {
		resp.Messages = append(resp.Messages, response.MessageResponse{
			CreatedAt:       m.CreatedAt,
			Role:            m.Role,
			Content:         m.Content,
			ImmediateSteps:  m.ImmediateSteps,
			ToolCallResults: m.ToolCallResults,
		})
	}

	c.JSON(http.StatusOK, response.Response{
		Data: resp,
	})
}

func UpdateSessionTitle(c *gin.Context) {
	var req request.UpdateSessionTitleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error(ErrParseRequest.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.Response{
			Msg: ErrParseRequest.Error(),
		})
		return
	}

	email := c.GetString("email")
	if err := dao.UpdateSessionTitle(email, req.SessionID, req.Title); err != nil {
		slog.Error(ErrUpdateSessionTitle.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrUpdateSessionTitle.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{})
}
