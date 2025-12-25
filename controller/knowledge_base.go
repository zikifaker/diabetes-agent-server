package controller

import (
	"diabetes-agent-backend/dao"
	"diabetes-agent-backend/model"
	"diabetes-agent-backend/request"
	"diabetes-agent-backend/response"
	knowledgebase "diabetes-agent-backend/service/knowledge-base"
	"diabetes-agent-backend/service/knowledge-base/etl"
	"diabetes-agent-backend/service/mq"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

func GetPolicyToken(c *gin.Context) {
	email := c.GetString("email")
	policyToken, err := knowledgebase.GeneratePolicyToken(email)
	if err != nil {
		slog.Error(ErrGeneratePolicyToken.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGeneratePolicyToken.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Data: policyToken,
	})
}

func GetKnowledgeMetadata(c *gin.Context) {
	email := c.GetString("email")
	metadata, err := dao.GetKnowledgeMetadataByEmail(email)
	if err != nil {
		slog.Error(ErrGetKnowledgeMetadata.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGetKnowledgeMetadata.Error(),
		})
		return
	}

	var resp response.GetKnowledgeMetadataResponse
	for _, item := range metadata {
		resp.Metadata = append(resp.Metadata, response.MetadataResponse{
			FileName: item.FileName,
			FileType: string(item.FileType),
			FileSize: item.FileSize,
		})
	}

	c.JSON(http.StatusOK, response.Response{
		Data: resp,
	})
}

// UploadKnowledgeMetadata 在前端将文件成功传输到OSS后调用
// 存储知识文件元数据，向MQ发送向量化任务
func UploadKnowledgeMetadata(c *gin.Context) {
	var req request.UploadKnowledgeMetadataRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.Error(ErrParseRequest.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusBadRequest, response.Response{
			Msg: ErrParseRequest.Error(),
		})
		return
	}

	email := c.GetString("email")
	err := knowledgebase.UploadKnowledgeMetadata(req, email)
	if err != nil {
		slog.Error(ErrUploadKnowledgeMetadata.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrUploadKnowledgeMetadata.Error(),
		})
		return
	}

	mq.SendMessage(c.Request.Context(), &mq.Message{
		Topic: mq.TopicKnowledgeBase,
		Tag:   mq.TagETL,
		Payload: etl.ETLMessage{
			FileType:   model.FileType(req.FileType),
			ObjectName: req.ObjectName,
		},
	})

	c.JSON(http.StatusOK, response.Response{})
}

// DeleteKnowledgeMetadata 删除知识文件元数据和OSS上的文件，向MQ发送删除任务
func DeleteKnowledgeMetadata(c *gin.Context) {
	email := c.GetString("email")
	fileName := c.Query("file-name")
	err := dao.DeleteKnowledgeMetadataByEmailAndFileName(email, fileName)
	if err != nil {
		slog.Error(ErrDeleteKnowledgeMetadata.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrDeleteKnowledgeMetadata.Error(),
		})
		return
	}

	// 获取文件扩展名（包含.）
	extension := filepath.Ext(fileName)
	fileType := strings.TrimPrefix(extension, ".")

	mq.SendMessage(c.Request.Context(), &mq.Message{
		Topic: mq.TopicKnowledgeBase,
		Tag:   mq.TagDelete,
		Payload: etl.DeleteMessage{
			FileType:   model.FileType(fileType),
			ObjectName: email + "/" + fileName,
		},
	})

	c.JSON(http.StatusOK, response.Response{})
}

func GetPresignedURL(c *gin.Context) {
	email := c.GetString("email")
	fileName := c.Query("file-name")
	objectName := email + "/" + fileName

	url, err := knowledgebase.GeneratePresignedURL(objectName)
	if err != nil {
		slog.Error(ErrGetPreSignedURL.Error(), "err", err)
		c.AbortWithStatusJSON(http.StatusInternalServerError, response.Response{
			Msg: ErrGetPreSignedURL.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Data: response.GetPreSignedURLResponse{
			URL: url,
		},
	})
}
