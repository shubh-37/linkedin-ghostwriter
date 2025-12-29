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

func (h *MessageHandler) HandleMessage(ctx context.Context, event *slackevents.MessageEvent) error {
	if event.BotID != "" {
		return nil
	}

	if event.User == h.client.GetBotID() {
		return nil
	}

	if event.SubType != "" {
		return nil
	}

	if strings.TrimSpace(event.Text) == "" {
		return nil
	}

	if event.ThreadTimeStamp != "" && event.ThreadTimeStamp != event.TimeStamp {
		return nil
	}

	if strings.HasPrefix(strings.TrimSpace(event.Text), "<@") {
		return nil
	}

	text := strings.ToLower(strings.TrimSpace(event.Text))
	commandPrefixes := []string{"generate", "schedule", "drafts", "brainstorm", "stats", "help", "view"}
	for _, prefix := range commandPrefixes {
		if strings.HasPrefix(text, prefix) {
			return nil
		}
	}

	thought := models.NewThought(event.Text, "slack")

	if err := h.categorizer.CategorizeThought(ctx, thought); err != nil {
		thought.Category = "uncategorized"
		thought.TopicTags = []string{"general"}
	}

	if err := h.thoughtRepo.Create(ctx, thought); err != nil {
		log.Printf("Failed to save thought: %v", err)
		return err
	}

	confirmationMsg := fmt.Sprintf("Got it! Categorized as: *%s* | Tags: %s",
		thought.Category,
		strings.Join(thought.TopicTags, ", "))

	if err := h.client.SendMessage(event.Channel, confirmationMsg); err != nil {
		log.Printf("Failed to send confirmation: %v", err)
	}

	return nil
}

func (h *MessageHandler) HandleAppMention(ctx context.Context, event *slackevents.AppMentionEvent) error {
	text := strings.TrimSpace(strings.Replace(event.Text, "<@"+h.client.GetBotID()+">", "", 1))

	if strings.HasPrefix(text, "help") {
		return h.sendHelpMessage(event.Channel)
	}

	if strings.HasPrefix(text, "stats") {
		return h.sendStatsMessage(ctx, event.Channel)
	}

	if strings.HasPrefix(text, "generate") {
		parts := strings.Fields(text)

		if len(parts) == 1 {
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

		topic := strings.Join(parts[1:], " ")

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

		offerMsg := fmt.Sprintf("I don't have any thoughts categorized as '%s' yet.\n\n", topic)
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

	if strings.HasPrefix(text, "sync linear") || strings.HasPrefix(text, "linear sync") {
		return h.commandHandler.HandleLinearSync(ctx, event.Channel)
	}

	if text != "" {
		thought := models.NewThought(text, "slack")

		if err := h.categorizer.CategorizeThought(ctx, thought); err != nil {
			thought.Category = "uncategorized"
			thought.TopicTags = []string{"general"}
		}

		if err := h.thoughtRepo.Create(ctx, thought); err != nil {
			log.Printf("Failed to save thought: %v", err)
			return err
		}

		confirmationMsg := fmt.Sprintf("Captured! Category: *%s* | Tags: %s",
			thought.Category,
			strings.Join(thought.TopicTags, ", "))

		return h.client.SendMessage(event.Channel, confirmationMsg)
	}

	return nil
}

func (h *MessageHandler) sendMessageAndGetTS(channelID, message string) (string, error) {
	_, timestamp, err := h.client.GetAPI().PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	return timestamp, err
}

func (h *MessageHandler) sendHelpMessage(channelID string) error {
	helpText := `*LinkedIn Ghostwriter Bot*

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

func (h *MessageHandler) sendStatsMessage(ctx context.Context, channelID string) error {
	count, err := h.thoughtRepo.Count(ctx)
	if err != nil {
		return h.client.SendMessage(channelID, "Failed to fetch stats")
	}

	thoughts, err := h.thoughtRepo.GetAll(ctx)
	if err != nil {
		return h.client.SendMessage(channelID, "Failed to fetch thoughts")
	}

	categoryCount := make(map[string]int)
	for _, thought := range thoughts {
		categoryCount[thought.Category]++
	}

	statsText := "*Thought Statistics*\n\n"
	statsText += fmt.Sprintf("Total captured: *%d*\n\n", count)
	statsText += "*By Category:*\n"
	for category, cnt := range categoryCount {
		statsText += fmt.Sprintf("• %s: %d\n", category, cnt)
	}

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