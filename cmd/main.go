package main

import (
	"log"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/googleAuth/config"
	"github.com/googleAuth/handlers"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize router
	r := gin.Default()

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg)

	// Setup sessions middleware
	r.Use(sessions.Sessions("googleauth_session", authHandler.GetStore()))

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Google Auth API",
			"endpoints": gin.H{
				"login":    "/auth/google",
				"callback": "/auth/google/callback",
				"profile":  "/auth/profile",
				"logout":   "/auth/logout",
			},
		})
	})

	// Auth routes
	r.GET("/auth/google", authHandler.Login)
	r.GET("/auth/google/callback", authHandler.Callback)
	r.GET("/auth/profile", authHandler.Profile)
	r.GET("/auth/logout", authHandler.Logout)

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
