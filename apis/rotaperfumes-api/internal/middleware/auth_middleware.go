// Package middleware provides HTTP middleware functions.
package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
	"rotaperfumes/api/internal/presenter"
	"rotaperfumes/shared/pkg/jwt"
	"rotaperfumes/shared/pkg/logger"
)

// AuthMiddleware provides JWT authentication middleware.
type AuthMiddleware struct {
	jwtService *jwt.JWTService
	log        *logger.Logger
}

// NewAuthMiddleware creates a new authentication middleware.
func NewAuthMiddleware(jwtService *jwt.JWTService) *AuthMiddleware {
	return &AuthMiddleware{
		jwtService: jwtService,
		log:        logger.Get(),
	}
}

// RequireAuth is a Gin middleware that validates JWT tokens.
func (m *AuthMiddleware) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		m.log.DebugVerbose("Auth middleware: checking request")

		// Extract Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			m.log.DebugVerbose("Auth middleware: missing Authorization header")
			c.AbortWithStatusJSON(401, gin.H{
				"error": "Missing Authorization header",
				"code":  "MISSING_AUTH",
			})
			return
		}

		// Check Bearer prefix
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			m.log.DebugVerbose("Auth middleware: invalid Authorization format")
			c.AbortWithStatusJSON(401, gin.H{
				"error": "Invalid Authorization header format. Use: Bearer <token>",
				"code":  "INVALID_AUTH_FORMAT",
			})
			return
		}

		tokenString := parts[1]

		// Validate token
		m.log.DebugVerbose("Auth middleware: validating token")
		claims, err := m.jwtService.ValidateToken(tokenString)
		if err != nil {
			m.log.DebugVerbose("Auth middleware: token validation failed", "error", err)
			c.AbortWithStatusJSON(401, gin.H{
				"error": "Invalid or expired token",
				"code":  "INVALID_TOKEN",
			})
			return
		}

		m.log.DebugVerbose("Auth middleware: token valid", "user_id", claims.UserID, "role", claims.Role)

		// Set user info in context for handlers
		c.Set("user_id", claims.UserID)
		c.Set("login", claims.Login)
		c.Set("role", claims.Role)

		c.Next()
	}
}

// RequireRole creates a middleware that requires a specific role.
func (m *AuthMiddleware) RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole, exists := c.Get("role")
		if !exists {
			presenter.Unauthorized(c, "No role found in context")
			c.Abort()
			return
		}

		roleStr, ok := userRole.(string)
		if !ok {
			presenter.Unauthorized(c, "Invalid role type")
			c.Abort()
			return
		}

		// Check if user has one of the required roles
		for _, role := range roles {
			if roleStr == role {
				c.Next()
				return
			}
		}

		m.log.Warn("Access denied: insufficient role", "required", roles, "actual", roleStr)
		c.AbortWithStatusJSON(403, gin.H{
			"error": "Insufficient permissions",
			"code":  "FORBIDDEN",
		})
	}
}
