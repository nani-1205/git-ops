package main

import (
	"log"
	"os"

	"gitlab-code-scan/internal/config"
	"gitlab-code-scan/internal/handlers"
	"gitlab-code-scan/internal/repository"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

func main() {
	config.Load()
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "gitlab-scan.db"
	}
	repository.InitDB(dbPath)

	r := gin.Default()

	// Setup Sessions
	store := cookie.NewStore(config.SessionKey)
	r.Use(sessions.Sessions("gitlab-scan-session", store))

	// Serve Static Files
	r.Static("/static", "./web/static")
	r.LoadHTMLGlob("web/templates/*")

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
		api.POST("/export", handlers.ExportPDFHandler)
	}

	log.Println("Server starting on :5050")
	if err := r.Run(":5050"); err != nil {
		log.Fatal(err)
	}
}
