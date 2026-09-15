package reporter

import (
	"fmt"

	"github.com/jung-kurt/gofpdf"
	"github.com/xuri/excelize/v2"
)

func GenerateCommitsPDF(results []CommitResult, groupID string) (*gofpdf.Fpdf, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, tr("GitLab Commits Report"))
	pdf.Ln(8)
	
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, tr(fmt.Sprintf("Group: %s", groupID)))
	pdf.Ln(6)
	pdf.Cell(40, 6, tr(fmt.Sprintf("Total Commits: %d", len(results))))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 9)
	colWidths := []float64{60, 20, 30, 35, 15, 15, 100}
	headers := []string{"Project", "ID", "Author", "Date", "(+)", "(-)", "Message"}
	
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 8, tr(h), "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 8)
	for _, r := range results {
		pdf.CellFormat(colWidths[0], 8, tr(truncate(r.ProjectName, 35)), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 8, tr(r.ShortID), "1", 0, "C", false, 0, r.WebURL)
		pdf.CellFormat(colWidths[2], 8, tr(truncate(r.AuthorName, 15)), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[3], 8, tr(r.CommittedDate), "1", 0, "C", false, 0, "")
		
		pdf.SetTextColor(0, 128, 0)
		pdf.CellFormat(colWidths[4], 8, tr(fmt.Sprintf("+%d", r.Additions)), "1", 0, "C", false, 0, "")
		pdf.SetTextColor(255, 0, 0)
		pdf.CellFormat(colWidths[5], 8, tr(fmt.Sprintf("-%d", r.Deletions)), "1", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		
		pdf.CellFormat(colWidths[6], 8, tr(truncate(r.Title, 60)), "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}

	return pdf, nil
}

func GenerateCommitsXLSX(results []CommitResult) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"

	headers := []string{"Project", "ID", "Author", "Date", "(+)", "(-)", "Message", "URL"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for i, r := range results {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.ProjectName)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.ShortID)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.AuthorName)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.CommittedDate)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.Additions)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.Deletions)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.Title)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), r.WebURL)
	}

	return f, nil
}

func GenerateComparePDF(results []CompareResult, groupID string) (*gofpdf.Fpdf, error) {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	tr := pdf.UnicodeTranslatorFromDescriptor("")

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(40, 10, tr("GitLab Branch Comparison Report"))
	pdf.Ln(8)
	
	pdf.SetFont("Arial", "", 10)
	pdf.Cell(40, 6, tr(fmt.Sprintf("Group: %s", groupID)))
	pdf.Ln(10)

	pdf.SetFont("Arial", "B", 8)
	colWidths := []float64{50, 20, 20, 12, 12, 12, 12, 12, 18, 110}
	headers := []string{"Project", "Source", "Target", "Ahead", "Behind", "Files", "(+)", "(-)", "Status", "Modified Files"}
	
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 10, tr(h), "1", 0, "C", false, 0, "")
	}
	pdf.Ln(-1)

	pdf.SetFont("Arial", "", 7)
	for _, r := range results {
		pdf.CellFormat(colWidths[0], 10, tr(truncate(r.ProjectName, 35)), "1", 0, "L", false, 0, "")
		pdf.CellFormat(colWidths[1], 10, tr(r.SourceBranch), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[2], 10, tr(r.TargetBranch), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[3], 10, tr(fmt.Sprintf("%d", r.Ahead)), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[4], 10, tr(fmt.Sprintf("%d", r.Behind)), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[5], 10, tr(fmt.Sprintf("%d", r.FilesChanged)), "1", 0, "C", false, 0, "")
		
		pdf.SetTextColor(0, 128, 0)
		pdf.CellFormat(colWidths[6], 10, tr(fmt.Sprintf("+%d", r.Additions)), "1", 0, "C", false, 0, "")
		pdf.SetTextColor(255, 0, 0)
		pdf.CellFormat(colWidths[7], 10, tr(fmt.Sprintf("-%d", r.Deletions)), "1", 0, "C", false, 0, "")
		pdf.SetTextColor(0, 0, 0)
		
		pdf.CellFormat(colWidths[8], 10, tr(r.Status), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colWidths[9], 10, tr(truncate(r.ModifiedFiles, 85)), "1", 0, "L", false, 0, "")
		pdf.Ln(-1)
	}

	return pdf, nil
}

func GenerateCompareXLSX(results []CompareResult) (*excelize.File, error) {
	f := excelize.NewFile()
	sheet := "Sheet1"

	headers := []string{"Project", "Source", "Target", "Ahead", "Behind", "Files", "(+)", "(-)", "Status", "Modified Files"}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	for i, r := range results {
		row := i + 2
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), r.ProjectName)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), r.SourceBranch)
		f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.TargetBranch)
		f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Ahead)
		f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.Behind)
		f.SetCellValue(sheet, fmt.Sprintf("F%d", row), r.FilesChanged)
		f.SetCellValue(sheet, fmt.Sprintf("G%d", row), r.Additions)
		f.SetCellValue(sheet, fmt.Sprintf("H%d", row), r.Deletions)
		f.SetCellValue(sheet, fmt.Sprintf("I%d", row), r.Status)
		f.SetCellValue(sheet, fmt.Sprintf("J%d", row), r.ModifiedFiles)
	}

	return f, nil
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max-3] + "..."
	}
	return s
}
