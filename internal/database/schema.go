package database

import (
	"context"
	"log"
)

// CreateTables creates all necessary database tables
func (db *DB) CreateTables(ctx context.Context) error {
	log.Println("Creating database tables...")

	// Thoughts table
	thoughtsTable := `
	CREATE TABLE IF NOT EXISTS thoughts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		source VARCHAR(50) NOT NULL,
		content TEXT NOT NULL,
		category VARCHAR(100),
		topic_tags TEXT[],
		status VARCHAR(50) DEFAULT 'raw',
		timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		related_thoughts UUID[],
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_thoughts_status ON thoughts(status);
	CREATE INDEX IF NOT EXISTS idx_thoughts_category ON thoughts(category);
	CREATE INDEX IF NOT EXISTS idx_thoughts_timestamp ON thoughts(timestamp DESC);
	`

	// Brainstorm sessions table
	brainstormTable := `
	CREATE TABLE IF NOT EXISTS brainstorm_sessions (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		topic VARCHAR(255) NOT NULL,
		thought_ids UUID[],
		brainstorm_content TEXT,
		key_angles TEXT[],
		status VARCHAR(50) DEFAULT 'in_progress',
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_brainstorm_status ON brainstorm_sessions(status);
	`

	// Posts table
	postsTable := `
	CREATE TABLE IF NOT EXISTS posts (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		content TEXT NOT NULL,
		status VARCHAR(50) DEFAULT 'draft',
		source_thought_ids UUID[],
		brainstorm_session_id UUID,
		post_type VARCHAR(50),
		tone VARCHAR(50),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		scheduled_at TIMESTAMP,
		published_at TIMESTAMP,
		metrics JSONB DEFAULT '{"likes": 0, "comments": 0, "shares": 0, "views": 0}',
		performance_score DECIMAL(5,2) DEFAULT 0.0,
		FOREIGN KEY (brainstorm_session_id) REFERENCES brainstorm_sessions(id) ON DELETE SET NULL
	);
	CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
	CREATE INDEX IF NOT EXISTS idx_posts_scheduled ON posts(scheduled_at);
	CREATE INDEX IF NOT EXISTS idx_posts_published ON posts(published_at DESC);
	`

	// Writing style profile table
	styleTable := `
	CREATE TABLE IF NOT EXISTS writing_style_profile (
		id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		user_id VARCHAR(100) NOT NULL UNIQUE,
		style_patterns JSONB,
		tone_preferences JSONB,
		high_performing_elements JSONB,
		sample_posts JSONB,
		last_updated TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	// Execute all table creations
	tables := []string{thoughtsTable, brainstormTable, postsTable, styleTable}
	
	for _, table := range tables {
		if _, err := db.Pool.Exec(ctx, table); err != nil {
			return err
		}
	}

	log.Println("✅ All tables created successfully")
	return nil
}