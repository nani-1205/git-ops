package handlers

import (
	"log"
	"net/http"
	"sync"
	"time"

	"gitlab-code-scan/internal/services/scanner"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// ---------------------------------------------------------------------------
// Simple TTL cache for group projects — avoids repeated GitLab API calls when
// the user clicks multiple checkboxes in quick succession.
// ---------------------------------------------------------------------------

const (
	groupProjectsCacheTTL = 5 * time.Minute
	groupListCacheTTL     = 10 * time.Minute
)

// ---------------------------------------------------------------------------
// Cache: group projects (keyed by group ID)
// ---------------------------------------------------------------------------

type groupProjectsCacheEntry struct {
	projects  []*gitlab.Project
	fetchedAt time.Time
}

var (
	groupProjectsCache   = map[string]*groupProjectsCacheEntry{}
	groupProjectsCacheMu sync.RWMutex
)

func getCachedGroupProjects(groupID string) ([]*gitlab.Project, bool) {
	groupProjectsCacheMu.RLock()
	defer groupProjectsCacheMu.RUnlock()
	entry, ok := groupProjectsCache[groupID]
	if !ok || time.Since(entry.fetchedAt) > groupProjectsCacheTTL {
		return nil, false
	}
	return entry.projects, true
}

func setCachedGroupProjects(groupID string, projects []*gitlab.Project) {
	groupProjectsCacheMu.Lock()
	defer groupProjectsCacheMu.Unlock()
	groupProjectsCache[groupID] = &groupProjectsCacheEntry{
		projects:  projects,
		fetchedAt: time.Now(),
	}
}

// ---------------------------------------------------------------------------
// Cache: groups list (keyed by token — one list per user)
// ---------------------------------------------------------------------------

type groupListCacheEntry struct {
	groups    []*gitlab.Group
	fetchedAt time.Time
}

var (
	groupListCache   = map[string]*groupListCacheEntry{}
	groupListCacheMu sync.RWMutex
)

func getCachedGroupList(token string) ([]*gitlab.Group, bool) {
	groupListCacheMu.RLock()
	defer groupListCacheMu.RUnlock()
	entry, ok := groupListCache[token]
	if !ok || time.Since(entry.fetchedAt) > groupListCacheTTL {
		return nil, false
	}
	return entry.groups, true
}

func setCachedGroupList(token string, groups []*gitlab.Group) {
	groupListCacheMu.Lock()
	defer groupListCacheMu.Unlock()
	groupListCache[token] = &groupListCacheEntry{
		groups:    groups,
		fetchedAt: time.Now(),
	}
}

func ListGroupsHandler(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("access_token").(string)

	// Serve from cache (keyed per user token).
	if cached, ok := getCachedGroupList(token); ok {
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

	log.Printf("🌐 [CACHE] MISS groups list — fetched %d groups from GitLab, caching for %s",
		len(allGroups), groupListCacheTTL)
	setCachedGroupList(token, allGroups)

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

	log.Printf("🌐 [CACHE] MISS group=%s — fetched %d projects from GitLab, caching for %s",
		groupID, len(allProjects), groupProjectsCacheTTL)
	setCachedGroupProjects(groupID, allProjects)

	c.JSON(http.StatusOK, gin.H{"projects": allProjects})
}
