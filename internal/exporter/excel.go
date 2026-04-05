package exporter

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"

	"sql-query/internal/parser"
)

// ExcelExporter exports data to an Excel file.
type ExcelExporter struct {
	outputPath string
}

// NewExcelExporter creates a new ExcelExporter.
func NewExcelExporter(outputPath string) *ExcelExporter {
	return &ExcelExporter{outputPath: outputPath}
}

// Export writes columns and data to an Excel file.
func (e *ExcelExporter) Export(columns []*parser.Column, data [][]*string) error {
	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Sheet1"

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E0E0"}, Pattern: 1},
		Border: []excelize.Border{{Type: "bottom", Color: "#000000", Style: 1}},
	})
	if err != nil {
		return fmt.Errorf("创建表头样式失败: %w", err)
	}

	linkStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Color: "#0563C1", Underline: "single"},
	})
	if err != nil {
		return fmt.Errorf("创建超链接样式失败: %w", err)
	}

	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheetName, cell, col.DisplayName)
		f.SetCellStyle(sheetName, cell, cell, headerStyle)
	}

	for rowIdx, row := range data {
		excelRow := rowIdx + 2
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, excelRow)
			if value == nil {
				f.SetCellValue(sheetName, cell, "")
				continue
			}
			col := columns[colIdx]
			if col.HasMeta("URL") || isURL(*value) {
				f.SetCellValue(sheetName, cell, *value)
				f.SetCellHyperLink(sheetName, cell, *value, "External")
				f.SetCellStyle(sheetName, cell, cell, linkStyle)
			} else {
				f.SetCellValue(sheetName, cell, *value)
			}
		}
	}

	for i := range columns {
		colName, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheetName, colName, colName, 20)
	}

	if err := f.SaveAs(e.outputPath); err != nil {
		return fmt.Errorf("保存 Excel 文件失败: %w", err)
	}
	return nil
}

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}
