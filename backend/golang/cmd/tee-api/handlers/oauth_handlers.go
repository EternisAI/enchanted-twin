package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"oauth-proxy/models"
	"oauth-proxy/services"
)

type OAuthHandler struct {
	oauthService *services.OAuthService
}

func NewOAuthHandler(oauthService *services.OAuthService) *OAuthHandler {
	return &OAuthHandler{
		oauthService: oauthService,
	}
}

// ExchangeToken handles token exchange endpoint
// POST /auth/exchange
func (h *OAuthHandler) ExchangeToken(c *gin.Context) {
	var req models.TokenExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {

		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "invalid_request",
			Description: err.Error(),
			Code:        http.StatusBadRequest,
		})
		return
	}

	tokenResponse, err := h.oauthService.ExchangeToken(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "exchange_failed",
			Description: err.Error(),
			Code:        http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, tokenResponse)
}

// RefreshToken handles token refresh endpoint
// POST /auth/refresh
func (h *OAuthHandler) RefreshToken(c *gin.Context) {
	var req models.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "invalid_request",
			Description: err.Error(),
			Code:        http.StatusBadRequest,
		})
		return
	}

	tokenResponse, err := h.oauthService.RefreshToken(req.Platform, req.RefreshToken)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "refresh_failed",
			Description: err.Error(),
			Code:        http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, tokenResponse)
}

// GoogleCallback handles Google OAuth callback
// GET /auth/google/callback
func (h *OAuthHandler) GoogleCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	error := c.Query("error")

	if error != "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       error,
			Description: "OAuth authorization failed",
			Code:        http.StatusBadRequest,
		})
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "missing_code",
			Description: "Authorization code is required",
			Code:        http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     code,
		"state":    state,
		"platform": "google",
		"message":  "Callback received. Use /auth/exchange to exchange code for tokens.",
	})
}

// SlackCallback handles Slack OAuth callback
// GET /auth/slack/callback
func (h *OAuthHandler) SlackCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	error := c.Query("error")

	if error != "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       error,
			Description: "OAuth authorization failed",
			Code:        http.StatusBadRequest,
		})
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "missing_code",
			Description: "Authorization code is required",
			Code:        http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     code,
		"state":    state,
		"platform": "slack",
		"message":  "Callback received. Use /auth/exchange to exchange code for tokens.",
	})
}

// TwitterCallback handles Twitter OAuth callback
// GET /auth/twitter/callback
func (h *OAuthHandler) TwitterCallback(c *gin.Context) {
	code := c.Query("code")
	state := c.Query("state")
	error := c.Query("error")

	if error != "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       error,
			Description: "OAuth authorization failed",
			Code:        http.StatusBadRequest,
		})
		return
	}

	if code == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error:       "missing_code",
			Description: "Authorization code is required",
			Code:        http.StatusBadRequest,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":     code,
		"state":    state,
		"platform": "twitter",
		"message":  "Callback received. Use /auth/exchange to exchange code for tokens.",
	})
}

// HealthCheck handles health check endpoint
// GET /health
func (h *OAuthHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "oauth-proxy",
	})
}
