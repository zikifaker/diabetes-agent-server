package main

import (
	"crypto/rand"
	"encoding/base64"
	"log/slog"
)

func generateJWTSecret() (string, error) {
	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(key), nil
}

func main() {
	secret, err := generateJWTSecret()
	if err != nil {
		slog.Error("Error generating secret", "err", err)
		return
	}

	slog.Info("Generated JWT Secret:", "secret", secret)
}
