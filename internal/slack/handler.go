package slack

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/shubh-37/linkedin-ghostwriter/internal/agents"
	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// Add approval handler to MessageHandler struct
type MessageHandler struct {
	client          *Client
	thoughtRepo     *database.ThoughtRepository
	categorizer     *agents.CategorizerAgent
	commandHandler  *CommandHandler
	approvalHandler *ApprovalHandler  // Add this
}

// Update NewMessageHandler
func NewMessageHandler(
	client *Client, 
	thoughtRepo *database.ThoughtRepository, 
	categorizer *agents.CategorizerAgent,
	commandHandler *CommandHandler,
	approvalHandler *ApprovalHandler,  // Add this parameter
) *MessageHandler {
	return &MessageHandler{
		client:          client,
		thoughtRepo:     thoughtRepo,
		categorizer:     categorizer,
		commandHandler:  commandHandler,
		approvalHandler: approvalHandler,  // Add this
	}
}

// Update HandleAppMention to add schedule commands
func (h *MessageHandler) HandleAppMention(ctx context.Context, event *slackevents.AppMentionEvent) error {
	log.Printf("📣 Bot mentioned: %s", event.Text)

	// Remove the bot mention from the text
	text := strings.TrimSpace(strings.Replace(event.Text, "<@"+h.client.GetBotID()+">", "", 1))

	// Check for commands
	if strings.HasPrefix(text, "help") {
		return h.sendHelpMessage(event.Channel)
	}

	if strings.HasPrefix(text, "stats") {
		return h.sendStatsMessage(ctx, event.Channel)
	}

	if strings.HasPrefix(text, "generate") {
		// Extract category if provided
		parts := strings.Fields(text)
		category := "all"
		if len(parts) > 1 {
			category = parts[1]
		}

		// Generate drafts and get message/postIDs
		message, postIDs, err := h.commandHandler.HandleGenerateDraft(ctx, event.Channel, category)
		if err != nil {
			return err
		}

		// Send message and store the timestamp mapping
		messageTS, err := h.sendMessageAndGetTS(event.Channel, message)
		if err != nil {
			return err
		}

		// Store the mapping for approval
		h.approvalHandler.StoreDraftMessage(messageTS, postIDs)

		return nil
	}

	if strings.HasPrefix(text, "drafts") {
		return h.commandHandler.HandleListDrafts(ctx, event.Channel)
	}

	if strings.HasPrefix(text, "schedule") {
		// Extract arguments
		parts := strings.Fields(text)
		args := []string{}
		if len(parts) > 1 {
			args = parts[1:]
		}
		return h.commandHandler.HandleSchedule(ctx, event.Channel, args)
	}

	if strings.HasPrefix(text, "view schedule") || strings.HasPrefix(text, "show schedule") {
		// Extract days if provided
		days := 7
		parts := strings.Fields(text)
		if len(parts) > 2 {
			fmt.Sscanf(parts[2], "%d", &days)
		}
		return h.commandHandler.HandleViewSchedule(ctx, event.Channel, days)
	}

	if strings.HasPrefix(text, "brainstorm") {
		// Extract topic
		topic := strings.TrimPrefix(text, "brainstorm")
		topic = strings.TrimSpace(topic)
		if topic == "" {
			return h.client.SendMessage(event.Channel, "Please provide a topic: `@LinkedIn Ghostwriter brainstorm [your topic]`")
		}
		return h.commandHandler.HandleBrainstorm(ctx, event.Channel, topic)
	}

	// Otherwise, treat it as a thought
	if text != "" {
		thought := models.NewThought(text, "slack")

		// Categorize with AI
		if err := h.categorizer.CategorizeThought(ctx, thought); err != nil {
			log.Printf("⚠️ Failed to categorize thought: %v", err)
			thought.Category = "uncategorized"
			thought.TopicTags = []string{"general"}
		}

		if err := h.thoughtRepo.Create(ctx, thought); err != nil {
			log.Printf("❌ Failed to save thought: %v", err)
			return err
		}

		log.Printf("✅ Saved thought from mention with ID: %s | Category: %s", 
			thought.ID, thought.Category)
		
		confirmationMsg := fmt.Sprintf("💭 Captured! Category: *%s* | Tags: %s", 
			thought.Category, 
			strings.Join(thought.TopicTags, ", "))
		
		return h.client.SendMessage(event.Channel, confirmationMsg)
	}

	return nil
}

// Update help message
func (h *MessageHandler) sendHelpMessage(channelID string) error {
	helpText := `*LinkedIn Ghostwriter Bot* 🤖

I capture your thoughts and help generate LinkedIn posts!

*Commands:*
- \@LinkedIn Ghostwriter generate - Generate post drafts
- \@LinkedIn Ghostwriter generate [category] - Generate from category
- \@LinkedIn Ghostwriter brainstorm [topic] - Brainstorm ideas
- \@LinkedIn Ghostwriter drafts - View pending drafts
- \@LinkedIn Ghostwriter schedule [1-4] - Schedule approved posts
- \@LinkedIn Ghostwriter view schedule - See posting schedule
- \@LinkedIn Ghostwriter stats - Show statistics
- \@LinkedIn Ghostwriter help - Show this help

*Workflow:*
1. Share thoughts naturally
2. Generate posts: \@LinkedIn Ghostwriter generate
3. React with ✅ to approve drafts
4. Schedule: \@LinkedIn Ghostwriter schedule 2 (2 posts/day)
5. Posts publish automatically!

*Categories:*
technical, business, learning, product_update, personal, industry_insight, milestone`

	return h.client.SendMessage(channelID, helpText)
}

// Helper function to send message and get timestamp
func (h *MessageHandler) sendMessageAndGetTS(channelID, message string) (string, error) {
	_, timestamp, err := h.client.GetAPI().PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	return timestamp, err
}

// HandleMessage processes incoming Slack messages
func (h *MessageHandler) HandleMessage(ctx context.Context, event *slackevents.MessageEvent) error {
	// Ignore messages from bots (including ourselves)
	if event.BotID != "" {
		return nil
	}

	// Ignore empty messages
	if strings.TrimSpace(event.Text) == "" {
		return nil
	}

	// Ignore thread replies (optional - you can change this)
	if event.ThreadTimeStamp != "" && event.ThreadTimeStamp != event.TimeStamp {
		log.Printf("Ignoring thread reply")
		return nil
	}

	log.Printf("📨 Received message: %s (from user: %s, channel: %s)", 
		event.Text, event.User, event.Channel)

	// Create a thought from the message
	thought := models.NewThought(event.Text, "slack")

	// Categorize with AI
	if err := h.categorizer.CategorizeThought(ctx, thought); err != nil {
		log.Printf("⚠️ Failed to categorize thought: %v", err)
		// Continue anyway with default values
		thought.Category = "uncategorized"
		thought.TopicTags = []string{"general"}
	}

	// Save to database
	if err := h.thoughtRepo.Create(ctx, thought); err != nil {
		log.Printf("❌ Failed to save thought: %v", err)
		return err
	}

	log.Printf("✅ Saved thought with ID: %s | Category: %s | Tags: %v", 
		thought.ID, thought.Category, thought.TopicTags)

	// Send confirmation back to Slack
	confirmationMsg := fmt.Sprintf("💭 Got it! Categorized as: *%s* | Tags: %s", 
		thought.Category, 
		strings.Join(thought.TopicTags, ", "))
	
	if err := h.client.SendMessage(event.Channel, confirmationMsg); err != nil {
		log.Printf("Failed to send confirmation: %v", err)
	}

	return nil
}

// sendStatsMessage sends statistics about captured thoughts
func (h *MessageHandler) sendStatsMessage(ctx context.Context, channelID string) error {
	count, err := h.thoughtRepo.Count(ctx)
	if err != nil {
		return h.client.SendMessage(channelID, "❌ Failed to fetch stats")
	}

	thoughts, err := h.thoughtRepo.GetAll(ctx)
	if err != nil {
		return h.client.SendMessage(channelID, "❌ Failed to fetch thoughts")
	}

	// Count by category
	categoryCount := make(map[string]int)
	for _, thought := range thoughts {
		categoryCount[thought.Category]++
	}

	statsText := "📊 *Thought Statistics*\n\n"
	statsText += fmt.Sprintf("Total captured: *%d*\n\n", count)
	statsText += "*By Category:*\n"
	for category, cnt := range categoryCount {
		statsText += fmt.Sprintf("• %s: %d\n", category, cnt)
	}

	// Show recent thoughts
	statsText += "\n*Recent Thoughts:*\n"
	recentCount := 3
	if len(thoughts) < recentCount {
		recentCount = len(thoughts)
	}
	
	for i := 0; i < recentCount; i++ {
		preview := thoughts[i].Content
		if len(preview) > 60 {
			preview = preview[:60] + "..."
		}
		statsText += fmt.Sprintf("%d. [%s] %s\n", i+1, thoughts[i].Category, preview)
	}

	return h.client.SendMessage(channelID, statsText)
}