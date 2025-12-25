package agents

import (
	"context"
	"fmt"
	"time"

	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type SchedulerAgent struct {
	postRepo *database.PostRepository
}

type ScheduleConfig struct {
	PostsPerDay    int
	PreferredTimes []string
	StartDate      time.Time
	Timezone       string
}

func NewSchedulerAgent(postRepo *database.PostRepository) *SchedulerAgent {
	return &SchedulerAgent{
		postRepo: postRepo,
	}
}

func (s *SchedulerAgent) ScheduleApprovedPosts(ctx context.Context, config ScheduleConfig) (int, error) {
	approvedPosts, err := s.postRepo.GetByStatus(ctx, "approved")
	if err != nil {
		return 0, fmt.Errorf("failed to get approved posts: %w", err)
	}

	if len(approvedPosts) == 0 {
		return 0, nil
	}

	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		location = time.UTC
	}

	if len(config.PreferredTimes) == 0 {
		config.PreferredTimes = s.getDefaultTimes(config.PostsPerDay)
	}

	scheduledCount := 0
	currentDate := config.StartDate
	timeSlotIndex := 0

	for _, post := range approvedPosts {
		scheduledTime, err := s.calculateScheduledTime(currentDate, config.PreferredTimes[timeSlotIndex], location)
		if err != nil {
			continue
		}

		post.ScheduledAt = &scheduledTime
		post.Status = "scheduled"

		if err := s.postRepo.Update(ctx, post); err != nil {
			continue
		}

		scheduledCount++

		timeSlotIndex++
		if timeSlotIndex >= len(config.PreferredTimes) {
			timeSlotIndex = 0
			currentDate = currentDate.AddDate(0, 0, 1)
		}
	}

	return scheduledCount, nil
}

func (s *SchedulerAgent) GetSchedule(ctx context.Context, days int) ([]*models.Post, error) {
	scheduledPosts, err := s.postRepo.GetByStatus(ctx, "scheduled")
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled posts: %w", err)
	}

	cutoffDate := time.Now().AddDate(0, 0, days)
	var filteredPosts []*models.Post

	for _, post := range scheduledPosts {
		if post.ScheduledAt != nil && post.ScheduledAt.Before(cutoffDate) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	return filteredPosts, nil
}

func (s *SchedulerAgent) ReschedulePost(ctx context.Context, postID string, newTime time.Time) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	if post.Status != "scheduled" {
		return fmt.Errorf("post is not scheduled (status: %s)", post.Status)
	}

	post.ScheduledAt = &newTime
	if err := s.postRepo.Update(ctx, post); err != nil {
		return fmt.Errorf("failed to reschedule post: %w", err)
	}

	return nil
}

func (s *SchedulerAgent) CancelSchedule(ctx context.Context, postID string) error {
	post, err := s.postRepo.GetByID(ctx, postID)
	if err != nil {
		return fmt.Errorf("failed to get post: %w", err)
	}

	post.Status = "approved"
	post.ScheduledAt = nil

	if err := s.postRepo.Update(ctx, post); err != nil {
		return fmt.Errorf("failed to cancel schedule: %w", err)
	}

	return nil
}

func (s *SchedulerAgent) calculateScheduledTime(date time.Time, timeStr string, location *time.Location) (time.Time, error) {
	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format: %w", err)
	}

	scheduledTime := time.Date(
		date.Year(),
		date.Month(),
		date.Day(),
		parsedTime.Hour(),
		parsedTime.Minute(),
		0, 0,
		location,
	)

	return scheduledTime, nil
}

func (s *SchedulerAgent) getDefaultTimes(postsPerDay int) []string {
	switch postsPerDay {
	case 1:
		return []string{"09:00"}
	case 2:
		return []string{"09:00", "15:00"}
	case 3:
		return []string{"09:00", "13:00", "17:00"}
	case 4:
		return []string{"09:00", "12:00", "15:00", "18:00"}
	default:
		return []string{"09:00", "13:00", "17:00"}
	}
}

func (s *SchedulerAgent) GetNextScheduledPost(ctx context.Context) (*models.Post, error) {
	scheduledPosts, err := s.postRepo.GetScheduledPosts(ctx)
	if err != nil {
		return nil, err
	}

	if len(scheduledPosts) == 0 {
		return nil, nil
	}

	return scheduledPosts[0], nil
}