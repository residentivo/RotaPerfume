// Package presenter provides response formatting utilities.
package presenter

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"rotaperfumes/shared/pkg/dto"
)

// Success sends a successful JSON response.
func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, data)
}

// Created sends a 201 Created response.
func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, data)
}

// Error sends an error JSON response.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error: message,
	})
}

// ErrorWithCode sends an error with a code.
func ErrorWithCode(c *gin.Context, status int, code, message string) {
	c.JSON(status, dto.ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// BadRequest sends a 400 Bad Request response.
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

// Unauthorized sends a 401 Unauthorized response.
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

// Forbidden sends a 403 Forbidden response.
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

// NotFound sends a 404 Not Found response.
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

// InternalError sends a 500 Internal Server Error response.
func InternalError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}
