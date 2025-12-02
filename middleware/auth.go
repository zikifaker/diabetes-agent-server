package middleware

import (
	"diabetes-agent-backend/config"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Email string
	jwt.RegisteredClaims
}

func GenerateToken(email string) (string, error) {
	claims := Claims{
		Email: email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	secretKey := []byte(config.Cfg.JWT.SecretKey)
	return token.SignedString(secretKey)
}

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			slog.Info("Authorization header required")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			slog.Info("Invalid authorization format")
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		tokenString := parts[1]
		claims := &Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Cfg.JWT.SecretKey), nil
		})

		if err != nil || !token.Valid {
			slog.Info("Invalid token", "err", err, "user_email", claims.Email)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		c.Set("email", claims.Email)
		c.Next()
	}
}
