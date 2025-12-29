package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/shubh-37/linkedin-ghostwriter/config"
	"github.com/shubh-37/linkedin-ghostwriter/internal/agents"
	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/shubh-37/linkedin-ghostwriter/internal/linear"
	slackpkg "github.com/shubh-37/linkedin-ghostwriter/internal/slack"
)

func main() {
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

	var linearWebhookHandler *linear.WebhookHandler
	if cfg.LinearToken != "" {
		linearClient := linear.NewClient(cfg.LinearToken)
		linearWebhookHandler = linear.NewWebhookHandler(
			linearClient,
			thoughtRepo,
			categorizer,
		)
		log.Println("Linear webhook handler initialized")
	} else {
		log.Println("Linear API key not configured")
		log.Println("Add LINEAR_API_KEY to .env to enable Linear integration")
	}

	if linearWebhookHandler != nil {
		http.HandleFunc("/linear/webhook", linearWebhookHandler.HandleWebhook)
		log.Println("Linear webhook endpoint: http://localhost:3000/linear/webhook")
	}

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
}