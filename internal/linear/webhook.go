package linear

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/shubh-37/linkedin-ghostwriter/internal/agents"
	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type WebhookHandler struct {
	linearClient   *Client
	thoughtRepo    *database.ThoughtRepository
	categorizer    *agents.CategorizerAgent
	processedIssues map[string]bool
	mu             sync.Mutex
}

type WebhookPayload struct {
	Action      string          `json:"action"`
	Type        string          `json:"type"`
	Data        json.RawMessage `json:"data"`
	UpdatedFrom json.RawMessage `json:"updatedFrom,omitempty"`
}

type WebhookIssueData struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	State       struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Team struct {
		Name string `json:"name"`
	} `json:"team"`
}

func NewWebhookHandler(
	linearClient *Client,
	thoughtRepo *database.ThoughtRepository,
	categorizer *agents.CategorizerAgent,
) *WebhookHandler {
	return &WebhookHandler{
		linearClient:    linearClient,
		thoughtRepo:     thoughtRepo,
		categorizer:     categorizer,
		processedIssues: make(map[string]bool),
	}
}

func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("failed to read webhook body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		log.Printf("failed to parse webhook payload: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("received linear webhook: %s %s", payload.Action, payload.Type)

	if payload.Type != "Issue" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if payload.Action != "update" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var issueData WebhookIssueData
	if err := json.Unmarshal(payload.Data, &issueData); err != nil {
		log.Printf("failed to parse issue data: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if issueData.State.Type != "completed" {
		w.WriteHeader(http.StatusOK)
		return
	}

	h.mu.Lock()
	if h.processedIssues[issueData.ID] {
		h.mu.Unlock()
		log.Printf("skipping duplicate issue: %s", issueData.ID)
		w.WriteHeader(http.StatusOK)
		return
	}
	h.processedIssues[issueData.ID] = true
	h.mu.Unlock()

	log.Printf("issue completed: %s - %s", issueData.ID, issueData.Title)

	ctx := context.Background()
	if err := h.createThoughtFromIssue(ctx, &issueData); err != nil {
		log.Printf("failed to create thought: %v", err)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) createThoughtFromIssue(ctx context.Context, issue *WebhookIssueData) error {
	content := fmt.Sprintf("Completed: %s", issue.Title)
	if issue.Description != "" {
		content += fmt.Sprintf("\n\nDetails: %s", issue.Description)
	}

	thought := models.NewThought(content, "linear")

	if err := h.categorizer.CategorizeThought(ctx, thought); err != nil {
		log.Printf("failed to categorize thought: %v", err)
		thought.Category = "product_update"
		thought.TopicTags = []string{"development", issue.Team.Name}
	}

	if err := h.thoughtRepo.Create(ctx, thought); err != nil {
		return fmt.Errorf("failed to save thought: %w", err)
	}

	log.Printf("created thought from linear issue: %s", thought.ID)

	return nil
}
