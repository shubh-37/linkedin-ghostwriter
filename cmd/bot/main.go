package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"github.com/shubh-37/linkedin-ghostwriter/config"
	"github.com/shubh-37/linkedin-ghostwriter/internal/agents"
	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	slackpkg "github.com/shubh-37/linkedin-ghostwriter/internal/slack"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	log.Println("Starting LinkedIn Ghostwriter Bot")

	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	ctx := context.Background()

	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.CreateTables(ctx); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	thoughtRepo := database.NewThoughtRepository(db)
	postRepo := database.NewPostRepository(db)
	brainstormRepo := database.NewBrainstormRepository(db)

	categorizer := agents.NewCategorizerAgent(cfg.AnthropicKey)
	contentGenerator := agents.NewContentGeneratorAgent(cfg.AnthropicKey)
	scheduler := agents.NewSchedulerAgent(postRepo)

	slackClient := slackpkg.NewClient(cfg.SlackToken)

	approvalHandler := slackpkg.NewApprovalHandler(slackClient, postRepo)

	commandHandler := slackpkg.NewCommandHandler(
		slackClient,
		thoughtRepo,
		postRepo,
		brainstormRepo,
		contentGenerator,
		scheduler,
	)

	messageHandler := slackpkg.NewMessageHandler(
		slackClient,
		thoughtRepo,
		categorizer,
		commandHandler,
		approvalHandler,
	)

	slackServer := slackpkg.NewServer(slackClient, messageHandler, approvalHandler, cfg.SlackSigningSecret)

	go func() {
		if err := slackServer.Start("3000"); err != nil {
			log.Fatalf("Failed to start Slack server: %v", err)
		}
	}()

	log.Println("Bot is running. Press Ctrl+C to stop...")

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")
}