package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"rotaperfumes/shared/pkg/config"
	"rotaperfumes/shared/pkg/jwt"
	"rotaperfumes/shared/pkg/logger"
)

func setupTestMiddleware(t *testing.T) (*AuthMiddleware, *jwt.JWTService) {
	t.Helper()

	// Initialize logger
	logger.Init("error", "text")

	cfg := &config.Config{
		JWTSecret: "test-secret-key-32-chars-minimum-for-testing",
		JWTTTL:    "1h",
		JWTIssuer: "test",
	}

	jwtSvc, err := jwt.NewJWTService(cfg)
	if err != nil {
		t.Fatalf("Failed to create JWT service: %v", err)
	}

	mw := NewAuthMiddleware(jwtSvc)
	return mw, jwtSvc
}

func TestAuthMiddleware_RequireAuth_MissingHeader(t *testing.T) {
	mw, _ := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_RequireAuth_InvalidFormat(t *testing.T) {
	mw, _ := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_RequireAuth_BasicAuth(t *testing.T) {
	mw, _ := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	// Basic auth instead of Bearer
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_RequireAuth_InvalidToken(t *testing.T) {
	mw, _ := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", mw.RequireAuth(), func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status %d, got %d", http.StatusUnauthorized, w.Code)
	}
}

func TestAuthMiddleware_RequireAuth_ValidToken(t *testing.T) {
	mw, jwtSvc := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/test", mw.RequireAuth(), func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("role")
		c.JSON(200, gin.H{
			"user_id": userID,
			"role":    role,
		})
	})

	token, _, err := jwtSvc.GenerateToken(42, "admin", "RH")
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d. Body: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_RequireRole_Authorized(t *testing.T) {
	mw, jwtSvc := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	auth := router.Group("/")
	auth.Use(mw.RequireAuth())
	auth.Use(mw.RequireRole("RH", "GERENTE"))
	auth.GET("test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	token, _, _ := jwtSvc.GenerateToken(1, "admin", "RH")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status %d, got %d", http.StatusOK, w.Code)
	}
}

func TestAuthMiddleware_RequireRole_Forbidden(t *testing.T) {
	mw, jwtSvc := setupTestMiddleware(t)
	gin.SetMode(gin.TestMode)
	router := gin.New()

	auth := router.Group("/")
	auth.Use(mw.RequireAuth())
	auth.Use(mw.RequireRole("RH"))
	auth.GET("test", func(c *gin.Context) {
		c.JSON(200, gin.H{"ok": true})
	})

	token, _, _ := jwtSvc.GenerateToken(1, "vendedor", "VENDEDOR")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status %d, got %d", http.StatusForbidden, w.Code)
	}
}
