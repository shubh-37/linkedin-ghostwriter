package slack

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shubh-37/linkedin-ghostwriter/internal/agents"
	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

// Add scheduler to CommandHandler struct
type CommandHandler struct {
	client           *Client
	thoughtRepo      *database.ThoughtRepository
	postRepo         *database.PostRepository
	brainstormRepo   *database.BrainstormRepository
	contentGenerator *agents.ContentGeneratorAgent
	scheduler        *agents.SchedulerAgent  // Add this
}

// Update NewCommandHandler
func NewCommandHandler(
	client *Client,
	thoughtRepo *database.ThoughtRepository,
	postRepo *database.PostRepository,
	brainstormRepo *database.BrainstormRepository,
	contentGenerator *agents.ContentGeneratorAgent,
	scheduler *agents.SchedulerAgent,  // Add this parameter
) *CommandHandler {
	return &CommandHandler{
		client:           client,
		thoughtRepo:      thoughtRepo,
		postRepo:         postRepo,
		brainstormRepo:   brainstormRepo,
		contentGenerator: contentGenerator,
		scheduler:        scheduler,  // Add this
	}
}

// HandleSchedule schedules approved posts
func (h *CommandHandler) HandleSchedule(ctx context.Context, channelID string, args []string) error {
	log.Printf("📅 Handling schedule command with args: %v", args)

	// Parse arguments
	postsPerDay := 2 // Default
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &postsPerDay)
	}

	if postsPerDay < 1 || postsPerDay > 4 {
		return h.client.SendMessage(channelID, "❌ Posts per day must be between 1 and 4")
	}

	// Create schedule config
	config := agents.ScheduleConfig{
		PostsPerDay:    postsPerDay,
		PreferredTimes: []string{}, // Use defaults
		StartDate:      time.Now().AddDate(0, 0, 1), // Start tomorrow
		Timezone:       "Asia/Kolkata", // Change to your timezone
	}

	// Send progress message
	h.client.SendMessage(channelID, fmt.Sprintf("📅 Scheduling approved posts... (%d posts per day)", postsPerDay))

	// Schedule posts
	scheduledCount, err := h.scheduler.ScheduleApprovedPosts(ctx, config)
	if err != nil {
		log.Printf("❌ Failed to schedule posts: %v", err)
		return h.client.SendMessage(channelID, "❌ Failed to schedule posts. Please try again.")
	}

	if scheduledCount == 0 {
		return h.client.SendMessage(channelID, "📭 No approved posts to schedule. Approve some drafts first with ✅ reaction!")
	}

	// Get schedule to show user
	schedule, err := h.scheduler.GetSchedule(ctx, 7)
	if err != nil {
		log.Printf("⚠️ Failed to get schedule: %v", err)
	}

	// Format response
	message := fmt.Sprintf("✅ *Scheduled %d posts!*\n\n", scheduledCount)
	message += fmt.Sprintf("📊 Posting %d times per day\n\n", postsPerDay)

	if len(schedule) > 0 {
		message += "*Upcoming Posts:*\n"
		for i, post := range schedule {
			if i >= 10 { // Show max 10
				message += fmt.Sprintf("_...and %d more_\n", len(schedule)-10)
				break
			}

			preview := post.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}

			timeStr := "unknown"
			if post.ScheduledAt != nil {
				timeStr = post.ScheduledAt.Format("Jan 02 at 3:04 PM")
			}

			message += fmt.Sprintf("%d. %s\n   _%s_\n\n", i+1, timeStr, preview)
		}
	}

	message += "\n🚀 Posts will be published automatically at scheduled times!"

	return h.client.SendMessage(channelID, message)
}

// HandleViewSchedule shows the current schedule
func (h *CommandHandler) HandleViewSchedule(ctx context.Context, channelID string, days int) error {
	if days <= 0 {
		days = 7 // Default to next 7 days
	}

	schedule, err := h.scheduler.GetSchedule(ctx, days)
	if err != nil {
		return h.client.SendMessage(channelID, "❌ Failed to fetch schedule")
	}

	if len(schedule) == 0 {
		return h.client.SendMessage(channelID, "📭 No posts scheduled. Use `@LinkedIn Ghostwriter schedule` to schedule approved posts!")
	}

	message := fmt.Sprintf("📅 *Posting Schedule* (Next %d days)\n\n", days)

	for i, post := range schedule {
		preview := post.Content
		if len(preview) > 80 {
			preview = preview[:80] + "..."
		}

		timeStr := "unknown"
		if post.ScheduledAt != nil {
			timeStr = post.ScheduledAt.Format("Jan 02 at 3:04 PM")
		}

		message += fmt.Sprintf("*%d. %s*\n%s\n\n", i+1, timeStr, preview)
	}

	message += fmt.Sprintf("\n_Total: %d scheduled posts_", len(schedule))

	return h.client.SendMessage(channelID, message)
}

// HandleGenerateDraft generates a LinkedIn post draft from recent thoughts
func (h *CommandHandler) HandleGenerateDraft(ctx context.Context, channelID string, category string) (string, []string, error) {
	log.Printf("📝 Generating draft for category: %s", category)

	// Get thoughts by category or all recent thoughts
	var thoughts []*models.Thought
	var err error

	if category != "" && category != "all" {
		thoughts, err = h.thoughtRepo.GetByCategory(ctx, category)
	} else {
		thoughts, err = h.thoughtRepo.GetByStatus(ctx, "raw")
	}

	if err != nil {
		h.client.SendMessage(channelID, "❌ Failed to fetch thoughts")
		return "", nil, err
	}

	if len(thoughts) == 0 {
		h.client.SendMessage(channelID, "📭 No thoughts found to generate posts from. Share some thoughts first!")
		return "", nil, fmt.Errorf("no thoughts found")
	}

	// Take the most recent thought(s)
	selectedThoughts := thoughts
	if len(thoughts) > 3 {
		selectedThoughts = thoughts[:3]
	}

	// Send "generating" message
	h.client.SendMessage(channelID, "✨ Generating LinkedIn post drafts... This may take a moment.")

	// Generate post variations
	variations, err := h.contentGenerator.GeneratePost(ctx, selectedThoughts, "")
	if err != nil {
		log.Printf("❌ Failed to generate post: %v", err)
		h.client.SendMessage(channelID, "❌ Failed to generate post. Please try again.")
		return "", nil, err
	}

	// Save drafts to database
	var postIDs []string
	for i, variation := range variations {
		thoughtIDs := make([]string, len(selectedThoughts))
		for j, t := range selectedThoughts {
			thoughtIDs[j] = t.ID
		}

		post := models.NewPost(variation, thoughtIDs, "insight", "professional")
		post.Status = "draft"

		if err := h.postRepo.Create(ctx, post); err != nil {
			log.Printf("⚠️ Failed to save draft %d: %v", i+1, err)
			continue
		}

		postIDs = append(postIDs, post.ID)
	}

	// Format message
	message := "🎯 *Generated LinkedIn Post Drafts*\n\n"
	message += fmt.Sprintf("_Based on %d recent thought(s)_\n\n", len(selectedThoughts))

	for i, variation := range variations {
		message += fmt.Sprintf("━━━━━━━━━━━━━━━━━━\n")
		message += fmt.Sprintf("*Variation %d:*\n\n", i+1)
		message += variation + "\n\n"
	}

	message += "━━━━━━━━━━━━━━━━━━\n\n"
	message += "💡 *React to approve:*\n"
	message += "• ✅ to approve all drafts\n"
	message += "• ❌ to reject all drafts\n"
	message += "• 📅 to schedule for later\n\n"
	message += "_Or use: `@LinkedIn Ghostwriter schedule`_"

	return message, postIDs, nil
}

// HandleBrainstorm generates a brainstorm for incomplete thoughts
func (h *CommandHandler) HandleBrainstorm(ctx context.Context, channelID, topic string) error {
	log.Printf("💡 Starting brainstorm for: %s", topic)

	// Create a temporary thought for brainstorming
	thought := models.NewThought(topic, "slack")

	// Send "brainstorming" message
	h.client.SendMessage(channelID, "🧠 Brainstorming ideas... This may take a moment.")

	// Generate brainstorm
	brainstormContent, angles, err := h.contentGenerator.GenerateBrainstorm(ctx, thought)
	if err != nil {
		log.Printf("❌ Failed to generate brainstorm: %v", err)
		return h.client.SendMessage(channelID, "❌ Failed to generate brainstorm. Please try again.")
	}

	// Save brainstorm session
	session := models.NewBrainstormSession(topic, []string{})
	session.BrainstormContent = brainstormContent
	session.KeyAngles = angles

	if err := h.brainstormRepo.Create(ctx, session); err != nil {
		log.Printf("⚠️ Failed to save brainstorm: %v", err)
	}

	// Format and send to Slack
	message := "🧠 *Brainstorm Session*\n\n"
	message += fmt.Sprintf("*Topic:* %s\n\n", topic)
	message += "━━━━━━━━━━━━━━━━━━\n\n"
	message += brainstormContent + "\n\n"
	message += "━━━━━━━━━━━━━━━━━━\n\n"
	message += "*Key Angles:*\n"
	for i, angle := range angles {
		message += fmt.Sprintf("%d. %s\n", i+1, angle)
	}
	message += "\n💡 Add more context and use `@LinkedIn Ghostwriter generate` when ready!"

	return h.client.SendMessage(channelID, message)
}

// HandleListDrafts shows all pending drafts
func (h *CommandHandler) HandleListDrafts(ctx context.Context, channelID string) error {
	drafts, err := h.postRepo.GetByStatus(ctx, "draft")
	if err != nil {
		return h.client.SendMessage(channelID, "❌ Failed to fetch drafts")
	}

	if len(drafts) == 0 {
		return h.client.SendMessage(channelID, "📭 No pending drafts. Use `@LinkedIn Ghostwriter generate` to create some!")
	}

	message := fmt.Sprintf("📝 *Pending Drafts* (%d)\n\n", len(drafts))

	for i, draft := range drafts {
		preview := draft.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}

		message += fmt.Sprintf("*Draft %d:*\n%s\n\n", i+1, preview)

		if i >= 4 { // Show max 5 drafts
			message += fmt.Sprintf("_...and %d more_\n", len(drafts)-5)
			break
		}
	}

	return h.client.SendMessage(channelID, message)
}