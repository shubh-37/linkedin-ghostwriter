package models

import "time"

// Thought represents a captured idea from Slack or Linear
type Thought struct {
	ID              string    `json:"id" bson:"_id"`
	Source          string    `json:"source" bson:"source"`           // "slack" or "linear"
	Content         string    `json:"content" bson:"content"`
	Category        string    `json:"category" bson:"category"`       // "technical", "business", etc.
	TopicTags       []string  `json:"topic_tags" bson:"topic_tags"`
	Status          string    `json:"status" bson:"status"`           // "raw", "in_brainstorm", "used_in_draft"
	Timestamp       time.Time `json:"timestamp" bson:"timestamp"`
	RelatedThoughts []string  `json:"related_thoughts" bson:"related_thoughts"`
}

// NewThought creates a new thought with defaults
func NewThought(content, source string) *Thought {
	return &Thought{
		Content:         content,
		Source:          source,
		Status:          "raw",
		Timestamp:       time.Now(),
		TopicTags:       []string{},
		RelatedThoughts: []string{},
	}
}