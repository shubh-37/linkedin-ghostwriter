package models

import "time"

type Thought struct {
	ID              string    `json:"id" bson:"_id"`
	Source          string    `json:"source" bson:"source"`
	Content         string    `json:"content" bson:"content"`
	Category        string    `json:"category" bson:"category"`
	TopicTags       []string  `json:"topic_tags" bson:"topic_tags"`
	Status          string    `json:"status" bson:"status"`
	Timestamp       time.Time `json:"timestamp" bson:"timestamp"`
	RelatedThoughts []string  `json:"related_thoughts" bson:"related_thoughts"`
}

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