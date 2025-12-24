package slack

import (
	"context"
	"fmt"
	"log"

	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/slack-go/slack/slackevents"
)

type ApprovalHandler struct {
	client     *Client
	postRepo   *database.PostRepository
	draftCache map[string][]string // messageTS -> []postIDs
}

func NewApprovalHandler(client *Client, postRepo *database.PostRepository) *ApprovalHandler {
	return &ApprovalHandler{
		client:     client,
		postRepo:   postRepo,
		draftCache: make(map[string][]string),
	}
}

// StoreDraftMessage stores the mapping between Slack message and post IDs
func (h *ApprovalHandler) StoreDraftMessage(messageTS string, postIDs []string) {
	h.draftCache[messageTS] = postIDs
	log.Printf("📌 Stored draft message mapping: %s -> %v", messageTS, postIDs)
}

// HandleReaction processes reactions added to messages
func (h *ApprovalHandler) HandleReaction(ctx context.Context, event *slackevents.ReactionAddedEvent) error {
	log.Printf("👍 Reaction added: %s on message %s", event.Reaction, event.Item.Timestamp)

	// Check if this is a reaction to a draft message
	postIDs, exists := h.draftCache[event.Item.Timestamp]
	if !exists {
		log.Printf("No draft found for this message")
		return nil
	}

	// Handle different reactions
	switch event.Reaction {
	case "white_check_mark", "heavy_check_mark", "✅":
		return h.approveDrafts(ctx, event, postIDs)
	case "x", "❌":
		return h.rejectDrafts(ctx, event, postIDs)
	case "one", "1️⃣":
		return h.approveSpecificDraft(ctx, event, postIDs, 0)
	case "two", "2️⃣":
		return h.approveSpecificDraft(ctx, event, postIDs, 1)
	case "three", "3️⃣":
		return h.approveSpecificDraft(ctx, event, postIDs, 2)
	case "calendar", "📅":
		return h.scheduleDrafts(ctx, event, postIDs)
	}

	return nil
}

// approveSpecificDraft approves a specific variation
func (h *ApprovalHandler) approveSpecificDraft(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string, index int) error {
	if index >= len(postIDs) {
		return h.client.SendMessage(event.Item.Channel, "❌ Invalid variation number")
	}

	postID := postIDs[index]
	post, err := h.postRepo.GetByID(ctx, postID)
	if err != nil {
		log.Printf("⚠️ Failed to get post %s: %v", postID, err)
		return err
	}

	// Update status to approved
	post.Status = "approved"
	if err := h.postRepo.Update(ctx, post); err != nil {
		log.Printf("⚠️ Failed to update post %s: %v", postID, err)
		return err
	}

	// Reject other variations
	for i, otherID := range postIDs {
		if i != index {
			h.postRepo.UpdateStatus(ctx, otherID, "rejected")
		}
	}

	message := fmt.Sprintf("✅ Approved Variation %d! Ready for scheduling.\n\nUse `@LinkedIn Ghostwriter schedule` to schedule it.", index+1)
	return h.client.SendMessage(event.Item.Channel, message)
}

// approveDrafts marks drafts as approved
func (h *ApprovalHandler) approveDrafts(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string) error {
	log.Printf("✅ Approving %d draft(s)", len(postIDs))

	var approvedCount int
	for _, postID := range postIDs {
		post, err := h.postRepo.GetByID(ctx, postID)
		if err != nil {
			log.Printf("⚠️ Failed to get post %s: %v", postID, err)
			continue
		}

		// Update status to approved (ready for scheduling)
		post.Status = "approved"
		if err := h.postRepo.Update(ctx, post); err != nil {
			log.Printf("⚠️ Failed to update post %s: %v", postID, err)
			continue
		}

		approvedCount++
	}

	// Send confirmation
	message := fmt.Sprintf("✅ Approved %d draft(s)! They're ready for scheduling.\n\nUse `@LinkedIn Ghostwriter schedule` to schedule them for posting.", approvedCount)
	return h.client.SendMessage(event.Item.Channel, message)
}

// rejectDrafts marks drafts as rejected
func (h *ApprovalHandler) rejectDrafts(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string) error {
	log.Printf("❌ Rejecting %d draft(s)", len(postIDs))

	var rejectedCount int
	for _, postID := range postIDs {
		if err := h.postRepo.UpdateStatus(ctx, postID, "rejected"); err != nil {
			log.Printf("⚠️ Failed to update post %s: %v", postID, err)
			continue
		}
		rejectedCount++
	}

	message := fmt.Sprintf("❌ Rejected %d draft(s). Generate new ones with `@LinkedIn Ghostwriter generate`", rejectedCount)
	return h.client.SendMessage(event.Item.Channel, message)
}

// scheduleDrafts marks drafts for scheduling
func (h *ApprovalHandler) scheduleDrafts(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string) error {
	log.Printf("📅 Scheduling %d draft(s)", len(postIDs))

	var scheduledCount int
	for _, postID := range postIDs {
		post, err := h.postRepo.GetByID(ctx, postID)
		if err != nil {
			log.Printf("⚠️ Failed to get post %s: %v", postID, err)
			continue
		}

		post.Status = "approved"
		if err := h.postRepo.Update(ctx, post); err != nil {
			log.Printf("⚠️ Failed to update post %s: %v", postID, err)
			continue
		}

		scheduledCount++
	}

	message := fmt.Sprintf("📅 Marked %d draft(s) for scheduling. Use `@LinkedIn Ghostwriter schedule` to set posting times.", scheduledCount)
	return h.client.SendMessage(event.Item.Channel, message)
}