package config

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DatabaseURL    string
	SlackToken     string
	SlackSigningSecret string
	LinearToken    string
	AnthropicKey   string
}

func LoadConfig() *Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or couldn't be loaded: %v", err)
	}
	
	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		SlackToken:         getEnv("SLACK_BOT_TOKEN", ""),
		SlackSigningSecret: getEnv("SLACK_SIGNING_SECRET", ""),
		LinearToken:        getEnv("LINEAR_API_KEY", ""),
		AnthropicKey:       getEnv("ANTHROPIC_API_KEY", ""),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}
	if c.SlackToken == "" {
		return fmt.Errorf("SLACK_BOT_TOKEN is required")
	}
	if c.SlackSigningSecret == "" {
		return fmt.Errorf("SLACK_SIGNING_SECRET is required")
	}
	if c.AnthropicKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY is required")
	}
	return nil
}