package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	AgentURL      string
	Port          string
	SessionSecret string
}

func Load() *Config {
	_ = godotenv.Load()

	agentURL := os.Getenv("AGENT_URL")
	if agentURL == "" {
		log.Fatal("AGENT_URL environment variable is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		b := make([]byte, 32)
		_, _ = rand.Read(b)
		sessionSecret = hex.EncodeToString(b)
	}

	return &Config{
		AgentURL:      agentURL,
		Port:          port,
		SessionSecret: sessionSecret,
	}
}
