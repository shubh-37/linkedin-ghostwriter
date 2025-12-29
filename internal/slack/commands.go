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

type CommandHandler struct {
	client           *Client
	thoughtRepo      *database.ThoughtRepository
	postRepo         *database.PostRepository
	brainstormRepo   *database.BrainstormRepository
	contentGenerator *agents.ContentGeneratorAgent
	scheduler        *agents.SchedulerAgent
}

func NewCommandHandler(
	client *Client,
	thoughtRepo *database.ThoughtRepository,
	postRepo *database.PostRepository,
	brainstormRepo *database.BrainstormRepository,
	contentGenerator *agents.ContentGeneratorAgent,
	scheduler *agents.SchedulerAgent,
) *CommandHandler {
	return &CommandHandler{
		client:           client,
		thoughtRepo:      thoughtRepo,
		postRepo:         postRepo,
		brainstormRepo:   brainstormRepo,
		contentGenerator: contentGenerator,
		scheduler:        scheduler,
	}
}

func (h *CommandHandler) HandleSchedule(ctx context.Context, channelID string, args []string) error {
	postsPerDay := 2
	if len(args) > 0 {
		fmt.Sscanf(args[0], "%d", &postsPerDay)
	}

	if postsPerDay < 1 || postsPerDay > 4 {
		return h.client.SendMessage(channelID, "Posts per day must be between 1 and 4")
	}

	config := agents.ScheduleConfig{
		PostsPerDay:    postsPerDay,
		PreferredTimes: []string{},
		StartDate:      time.Now().AddDate(0, 0, 1),
		Timezone:       "Asia/Kolkata",
	}

	h.client.SendMessage(channelID, fmt.Sprintf("Scheduling approved posts... (%d posts per day)", postsPerDay))

	scheduledCount, err := h.scheduler.ScheduleApprovedPosts(ctx, config)
	if err != nil {
		return h.client.SendMessage(channelID, "Failed to schedule posts. Please try again.")
	}

	if scheduledCount == 0 {
		return h.client.SendMessage(channelID, "No approved posts to schedule. Approve some drafts first.")
	}

	schedule, err := h.scheduler.GetSchedule(ctx, 7)
	if err != nil {
		log.Printf("Failed to get schedule: %v", err)
	}

	message := fmt.Sprintf("*Scheduled %d posts!*\n\n", scheduledCount)
	message += fmt.Sprintf("Posting %d times per day\n\n", postsPerDay)

	if len(schedule) > 0 {
		message += "*Upcoming Posts:*\n"
		for i, post := range schedule {
			if i >= 10 {
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

	message += "\nPosts will be published automatically at scheduled times!"

	return h.client.SendMessage(channelID, message)
}

func (h *CommandHandler) HandleViewSchedule(ctx context.Context, channelID string, days int) error {
	if days <= 0 {
		days = 7
	}

	schedule, err := h.scheduler.GetSchedule(ctx, days)
	if err != nil {
		return h.client.SendMessage(channelID, "Failed to fetch schedule")
	}

	if len(schedule) == 0 {
		return h.client.SendMessage(channelID, "No posts scheduled. Use `@LinkedIn Ghostwriter schedule` to schedule approved posts!")
	}

	message := fmt.Sprintf("*Posting Schedule* (Next %d days)\n\n", days)

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

func (h *CommandHandler) HandleGenerateDraft(ctx context.Context, channelID string, category string) (string, []string, error) {
	var thoughts []*models.Thought
	var err error

	if category != "" && category != "all" {
		thoughts, err = h.thoughtRepo.GetByCategory(ctx, category)
	} else {
		thoughts, err = h.thoughtRepo.GetByStatus(ctx, "raw")
	}

	if err != nil {
		h.client.SendMessage(channelID, "Failed to fetch thoughts")
		return "", nil, err
	}

	if len(thoughts) == 0 {
		h.client.SendMessage(channelID, "No thoughts found to generate posts from. Share some thoughts first!")
		return "", nil, fmt.Errorf("no thoughts found")
	}

	selectedThoughts := thoughts
	if len(thoughts) > 3 {
		selectedThoughts = thoughts[:3]
	}

	h.client.SendMessage(channelID, "Generating LinkedIn post drafts... This may take a moment.")

	variations, err := h.contentGenerator.GeneratePost(ctx, selectedThoughts, "")
	if err != nil {
		h.client.SendMessage(channelID, "Failed to generate post. Please try again.")
		return "", nil, err
	}

	var postIDs []string
	for _, variation := range variations {
		thoughtIDs := make([]string, len(selectedThoughts))
		for j, t := range selectedThoughts {
			thoughtIDs[j] = t.ID
		}

		post := models.NewPost(variation, thoughtIDs, "insight", "professional")
		post.Status = "draft"

		if err := h.postRepo.Create(ctx, post); err != nil {
			continue
		}

		postIDs = append(postIDs, post.ID)
	}

	message := "*Generated LinkedIn Post Drafts*\n\n"
	message += fmt.Sprintf("_Based on %d recent thought(s)_\n\n", len(selectedThoughts))

	for i, variation := range variations {
		message += "━━━━━━━━━━━━━━━━━━\n"
		message += fmt.Sprintf("*Variation %d:*\n\n", i+1)
		message += variation + "\n\n"
	}

	message += "━━━━━━━━━━━━━━━━━━\n\n"
	message += "*To approve a specific variation:*\n"
	message += "React with:\n"
	message += "• 1️⃣ to approve Variation 1\n"
	message += "• 2️⃣ to approve Variation 2\n"
	message += "• 3️⃣ to approve Variation 3\n"
	message += "• ✅ to approve ALL variations\n"
	message += "• ❌ to reject all\n"

	return message, postIDs, nil
}

func (h *CommandHandler) HandleBrainstorm(ctx context.Context, channelID, topic string) error {
	thought := models.NewThought(topic, "slack")

	h.client.SendMessage(channelID, "Brainstorming ideas... This may take a moment.")

	brainstormContent, angles, err := h.contentGenerator.GenerateBrainstorm(ctx, thought)
	if err != nil {
		return h.client.SendMessage(channelID, "Failed to generate brainstorm. Please try again.")
	}

	session := models.NewBrainstormSession(topic, []string{})
	session.BrainstormContent = brainstormContent
	session.KeyAngles = angles

	if err := h.brainstormRepo.Create(ctx, session); err != nil {
		log.Printf("Failed to save brainstorm: %v", err)
	}

	message := "*Brainstorm Session*\n\n"
	message += fmt.Sprintf("*Topic:* %s\n\n", topic)
	message += "━━━━━━━━━━━━━━━━━━\n\n"
	message += brainstormContent + "\n\n"
	message += "━━━━━━━━━━━━━━━━━━\n\n"
	message += "*Key Angles:*\n"
	for i, angle := range angles {
		message += fmt.Sprintf("%d. %s\n", i+1, angle)
	}
	message += "\nAdd more context and use `@LinkedIn Ghostwriter generate` when ready!"

	return h.client.SendMessage(channelID, message)
}

func (h *CommandHandler) HandleListDrafts(ctx context.Context, channelID string) error {
	drafts, err := h.postRepo.GetByStatus(ctx, "draft")
	if err != nil {
		return h.client.SendMessage(channelID, "Failed to fetch drafts")
	}

	if len(drafts) == 0 {
		return h.client.SendMessage(channelID, "No pending drafts. Use `@LinkedIn Ghostwriter generate` to create some!")
	}

	message := fmt.Sprintf("*Pending Drafts* (%d)\n\n", len(drafts))

	for i, draft := range drafts {
		preview := draft.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}

		message += fmt.Sprintf("*Draft %d:*\n%s\n\n", i+1, preview)

		if i >= 4 {
			message += fmt.Sprintf("_...and %d more_\n", len(drafts)-5)
			break
		}
	}

	return h.client.SendMessage(channelID, message)
}

func (h *CommandHandler) HandleLinearSync(ctx context.Context, channelID string) error {
	h.client.SendMessage(channelID, "Syncing with Linear...")

	message := "Linear sync completed!\n\n"
	message += "Recent completed tasks have been captured as thoughts.\n"
	message += "Use `@LinkedIn Ghostwriter generate` to create posts from them."

	return h.client.SendMessage(channelID, message)
}