package exporter

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"os"

	"sql-query/internal/parser"
)

//go:embed templates/report.tmpl
var templateFS embed.FS

// HTMLExporter exports data to an HTML file.
type HTMLExporter struct {
	outputPath string
}

// NewHTMLExporter creates a new HTMLExporter.
func NewHTMLExporter(outputPath string) *HTMLExporter {
	return &HTMLExporter{outputPath: outputPath}
}

// ColumnJSON is used for JSON serialization in the template.
type ColumnJSON struct {
	Name        string                        `json:"name"`
	DisplayName string                        `json:"displayName"`
	Meta        map[string]*parser.ColumnMeta `json:"meta,omitempty"`
}

// TemplateData is passed to the HTML template.
type TemplateData struct {
	ColumnsJSON template.JS
	DataJSON    template.JS
}

// Export writes columns and data to an HTML file.
func (e *HTMLExporter) Export(columns []*parser.Column, data [][]*string) error {
	columnsJSON := make([]ColumnJSON, len(columns))
	for i, col := range columns {
		columnsJSON[i] = ColumnJSON{
			Name:        col.RawName,
			DisplayName: col.DisplayName,
			Meta:        col.Meta,
		}
	}

	colBytes, err := json.Marshal(columnsJSON)
	if err != nil {
		return fmt.Errorf("序列化列数据失败: %w", err)
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化行数据失败: %w", err)
	}

	tmplData := TemplateData{
		ColumnsJSON: template.JS(colBytes),
		DataJSON:    template.JS(dataBytes),
	}

	tmplContent, err := templateFS.ReadFile("templates/report.tmpl")
	if err != nil {
		return fmt.Errorf("读取模板失败: %w", err)
	}

	tmpl, err := template.New("report").Parse(string(tmplContent))
	if err != nil {
		return fmt.Errorf("解析模板失败: %w", err)
	}

	f, err := os.Create(e.outputPath)
	if err != nil {
		return fmt.Errorf("创建输出文件失败: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, tmplData); err != nil {
		return fmt.Errorf("渲染模板失败: %w", err)
	}

	return nil
}
