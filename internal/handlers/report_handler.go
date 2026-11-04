package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab-code-scan/internal/models"
	"gitlab-code-scan/internal/repository"
	"gitlab-code-scan/internal/services/reporter"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

type CommitsRequest struct {
	ProjectIDs []int  `json:"project_ids" binding:"required"`
	Branch     string `json:"branch" binding:"required"`
	StartDate  string `json:"start_date" binding:"required"`
	EndDate    string `json:"end_date" binding:"required"`
}

type CompareRequest struct {
	ProjectIDs   []int  `json:"project_ids" binding:"required"`
	SourceBranch string `json:"source_branch" binding:"required"`
	TargetBranch string `json:"target_branch" binding:"required"`
}

func ReportCommitsHandler(c *gin.Context) {
	var req CommitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := sessions.Default(c)
	token := session.Get("access_token").(string)

	job := models.ScanJob{
		JobType: "commits",
		Status:  "running",
	}
	if err := repository.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	go func(jobID uint, projectIDs []int, branch, start, end, token string) {
		results, err := reporter.GenerateCommitReport(projectIDs, branch, start, end, token)

		var update models.ScanJob
		if dbErr := repository.DB.First(&update, jobID).Error; dbErr == nil {
			if err != nil {
				update.Status = "failed"
				update.ErrorMessage = err.Error()
			} else {
				update.Status = "completed"
				if resultsJSON, jErr := json.Marshal(results); jErr == nil {
					update.Results = string(resultsJSON)
				}
			}
			repository.DB.Save(&update)
		}
	}(job.ID, req.ProjectIDs, req.Branch, req.StartDate, req.EndDate, token)

	c.JSON(http.StatusOK, gin.H{"job_id": job.ID})
}

func ReportCompareHandler(c *gin.Context) {
	var req CompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	session := sessions.Default(c)
	token := session.Get("access_token").(string)

	job := models.ScanJob{
		JobType: "compare",
		Status:  "running",
	}
	if err := repository.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create job"})
		return
	}

	go func(jobID uint, projectIDs []int, source, target, token string) {
		results, err := reporter.CompareBranches(projectIDs, source, target, token)

		var update models.ScanJob
		if dbErr := repository.DB.First(&update, jobID).Error; dbErr == nil {
			if err != nil {
				update.Status = "failed"
				update.ErrorMessage = err.Error()
			} else {
				update.Status = "completed"
				if resultsJSON, jErr := json.Marshal(results); jErr == nil {
					update.Results = string(resultsJSON)
				}
			}
			repository.DB.Save(&update)
		}
	}(job.ID, req.ProjectIDs, req.SourceBranch, req.TargetBranch, token)

	c.JSON(http.StatusOK, gin.H{"job_id": job.ID})
}

type ExportCommitsRequest struct {
	GroupID string                  `json:"group_id" binding:"required"`
	Format  string                  `json:"format" binding:"required"` // pdf or xlsx
	Results []reporter.CommitResult `json:"results" binding:"required"`
}

func ExportCommitsHandler(c *gin.Context) {
	var req ExportCommitsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Format == "pdf" {
		pdfDoc, err := reporter.GenerateCommitsPDF(req.Results, req.GroupID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF generation failed"})
			return
		}
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"commits_%s.pdf\"", req.GroupID))
		pdfDoc.Output(c.Writer)
	} else if req.Format == "xlsx" {
		f, err := reporter.GenerateCommitsXLSX(req.Results)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "XLSX generation failed"})
			return
		}
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"commits_%s.xlsx\"", req.GroupID))
		f.Write(c.Writer)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format"})
	}
}

type ExportCompareRequest struct {
	GroupID string                   `json:"group_id" binding:"required"`
	Format  string                   `json:"format" binding:"required"` // pdf or xlsx
	Results []reporter.CompareResult `json:"results" binding:"required"`
}

func ExportCompareHandler(c *gin.Context) {
	var req ExportCompareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Format == "pdf" {
		pdfDoc, err := reporter.GenerateComparePDF(req.Results, req.GroupID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "PDF generation failed"})
			return
		}
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"compare_%s.pdf\"", req.GroupID))
		pdfDoc.Output(c.Writer)
	} else if req.Format == "xlsx" {
		f, err := reporter.GenerateCompareXLSX(req.Results)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "XLSX generation failed"})
			return
		}
		c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"compare_%s.xlsx\"", req.GroupID))
		f.Write(c.Writer)
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format"})
	}
}
