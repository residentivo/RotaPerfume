package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"rotaperfumes/shared/pkg/dto"
	"rotaperfumes/shared/pkg/database"
	"rotaperfumes/shared/pkg/logger"
)

// HealthHandler handles health check HTTP requests.
type HealthHandler struct {
	log *logger.Logger
}

// NewHealthHandler creates a new health handler.
func NewHealthHandler() *HealthHandler {
	return &HealthHandler{
		log: logger.Get(),
	}
}

// Health handles GET /health requests (public).
func (h *HealthHandler) Health(c *gin.Context) {
	dbStatus := "ok"
	if err := database.HealthCheck(c.Request.Context()); err != nil {
		dbStatus = "error: " + err.Error()
	}

	c.JSON(http.StatusOK, dto.HealthResponse{
		Status:   "ok",
		Database: dbStatus,
		Version:  "1.0.0",
	})
}

// ProtectedHealth handles GET /health/protected requests (requires auth middleware).
func (h *HealthHandler) ProtectedHealth(c *gin.Context) {
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
			Error: "Unauthorized",
			Code:  "NO_AUTH",
		})
		return
	}

	role, _ := c.Get("role")

	h.log.Info("Protected health check accessed", "user_id", userID, "role", role)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"message": "You have access to protected resources",
		"user_id": userID,
		"role":    role,
	})
}

// Ready handles GET /ready requests (kubernetes readiness probe).
func (h *HealthHandler) Ready(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	if err := database.HealthCheck(ctx); err != nil {
		c.JSON(http.StatusServiceUnavailable, dto.ErrorResponse{
			Error: "Database not ready",
			Code:  "DB_NOT_READY",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
