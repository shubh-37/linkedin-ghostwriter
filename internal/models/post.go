package models

import "time"

type Post struct {
	ID                  string            `json:"id" bson:"_id"`
	Content             string            `json:"content" bson:"content"`
	Status              string            `json:"status" bson:"status"`
	SourceThoughtIDs    []string          `json:"source_thought_ids" bson:"source_thought_ids"`
	BrainstormSessionID *string           `json:"brainstorm_session_id,omitempty" bson:"brainstorm_session_id,omitempty"`
	PostType            string            `json:"post_type" bson:"post_type"`
	Tone                string            `json:"tone" bson:"tone"`
	CreatedAt           time.Time         `json:"created_at" bson:"created_at"`
	ScheduledAt         *time.Time        `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`
	PublishedAt         *time.Time        `json:"published_at,omitempty" bson:"published_at,omitempty"`
	Metrics             map[string]int    `json:"metrics" bson:"metrics"`
	PerformanceScore    float64           `json:"performance_score" bson:"performance_score"`
}

func NewPost(content string, thoughtIDs []string, postType, tone string) *Post {
	return &Post{
		Content:          content,
		Status:           "draft",
		SourceThoughtIDs: thoughtIDs,
		PostType:         postType,
		Tone:             tone,
		CreatedAt:        time.Now(),
		Metrics: map[string]int{
			"likes":    0,
			"comments": 0,
			"shares":   0,
			"views":    0,
		},
		PerformanceScore: 0.0,
	}
}