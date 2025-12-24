package database

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type PostRepository struct {
	db *DB
}

func NewPostRepository(db *DB) *PostRepository {
	return &PostRepository{db: db}
}

// Create inserts a new post into the database
func (r *PostRepository) Create(ctx context.Context, post *models.Post) error {
	if post.ID == "" {
		post.ID = uuid.New().String()
	}

	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now()
	}

	// Convert metrics map to JSON
	metricsJSON, err := json.Marshal(post.Metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	query := `
		INSERT INTO posts (id, content, status, source_thought_ids, brainstorm_session_id, 
		                   post_type, tone, created_at, scheduled_at, published_at, 
		                   metrics, performance_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`

	_, err = r.db.Pool.Exec(ctx, query,
		post.ID,
		post.Content,
		post.Status,
		post.SourceThoughtIDs,
		post.BrainstormSessionID,
		post.PostType,
		post.Tone,
		post.CreatedAt,
		post.ScheduledAt,
		post.PublishedAt,
		metricsJSON,
		post.PerformanceScore,
	)

	if err != nil {
		return fmt.Errorf("failed to create post: %w", err)
	}

	return nil
}

// GetByID retrieves a post by its ID
func (r *PostRepository) GetByID(ctx context.Context, id string) (*models.Post, error) {
	query := `
		SELECT id, content, status, source_thought_ids, brainstorm_session_id, 
		       post_type, tone, created_at, scheduled_at, published_at, 
		       metrics, performance_score
		FROM posts
		WHERE id = $1
	`

	post := &models.Post{}
	var metricsJSON []byte

	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&post.ID,
		&post.Content,
		&post.Status,
		&post.SourceThoughtIDs,
		&post.BrainstormSessionID,
		&post.PostType,
		&post.Tone,
		&post.CreatedAt,
		&post.ScheduledAt,
		&post.PublishedAt,
		&metricsJSON,
		&post.PerformanceScore,
	)

	if err != nil {
		return nil, fmt.Errorf("post not found: %w", err)
	}

	// Unmarshal metrics
	if err := json.Unmarshal(metricsJSON, &post.Metrics); err != nil {
		return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
	}

	return post, nil
}

// GetByStatus retrieves posts by status
func (r *PostRepository) GetByStatus(ctx context.Context, status string) ([]*models.Post, error) {
	query := `
		SELECT id, content, status, source_thought_ids, brainstorm_session_id, 
		       post_type, tone, created_at, scheduled_at, published_at, 
		       metrics, performance_score
		FROM posts
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query posts: %w", err)
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		post := &models.Post{}
		var metricsJSON []byte

		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.Status,
			&post.SourceThoughtIDs,
			&post.BrainstormSessionID,
			&post.PostType,
			&post.Tone,
			&post.CreatedAt,
			&post.ScheduledAt,
			&post.PublishedAt,
			&metricsJSON,
			&post.PerformanceScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		// Unmarshal metrics
		if err := json.Unmarshal(metricsJSON, &post.Metrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// GetScheduledPosts retrieves posts that are scheduled but not yet published
func (r *PostRepository) GetScheduledPosts(ctx context.Context) ([]*models.Post, error) {
	query := `
		SELECT id, content, status, source_thought_ids, brainstorm_session_id, 
		       post_type, tone, created_at, scheduled_at, published_at, 
		       metrics, performance_score
		FROM posts
		WHERE status = 'scheduled' AND scheduled_at <= $1
		ORDER BY scheduled_at ASC
	`

	rows, err := r.db.Pool.Query(ctx, query, time.Now())
	if err != nil {
		return nil, fmt.Errorf("failed to query scheduled posts: %w", err)
	}
	defer rows.Close()

	var posts []*models.Post
	for rows.Next() {
		post := &models.Post{}
		var metricsJSON []byte

		err := rows.Scan(
			&post.ID,
			&post.Content,
			&post.Status,
			&post.SourceThoughtIDs,
			&post.BrainstormSessionID,
			&post.PostType,
			&post.Tone,
			&post.CreatedAt,
			&post.ScheduledAt,
			&post.PublishedAt,
			&metricsJSON,
			&post.PerformanceScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan post: %w", err)
		}

		if err := json.Unmarshal(metricsJSON, &post.Metrics); err != nil {
			return nil, fmt.Errorf("failed to unmarshal metrics: %w", err)
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// Update updates a post
func (r *PostRepository) Update(ctx context.Context, post *models.Post) error {
	metricsJSON, err := json.Marshal(post.Metrics)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	query := `
		UPDATE posts
		SET content = $2, status = $3, source_thought_ids = $4, brainstorm_session_id = $5,
		    post_type = $6, tone = $7, scheduled_at = $8, published_at = $9,
		    metrics = $10, performance_score = $11
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		post.ID,
		post.Content,
		post.Status,
		post.SourceThoughtIDs,
		post.BrainstormSessionID,
		post.PostType,
		post.Tone,
		post.ScheduledAt,
		post.PublishedAt,
		metricsJSON,
		post.PerformanceScore,
	)

	if err != nil {
		return fmt.Errorf("failed to update post: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

// UpdateStatus updates only the status of a post
func (r *PostRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `UPDATE posts SET status = $2 WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id, status)
	if err != nil {
		return fmt.Errorf("failed to update post status: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}

// Delete deletes a post by ID
func (r *PostRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM posts WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete post: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("post not found")
	}

	return nil
}