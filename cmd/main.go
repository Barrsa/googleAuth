package main

import (
	"log"
	"net/http"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/googleAuth/config"
	"github.com/googleAuth/database"
	"github.com/googleAuth/handlers"
	"github.com/googleAuth/middleware"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to PostgreSQL
	db, err := database.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL")

	// Set Gin mode
	if cfg.Environment == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	// Initialize router
	r := gin.Default()

	// CORS middleware
	origins := strings.Split(cfg.CORSOrigins, ",")
	r.Use(cors.New(cors.Config{
		AllowOrigins:     origins,
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(cfg, db)

	// Setup sessions middleware (for OAuth state CSRF)
	r.Use(sessions.Sessions("googleauth_session", authHandler.GetStore()))

	// Health / info
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"service": "Barrsa Auth",
			"status":  "ok",
		})
	})

	// --- Public auth routes ---
	r.GET("/auth/google", authHandler.Login)
	r.GET("/auth/google/callback", authHandler.Callback)
	r.GET("/auth/logout", authHandler.Logout)

	// --- JWT-protected routes ---
	protected := r.Group("/")
	protected.Use(middleware.JWTAuth(cfg.JWTSecret))
	{
		// /auth/me  — used by frontend callback to get user after OAuth redirect
		// /api/v1/auth/me — alias so the frontend can call either path
		protected.GET("/auth/me", authHandler.AuthMe)
		protected.GET("/api/v1/auth/me", authHandler.AuthMe)

		// User CRUD
		protected.GET("/users/me", authHandler.GetMe)
		protected.PATCH("/users/me", authHandler.UpdateMe)
		protected.GET("/users/check-username", authHandler.CheckUsername)
	}

	// Legacy session-based profile (kept for backward compat)
	r.GET("/auth/profile", authHandler.Profile)

	// Start server
	addr := ":" + cfg.Port
	log.Printf("Barrsa Auth server starting on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
