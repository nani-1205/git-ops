package config

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

var (
	OAuthConfig *oauth2.Config
	GitLabURL   string
	SessionKey  []byte

	// DatabaseDSN is the fully assembled PostgreSQL connection string.
	DatabaseDSN string

	// Redis Config
	RedisHost         string
	RedisPort         string
	RedisPassword     string
	CacheTTLGroups    int
	CacheTTLProjects  int
)

// InsecureHTTPClient is a globally available HTTP client with SSL verification disabled.
var InsecureHTTPClient *http.Client

func Load() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on environment variables")
	}

	// ── GitLab ────────────────────────────────────────────────────────────────
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

	// ── Session ───────────────────────────────────────────────────────────────
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		sessionSecret = "super-secret-key-replace-in-production"
	}
	SessionKey = []byte(sessionSecret)

	// ── Database ──────────────────────────────────────────────────────────────
	DatabaseDSN = buildDSN()

	// ── Redis ─────────────────────────────────────────────────────────────────
	RedisHost = getEnvOrDefault("REDIS_HOST", "localhost")
	RedisPort = getEnvOrDefault("REDIS_PORT", "6379")
	RedisPassword = getEnvOrDefault("REDIS_PASSWORD", "")
	
	groupsTTLStr := getEnvOrDefault("CACHE_TTL_GROUPS", "30")
	if ttl, err := strconv.Atoi(groupsTTLStr); err == nil {
		CacheTTLGroups = ttl
	} else {
		CacheTTLGroups = 30
	}

	projectsTTLStr := getEnvOrDefault("CACHE_TTL_PROJECTS", "15")
	if ttl, err := strconv.Atoi(projectsTTLStr); err == nil {
		CacheTTLProjects = ttl
	} else {
		CacheTTLProjects = 15
	}

	// ── HTTP client (SSL verification disabled) ───────────────────────────────
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	InsecureHTTPClient = &http.Client{Transport: tr}
}

// buildDSN assembles a PostgreSQL DSN from environment variables.
// All values fall back to the defaults that match docker-compose.yml.
func buildDSN() string {
	host := getEnvOrDefault("DB_HOST", "localhost")
	port := getEnvOrDefault("DB_PORT", "5433")
	user := getEnvOrDefault("DB_USER", "scanner")
	password := getEnvOrDefault("DB_PASSWORD", "scanner_password")
	name := getEnvOrDefault("DB_NAME", "scanner_db")
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=UTC",
		host, user, password, name, port,
	)
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

