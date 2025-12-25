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

type ContentGeneratorAgent struct {
	apiKey     string
	httpClient *http.Client
}

func NewContentGeneratorAgent(apiKey string) *ContentGeneratorAgent {
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}

	return &ContentGeneratorAgent{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}
}

func (a *ContentGeneratorAgent) GeneratePost(ctx context.Context, thoughts []*models.Thought, userStyle string) ([]string, error) {
	if len(thoughts) == 0 {
		return nil, fmt.Errorf("no thoughts provided")
	}

	var thoughtsText string
	for i, thought := range thoughts {
		thoughtsText += fmt.Sprintf("\nThought %d: %s", i+1, thought.Content)
	}
	prompt := fmt.Sprintf(`You are a LinkedIn ghostwriter helping create authentic, engaging posts.

Input thoughts:%s

Create a LinkedIn post that:
1. Sounds natural and conversational (not corporate or salesy)
2. Starts with a strong hook that grabs attention
3. Uses short paragraphs and line breaks for readability
4. Includes a clear insight or takeaway
5. Ends with engagement (question, call to action, or thought-provoking statement)
6. Is between 150-300 words
7. Uses emojis sparingly (1-2 max)

Writing style guidelines:
- Be authentic and personal
- Use "I" and "we" pronouns
- Share specific details and numbers when available
- Avoid buzzwords and jargon
- Keep it concise and punchy

Generate 3 different variations with different angles:
- Variation 1: Story-driven approach
- Variation 2: Insight/lesson-focused
- Variation 3: Data/results-focused

Format your response as:
===VARIATION 1===
[post content]

===VARIATION 2===
[post content]

===VARIATION 3===
[post content]`, thoughtsText)

	responseText, err := a.callClaude(ctx, prompt)
	if err != nil {
		return nil, err
	}

	variations := a.parseVariations(responseText)

	if len(variations) == 0 {
		return nil, fmt.Errorf("failed to generate variations")
	}

	return variations, nil
}

func (a *ContentGeneratorAgent) GenerateBrainstorm(ctx context.Context, thought *models.Thought) (string, []string, error) {

	prompt := fmt.Sprintf(`You are helping brainstorm LinkedIn content ideas.

The user shared this incomplete thought:
"%s"

Help develop this into a complete LinkedIn post idea by:
1. Exploring different angles to approach this topic
2. Identifying what additional context or examples would strengthen it
3. Suggesting 3-4 specific directions this could go

Respond in this format:
EXPLORATION:
[2-3 paragraphs exploring the topic and why it matters]

KEY ANGLES:
1. [Angle 1 description]
2. [Angle 2 description]
3. [Angle 3 description]
4. [Angle 4 description]

QUESTIONS TO CONSIDER:
- [Question 1]
- [Question 2]
- [Question 3]`, thought.Content)

	responseText, err := a.callClaude(ctx, prompt)
	if err != nil {
		return "", nil, err
	}

	brainstormContent, angles := a.parseBrainstorm(responseText)

	return brainstormContent, angles, nil
}

func (a *ContentGeneratorAgent) callClaude(ctx context.Context, prompt string) (string, error) {
	reqBody := anthropicRequest{
		Model:     "claude-sonnet-4-5-20250929",
		MaxTokens: 2000,
		Messages: []anthropicMessage{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", a.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call Anthropic API: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("Anthropic API error (status %d): %s", resp.StatusCode, string(body))
		return "", fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if apiResp.Error != nil {
		return "", fmt.Errorf("API error: %s - %s", apiResp.Error.Type, apiResp.Error.Message)
	}

	if len(apiResp.Content) > 0 && apiResp.Content[0].Type == "text" {
		return apiResp.Content[0].Text, nil
	}

	return "", fmt.Errorf("unexpected response format")
}

func (a *ContentGeneratorAgent) parseVariations(response string) []string {
	var variations []string

	parts := strings.Split(response, "===VARIATION")

	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			continue
		}

		lines := strings.Split(part, "\n")
		if len(lines) < 2 {
			continue
		}

		content := strings.Join(lines[1:], "\n")
		content = strings.TrimSpace(content)

		if content != "" {
			variations = append(variations, content)
		}
	}

	return variations
}

func (a *ContentGeneratorAgent) parseBrainstorm(response string) (string, []string) {
	var brainstormContent string
	var angles []string

	if idx := strings.Index(response, "EXPLORATION:"); idx != -1 {
		endIdx := strings.Index(response, "KEY ANGLES:")
		if endIdx == -1 {
			endIdx = len(response)
		}
		brainstormContent = strings.TrimSpace(response[idx+len("EXPLORATION:"):endIdx])
	}

	if idx := strings.Index(response, "KEY ANGLES:"); idx != -1 {
		endIdx := strings.Index(response, "QUESTIONS TO CONSIDER:")
		if endIdx == -1 {
			endIdx = len(response)
		}
		anglesSection := response[idx+len("KEY ANGLES:"):endIdx]
		lines := strings.Split(anglesSection, "\n")

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if len(line) > 3 && line[0] >= '1' && line[0] <= '9' && line[1] == '.' {
				angle := strings.TrimSpace(line[2:])
				if angle != "" {
					angles = append(angles, angle)
				}
			}
		}
	}

	return brainstormContent, angles
}