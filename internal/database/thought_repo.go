package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type ThoughtRepository struct {
	db *DB
}

func NewThoughtRepository(db *DB) *ThoughtRepository {
	return &ThoughtRepository{db: db}
}

// Create inserts a new thought into the database
func (r *ThoughtRepository) Create(ctx context.Context, thought *models.Thought) error {
	// Generate UUID if not provided
	if thought.ID == "" {
		thought.ID = uuid.New().String()
	}

	// Set timestamp if not provided
	if thought.Timestamp.IsZero() {
		thought.Timestamp = time.Now()
	}

	query := `
		INSERT INTO thoughts (id, source, content, category, topic_tags, status, timestamp, related_thoughts)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		thought.ID,
		thought.Source,
		thought.Content,
		thought.Category,
		thought.TopicTags,
		thought.Status,
		thought.Timestamp,
		thought.RelatedThoughts,
	)

	if err != nil {
		return fmt.Errorf("failed to create thought: %w", err)
	}

	return nil
}

// GetByID retrieves a thought by its ID
func (r *ThoughtRepository) GetByID(ctx context.Context, id string) (*models.Thought, error) {
	query := `
		SELECT id, source, content, category, topic_tags, status, timestamp, related_thoughts
		FROM thoughts
		WHERE id = $1
	`

	thought := &models.Thought{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&thought.ID,
		&thought.Source,
		&thought.Content,
		&thought.Category,
		&thought.TopicTags,
		&thought.Status,
		&thought.Timestamp,
		&thought.RelatedThoughts,
	)

	if err != nil {
		return nil, fmt.Errorf("thought not found: %w", err)
	}

	return thought, nil
}

// GetAll retrieves all thoughts
func (r *ThoughtRepository) GetAll(ctx context.Context) ([]*models.Thought, error) {
	query := `
		SELECT id, source, content, category, topic_tags, status, timestamp, related_thoughts
		FROM thoughts
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query thoughts: %w", err)
	}
	defer rows.Close()

	var thoughts []*models.Thought
	for rows.Next() {
		thought := &models.Thought{}
		err := rows.Scan(
			&thought.ID,
			&thought.Source,
			&thought.Content,
			&thought.Category,
			&thought.TopicTags,
			&thought.Status,
			&thought.Timestamp,
			&thought.RelatedThoughts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thought: %w", err)
		}
		thoughts = append(thoughts, thought)
	}

	return thoughts, nil
}

// GetByStatus retrieves thoughts by status
func (r *ThoughtRepository) GetByStatus(ctx context.Context, status string) ([]*models.Thought, error) {
	query := `
		SELECT id, source, content, category, topic_tags, status, timestamp, related_thoughts
		FROM thoughts
		WHERE status = $1
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query thoughts: %w", err)
	}
	defer rows.Close()

	var thoughts []*models.Thought
	for rows.Next() {
		thought := &models.Thought{}
		err := rows.Scan(
			&thought.ID,
			&thought.Source,
			&thought.Content,
			&thought.Category,
			&thought.TopicTags,
			&thought.Status,
			&thought.Timestamp,
			&thought.RelatedThoughts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thought: %w", err)
		}
		thoughts = append(thoughts, thought)
	}

	return thoughts, nil
}

// GetByCategory retrieves thoughts by category
func (r *ThoughtRepository) GetByCategory(ctx context.Context, category string) ([]*models.Thought, error) {
	query := `
		SELECT id, source, content, category, topic_tags, status, timestamp, related_thoughts
		FROM thoughts
		WHERE category = $1
		ORDER BY timestamp DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query thoughts: %w", err)
	}
	defer rows.Close()

	var thoughts []*models.Thought
	for rows.Next() {
		thought := &models.Thought{}
		err := rows.Scan(
			&thought.ID,
			&thought.Source,
			&thought.Content,
			&thought.Category,
			&thought.TopicTags,
			&thought.Status,
			&thought.Timestamp,
			&thought.RelatedThoughts,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thought: %w", err)
		}
		thoughts = append(thoughts, thought)
	}

	return thoughts, nil
}

// Update updates a thought
func (r *ThoughtRepository) Update(ctx context.Context, thought *models.Thought) error {
	query := `
		UPDATE thoughts
		SET source = $2, content = $3, category = $4, topic_tags = $5, 
		    status = $6, related_thoughts = $7
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		thought.ID,
		thought.Source,
		thought.Content,
		thought.Category,
		thought.TopicTags,
		thought.Status,
		thought.RelatedThoughts,
	)

	if err != nil {
		return fmt.Errorf("failed to update thought: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("thought not found")
	}

	return nil
}

// UpdateStatus updates only the status of a thought
func (r *ThoughtRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE thoughts SET status = $2 WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update thought status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("thought not found")
	}

	return nil
}

// Delete deletes a thought by ID
func (r *ThoughtRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM thoughts WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete thought: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("thought not found")
	}

	return nil
}

// Count returns total number of thoughts
func (r *ThoughtRepository) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM thoughts`

	err := r.db.Pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to count thoughts: %w", err)
	}

	return count, nil
}