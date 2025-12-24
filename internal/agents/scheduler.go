package agents

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/shubh-37/linkedin-ghostwriter/internal/database"
	"github.com/shubh-37/linkedin-ghostwriter/internal/models"
)

type SchedulerAgent struct {
	postRepo *database.PostRepository
}

// ScheduleConfig defines scheduling preferences
type ScheduleConfig struct {
	PostsPerDay    int       // 1-4 posts per day
	PreferredTimes []string  // e.g., ["09:00", "13:00", "17:00"]
	StartDate      time.Time // When to start scheduling
	Timezone       string    // e.g., "Asia/Kolkata"
}

// NewSchedulerAgent creates a new scheduler agent
func NewSchedulerAgent(postRepo *database.PostRepository) *SchedulerAgent {
	log.Println("✅ Scheduler Agent initialized")
	return &SchedulerAgent{
		postRepo: postRepo,
	}
}

// ScheduleApprovedPosts schedules all approved posts
func (s *SchedulerAgent) ScheduleApprovedPosts(ctx context.Context, config ScheduleConfig) (int, error) {
	log.Printf("📅 Starting post scheduling (posts per day: %d)", config.PostsPerDay)

	// Get all approved posts
	approvedPosts, err := s.postRepo.GetByStatus(ctx, "approved")
	if err != nil {
		return 0, fmt.Errorf("failed to get approved posts: %w", err)
	}

	if len(approvedPosts) == 0 {
		log.Println("No approved posts to schedule")
		return 0, nil
	}

	log.Printf("Found %d approved posts to schedule", len(approvedPosts))

	// Load timezone
	location, err := time.LoadLocation(config.Timezone)
	if err != nil {
		log.Printf("⚠️ Invalid timezone %s, using UTC", config.Timezone)
		location = time.UTC
	}

	// Get default times if not provided
	if len(config.PreferredTimes) == 0 {
		config.PreferredTimes = s.getDefaultTimes(config.PostsPerDay)
	}

	// Schedule each post
	scheduledCount := 0
	currentDate := config.StartDate
	timeSlotIndex := 0

	for _, post := range approvedPosts {
		// Calculate scheduled time
		scheduledTime, err := s.calculateScheduledTime(currentDate, config.PreferredTimes[timeSlotIndex], location)
		if err != nil {
			log.Printf("⚠️ Failed to calculate time for post %s: %v", post.ID, err)
			continue
		}

		// Update post
		post.ScheduledAt = &scheduledTime
		post.Status = "scheduled"

		if err := s.postRepo.Update(ctx, post); err != nil {
			log.Printf("⚠️ Failed to schedule post %s: %v", post.ID, err)
			continue
		}

		log.Printf("✅ Scheduled post %s for %s", post.ID, scheduledTime.Format("Jan 02, 2006 at 3:04 PM"))
		scheduledCount++

		// Move to next time slot
		timeSlotIndex++
		if timeSlotIndex >= len(config.PreferredTimes) {
			timeSlotIndex = 0
			currentDate = currentDate.AddDate(0, 0, 1) // Next day
		}
	}

	log.Printf("✅ Scheduled %d posts successfully", scheduledCount)
	return scheduledCount, nil
}

// GetSchedule returns the posting schedule
func (s *SchedulerAgent) GetSchedule(ctx context.Context, days int) ([]*models.Post, error) {
	scheduledPosts, err := s.postRepo.GetByStatus(ctx, "scheduled")
	if err != nil {
		return nil, fmt.Errorf("failed to get scheduled posts: %w", err)
	}

	// Filter posts within the next N days
	cutoffDate := time.Now().AddDate(0, 0, days)
	var filteredPosts []*models.Post

	for _, post := range scheduledPosts {
		if post.ScheduledAt != nil && post.ScheduledAt.Before(cutoffDate) {
			filteredPosts = append(filteredPosts, post)
		}
	}

	return filteredPosts, nil
}

// ReschedulePost changes the scheduled time for a post
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

	log.Printf("✅ Rescheduled post %s to %s", postID, newTime.Format("Jan 02, 2006 at 3:04 PM"))
	return nil
}

// CancelSchedule cancels a scheduled post
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

	log.Printf("✅ Cancelled schedule for post %s", postID)
	return nil
}

// calculateScheduledTime creates a specific datetime from date and time string
func (s *SchedulerAgent) calculateScheduledTime(date time.Time, timeStr string, location *time.Location) (time.Time, error) {
	// Parse time string (format: "HH:MM")
	parsedTime, err := time.Parse("15:04", timeStr)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid time format: %w", err)
	}

	// Combine date with time
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

// getDefaultTimes returns optimal posting times based on posts per day
func (s *SchedulerAgent) getDefaultTimes(postsPerDay int) []string {
	switch postsPerDay {
	case 1:
		return []string{"09:00"} // Morning
	case 2:
		return []string{"09:00", "15:00"} // Morning and afternoon
	case 3:
		return []string{"09:00", "13:00", "17:00"} // Morning, lunch, evening
	case 4:
		return []string{"09:00", "12:00", "15:00", "18:00"} // Throughout the day
	default:
		return []string{"09:00", "13:00", "17:00"} // Default to 3 times
	}
}

// GetNextScheduledPost returns the next post that should be published
func (s *SchedulerAgent) GetNextScheduledPost(ctx context.Context) (*models.Post, error) {
	scheduledPosts, err := s.postRepo.GetScheduledPosts(ctx)
	if err != nil {
		return nil, err
	}

	if len(scheduledPosts) == 0 {
		return nil, nil
	}

	// Return the first one (they're ordered by scheduled_at ASC)
	return scheduledPosts[0], nil
}