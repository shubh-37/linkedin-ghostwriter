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

type MessageHandler struct {
	client          *Client
	thoughtRepo     *database.ThoughtRepository
	categorizer     *agents.CategorizerAgent
	commandHandler  *CommandHandler
	approvalHandler *ApprovalHandler
}

func NewMessageHandler(
	client *Client,
	thoughtRepo *database.ThoughtRepository,
	categorizer *agents.CategorizerAgent,
	commandHandler *CommandHandler,
	approvalHandler *ApprovalHandler,
) *MessageHandler {
	return &MessageHandler{
		client:          client,
		thoughtRepo:     thoughtRepo,
		categorizer:     categorizer,
		commandHandler:  commandHandler,
		approvalHandler: approvalHandler,
	}
}

// HandleMessage processes incoming Slack messages
func (h *MessageHandler) HandleMessage(ctx context.Context, event *slackevents.MessageEvent) error {
	// Ignore messages from bots
	if event.BotID != "" {
		return nil
	}

	// Ignore messages from bot user
	if event.User == h.client.GetBotID() {
		return nil
	}

	// Ignore messages with subtypes (bot messages, etc)
	if event.SubType != "" {
		log.Printf("Ignoring message with subtype: %s", event.SubType)
		return nil
	}

	// Ignore empty messages
	if strings.TrimSpace(event.Text) == "" {
		return nil
	}

	// Ignore thread replies
	if event.ThreadTimeStamp != "" && event.ThreadTimeStamp != event.TimeStamp {
		log.Printf("Ignoring thread reply")
		return nil
	}

	// Ignore messages that are commands/mentions
	if strings.HasPrefix(strings.TrimSpace(event.Text), "<@") {
		log.Printf("Ignoring command/mention message")
		return nil
	}

	// Ignore command-like messages
	text := strings.ToLower(strings.TrimSpace(event.Text))
	commandPrefixes := []string{"generate", "schedule", "drafts", "brainstorm", "stats", "help", "view"}
	for _, prefix := range commandPrefixes {
		if strings.HasPrefix(text, prefix) {
			log.Printf("Ignoring command-like message: %s", prefix)
			return nil
		}
	}

	log.Printf("📨 Received message: %s (from user: %s, channel: %s)",
		event.Text, event.User, event.Channel)

	// Create a thought from the message
	thought := models.NewThought(event.Text, "slack")

	// Categorize with AI
	if err := h.categorizer.CategorizeThought(ctx, thought); err != nil {
		log.Printf("⚠️ Failed to categorize thought: %v", err)
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

// HandleAppMention processes when someone mentions the bot
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
		parts := strings.Fields(text)

		if len(parts) == 1 {
			// No topic specified
			message, postIDs, err := h.commandHandler.HandleGenerateDraft(ctx, event.Channel, "all")
			if err != nil {
				return err
			}

			messageTS, err := h.sendMessageAndGetTS(event.Channel, message)
			if err != nil {
				return err
			}

			h.approvalHandler.StoreDraftMessage(messageTS, postIDs)
			return nil
		}

		// Topic specified
		topic := strings.Join(parts[1:], " ")

		// Check if we have thoughts on this topic
		thoughts, err := h.thoughtRepo.GetByCategory(ctx, topic)
		if err == nil && len(thoughts) > 0 {
			message, postIDs, err := h.commandHandler.HandleGenerateDraft(ctx, event.Channel, topic)
			if err != nil {
				return err
			}

			messageTS, err := h.sendMessageAndGetTS(event.Channel, message)
			if err != nil {
				return err
			}

			h.approvalHandler.StoreDraftMessage(messageTS, postIDs)
			return nil
		}

		// No thoughts found, offer brainstorm
		offerMsg := fmt.Sprintf("🤔 I don't have any thoughts categorized as '%s' yet.\n\n", topic)
		offerMsg += "Would you like me to brainstorm ideas on this topic?\n\n"
		offerMsg += fmt.Sprintf("Use: `@LinkedIn Ghostwriter brainstorm %s`", topic)

		return h.client.SendMessage(event.Channel, offerMsg)
	}

	if strings.HasPrefix(text, "drafts") {
		return h.commandHandler.HandleListDrafts(ctx, event.Channel)
	}

	if strings.HasPrefix(text, "schedule") {
		parts := strings.Fields(text)
		args := []string{}
		if len(parts) > 1 {
			args = parts[1:]
		}
		return h.commandHandler.HandleSchedule(ctx, event.Channel, args)
	}

	if strings.HasPrefix(text, "view schedule") || strings.HasPrefix(text, "show schedule") {
		days := 7
		parts := strings.Fields(text)
		if len(parts) > 2 {
			fmt.Sscanf(parts[2], "%d", &days)
		}
		return h.commandHandler.HandleViewSchedule(ctx, event.Channel, days)
	}

	if strings.HasPrefix(text, "brainstorm") {
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

// ... rest of methods (sendHelpMessage, sendStatsMessage, etc)

// Helper function to send message and get timestamp
func (h *MessageHandler) sendMessageAndGetTS(channelID, message string) (string, error) {
	_, timestamp, err := h.client.GetAPI().PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	return timestamp, err
}

// sendHelpMessage sends usage instructions
func (h *MessageHandler) sendHelpMessage(channelID string) error {
	helpText := `*LinkedIn Ghostwriter Bot* 🤖

I capture your thoughts and help generate LinkedIn posts!

*Commands:*
- \@LinkedIn Ghostwriter generate - Generate from recent thoughts
- \@LinkedIn Ghostwriter generate [topic] - Generate from specific topic
- \@LinkedIn Ghostwriter brainstorm [topic] - Brainstorm ideas
- \@LinkedIn Ghostwriter drafts - View pending drafts
- \@LinkedIn Ghostwriter schedule [1-4] - Schedule approved posts
- \@LinkedIn Ghostwriter view schedule - See posting schedule
- \@LinkedIn Ghostwriter stats - Show statistics
- \@LinkedIn Ghostwriter help - Show this help

*Workflow:*
1. Share thoughts naturally
2. Generate posts: \@LinkedIn Ghostwriter generate
3. React with 1️⃣ 2️⃣ 3️⃣ or ✅ to approve
4. Schedule: \@LinkedIn Ghostwriter schedule 2 (2 posts/day)
5. Posts publish automatically!

*Categories:*
technical, business, learning, product_update, personal, industry_insight, milestone`

	return h.client.SendMessage(channelID, helpText)
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