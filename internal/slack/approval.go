package slack

import (
	"context"
	"fmt"

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

func (h *ApprovalHandler) StoreDraftMessage(messageTS string, postIDs []string) {
	h.draftCache[messageTS] = postIDs
}

func (h *ApprovalHandler) HandleReaction(ctx context.Context, event *slackevents.ReactionAddedEvent) error {
	postIDs, exists := h.draftCache[event.Item.Timestamp]
	if !exists {
		return nil
	}

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

func (h *ApprovalHandler) approveSpecificDraft(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string, index int) error {
	if index >= len(postIDs) {
		return h.client.SendMessage(event.Item.Channel, "Invalid variation number")
	}

	postID := postIDs[index]
	post, err := h.postRepo.GetByID(ctx, postID)
	if err != nil {
		return err
	}

	post.Status = "approved"
	if err := h.postRepo.Update(ctx, post); err != nil {
		return err
	}

	for i, otherID := range postIDs {
		if i != index {
			h.postRepo.UpdateStatus(ctx, otherID, "rejected")
		}
	}

	message := fmt.Sprintf("Approved Variation %d! Ready for scheduling.\n\nUse `@LinkedIn Ghostwriter schedule` to schedule it.", index+1)
	return h.client.SendMessage(event.Item.Channel, message)
}

func (h *ApprovalHandler) approveDrafts(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string) error {
	var approvedCount int
	for _, postID := range postIDs {
		post, err := h.postRepo.GetByID(ctx, postID)
		if err != nil {
			continue
		}

		post.Status = "approved"
		if err := h.postRepo.Update(ctx, post); err != nil {
			continue
		}

		approvedCount++
	}

	message := fmt.Sprintf("Approved %d draft(s)! They're ready for scheduling.\n\nUse `@LinkedIn Ghostwriter schedule` to schedule them for posting.", approvedCount)
	return h.client.SendMessage(event.Item.Channel, message)
}

func (h *ApprovalHandler) rejectDrafts(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string) error {
	var rejectedCount int
	for _, postID := range postIDs {
		if err := h.postRepo.UpdateStatus(ctx, postID, "rejected"); err != nil {
			continue
		}
		rejectedCount++
	}

	message := fmt.Sprintf("Rejected %d draft(s). Generate new ones with `@LinkedIn Ghostwriter generate`", rejectedCount)
	return h.client.SendMessage(event.Item.Channel, message)
}

func (h *ApprovalHandler) scheduleDrafts(ctx context.Context, event *slackevents.ReactionAddedEvent, postIDs []string) error {
	var scheduledCount int
	for _, postID := range postIDs {
		post, err := h.postRepo.GetByID(ctx, postID)
		if err != nil {
			continue
		}

		post.Status = "approved"
		if err := h.postRepo.Update(ctx, post); err != nil {
			continue
		}

		scheduledCount++
	}

	message := fmt.Sprintf("Marked %d draft(s) for scheduling. Use `@LinkedIn Ghostwriter schedule` to set posting times.", scheduledCount)
	return h.client.SendMessage(event.Item.Channel, message)
}