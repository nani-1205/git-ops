package main

import (
	"log"
	"net/http"
	"time"

	"gitlab-code-scan/internal/config"
	"gitlab-code-scan/internal/handlers"
	"gitlab-code-scan/internal/repository"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	config.Load()

	repository.InitDB(config.DatabaseDSN)

	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Setup Sessions
	store := cookie.NewStore(config.SessionKey)
	r.Use(sessions.Sessions("gitlab-scan-session", store))

	// ── Static files with HTTP cache headers (7-day browser cache) ────────────
	staticGroup := r.Group("/static")
	staticGroup.Use(func(c *gin.Context) {
		c.Header("Cache-Control", "public, max-age=604800, immutable")
		c.Header("Expires", time.Now().Add(7*24*time.Hour).UTC().Format(http.TimeFormat))
		c.Next()
	})
	staticGroup.Static("/", "./web/static")

	r.LoadHTMLGlob("web/templates/*")

	// Suppress browser favicon 404
	r.GET("/favicon.ico", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	// Frontend routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	// Auth routes
	auth := r.Group("/auth")
	{
		auth.GET("/login", handlers.LoginHandler)
		auth.GET("/callback", handlers.CallbackHandler)
		auth.GET("/logout", handlers.LogoutHandler)
		auth.GET("/me", handlers.CurrentUserHandler)
	}

	api := r.Group("/api")
	api.Use(handlers.RequireAuth())
	{
		api.GET("/groups", handlers.ListGroupsHandler)
		api.GET("/groups/:id/projects", handlers.ListGroupProjectsHandler)
		api.POST("/scan", handlers.ScanHandler)
		api.GET("/scan/:id", handlers.JobStatusHandler)
		api.POST("/export", handlers.ExportPDFHandler)
	}

	log.Println("Server starting on :5050")
	if err := r.Run(":5050"); err != nil {
		log.Fatal(err)
	}
}

