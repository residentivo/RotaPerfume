// Package dto contains Data Transfer Objects for API communication.
package dto

import "time"

// LoginRequest represents the login request payload.
type LoginRequest struct {
	Login string `json:"login" binding:"required"`
	Senha string `json:"senha" binding:"required"`
}

// LoginResponse represents the login response with JWT token.
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	TokenType string    `json:"token_type"`
	User      UserInfo  `json:"user"`
}

// UserInfo contains basic user information in responses.
type UserInfo struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
	Role  string `json:"role"`
}

// ErrorResponse represents an API error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    string `json:"code,omitempty"`
	Details string `json:"details,omitempty"`
}

// HealthResponse represents the health check response.
type HealthResponse struct {
	Status   string `json:"status"`
	Database string `json:"database"`
	Version  string `json:"version"`
}
