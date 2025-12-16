package controller

import (
	"diabetes-agent-backend/middleware"
	"diabetes-agent-backend/request"
	"diabetes-agent-backend/response"
	"diabetes-agent-backend/service/auth"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UserRegister(c *gin.Context) {
	var req request.UserRegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error(ErrParseRequest.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.Response{
			Msg: ErrParseRequest.Error(),
		})
		return
	}

	token, err := middleware.GenerateToken(req.Email)
	if err != nil {
		slog.Error(ErrGenerateToken.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGenerateToken.Error(),
		})
		return
	}

	user, err := auth.UserRegister(req)
	if err != nil {
		slog.Error(ErrUserRegister.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrUserRegister.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response.Response{
		Data: response.UserAuthResponse{
			Email:  user.Email,
			Avatar: user.Avatar,
			Token:  token,
		},
	})
}

func UserLogin(c *gin.Context) {
	var req request.UserLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error(ErrParseRequest.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.Response{
			Msg: ErrParseRequest.Error(),
		})
		return
	}

	user, err := auth.UserLogin(req)
	if err != nil {
		slog.Error(ErrUserLogin.Error(),
			"email", req.Email,
			"err", err,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrUserLogin.Error(),
		})
		return
	}

	token, err := middleware.GenerateToken(user.Email)
	if err != nil {
		slog.Error(ErrGenerateToken.Error(),
			"email", user.Email,
			"err", err,
		)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGenerateToken.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Data: response.UserAuthResponse{
			Email:  user.Email,
			Avatar: user.Avatar,
			Token:  token,
		},
	})
}
