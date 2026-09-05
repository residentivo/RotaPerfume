// Package handler contains HTTP request handlers.
package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"rotaperfumes/api/internal/presenter"
	"rotaperfumes/shared/pkg/dto"
	"rotaperfumes/shared/pkg/usecase"
	"rotaperfumes/shared/pkg/logger"
)

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authUseCase *usecase.AuthUseCase
	log         *logger.Logger
}

// NewAuthHandler creates a new authentication handler.
func NewAuthHandler(authUseCase *usecase.AuthUseCase) *AuthHandler {
	return &AuthHandler{
		authUseCase: authUseCase,
		log:         logger.Get(),
	}
}

// Login handles POST /auth/login requests.
// It validates user credentials and returns a JWT token.
func (h *AuthHandler) Login(c *gin.Context) {
	h.log.DebugVerbose("Handling login request")

	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.log.DebugVerbose("Login request: failed to bind JSON", "error", err)
		presenter.BadRequest(c, "Invalid request body: login and senha are required")
		return
	}

	h.log.DebugVerbose("Login request received", "login", req.Login)

	response, err := h.authUseCase.Authenticate(c.Request.Context(), &req)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) || errors.Is(err, usecase.ErrUserNotFound) {
			h.log.Warn("Login failed: invalid credentials", "login", req.Login)
			c.JSON(http.StatusUnauthorized, dto.ErrorResponse{
				Error: "Invalid login or password",
				Code:  "AUTH_FAILED",
			})
			return
		}

		if errors.Is(err, usecase.ErrUserInactive) {
			h.log.Warn("Login failed: user inactive", "login", req.Login)
			c.JSON(http.StatusForbidden, dto.ErrorResponse{
				Error: "User account is inactive",
				Code:  "USER_INACTIVE",
			})
			return
		}

		h.log.Error("Login error", "error", err)
		c.JSON(http.StatusInternalServerError, dto.ErrorResponse{
			Error: "Internal server error",
			Code:  "INTERNAL_ERROR",
		})
		return
	}

	h.log.Info("Login successful", "user_id", response.User.ID, "login", response.User.Login)
	presenter.Success(c, response)
}
