package router

import (
	"diabetes-agent-backend/controller"
	"diabetes-agent-backend/middleware"

	"github.com/gin-gonic/gin"
)

func Register() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.CORSMiddleware())

	api := r.Group("/api")
	{
		public := api.Group("/user")
		{
			public.POST("/register", controller.UserRegister)
			public.POST("/login", controller.UserLogin)
		}

		protected := api.Group("")
		protected.Use(middleware.AuthMiddleware())
		{
			protected.POST("/session", controller.CreateSession)
			protected.GET("/sessions", controller.GetSessions)
			protected.DELETE("/session/:id", controller.DeleteSession)
			protected.GET("/session/:id/messages", controller.GetSessionMessages)
			protected.PUT("/session/:id/title", controller.UpdateSessionTitle)

			protected.POST("/chat", controller.AgentChat)

			protected.POST("/voice-recognition", controller.ChatVoiceRecognition)

			protected.GET("/oss/policy-token", controller.GetPolicyToken)
			protected.GET("/kb/metadata", controller.GetKnowledgeMetadata)
			protected.POST("/kb/metadata", controller.UploadKnowledgeMetadata)
			protected.DELETE("/kb/metadata", controller.DeleteKnowledgeMetadata)
			protected.GET("/kb/download-link", controller.GetPresignedURL)
			protected.GET("/kb/metadata/search", controller.SearchKnowledgeMetadata)
		}
	}

	return r
}
