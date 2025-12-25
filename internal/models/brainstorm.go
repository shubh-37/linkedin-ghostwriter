package models

import "time"

type BrainstormSession struct {
	ID                string    `json:"id" bson:"_id"`
	Topic             string    `json:"topic" bson:"topic"`
	ThoughtIDs        []string  `json:"thought_ids" bson:"thought_ids"`
	BrainstormContent string    `json:"brainstorm_content" bson:"brainstorm_content"`
	KeyAngles         []string  `json:"key_angles" bson:"key_angles"`
	Status            string    `json:"status" bson:"status"`
	CreatedAt         time.Time `json:"created_at" bson:"created_at"`
}

func NewBrainstormSession(topic string, thoughtIDs []string) *BrainstormSession {
	return &BrainstormSession{
		Topic:      topic,
		ThoughtIDs: thoughtIDs,
		Status:     "in_progress",
		CreatedAt:  time.Now(),
		KeyAngles:  []string{},
	}
}