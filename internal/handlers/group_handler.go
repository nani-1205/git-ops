package handlers

import (
	"net/http"

	"gitlab-code-scan/internal/services/scanner"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"gitlab.com/gitlab-org/api/client-go"
)

func ListGroupsHandler(c *gin.Context) {
	session := sessions.Default(c)
	token := session.Get("access_token").(string)

	client, err := scanner.GetClient(token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create gitlab client"})
		return
	}

	opt := &gitlab.ListGroupsOptions{
		MinAccessLevel: gitlab.Ptr(gitlab.ReporterPermissions), // Or Guest, depending on what's needed
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

	c.JSON(http.StatusOK, gin.H{"groups": allGroups})
}

func ListGroupProjectsHandler(c *gin.Context) {
	groupID := c.Param("id")
	if groupID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Group ID is required"})
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

	c.JSON(http.StatusOK, gin.H{"projects": allProjects})
}
