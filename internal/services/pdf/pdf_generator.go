package pdf

import (
	"fmt"
	"gitlab-code-scan/internal/services/scanner"
	"time"

	"github.com/jung-kurt/gofpdf"
)

func GenerateReport(results []scanner.ScanResult, groupID string) (*gofpdf.Fpdf, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, tr("GitLab Code Scan Report"))
	pdf.Ln(10)

	pdf.SetFont("Arial", "", 12)
	pdf.Cell(40, 10, tr(fmt.Sprintf("Group/Subgroup: %s", groupID)))
	pdf.Ln(8)
	pdf.Cell(40, 10, tr(fmt.Sprintf("Date: %s", time.Now().Format("2006-01-02 15:04:05"))))
	pdf.Ln(8)
	pdf.Cell(40, 10, tr(fmt.Sprintf("Total Matches: %d", len(results))))
	pdf.Ln(15)

	if len(results) == 0 {
		pdf.Cell(40, 10, tr("No keywords found."))
		return pdf, nil
	}

	// Group results by project
	projectResults := make(map[string][]scanner.ScanResult)
	for _, res := range results {
		projectResults[res.ProjectName] = append(projectResults[res.ProjectName], res)
	}

	for projectName, projRes := range projectResults {
		pdf.SetFont("Arial", "B", 14)
		pdf.SetTextColor(0, 51, 102) // Dark blue
		pdf.CellFormat(0, 10, tr(projectName), "", 1, "L", false, 0, "")
		
		pdf.SetFont("Arial", "I", 10)
		pdf.SetTextColor(0, 0, 255)
		// Add link to project
		pdf.CellFormat(0, 6, tr(projRes[0].ProjectURL), "", 1, "L", false, 0, projRes[0].ProjectURL)
		pdf.SetTextColor(0, 0, 0)
		pdf.Ln(4)

		for _, res := range projRes {
			pdf.SetFont("Arial", "B", 11)
			pdf.CellFormat(0, 6, tr(fmt.Sprintf("File: %s (Line: %d)", res.Filename, res.LineNumber)), "", 1, "L", false, 0, "")
			
			pdf.SetFont("Arial", "", 10)
			pdf.CellFormat(0, 6, tr(fmt.Sprintf("Keyword: %s", res.KeywordFound)), "", 1, "L", false, 0, "")
			
			pdf.SetFont("Courier", "", 9)
			
			// Replace newlines and tabs so they render safely in PDF
			snippet := res.LineContent
			pdf.MultiCell(0, 5, tr(fmt.Sprintf("> %s", snippet)), "", "L", false)
			
			pdf.SetFont("Arial", "U", 9)
			pdf.SetTextColor(0, 0, 255)
			pdf.CellFormat(0, 6, tr("View in GitLab"), "", 1, "L", false, 0, res.DeepLink)
			pdf.SetTextColor(0, 0, 0)
			pdf.Ln(4)
		}
		pdf.Ln(5)
	}

	return pdf, nil
}
