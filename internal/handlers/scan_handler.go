package handlers

import (
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

	results, err := scanner.ScanProjects(req.ProjectIDs, keywordsList, req.Branch, token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Scan failed: " + err.Error()})
		return
	}

	// Save history (save the first project ID as a reference or a joined string)
	var groupIDStr string
	if len(req.ProjectIDs) > 0 {
		groupIDStr = fmt.Sprintf("projects-%d", len(req.ProjectIDs))
	}
	history := models.ScanHistory{
		UserID:     userID,
		GroupID:    groupIDStr,
		Keywords:   req.Keywords,
		Branch:     req.Branch,
		MatchCount: len(results),
	}
	repository.DB.Create(&history)

	c.JSON(http.StatusOK, gin.H{"results": results})
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
