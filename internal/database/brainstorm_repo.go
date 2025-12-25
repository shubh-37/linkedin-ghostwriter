package database

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type BrainstormRepository struct {
	db *DB
}

func NewBrainstormRepository(db *DB) *BrainstormRepository {
	return &BrainstormRepository{db: db}
}

func (r *BrainstormRepository) Create(ctx context.Context, session *models.BrainstormSession) error {
	if session.ID == "" {
		session.ID = uuid.New().String()
	}

	if session.CreatedAt.IsZero() {
		session.CreatedAt = time.Now()
	}

	query := `
		INSERT INTO brainstorm_sessions (id, topic, thought_ids, brainstorm_content, 
		                                 key_angles, status, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.Pool.Exec(ctx, query,
		session.ID,
		session.Topic,
		session.ThoughtIDs,
		session.BrainstormContent,
		session.KeyAngles,
		session.Status,
		session.CreatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create brainstorm session: %w", err)
	}

	return nil
}

func (r *BrainstormRepository) GetByID(ctx context.Context, id string) (*models.BrainstormSession, error) {
	query := `
		SELECT id, topic, thought_ids, brainstorm_content, key_angles, status, created_at
		FROM brainstorm_sessions
		WHERE id = $1
	`

	session := &models.BrainstormSession{}
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&session.ID,
		&session.Topic,
		&session.ThoughtIDs,
		&session.BrainstormContent,
		&session.KeyAngles,
		&session.Status,
		&session.CreatedAt,
	)

	if err != nil {
		return nil, fmt.Errorf("brainstorm session not found: %w", err)
	}

	return session, nil
}

func (r *BrainstormRepository) GetByStatus(ctx context.Context, status string) ([]*models.BrainstormSession, error) {
	query := `
		SELECT id, topic, thought_ids, brainstorm_content, key_angles, status, created_at
		FROM brainstorm_sessions
		WHERE status = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.Pool.Query(ctx, query, status)
	if err != nil {
		return nil, fmt.Errorf("failed to query brainstorm sessions: %w", err)
	}
	defer rows.Close()

	var sessions []*models.BrainstormSession
	for rows.Next() {
		session := &models.BrainstormSession{}
		err := rows.Scan(
			&session.ID,
			&session.Topic,
			&session.ThoughtIDs,
			&session.BrainstormContent,
			&session.KeyAngles,
			&session.Status,
			&session.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}
		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *BrainstormRepository) Update(ctx context.Context, session *models.BrainstormSession) error {
	query := `
		UPDATE brainstorm_sessions
		SET topic = $2, thought_ids = $3, brainstorm_content = $4, 
		    key_angles = $5, status = $6
		WHERE id = $1
	`

	result, err := r.db.Pool.Exec(ctx, query,
		session.ID,
		session.Topic,
		session.ThoughtIDs,
		session.BrainstormContent,
		session.KeyAngles,
		session.Status,
	)

	if err != nil {
		return fmt.Errorf("failed to update brainstorm session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("brainstorm session not found")
	}

	return nil
}

func (r *BrainstormRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM brainstorm_sessions WHERE id = $1`

	result, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete brainstorm session: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("brainstorm session not found")
	}

	return nil
}