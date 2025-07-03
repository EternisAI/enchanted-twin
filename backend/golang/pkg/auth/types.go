package auth

// TokenExchangeRequest represents the request model for token exchange.
type TokenExchangeRequest struct {
	GrantType    string `json:"grant_type" binding:"required"`
	Code         string `json:"code,omitempty"`
	CodeVerifier string `json:"code_verifier,omitempty"`
	Platform     string `json:"platform" binding:"required"`
	RefreshToken string `json:"refresh_token,omitempty"`
	RedirectURI  string `json:"redirect_uri,omitempty"`
}

// RefreshTokenRequest represents the request model for refresh token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
	Platform     string `json:"platform" binding:"required"`
}
