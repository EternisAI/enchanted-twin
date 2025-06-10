package holon

import (
	"context"
	"fmt"
)

// DataFetcher provides methods to fetch data for activities.
type DataFetcher struct {
	holonRepo *Repository
}

// NewDataFetcher creates a new DataFetcher instance.
func NewDataFetcher(repo *Repository) *DataFetcher {
	return &DataFetcher{
		holonRepo: repo,
	}
}

// ThreadReply represents a thread reply with additional metadata needed for pushing.
type ThreadReply struct {
	ID             string
	ThreadID       string // The thread this reply belongs to
	Content        string
	AuthorIdentity string
	CreatedAt      string
	Actions        []string
}

// GetPendingReplies returns pending thread messages (replies) with thread ID information.
func (f *DataFetcher) GetPendingReplies(ctx context.Context) ([]*ThreadReply, error) {
	// Get pending thread messages from database with thread ID info
	// We need to query the database directly to get the thread_id field
	query := `
		SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, tm.actions
		FROM thread_messages tm
		WHERE tm.state = 'pending'
		ORDER BY tm.created_at ASC
	`

	rows, err := f.holonRepo.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending thread messages: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	var replies []*ThreadReply
	for rows.Next() {
		var reply ThreadReply
		var actionsJSON string

		err := rows.Scan(
			&reply.ID, &reply.ThreadID, &reply.AuthorIdentity, &reply.Content,
			&reply.CreatedAt, &actionsJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending thread message row: %w", err)
		}

		// Parse actions JSON - simplified for now, just set empty slice
		reply.Actions = []string{}

		replies = append(replies, &reply)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending thread messages: %w", err)
	}

	return replies, nil
}
