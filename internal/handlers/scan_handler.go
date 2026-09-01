package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"gitlab-code-scan/internal/models"
	"gitlab-code-scan/internal/repository"
	"gitlab-code-scan/internal/services/pdf"
	"gitlab-code-scan/internal/services/scanner"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		session := sessions.Default(c)
		token := session.Get("access_token")
		if token == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized. Please log in."})
			return
		}
		c.Next()
	}
}

type ScanRequest struct {
	ProjectIDs []int  `json:"project_ids" binding:"required"`
	Keywords   string `json:"keywords" binding:"required"` // comma separated
	Branch     string `json:"branch"`
}

func ScanHandler(c *gin.Context) {
	var req ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := sessions.Default(c)
	token := session.Get("access_token").(string)
	userID := session.Get("user_id").(uint)

	keywordsList := strings.Split(req.Keywords, ",")
	for i := range keywordsList {
		keywordsList[i] = strings.TrimSpace(keywordsList[i])
	}

	// Create job in DB
	job := models.ScanJob{
		Status: "running",
	}
	if err := repository.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	go func(jobID uint, projectIDs []int, keywords []string, branch string, token string, uID uint) {
		results, scanErr := scanner.ScanProjects(projectIDs, keywords, branch, token)

		var update models.ScanJob
		if dbErr := repository.DB.First(&update, jobID).Error; dbErr == nil {
			if scanErr != nil {
				update.Status = "failed"
				update.ErrorMessage = scanErr.Error()
			} else {
				update.Status = "completed"
				if resultsJSON, jErr := json.Marshal(results); jErr == nil {
					update.Results = string(resultsJSON)
				}
			}
			repository.DB.Save(&update)
		}

		if scanErr == nil {
			// Save history
			var groupIDStr string
			if len(projectIDs) > 0 {
				groupIDStr = fmt.Sprintf("projects-%d", len(projectIDs))
			}
			history := models.ScanHistory{
				UserID:     uID,
				GroupID:    groupIDStr,
				Keywords:   strings.Join(keywords, ","),
				Branch:     branch,
				MatchCount: len(results),
			}
			repository.DB.Create(&history)
		}

	}(job.ID, req.ProjectIDs, keywordsList, req.Branch, token, userID)

	c.JSON(http.StatusOK, gin.H{"job_id": job.ID})
}

func JobStatusHandler(c *gin.Context) {
	jobID := c.Param("id")
	var job models.ScanJob
	if err := repository.DB.First(&job, jobID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
		return
	}

	if job.Status == "completed" {
		var results []scanner.ScanResult
		json.Unmarshal([]byte(job.Results), &results)
		c.JSON(http.StatusOK, gin.H{
			"status":  job.Status,
			"results": results,
		})
	} else if job.Status == "failed" {
		c.JSON(http.StatusOK, gin.H{
			"status": job.Status,
			"error":  job.ErrorMessage,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"status": job.Status,
		})
	}
}

type ExportRequest struct {
	GroupID string               `json:"group_id" binding:"required"`
	Results []scanner.ScanResult `json:"results" binding:"required"`
}

func ExportPDFHandler(c *gin.Context) {
	var req ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pdfDoc, err := pdf.GenerateReport(req.Results, req.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF generation failed: " + err.Error()})
		return
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"scan_report_%s.pdf\"", req.GroupID))

	if err := pdfDoc.Output(c.Writer); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send PDF: " + err.Error()})
	}
}
