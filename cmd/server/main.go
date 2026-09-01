package main

import (
	"fmt"
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
	
	dbHost := os.Getenv("DB_HOST")
	if dbHost == "" {
		dbHost = "localhost"
	}
	dbPort := os.Getenv("DB_PORT")
	if dbPort == "" {
		dbPort = "5433"
	}
	dbUser := os.Getenv("DB_USER")
	if dbUser == "" {
		dbUser = "scanner"
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		dbPassword = "scanner_password"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "scanner_db"
	}

	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC", dbHost, dbUser, dbPassword, dbName, dbPort)
	repository.InitDB(dsn)

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
		api.GET("/scan/:id", handlers.JobStatusHandler)
		api.POST("/export", handlers.ExportPDFHandler)
	}

	log.Println("Server starting on :5050")
	if err := r.Run(":5050"); err != nil {
		log.Fatal(err)
	}
}
