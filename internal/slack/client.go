package slack

import (
	"log"

	"github.com/slack-go/slack"
)

type Client struct {
	api    *slack.Client
	botID  string
}

func NewClient(token string) *Client {
	api := slack.New(token)
	
	authTest, err := api.AuthTest()
	if err != nil {
		log.Fatalf("Failed to authenticate with Slack: %v", err)
	}
	
	return &Client{
		api:   api,
		botID: authTest.UserID,
	}
}

func (c *Client) GetAPI() *slack.Client {
	return c.api
}

func (c *Client) GetBotID() string {
	return c.botID
}

func (c *Client) SendMessage(channelID, message string) error {
	_, _, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionText(message, false),
	)
	return err
}

func (c *Client) SendMessageWithBlocks(channelID string, blocks []slack.Block) error {
	_, _, err := c.api.PostMessage(
		channelID,
		slack.MsgOptionBlocks(blocks...),
	)
	return err
}

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