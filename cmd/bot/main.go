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
	// Load .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	log.Println("🚀 LinkedIn Ghostwriter Bot Starting...")

	// Load configuration
	cfg := config.LoadConfig()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Configuration error: %v", err)
	}

	// Create context
	ctx := context.Background()

	// Connect to database
	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Create tables
	if err := db.CreateTables(ctx); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	log.Println("✅ Database connected and ready")

	// Create repositories
	thoughtRepo := database.NewThoughtRepository(db)
	postRepo := database.NewPostRepository(db)
	brainstormRepo := database.NewBrainstormRepository(db)

	// Initialize AI Agents
	categorizer := agents.NewCategorizerAgent(cfg.AnthropicKey)
	contentGenerator := agents.NewContentGeneratorAgent(cfg.AnthropicKey)
	scheduler := agents.NewSchedulerAgent(postRepo)  // Add scheduler

	// Initialize Slack client
	slackClient := slackpkg.NewClient(cfg.SlackToken)

	// Create approval handler
	approvalHandler := slackpkg.NewApprovalHandler(slackClient, postRepo)

	// Create command handler with scheduler
	commandHandler := slackpkg.NewCommandHandler(
		slackClient,
		thoughtRepo,
		postRepo,
		brainstormRepo,
		contentGenerator,
		scheduler,  // Add scheduler
	)

	// Create message handler with approval handler
	messageHandler := slackpkg.NewMessageHandler(
		slackClient,
		thoughtRepo,
		categorizer,
		commandHandler,
		approvalHandler,
	)

	// Create Slack server with approval handler
	slackServer := slackpkg.NewServer(slackClient, messageHandler, approvalHandler, cfg.SlackSigningSecret)

	// Start Slack server in a goroutine
	go func() {
		if err := slackServer.Start("3000"); err != nil {
			log.Fatalf("Failed to start Slack server: %v", err)
		}
	}()

	log.Println("✅ System initialized successfully")
	log.Println("📊 Database: Connected and ready")
	log.Println("🤖 AI Agents: Categorizer, Content Generator & Scheduler active")
	log.Println("✅ Approval System: Ready for reactions")
	log.Println("💬 Slack: Connected and listening")
	log.Println("")
	log.Println("Bot is running. Press Ctrl+C to stop...")

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down gracefully...")
}