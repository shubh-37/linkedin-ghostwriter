package slack

import (
	"log"

	"github.com/slack-go/slack"
)

type Client struct {
	api    *slack.Client
	botID  string
}

// NewClient creates a new Slack client
func NewClient(token string) *Client {
	api := slack.New(token)
	
	// Get bot user info
	authTest, err := api.AuthTest()
	if err != nil {
		log.Fatalf("Failed to authenticate with Slack: %v", err)
	}
	
	log.Printf("✅ Connected to Slack as: %s (ID: %s)", authTest.User, authTest.UserID)
	
	return &Client{
		api:   api,
		botID: authTest.UserID,
	}
}

// GetAPI returns the underlying Slack API client
func (c *Client) GetAPI() *slack.Client {
	return c.api
}

// GetBotID returns the bot's user ID
func (c *Client) GetBotID() string {
	return c.botID
}

// SendMessage sends a message to a channel
func (c *Client) SendMessage(channelID, message string) error {
	_, _, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	return err
}

// SendMessageWithBlocks sends a message with blocks (rich formatting)
func (c *Client) SendMessageWithBlocks(channelID string, blocks []slack.Block) error {
	_, _, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks...),
	)
	return err
}

// GetChannelHistory retrieves recent messages from a channel
func (c *Client) GetChannelHistory(channelID string, limit int) ([]slack.Message, error) {
	params := &slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Limit:     limit,
	}
	
	history, err := c.api.GetConversationHistory(params)
	if err != nil {
		return nil, err
	}
	
	return history.Messages, nil
}