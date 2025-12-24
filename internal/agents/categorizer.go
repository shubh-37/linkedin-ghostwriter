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

// NewCategorizerAgent creates a new categorization agent
func NewCategorizerAgent(apiKey string) *CategorizerAgent {
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}

	log.Println("✅ Categorizer Agent initialized")

	return &CategorizerAgent{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

// CategorizeThought analyzes a thought and returns category, topic tags, and readiness assessment
func (a *CategorizerAgent) CategorizeThought(ctx context.Context, thought *models.Thought) error {
	log.Printf("🤖 Categorizing thought: %s", thought.Content[:min(50, len(thought.Content))])

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

	// Create request payload
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

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	// Make the request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Anthropic API error (status %d): %s", resp.StatusCode, string(body))
		return fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	// Parse response
	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	// Check for API errors
	if apiResp.Error != nil {
		return fmt.Errorf("API error: %s - %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	// Extract response text
	var responseText string
	if len(apiResp.Content) > 0 && apiResp.Content[0].Type == "text" {
		responseText = apiResp.Content[0].Text
	} else {
		return fmt.Errorf("unexpected response format")
	}

	log.Printf("📝 Claude response: %s", responseText)

	// Parse the response
	category, tags, readiness := a.parseResponse(responseText)

	// Update the thought
	thought.Category = category
	thought.TopicTags = tags
	
	// Update status based on readiness
	if readiness == "draft_ready" {
		thought.Status = "raw" // Will be picked up for drafting
	} else {
		thought.Status = "raw" // Will be picked up for brainstorming
	}

	log.Printf("✅ Categorized as: %s | Tags: %v | Readiness: %s", category, tags, readiness)

	return nil
}

// parseResponse extracts category, tags, and readiness from Claude's response
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
			// Remove brackets if present
			tagsStr = strings.Trim(tagsStr, "[]")
			// Split by comma
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

	// Default values if parsing failed
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