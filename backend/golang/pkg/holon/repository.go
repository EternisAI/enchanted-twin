package holon

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/EternisAI/enchanted-twin/graph/model"
)

type Repository struct {
	db *sqlx.DB
}

func NewRepository(db *sqlx.DB) *Repository {
	return &Repository{
		db: db,
	}
}

type dbThread struct {
	ID             string  `db:"id"`
	Title          string  `db:"title"`
	Content        string  `db:"content"`
	AuthorIdentity string  `db:"author_identity"`
	CreatedAt      string  `db:"created_at"`
	ExpiresAt      *string `db:"expires_at"`
	ImageURLs      string  `db:"image_urls"`
	Actions        string  `db:"actions"`
	Views          int32   `db:"views"`
	State          string  `db:"state"`
	RemoteThreadID *int32  `db:"remote_thread_id"`
}

type dbThreadMessage struct {
	ID             string `db:"id"`
	ThreadID       string `db:"thread_id"`
	AuthorIdentity string `db:"author_identity"`
	Content        string `db:"content"`
	CreatedAt      string `db:"created_at"`
	IsDelivered    bool   `db:"is_delivered"`
	Actions        string `db:"actions"`
	State          string `db:"state"`
}

type dbAuthor struct {
	Identity string  `db:"identity"`
	Alias    *string `db:"alias"`
}

func (r *Repository) GetThreads(ctx context.Context, first int32, offset int32) ([]*model.Thread, error) {
	query := `
		SELECT t.id, t.title, t.content, t.author_identity, t.created_at, t.expires_at, 
		       t.image_urls, t.actions, t.views, t.state, t.remote_thread_id,
		       a.identity, a.alias
		FROM threads t
		JOIN authors a ON t.author_identity = a.identity
		ORDER BY t.created_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.QueryContext(ctx, query, first, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to query threads: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			_ = cerr
		}
	}()

	var threads []*model.Thread
	for rows.Next() {
		var thread dbThread
		var threadAuthor dbAuthor

		err := rows.Scan(
			&thread.ID, &thread.Title, &thread.Content, &thread.AuthorIdentity,
			&thread.CreatedAt, &thread.ExpiresAt, &thread.ImageURLs, &thread.Actions,
			&thread.Views, &thread.State, &thread.RemoteThreadID, &threadAuthor.Identity, &threadAuthor.Alias,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread row: %w", err)
		}

		threadModel, err := r.dbThreadToModel(ctx, &thread, &threadAuthor)
		if err != nil {
			return nil, fmt.Errorf("failed to convert thread to model: %w", err)
		}

		threads = append(threads, threadModel)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating threads: %w", err)
	}

	return threads, nil
}

func (r *Repository) GetThread(ctx context.Context, threadID string) (*model.Thread, error) {
	query := `
		SELECT t.id, t.title, t.content, t.author_identity, t.created_at, t.expires_at, 
		       t.image_urls, t.actions, t.views, t.state, t.remote_thread_id,
		       a.identity, a.alias
		FROM threads t
		JOIN authors a ON t.author_identity = a.identity
		WHERE t.id = ?
	`

	var thread dbThread
	var threadAuthor dbAuthor

	err := r.db.QueryRowContext(ctx, query, threadID).Scan(
		&thread.ID, &thread.Title, &thread.Content, &thread.AuthorIdentity,
		&thread.CreatedAt, &thread.ExpiresAt, &thread.ImageURLs, &thread.Actions,
		&thread.Views, &thread.State, &thread.RemoteThreadID, &threadAuthor.Identity, &threadAuthor.Alias,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("thread not found")
		}
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}

	threadModel, err := r.dbThreadToModel(ctx, &thread, &threadAuthor)
	if err != nil {
		return nil, fmt.Errorf("failed to convert thread to model: %w", err)
	}

	return threadModel, nil
}

func (r *Repository) CreateThread(ctx context.Context, id, title, content string, authorIdentity string, imageURLs []string, actions []string, expiresAt *string, state string, createdAt string) (*model.Thread, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Ensure author exists (insert if not exists)
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO authors (identity, alias) 
		VALUES (?, ?)
	`, authorIdentity, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure author exists: %w", err)
	}

	imageURLsJSON, err := json.Marshal(imageURLs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal image URLs: %w", err)
	}

	actionsJSON, err := json.Marshal(actions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions: %w", err)
	}

	// Use provided createdAt timestamp
	timestamp := createdAt
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO threads (id, title, content, author_identity, created_at, expires_at, image_urls, actions, views, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, 0, ?)
	`, id, title, content, authorIdentity, timestamp, expiresAt, string(imageURLsJSON), string(actionsJSON), state)
	if err != nil {
		return nil, fmt.Errorf("failed to insert thread: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx = nil

	return r.GetThread(ctx, id)
}

func (r *Repository) GetThreadMessages(ctx context.Context, threadID string) ([]*model.ThreadMessage, error) {
	query := `
		SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
		       tm.is_delivered, tm.actions, tm.state,
		       a.identity, a.alias
		FROM thread_messages tm
		JOIN authors a ON tm.author_identity = a.identity
		WHERE tm.thread_id = ?
		ORDER BY tm.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to query thread messages: %w", err)
	}
	defer func() {
		if cerr := rows.Close(); cerr != nil {
			// Ignore error on close during defer
			_ = cerr
		}
	}()

	var messages []*model.ThreadMessage
	for rows.Next() {
		var dbMessage dbThreadMessage
		var messageAuthor dbAuthor

		err := rows.Scan(
			&dbMessage.ID, &dbMessage.ThreadID, &dbMessage.AuthorIdentity, &dbMessage.Content,
			&dbMessage.CreatedAt, &dbMessage.IsDelivered, &dbMessage.Actions, &dbMessage.State,
			&messageAuthor.Identity, &messageAuthor.Alias,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread message row: %w", err)
		}

		message, err := r.dbThreadMessageToModel(&dbMessage, &messageAuthor)
		if err != nil {
			return nil, fmt.Errorf("failed to convert thread message to model: %w", err)
		}

		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating thread messages: %w", err)
	}

	return messages, nil
}

func (r *Repository) CreateThreadMessage(ctx context.Context, id, threadID, authorIdentity, content string, actions []string, isDelivered *bool) (*model.ThreadMessage, error) {
	return r.CreateThreadMessageWithState(ctx, id, threadID, authorIdentity, content, actions, isDelivered, "pending", "")
}

func (r *Repository) CreateThreadMessageWithState(ctx context.Context, id, threadID, authorIdentity, content string, actions []string, isDelivered *bool, state string, createdAt string) (*model.ThreadMessage, error) {
	// Validate inputs
	if authorIdentity == "" {
		return nil, fmt.Errorf("author identity cannot be empty for thread message")
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	// Ensure author exists (insert if not exists)
	_, err = tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO authors (identity, alias) 
		VALUES (?, ?)
	`, authorIdentity, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure author exists: %w", err)
	}

	actionsJSON, err := json.Marshal(actions)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal actions: %w", err)
	}

	// Use provided createdAt timestamp
	timestamp := createdAt
	if timestamp == "" {
		timestamp = time.Now().Format(time.RFC3339)
	}

	delivered := false
	if isDelivered != nil {
		delivered = *isDelivered
	}

	_, err = tx.ExecContext(ctx, `
		INSERT INTO thread_messages (id, thread_id, author_identity, content, created_at, is_delivered, actions, state)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, id, threadID, authorIdentity, content, timestamp, delivered, string(actionsJSON), state)
	if err != nil {
		return nil, fmt.Errorf("failed to insert thread message: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}
	tx = nil

	var dbMessage dbThreadMessage
	var author dbAuthor

	query := `
		SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
		       tm.is_delivered, tm.actions, tm.state,
		       a.identity, a.alias
		FROM thread_messages tm
		JOIN authors a ON tm.author_identity = a.identity
		WHERE tm.id = ?
	`

	err = r.db.QueryRowContext(ctx, query, id).Scan(
		&dbMessage.ID, &dbMessage.ThreadID, &dbMessage.AuthorIdentity, &dbMessage.Content,
		&dbMessage.CreatedAt, &dbMessage.IsDelivered, &dbMessage.Actions, &dbMessage.State,
		&author.Identity, &author.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get created thread message: %w", err)
	}

	return r.dbThreadMessageToModel(&dbMessage, &author)
}

func (r *Repository) GetHolons(ctx context.Context, userID string) ([]string, error) {
	query := `
		SELECT h.name 
		FROM holons h
		INNER JOIN holon_participants hp ON h.id = hp.holon_id
		WHERE hp.author_identity = ?
		ORDER BY h.name
	`

	var holonNames []string
	err := r.db.SelectContext(ctx, &holonNames, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user's holons: %w", err)
	}

	return holonNames, nil
}

func (r *Repository) IncrementThreadViews(ctx context.Context, threadID string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE threads SET views = views + 1 WHERE id = ?", threadID)
	if err != nil {
		return fmt.Errorf("failed to increment thread views: %w", err)
	}
	return nil
}

func (r *Repository) CreateOrUpdateAuthor(ctx context.Context, identity, alias string) (*model.Author, error) {
	// Validate inputs before proceeding
	if identity == "" {
		return nil, fmt.Errorf("author identity cannot be empty")
	}

	// Use UPSERT (INSERT ... ON CONFLICT DO UPDATE) instead of INSERT OR REPLACE
	// This avoids the DELETE operation that triggers the foreign key SET NULL
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO authors (identity, alias) 
		VALUES (?, ?)
		ON CONFLICT(identity) DO UPDATE SET alias = excluded.alias
	`, identity, alias)
	if err != nil {
		return nil, fmt.Errorf("failed to create/update author: %w", err)
	}

	// Return the created/updated author
	var authorRecord dbAuthor
	err = r.db.QueryRowContext(ctx, "SELECT identity, alias FROM authors WHERE identity = ?", identity).Scan(
		&authorRecord.Identity, &authorRecord.Alias,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get created/updated author: %w", err)
	}

	return &model.Author{
		Identity: authorRecord.Identity,
		Alias:    authorRecord.Alias,
	}, nil
}

func (r *Repository) IsUserInHolon(ctx context.Context, userID, networkIdentifier string) (bool, error) {
	var holonID string
	err := r.db.QueryRowContext(ctx, "SELECT id FROM holons WHERE id = ? OR name = ?", networkIdentifier, networkIdentifier).Scan(&holonID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, fmt.Errorf("holon network '%s' not found", networkIdentifier)
		}
		return false, fmt.Errorf("failed to find holon network: %w", err)
	}

	var exists bool
	err = r.db.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM holon_participants WHERE holon_id = ? AND author_identity = ?)
	`, holonID, userID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check user holon membership: %w", err)
	}

	return exists, nil
}

func (r *Repository) AddUserToHolon(ctx context.Context, userID, networkIdentifier string) error {
	var holonID string
	err := r.db.QueryRowContext(ctx, "SELECT id FROM holons WHERE id = ? OR name = ?", networkIdentifier, networkIdentifier).Scan(&holonID)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("holon network '%s' not found", networkIdentifier)
		}
		return fmt.Errorf("failed to find holon network: %w", err)
	}

	_, err = r.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO holon_participants (holon_id, author_identity) 
		VALUES (?, ?)
	`, holonID, userID)
	if err != nil {
		return fmt.Errorf("failed to add user to holon: %w", err)
	}

	return nil
}

// GetThreadCount returns the total number of threads in the repository
func (r *Repository) GetThreadCount(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM threads
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get thread count: %w", err)
	}
	return count, nil
}

// GetPendingThreads returns all threads with state 'pending'
func (r *Repository) GetPendingThreads(ctx context.Context) ([]*model.Thread, error) {
	query := `
		SELECT t.id, t.title, t.content, t.author_identity, t.created_at, t.expires_at, 
		       t.image_urls, t.actions, t.views, t.state, t.remote_thread_id,
		       a.identity, a.alias
		FROM threads t
		JOIN authors a ON t.author_identity = a.identity
		WHERE t.state = 'pending'
		ORDER BY t.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending threads: %w", err)
	}
	defer rows.Close()

	var threads []*model.Thread
	for rows.Next() {
		var dbThread dbThread
		var author dbAuthor

		err := rows.Scan(
			&dbThread.ID, &dbThread.Title, &dbThread.Content, &dbThread.AuthorIdentity,
			&dbThread.CreatedAt, &dbThread.ExpiresAt, &dbThread.ImageURLs, &dbThread.Actions,
			&dbThread.Views, &dbThread.State, &dbThread.RemoteThreadID, &author.Identity, &author.Alias,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending thread row: %w", err)
		}

		thread, err := r.dbThreadToModel(ctx, &dbThread, &author)
		if err != nil {
			return nil, fmt.Errorf("failed to convert pending thread to model: %w", err)
		}

		threads = append(threads, thread)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending threads: %w", err)
	}

	return threads, nil
}

// UpdateThreadState updates the state of a thread
func (r *Repository) UpdateThreadState(ctx context.Context, threadID, state string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE threads SET state = ? WHERE id = ?
	`, state, threadID)
	if err != nil {
		return fmt.Errorf("failed to update thread state: %w", err)
	}
	return nil
}

// GetPendingThreadMessages returns all thread messages with state 'pending'
func (r *Repository) GetPendingThreadMessages(ctx context.Context) ([]*model.ThreadMessage, error) {
	query := `
		SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
		       tm.is_delivered, tm.actions, tm.state,
		       a.identity, a.alias
		FROM thread_messages tm
		JOIN authors a ON tm.author_identity = a.identity
		WHERE tm.state = 'pending'
		ORDER BY tm.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query pending thread messages: %w", err)
	}
	defer rows.Close()

	var messages []*model.ThreadMessage
	for rows.Next() {
		var dbMessage dbThreadMessage
		var author dbAuthor

		err := rows.Scan(
			&dbMessage.ID, &dbMessage.ThreadID, &dbMessage.AuthorIdentity, &dbMessage.Content,
			&dbMessage.CreatedAt, &dbMessage.IsDelivered, &dbMessage.Actions, &dbMessage.State,
			&author.Identity, &author.Alias,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan pending thread message row: %w", err)
		}

		message, err := r.dbThreadMessageToModel(&dbMessage, &author)
		if err != nil {
			return nil, fmt.Errorf("failed to convert pending thread message to model: %w", err)
		}

		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating pending thread messages: %w", err)
	}

	return messages, nil
}

// UpdateThreadMessageState updates the state of a thread message
func (r *Repository) UpdateThreadMessageState(ctx context.Context, messageID, state string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE thread_messages SET state = ? WHERE id = ?
	`, state, messageID)
	if err != nil {
		return fmt.Errorf("failed to update thread message state: %w", err)
	}
	return nil
}

// GetThreadMessagesByState returns all thread messages with a specific state
func (r *Repository) GetThreadMessagesByState(ctx context.Context, state string) ([]*model.ThreadMessage, error) {
	query := `
		SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
		       tm.is_delivered, tm.actions, tm.state,
		       a.identity, a.alias
		FROM thread_messages tm
		JOIN authors a ON tm.author_identity = a.identity
		WHERE tm.state = ?
		ORDER BY tm.created_at ASC
	`

	rows, err := r.db.QueryContext(ctx, query, state)
	if err != nil {
		return nil, fmt.Errorf("failed to query thread messages by state: %w", err)
	}
	defer rows.Close()

	var messages []*model.ThreadMessage
	for rows.Next() {
		var dbMessage dbThreadMessage
		var author dbAuthor

		err := rows.Scan(
			&dbMessage.ID, &dbMessage.ThreadID, &dbMessage.AuthorIdentity, &dbMessage.Content,
			&dbMessage.CreatedAt, &dbMessage.IsDelivered, &dbMessage.Actions, &dbMessage.State,
			&author.Identity, &author.Alias,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan thread message row: %w", err)
		}

		message, err := r.dbThreadMessageToModel(&dbMessage, &author)
		if err != nil {
			return nil, fmt.Errorf("failed to convert thread message to model: %w", err)
		}

		messages = append(messages, message)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating thread messages by state: %w", err)
	}

	return messages, nil
}

// UpdateThreadRemoteID updates the remote_thread_id of a thread
func (r *Repository) UpdateThreadRemoteID(ctx context.Context, threadID string, remoteThreadID int32) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE threads SET remote_thread_id = ? WHERE id = ?
	`, remoteThreadID, threadID)
	if err != nil {
		return fmt.Errorf("failed to update thread remote ID: %w", err)
	}
	return nil
}

// GetThreadByRemoteID returns a thread by its remote_thread_id
func (r *Repository) GetThreadByRemoteID(ctx context.Context, remoteThreadID int32) (*model.Thread, error) {
	query := `
		SELECT t.id, t.title, t.content, t.author_identity, t.created_at, t.expires_at, 
		       t.image_urls, t.actions, t.views, t.state, t.remote_thread_id,
		       a.identity, a.alias
		FROM threads t
		JOIN authors a ON t.author_identity = a.identity
		WHERE t.remote_thread_id = ?
	`

	var thread dbThread
	var threadAuthor dbAuthor

	err := r.db.QueryRowContext(ctx, query, remoteThreadID).Scan(
		&thread.ID, &thread.Title, &thread.Content, &thread.AuthorIdentity,
		&thread.CreatedAt, &thread.ExpiresAt, &thread.ImageURLs, &thread.Actions,
		&thread.Views, &thread.State, &thread.RemoteThreadID, &threadAuthor.Identity, &threadAuthor.Alias,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("thread with remote ID %d not found", remoteThreadID)
		}
		return nil, fmt.Errorf("failed to get thread by remote ID: %w", err)
	}

	threadModel, err := r.dbThreadToModel(ctx, &thread, &threadAuthor)
	if err != nil {
		return nil, fmt.Errorf("failed to convert thread to model: %w", err)
	}

	return threadModel, nil
}

// GetThreadMessage returns a thread message by its ID
func (r *Repository) GetThreadMessage(ctx context.Context, messageID string) (*model.ThreadMessage, error) {
	query := `
		SELECT tm.id, tm.thread_id, tm.author_identity, tm.content, tm.created_at, 
		       tm.is_delivered, tm.actions, tm.state,
		       a.identity, a.alias
		FROM thread_messages tm
		JOIN authors a ON tm.author_identity = a.identity
		WHERE tm.id = ?
	`

	var dbMessage dbThreadMessage
	var author dbAuthor

	err := r.db.QueryRowContext(ctx, query, messageID).Scan(
		&dbMessage.ID, &dbMessage.ThreadID, &dbMessage.AuthorIdentity, &dbMessage.Content,
		&dbMessage.CreatedAt, &dbMessage.IsDelivered, &dbMessage.Actions, &dbMessage.State,
		&author.Identity, &author.Alias,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Return nil, nil when not found (not an error)
		}
		return nil, fmt.Errorf("failed to get thread message: %w", err)
	}

	return r.dbThreadMessageToModel(&dbMessage, &author)
}

func (r *Repository) dbThreadToModel(ctx context.Context, dbThread *dbThread, author *dbAuthor) (*model.Thread, error) {
	var imageURLs []string
	if err := json.Unmarshal([]byte(dbThread.ImageURLs), &imageURLs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal image URLs: %w", err)
	}

	var actions []string
	if err := json.Unmarshal([]byte(dbThread.Actions), &actions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actions: %w", err)
	}

	messages, err := r.GetThreadMessages(ctx, dbThread.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread messages: %w", err)
	}

	return &model.Thread{
		ID:             dbThread.ID,
		Title:          dbThread.Title,
		Content:        dbThread.Content,
		CreatedAt:      dbThread.CreatedAt,
		ExpiresAt:      dbThread.ExpiresAt,
		ImageURLs:      imageURLs,
		Actions:        actions,
		Views:          dbThread.Views,
		RemoteThreadID: dbThread.RemoteThreadID,
		Author: &model.Author{
			Identity: author.Identity,
			Alias:    author.Alias,
		},
		Messages: messages,
	}, nil
}

func (r *Repository) dbThreadMessageToModel(dbMessage *dbThreadMessage, author *dbAuthor) (*model.ThreadMessage, error) {
	var actions []string
	if err := json.Unmarshal([]byte(dbMessage.Actions), &actions); err != nil {
		return nil, fmt.Errorf("failed to unmarshal actions: %w", err)
	}

	// Convert bool to *bool for IsDelivered
	var isDelivered *bool
	if dbMessage.IsDelivered {
		isDelivered = &dbMessage.IsDelivered
	}

	return &model.ThreadMessage{
		ID:          dbMessage.ID,
		Content:     dbMessage.Content,
		CreatedAt:   dbMessage.CreatedAt,
		IsDelivered: isDelivered,
		Actions:     actions,
		State:       dbMessage.State,
		Author: &model.Author{
			Identity: author.Identity,
			Alias:    author.Alias,
		},
	}, nil
}
