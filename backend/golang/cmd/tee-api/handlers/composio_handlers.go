package handlers

import (
	"net/http"

	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/models"
	"github.com/eternisai/enchanted-twin/backend/golang/cmd/tee-api/services"
	"github.com/gin-gonic/gin"
)

type ComposioHandler struct {
	composioService *services.ComposioService
}

// NewComposioHandler creates a new ComposioHandler instance
func NewComposioHandler(composioService *services.ComposioService) *ComposioHandler {
	return &ComposioHandler{
		composioService: composioService,
	}
}

// CreateConnectedAccount handles the creation of a new connected account
// POST /composio/connect
func (h *ComposioHandler) CreateConnectedAccount(c *gin.Context) {
	var req models.CreateConnectedAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate required fields
	if req.UserID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "user_id is required",
		})
		return
	}

	if req.Provider == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "provider is required",
		})
		return
	}

	// Call the service
	response, err := h.composioService.CreateConnectedAccount(req.UserID, req.Provider, req.RedirectURI)
	if err != nil {

		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to create connected account",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ComposioHandler) GetConnectedAccount(c *gin.Context) {
	accountID := c.Query("account_id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "account_id is required",
		})
		return
	}

	response, err := h.composioService.GetConnectedAccount(accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to get connected account",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ComposioHandler) RefreshToken(c *gin.Context) {
	accountID := c.Query("account_id")
	if accountID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "account_id is required in query params",
		})
	}

	response, err := h.composioService.RefreshToken(accountID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to refresh token",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetToolBySlug handles retrieving toolkit information by slug
// GET /composio/tools/:slug
func (h *ComposioHandler) GetToolBySlug(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "toolkit slug is required",
		})
		return
	}

	// Call the service
	toolkit, err := h.composioService.GetToolBySlug(slug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve toolkit",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, toolkit)
}

// ExecuteTool handles tool execution
// POST /composio/tools/:slug/execute
func (h *ComposioHandler) ExecuteTool(c *gin.Context) {
	slug := c.Param("slug")
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "tool slug is required",
		})
		return
	}

	var req models.ExecuteToolRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request format",
			"details": err.Error(),
		})
		return
	}

	// Validate that either UserID or EntityID is provided
	if req.UserID == "" && req.EntityID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "either user_id or entity_id is required",
		})
		return
	}

	// Call the service
	response, err := h.composioService.ExecuteTool(slug, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to execute tool",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response)
}

// GetConnectedAccounts handles retrieving connected accounts for a user
// GET /composio/accounts/:user_id
func (h *ComposioHandler) GetConnectedAccounts(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "user_id is required",
		})
		return
	}

	// Call the service
	accounts, err := h.composioService.GetConnectedAccountByUserID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to retrieve connected accounts",
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"accounts": accounts,
		"count":    len(accounts),
	})
}

// HealthCheck provides a health check endpoint for the Composio service
// GET /composio/health
func (h *ComposioHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "composio",
		"message": "Composio service is running",
	})
}
