package models

import "time"

// Post represents a LinkedIn post in any stage
type Post struct {
	ID                  string            `json:"id" bson:"_id"`
	Content             string            `json:"content" bson:"content"`
	Status              string            `json:"status" bson:"status"` // "draft", "scheduled", "published"
	SourceThoughtIDs    []string          `json:"source_thought_ids" bson:"source_thought_ids"`
	BrainstormSessionID *string           `json:"brainstorm_session_id,omitempty" bson:"brainstorm_session_id,omitempty"`
	PostType            string            `json:"post_type" bson:"post_type"` // "story", "insight", "update", "how-to"
	Tone                string            `json:"tone" bson:"tone"`           // "professional", "casual", "technical"
	CreatedAt           time.Time         `json:"created_at" bson:"created_at"`
	ScheduledAt         *time.Time        `json:"scheduled_at,omitempty" bson:"scheduled_at,omitempty"`
	PublishedAt         *time.Time        `json:"published_at,omitempty" bson:"published_at,omitempty"`
	Metrics             map[string]int    `json:"metrics" bson:"metrics"`
	PerformanceScore    float64           `json:"performance_score" bson:"performance_score"`
}

// NewPost creates a new post draft
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


// docker run -d --name linkedin-ghostwriter-db -e POSTGRES_USER=ghostwriter_user -e POSTGRES_PASSWORD=ghostwriter_pass_2024 -e POSTGRES_DB=linkedin_ghostwriter -p 5432:5432 -v postgres_data:/var/lib/postgresql/data postgres:15-alpine