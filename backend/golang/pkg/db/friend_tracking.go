// Owner: slimane@eternis.ai
package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type FriendActivityTracking struct {
	ID           string    `db:"id"`
	ChatID       string    `db:"chat_id"`
	ActivityType string    `db:"activity_type"`
	Timestamp    time.Time `db:"timestamp"`
	CreatedAt    time.Time `db:"created_at"`
}

func (s *Store) StoreFriendActivity(ctx context.Context, chatID, activityType string, timestamp time.Time) error {
	id := uuid.New().String()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO friend_activity_tracking (id, chat_id, activity_type, timestamp, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, id, chatID, activityType, timestamp, time.Now())
	return err
}

func (s *Store) GetFriendActivitiesByChatID(ctx context.Context, chatID string) ([]FriendActivityTracking, error) {
	var activities []FriendActivityTracking
	err := s.db.SelectContext(ctx, &activities, `
		SELECT id, chat_id, activity_type, timestamp, created_at
		FROM friend_activity_tracking
		WHERE chat_id = ?
		ORDER BY timestamp DESC
	`, chatID)
	return activities, err
}

func (s *Store) GetRecentFriendActivities(ctx context.Context, limit int) ([]FriendActivityTracking, error) {
	var activities []FriendActivityTracking
	err := s.db.SelectContext(ctx, &activities, `
		SELECT id, chat_id, activity_type, timestamp, created_at
		FROM friend_activity_tracking
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit)
	return activities, err
}
