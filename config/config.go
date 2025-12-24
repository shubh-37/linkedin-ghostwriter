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

// LoadConfig loads configuration from environment variables
// It first tries to load from .env file, then falls back to system environment variables
func LoadConfig() *Config {
	// Load .env file if it exists (ignore error if file doesn't exist)
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found or couldn't be loaded: %v", err)
	}
	
	return &Config{
		DatabaseURL:        getEnv("DATABASE_URL", "postgres://ghostwriter_user:ghostwriter_pass_2024@localhost:5432/linkedin_ghostwriter?sslmode=disable"),
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