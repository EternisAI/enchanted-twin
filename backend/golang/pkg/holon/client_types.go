package holon

import "time"

// Client types based on HolonZero API swagger specification

// Participant represents a participant in the system.
type Participant struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	CollisionCounter int       `json:"collision_counter"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Thread represents a discussion thread.
type Thread struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	Content       string    `json:"content"`
	CreatorID     int       `json:"creatorId"`
	CreatorName   string    `json:"creatorName"`
	DedupThreadID string    `json:"dedupThreadId"`
	ImageURLs     []string  `json:"imageUrls,omitempty"`
	CreatedAt     time.Time `json:"createdAt"`
	UpdatedAt     time.Time `json:"updatedAt"`
}

// Reply represents a reply to a thread.
type Reply struct {
	ID                     int       `json:"id"`
	ThreadID               int       `json:"threadId"`
	ParticipantID          int       `json:"participantId"`
	ParticipantDisplayName string    `json:"participantDisplayName"`
	Content                string    `json:"content"`
	DedupReplyID           string    `json:"dedupReplyId"`
	ImageURLs              []string  `json:"imageUrls,omitempty"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
}

// Request types

// CreateParticipantRequest for creating a new participant.
type CreateParticipantRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UpdateParticipantRequest for updating a participant.
type UpdateParticipantRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// CreateThreadRequest for creating a new thread.
type CreateThreadRequest struct {
	Title         string   `json:"title"`
	Content       string   `json:"content,omitempty"`
	DedupThreadID string   `json:"dedupThreadId"`
	ImageURLs     []string `json:"imageUrls,omitempty"`
}

// UpdateThreadRequest for updating a thread.
type UpdateThreadRequest struct {
	Title     string   `json:"title"`
	Content   string   `json:"content,omitempty"`
	ImageURLs []string `json:"imageUrls,omitempty"`
}

// CreateReplyRequest for creating a new reply.
type CreateReplyRequest struct {
	ThreadID      int      `json:"threadId"`
	ParticipantID int      `json:"participantId"`
	Content       string   `json:"content"`
	DedupReplyID  string   `json:"dedupReplyId"`
	ImageURLs     []string `json:"imageUrls,omitempty"`
}

// UpdateReplyRequest for updating a reply.
type UpdateReplyRequest struct {
	Content   string   `json:"content"`
	ImageURLs []string `json:"imageUrls,omitempty"`
}

// Response types

// PaginatedThreadsResponse for paginated thread listing.
type PaginatedThreadsResponse struct {
	Threads    []Thread `json:"threads"`
	Page       int      `json:"page"`
	PageSize   int      `json:"pageSize"`
	TotalCount int      `json:"totalCount"`
	HasMore    bool     `json:"hasMore"`
}

// PaginatedRepliesResponse for paginated reply listing.
type PaginatedRepliesResponse struct {
	Replies    []Reply `json:"replies"`
	Page       int     `json:"page"`
	PageSize   int     `json:"pageSize"`
	TotalCount int     `json:"totalCount"`
	HasMore    bool    `json:"hasMore"`
}

// SyncMetadataResponse for sync optimization.
type SyncMetadataResponse struct {
	ServerTime       time.Time `json:"serverTime"`
	TotalThreads     int       `json:"totalThreads"`
	TotalReplies     int       `json:"totalReplies"`
	LastThreadUpdate time.Time `json:"lastThreadUpdate"`
	LastReplyUpdate  time.Time `json:"lastReplyUpdate"`
}

// HealthResponse for health check.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database,omitempty"`
}

// ErrorResponse represents API error responses.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// ParticipantAuthResponse for OAuth authentication.
type ParticipantAuthResponse struct {
	ID          int    `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Message     string `json:"message"`
}

// ParticipantResponse for participant operations.
type ParticipantResponse struct {
	ID               int       `json:"id"`
	Name             string    `json:"name"`
	Email            string    `json:"email"`
	DisplayName      string    `json:"display_name"`
	CollisionCounter int       `json:"collision_counter"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// Query parameters for pagination and filtering.
type ThreadsQuery struct {
	Page         int       `json:"page,omitempty"`
	Limit        int       `json:"limit,omitempty"`
	UpdatedAfter time.Time `json:"updatedAfter,omitempty"`
}

type RepliesQuery struct {
	Page  int `json:"page,omitempty"`
	Limit int `json:"limit,omitempty"`
}
