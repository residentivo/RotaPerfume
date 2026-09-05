// Package main is the entry point for the RotaPerfumes API.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"rotaperfumes/api/internal/handler"
	"rotaperfumes/api/internal/middleware"
	"rotaperfumes/shared/pkg/repository"
	"rotaperfumes/shared/pkg/usecase"
	"rotaperfumes/shared/pkg/config"
	"rotaperfumes/shared/pkg/database"
	"rotaperfumes/shared/pkg/jwt"
	"rotaperfumes/shared/pkg/logger"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize logger
	log := logger.Init(cfg.LogLevel, cfg.LogFormat)
	log.Info("Starting RotaPerfumes API", "port", cfg.Port)

	// Connect to database
	db, err := database.Connect(cfg)
	if err != nil {
		log.Error("Failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	// Initialize JWT service
	jwtService, err := jwt.NewJWTService(cfg)
	if err != nil {
		log.Error("Failed to initialize JWT service", "error", err)
		os.Exit(1)
	}

	// Initialize repository
	userRepo := repository.NewMySQLUserRepository(db)

	// Initialize use cases
	authUseCase := usecase.NewAuthUseCase(userRepo, jwtService, cfg.BcryptCost)

	// Initialize handlers
	authHandler := handler.NewAuthHandler(authUseCase)
	healthHandler := handler.NewHealthHandler()

	// Initialize middleware
	authMiddleware := middleware.NewAuthMiddleware(jwtService)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()

	// Global middleware
	router.Use(gin.Recovery())
	router.Use(requestLogger(log))
	router.Use(corsMiddleware())

	// Health endpoints (public)
	router.GET("/health", healthHandler.Health)
	router.GET("/ready", healthHandler.Ready)

	// Auth endpoints (public)
	auth := router.Group("/auth")
	{
		auth.POST("/login", authHandler.Login)
	}

	// Protected endpoints (require authentication)
	api := router.Group("/")
	api.Use(authMiddleware.RequireAuth())
	{
		// Protected health check (demo)
		api.GET("health/protected", healthHandler.ProtectedHealth)

		// Example protected user listing (demo for other endpoints)
		api.GET("users", listUsersHandler(userRepo, log))
	}

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Info("API server listening", "port", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("Server forced to shutdown", "error", err)
	}

	log.Info("Server stopped")
}

// requestLogger returns a Gin middleware for logging HTTP requests.
func requestLogger(log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		log.Info("HTTP request",
			"method", c.Request.Method,
			"path", path,
			"status", status,
			"latency", latency,
			"ip", c.ClientIP(),
		)
	}
}

// corsMiddleware returns a Gin middleware for CORS.
func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization")
		c.Writer.Header().Set("Access-Control-Max-Age", "86400")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// listUsersHandler is a demo handler for listing users (protected).
func listUsersHandler(userRepo *repository.MySQLUserRepository, log *logger.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("user_id")
		role, _ := c.Get("role")

		log.Info("Users list requested", "by_user", userID, "role", role)

		c.JSON(http.StatusOK, gin.H{
			"message": "This is a protected endpoint. Add your business logic here.",
			"user_id": userID,
			"role":    role,
		})
	}
}
