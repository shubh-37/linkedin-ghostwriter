package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type CategorizerAgent struct {
	apiKey     string
	httpClient *http.Client
}

type anthropicRequest struct {
	Model     string              `json:"model"`
	MaxTokens int                 `json:"max_tokens"`
	Messages  []anthropicMessage  `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Error   *anthropicError    `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func NewCategorizerAgent(apiKey string) *CategorizerAgent {
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}

	return &CategorizerAgent{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (a *CategorizerAgent) CategorizeThought(ctx context.Context, thought *models.Thought) error {

	prompt := fmt.Sprintf(`You are an AI assistant helping to categorize LinkedIn content ideas.

Analyze this thought and provide:
1. Category (choose ONE): technical, business, learning, product_update, personal, industry_insight, milestone
2. Topic tags (2-4 relevant keywords)
3. Content readiness (choose ONE): draft_ready, needs_brainstorm

Thought: "%s"

Respond in this exact format:
CATEGORY: [category]
TAGS: [tag1, tag2, tag3]
READINESS: [draft_ready or needs_brainstorm]
REASON: [brief explanation why]`, thought.Content)

	reqBody := anthropicRequest{
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 500,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return fmt.Errorf("API error: %s - %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	var responseText string
	if len(apiResp.Content) > 0 && apiResp.Content[0].Type == "text" {
		responseText = apiResp.Content[0].Text
	} else {
		return fmt.Errorf("unexpected response format")
	}

	category, tags, readiness := a.parseResponse(responseText)

	thought.Category = category
	thought.TopicTags = tags
	
	if readiness == "draft_ready" {
		thought.Status = "raw"
	} else {
		thought.Status = "raw"
	}

	return nil
}

func (a *CategorizerAgent) parseResponse(response string) (string, []string, string) {
	var category string
	var tags []string
	var readiness string

	lines := strings.Split(response, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		if strings.HasPrefix(line, "CATEGORY:") {
			category = strings.TrimSpace(strings.TrimPrefix(line, "CATEGORY:"))
			category = strings.ToLower(category)
		}
		
		if strings.HasPrefix(line, "TAGS:") {
			tagsStr := strings.TrimSpace(strings.TrimPrefix(line, "TAGS:"))
			tagsStr = strings.Trim(tagsStr, "[]")
			tagList := strings.Split(tagsStr, ",")
			for _, tag := range tagList {
				tag = strings.TrimSpace(tag)
				tag = strings.ToLower(tag)
				if tag != "" {
					tags = append(tags, tag)
				}
			}
		}
		
		if strings.HasPrefix(line, "READINESS:") {
			readiness = strings.TrimSpace(strings.TrimPrefix(line, "READINESS:"))
			readiness = strings.ToLower(readiness)
		}
	}

	if category == "" {
		category = "uncategorized"
	}
	if len(tags) == 0 {
		tags = []string{"general"}
	}
	if readiness == "" {
		readiness = "needs_brainstorm"
	}

	return category, tags, readiness
}