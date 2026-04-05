package exporter

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"sql-query/internal/parser"
)

// JSONExporter exports data as a JSON array of objects.
type JSONExporter struct {
	outputPath string // empty means stdout
}

// NewJSONExporter creates a new JSONExporter. Pass "" for stdout.
func NewJSONExporter(outputPath string) *JSONExporter {
	return &JSONExporter{outputPath: outputPath}
}

// Export writes columns and data as JSON.
// Keys use Column.DisplayName. null cells become JSON null.
func (e *JSONExporter) Export(columns []*parser.Column, data [][]*string) error {
	var w io.Writer
	if e.outputPath != "" {
		f, err := os.Create(e.outputPath)
		if err != nil {
			return fmt.Errorf("创建输出文件失败: %w", err)
		}
		defer f.Close()
		w = f
	} else {
		w = os.Stdout
	}

	rows := make([]map[string]interface{}, 0, len(data))
	for _, row := range data {
		obj := make(map[string]interface{}, len(columns))
		for i, col := range columns {
			if i < len(row) && row[i] != nil {
				obj[col.DisplayName] = *row[i]
			} else {
				obj[col.DisplayName] = nil
			}
		}
		rows = append(rows, obj)
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rows); err != nil {
		return fmt.Errorf("JSON 编码失败: %w", err)
	}
	return nil
}
