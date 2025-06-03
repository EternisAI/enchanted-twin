package models

import "time"

// CreateConnectedAccountRequest represents the request to create a new connected account
type CreateConnectedAccountRequest struct {
	Provider    string `json:"provider" binding:"required"`
	UserID      string `json:"user_id" binding:"required"`
	RedirectURI string `json:"redirect_uri,omitempty"`
}

type CreateConnectedAccountResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Toolkit represents a toolkit/tool information
type Toolkit struct {
	Slug              string             `json:"slug"`
	Name              string             `json:"name"`
	Enabled           bool               `json:"enabled"`
	IsLocalToolkit    bool               `json:"is_local_toolkit"`
	Meta              ToolkitMeta        `json:"meta"`
	AuthConfigDetails []AuthConfigDetail `json:"auth_config_details,omitempty"`
}

// ToolkitMeta contains metadata about a toolkit
type ToolkitMeta struct {
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Description   string     `json:"description"`
	Logo          string     `json:"logo"`
	Categories    []Category `json:"categories"`
	TriggersCount int        `json:"triggers_count"`
	ToolsCount    int        `json:"tools_count"`
	AppURL        string     `json:"app_url"`
}

// Category represents a toolkit category
type Category struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// AuthConfigDetail represents authentication configuration details
type AuthConfigDetail struct {
	Mode   string                 `json:"mode"`
	Fields map[string]interface{} `json:"fields"`
	Name   string                 `json:"name"`
	Proxy  *ProxyConfig           `json:"proxy,omitempty"`
}

// ProxyConfig represents proxy configuration
type ProxyConfig struct {
	BaseURL string `json:"base_url"`
}

// ExecuteToolRequest represents the request to execute a tool
type ExecuteToolRequest struct {
	ConnectedAccountID string                 `json:"connected_account_id,omitempty"`
	EntityID           string                 `json:"entity_id,omitempty"`
	UserID             string                 `json:"user_id,omitempty"` // For backward compatibility
	Version            string                 `json:"version,omitempty"`
	Arguments          map[string]interface{} `json:"arguments,omitempty"`
	Text               string                 `json:"text,omitempty"`
	AllowTracing       bool                   `json:"allow_tracing,omitempty"`
}

// ExecuteToolResponse represents the response from executing a tool
type ExecuteToolResponse struct {
	Data        interface{} `json:"data"`
	Successful  bool        `json:"successful"`
	Error       string      `json:"error,omitempty"`
	LogID       string      `json:"log_id,omitempty"`
	SessionInfo interface{} `json:"session_info"`
}

// DetailedConnectedAccountResponse represents a detailed response from creating/getting a connected account
type ConnectedAccountResponse struct {
	ID          string `json:"id"`
	Status      string `json:"status"`
	RedirectURL string `json:"redirect_url"`
}

// ComposioError represents an error response from Composio API
type ComposioError struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

func (e ComposioError) Error() string {
	if e.Details != "" {
		return e.Message + ": " + e.Details
	}
	return e.Message
}

// ConnectedAccountDetails represents the detailed connected account response
type ConnectedAccountDetailResponse struct {
	UserID string    `json:"user_id"`
	ID     string    `json:"id"`
	Status string    `json:"status"`
	State  AuthState `json:"state"`
}

// AuthState represents the authentication state with OAuth details
type AuthState struct {
	AuthScheme string        `json:"authScheme"`
	Val        OAuth2Details `json:"val"`
}

// OAuth2Details represents OAuth2 authentication details
type OAuth2Details struct {
	Status       string `json:"status"`
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type ComposioRefreshTokenResponse struct {
	ID          string `json:"id"`
	RedirectURL string `json:"redirect_url"`
	Status      string `json:"status"`
}
