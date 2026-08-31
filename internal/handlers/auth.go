package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"

	"gitlab-code-scan/internal/config"
	"gitlab-code-scan/internal/models"
	"gitlab-code-scan/internal/repository"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

// Generate a random state string
func generateStateOauthCookie(c *gin.Context) string {
	b := strToBytes(16)
	state := base64.URLEncoding.EncodeToString(b)
	session := sessions.Default(c)
	session.Set("oauthstate", state)
	session.Save()
	return state
}

func strToBytes(n int) []byte {
	b := make([]byte, n)
	rand.Read(b)
	return b
}

func LoginHandler(c *gin.Context) {
	oauthState := generateStateOauthCookie(c)
	// We want to force prompt? Usually GitLab handles it.
	u := config.OAuthConfig.AuthCodeURL(oauthState)
	c.Redirect(http.StatusTemporaryRedirect, u)
}

func CallbackHandler(c *gin.Context) {
	session := sessions.Default(c)
	oauthState := session.Get("oauthstate")

	if c.Query("state") != oauthState {
		c.String(http.StatusBadRequest, "Invalid OAuth state")
		return
	}

	code := c.Query("code")
	// Important: Use the insecure HTTP client for token exchange because GitLab might be self-hosted with invalid SSL
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, config.InsecureHTTPClient)
	
	token, err := config.OAuthConfig.Exchange(ctx, code)
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to exchange token: "+err.Error())
		return
	}

	// Fetch user info from GitLab
	client := config.OAuthConfig.Client(ctx, token)
	resp, err := client.Get(config.GitLabURL + "/api/v4/user")
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to get user info: "+err.Error())
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var gitlabUser struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	json.Unmarshal(body, &gitlabUser)

	// Save or update user in DB
	var user models.User
	result := repository.DB.Where("git_lab_id = ?", gitlabUser.ID).First(&user)
	if result.Error != nil {
		// Create new user
		user = models.User{
			GitLabID:  gitlabUser.ID,
			Username:  gitlabUser.Username,
			Email:     gitlabUser.Email,
			AvatarURL: gitlabUser.AvatarURL,
		}
		repository.DB.Create(&user)
	} else {
		// Update user info
		user.Username = gitlabUser.Username
		user.Email = gitlabUser.Email
		user.AvatarURL = gitlabUser.AvatarURL
		repository.DB.Save(&user)
	}

	// Save token in session so we can act on behalf of the user
	session.Set("user_id", user.ID)
	session.Set("access_token", token.AccessToken)
	session.Save()

	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func LogoutHandler(c *gin.Context) {
	session := sessions.Default(c)
	session.Clear()
	session.Save()
	c.Redirect(http.StatusTemporaryRedirect, "/")
}

func CurrentUserHandler(c *gin.Context) {
	session := sessions.Default(c)
	userID := session.Get("user_id")
	if userID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	var user models.User
	if err := repository.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"username":   user.Username,
		"avatar_url": user.AvatarURL,
	})
}
