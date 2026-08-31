package config

import (
	"crypto/tls"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

var (
	OAuthConfig *oauth2.Config
	GitLabURL   string
	SessionKey  []byte
)

// InsecureHTTPClient is a globally available HTTP client with SSL verification disabled
var InsecureHTTPClient *http.Client

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	GitLabURL = os.Getenv("GITLAB_URL")
	if GitLabURL == "" {
		log.Fatal("GITLAB_URL is required")
	}

	clientID := os.Getenv("GITLAB_CLIENT_ID")
	clientSecret := os.Getenv("GITLAB_CLIENT_SECRET")
	redirectURL := os.Getenv("OAUTH_REDIRECT_URL")
	if redirectURL == "" {
		redirectURL = "http://localhost:5050/auth/callback"
	}

	OAuthConfig = &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURL,
		Scopes:       []string{"read_api", "read_user"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  GitLabURL + "/oauth/authorize",
			TokenURL: GitLabURL + "/oauth/token",
		},
	}

	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "super-secret-key-replace-in-production"
	}
	SessionKey = []byte(sessionSecret)

	// Create a global insecure HTTP client for when we need to bypass SSL
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	InsecureHTTPClient = &http.Client{Transport: tr}
}
