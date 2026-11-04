package handlers

import (
	"log"
	"net/http"
	"time"

	"encoding/json"
	"fmt"

	"gitlab-code-scan/internal/config"
	"gitlab-code-scan/internal/repository"
	"gitlab-code-scan/internal/services/scanner"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// ---------------------------------------------------------------------------
// Cache: group projects (keyed by group ID)
// ---------------------------------------------------------------------------

func getCachedGroupProjects(groupID string) ([]*gitlab.Project, bool) {
	if repository.RedisClient == nil {
		return nil, false
	}
	key := fmt.Sprintf("gitlab_projects_%s", groupID)
	val, err := repository.RedisClient.Get(repository.Ctx, key).Result()
	if err == redis.Nil || err != nil {
		return nil, false
	}

	var projects []*gitlab.Project
	if err := json.Unmarshal([]byte(val), &projects); err != nil {
		return nil, false
	}
	return projects, true
}

func setCachedGroupProjects(groupID string, projects []*gitlab.Project) {
	if repository.RedisClient == nil {
		return
	}
	key := fmt.Sprintf("gitlab_projects_%s", groupID)
	data, err := json.Marshal(projects)
	if err != nil {
		return
	}
	ttl := time.Duration(config.CacheTTLProjects) * time.Minute
	repository.RedisClient.Set(repository.Ctx, key, data, ttl)
}

// ---------------------------------------------------------------------------
// Cache: groups list (keyed by user ID — one list per user)
// ---------------------------------------------------------------------------

func getCachedGroupList(userID uint) ([]*gitlab.Group, bool) {
	if repository.RedisClient == nil {
		return nil, false
	}
	key := fmt.Sprintf("gitlab_groups_user_%d", userID)
	val, err := repository.RedisClient.Get(repository.Ctx, key).Result()
	if err == redis.Nil || err != nil {
		return nil, false
	}

	var groups []*gitlab.Group
	if err := json.Unmarshal([]byte(val), &groups); err != nil {
		return nil, false
	}
	return groups, true
}

func setCachedGroupList(userID uint, groups []*gitlab.Group) {
	if repository.RedisClient == nil {
		return
	}
	key := fmt.Sprintf("gitlab_groups_user_%d", userID)
	data, err := json.Marshal(groups)
	if err != nil {
		return
	}
	ttl := time.Duration(config.CacheTTLGroups) * time.Minute
	repository.RedisClient.Set(repository.Ctx, key, data, ttl)
}

func ListGroupsHandler(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("access_token").(string)
	
	rawUserID := session.Get("user_id")
	if rawUserID == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}
	userID := rawUserID.(uint)

	// Serve from cache (keyed per user ID).
	if cached, ok := getCachedGroupList(userID); ok {
		log.Printf("📦 [CACHE] HIT  groups list (%d groups)", len(cached))
		c.JSON(http.StatusOK, gin.H{"groups": cached})
		return
	}

	client, err := scanner.GetClient(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gitlab client"})
		return
	}

	opt := &gitlab.ListGroupsOptions{
		MinAccessLevel: gitlab.Ptr(gitlab.ReporterPermissions),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	var allGroups []*gitlab.Group
	for {
		groups, resp, err := client.Groups.ListGroups(opt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list groups: " + err.Error()})
			return
		}
		allGroups = append(allGroups, groups...)
		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opt.Page = resp.NextPage
	}

	log.Printf("🌐 [CACHE] MISS groups list — fetched %d groups from GitLab, caching for %d minutes",
		len(allGroups), config.CacheTTLGroups)
	setCachedGroupList(userID, allGroups)

	c.JSON(http.StatusOK, gin.H{"groups": allGroups})
}

func ListGroupProjectsHandler(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
		return
	}

	// Serve from cache if available.
	if cached, ok := getCachedGroupProjects(groupID); ok {
		log.Printf("📦 [CACHE] HIT  group=%s (%d projects)", groupID, len(cached))
		c.JSON(http.StatusOK, gin.H{"projects": cached})
		return
	}

	session := sessions.Default(c)
	token := session.Get("access_token").(string)

	client, err := scanner.GetClient(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gitlab client"})
		return
	}

	opt := &gitlab.ListGroupProjectsOptions{
		IncludeSubGroups: gitlab.Ptr(false),
		ListOptions: gitlab.ListOptions{
			PerPage: 100,
			Page:    1,
		},
	}

	var allProjects []*gitlab.Project
	for {
		projects, resp, err := client.Groups.ListGroupProjects(groupID, opt)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list group projects"})
			return
		}
		allProjects = append(allProjects, projects...)
		if resp.CurrentPage >= resp.TotalPages {
			break
		}
		opt.Page = resp.NextPage
	}

	log.Printf("🌐 [CACHE] MISS group=%s — fetched %d projects, caching for %d minutes",
		groupID, len(allProjects), config.CacheTTLProjects)
	setCachedGroupProjects(groupID, allProjects)

	c.JSON(http.StatusOK, gin.H{"projects": allProjects})
}
