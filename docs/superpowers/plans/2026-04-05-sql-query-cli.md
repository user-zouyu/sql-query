# sql-query CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a CLI tool with three subcommands (tables, table, query) for MySQL schema inspection and SQL query export, designed for AI agent consumption.

**Architecture:** Cobra-based CLI with shared DB connection via PersistentPreRunE. Modules migrated from sql-exporter with PostgreSQL removed, logs redirected to stderr. Phase 1 excludes S3 presigning — the processor/s3 packages are deferred.

**Tech Stack:** Go 1.23, Cobra, GORM (MySQL only), excelize/v2, godotenv, html/template (embed)

---

## File Structure

```
sql-query/
├── main.go                              # Entry point — calls cmd.Execute()
├── go.mod / go.sum                      # Module definition
├── cmd/
│   ├── root.go                          # Root command, PersistentFlags (-e, --json), PersistentPreRunE (load config + connect DB)
│   ├── tables.go                        # `tables` subcommand — list all tables
│   ├── table.go                         # `table <name>` subcommand — show DDL
│   └── query.go                         # `query` subcommand — execute SQL + export
├── internal/
│   ├── config/
│   │   └── config.go                    # Load DB_DSN + QUERY_TIMEOUT from env
│   ├── db/
│   │   ├── connect.go                   # MySQL-only GORM connection
│   │   ├── execute.go                   # SQL execution with timeout
│   │   ├── tables.go                    # GetTables() — INFORMATION_SCHEMA query
│   │   └── ddl.go                       # GetTableDDL() — DDL + columns + indexes
│   ├── parser/
│   │   └── meta.go                      # Metadata protocol parser (from sql-exporter)
│   ├── exporter/
│   │   ├── exporter.go                  # Exporter interface
│   │   ├── excel.go                     # Excel exporter (from sql-exporter)
│   │   ├── html.go                      # HTML exporter (from sql-exporter)
│   │   ├── json.go                      # JSON exporter (new)
│   │   └── templates/
│   │       └── report.tmpl              # HTML template (from sql-exporter)
│   └── errutil/
│       └── errutil.go                   # Exit codes, JSON error output helper
└── .env.example                         # Example config
```

---

### Task 1: Project Init + Config Module

**Files:**
- Create: `go.mod`
- Create: `internal/config/config.go`
- Create: `.env.example`

- [ ] **Step 1: Initialize Go module and add dependencies**

```bash
cd /Users/mac/workspaces/github/sql-query
go mod init sql-query
go get github.com/spf13/cobra@v1.8.1
go get gorm.io/gorm@v1.25.12
go get gorm.io/driver/mysql@v1.5.7
go get github.com/xuri/excelize/v2@v2.9.0
go get github.com/joho/godotenv@v1.5.1
```

- [ ] **Step 2: Create .env.example**

```bash
# .env.example
# 数据库连接（必填）
DB_DSN=user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local

# 查询超时（秒，默认 300）
# QUERY_TIMEOUT=300
```

- [ ] **Step 3: Write config.go**

```go
// internal/config/config.go
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds application configuration loaded from environment variables.
type Config struct {
	DBDSN        string
	QueryTimeout int // seconds, default 300
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		DBDSN:        os.Getenv("DB_DSN"),
		QueryTimeout: 300,
	}

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("环境变量 DB_DSN 未配置")
	}

	if v := os.Getenv("QUERY_TIMEOUT"); v != "" {
		t, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("QUERY_TIMEOUT 格式无效: %s", v)
		}
		cfg.QueryTimeout = t
	}

	return cfg, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add go.mod go.sum internal/config/config.go .env.example
git commit -m "feat: project init with config module"
```

---

### Task 2: Error Utilities

**Files:**
- Create: `internal/errutil/errutil.go`

- [ ] **Step 1: Write errutil.go**

```go
// internal/errutil/errutil.go
package errutil

import (
	"encoding/json"
	"fmt"
	"os"
)

// Exit codes
const (
	ExitOK             = 0
	ExitGenericError   = 1
	ExitConnectFailed  = 2
	ExitTableNotFound  = 3
)

// ErrorResponse is the JSON error structure written to stdout in --json mode.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Exit prints the error to stderr, optionally writes JSON to stdout, and exits.
func Exit(code int, errorCode, message string, jsonMode bool) {
	fmt.Fprintln(os.Stderr, "Error:", message)
	if jsonMode {
		resp := ErrorResponse{Error: errorCode, Message: message}
		enc := json.NewEncoder(os.Stdout)
		enc.SetEscapeHTML(false)
		enc.Encode(resp)
	}
	os.Exit(code)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/errutil/errutil.go
git commit -m "feat: add error utilities with exit codes and JSON error output"
```

---

### Task 3: Database Connection + Execute

**Files:**
- Create: `internal/db/connect.go`
- Create: `internal/db/execute.go`

- [ ] **Step 1: Write connect.go**

```go
// internal/db/connect.go
package db

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect opens a MySQL connection via GORM.
func Connect(dsn string) (*gorm.DB, error) {
	return gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
}
```

- [ ] **Step 2: Write execute.go**

This is migrated from `sql-exporter/internal/db/database.go` Execute function, with logging changed to stderr and timeout support added.

```go
// internal/db/execute.go
package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"gorm.io/gorm"
)

// Execute runs a SQL query and returns column names and rows.
// Each cell is *string (nil = SQL NULL). timeoutSec <= 0 means no timeout.
func Execute(db *gorm.DB, sqlContent string, timeoutSec int) ([]string, [][]*string, error) {
	var rows *sql.Rows
	var err error

	if timeoutSec > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
		defer cancel()
		rows, err = db.WithContext(ctx).Raw(sqlContent).Rows()
	} else {
		rows, err = db.Raw(sqlContent).Rows()
	}
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var data [][]*string
	rowCount := 0
	for rows.Next() {
		values := make([]sql.NullString, len(columns))
		scanArgs := make([]interface{}, len(columns))
		for i := range values {
			scanArgs[i] = &values[i]
		}

		if err := rows.Scan(scanArgs...); err != nil {
			return nil, nil, err
		}

		row := make([]*string, len(columns))
		for i, v := range values {
			if v.Valid {
				s := v.String
				row[i] = &s
			}
		}
		data = append(data, row)

		rowCount++
		if rowCount%1000 == 0 {
			fmt.Fprintf(os.Stderr, "  已读取 %d 行...\n", rowCount)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return columns, data, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/db/connect.go internal/db/execute.go
git commit -m "feat: add MySQL connection and SQL execution with timeout"
```

---

### Task 4: Tables Query (GetTables)

**Files:**
- Create: `internal/db/tables.go`

- [ ] **Step 1: Write tables.go**

```go
// internal/db/tables.go
package db

import "gorm.io/gorm"

// TableInfo represents a row from INFORMATION_SCHEMA.TABLES.
type TableInfo struct {
	TableName    string `json:"table_name" gorm:"column:table_name"`
	TableComment string `json:"table_comment" gorm:"column:table_comment"`
}

// GetTables lists all base tables in the current database.
func GetTables(db *gorm.DB) ([]TableInfo, error) {
	var tables []TableInfo
	err := db.Raw(`
		SELECT TABLE_NAME AS table_name,
		       COALESCE(TABLE_COMMENT, '') AS table_comment
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE()
		  AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`).Scan(&tables).Error
	return tables, err
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/db/tables.go
git commit -m "feat: add GetTables for listing database tables"
```

---

### Task 5: DDL Query (GetTableDDL)

**Files:**
- Create: `internal/db/ddl.go`

- [ ] **Step 1: Write ddl.go**

```go
// internal/db/ddl.go
package db

import (
	"fmt"

	"gorm.io/gorm"
)

// ColumnInfo represents a column definition from INFORMATION_SCHEMA.COLUMNS.
type ColumnInfo struct {
	Name     string  `json:"name" gorm:"column:COLUMN_NAME"`
	Type     string  `json:"type" gorm:"column:COLUMN_TYPE"`
	Nullable string  `json:"nullable" gorm:"column:IS_NULLABLE"`
	Default  *string `json:"default" gorm:"column:COLUMN_DEFAULT"`
	Comment  string  `json:"comment" gorm:"column:COLUMN_COMMENT"`
}

// IndexInfo represents an index from INFORMATION_SCHEMA.STATISTICS.
type IndexInfo struct {
	Name      string `json:"name" gorm:"column:INDEX_NAME"`
	Columns   string `json:"columns" gorm:"column:COLUMNS"`
	IsUnique  bool   `json:"is_unique" gorm:"column:IS_UNIQUE"`
	IsPrimary bool   `json:"is_primary" gorm:"column:IS_PRIMARY"`
	IndexType string `json:"index_type" gorm:"column:INDEX_TYPE"`
}

// TableDDL holds the full DDL and structured metadata for a table.
type TableDDL struct {
	TableName string       `json:"table_name"`
	Comment   string       `json:"comment,omitempty"`
	RawDDL    string       `json:"ddl"`
	Columns   []ColumnInfo `json:"columns"`
	Indexes   []IndexInfo  `json:"indexes"`
}

// GetTableDDL retrieves DDL, columns, and indexes for a given table.
// Returns (nil, nil) — not an error — is impossible; table existence is checked first.
func GetTableDDL(db *gorm.DB, tableName string) (*TableDDL, error) {
	// Check table exists
	var count int64
	err := db.Raw(
		"SELECT COUNT(*) FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?",
		tableName,
	).Scan(&count).Error
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, nil // caller should treat nil result as "table not found"
	}

	result := &TableDDL{TableName: tableName}

	// 1. Raw DDL
	var createRow struct {
		Table       string `gorm:"column:Table"`
		CreateTable string `gorm:"column:Create Table"`
	}
	err = db.Raw(fmt.Sprintf("SHOW CREATE TABLE `%s`", tableName)).Scan(&createRow).Error
	if err != nil {
		return nil, fmt.Errorf("SHOW CREATE TABLE 失败: %w", err)
	}
	result.RawDDL = createRow.CreateTable

	// 2. Columns
	err = db.Raw(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_COMMENT
		FROM INFORMATION_SCHEMA.COLUMNS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`, tableName).Scan(&result.Columns).Error
	if err != nil {
		return nil, fmt.Errorf("查询列信息失败: %w", err)
	}

	// 3. Indexes
	err = db.Raw(`
		SELECT INDEX_NAME,
		       GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS COLUMNS,
		       NOT NON_UNIQUE AS IS_UNIQUE,
		       INDEX_NAME = 'PRIMARY' AS IS_PRIMARY,
		       INDEX_TYPE
		FROM INFORMATION_SCHEMA.STATISTICS
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
		GROUP BY INDEX_NAME, NON_UNIQUE, INDEX_TYPE
	`, tableName).Scan(&result.Indexes).Error
	if err != nil {
		return nil, fmt.Errorf("查询索引信息失败: %w", err)
	}

	// 4. Table comment
	var comment string
	err = db.Raw(`
		SELECT COALESCE(TABLE_COMMENT, '')
		FROM INFORMATION_SCHEMA.TABLES
		WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
	`, tableName).Scan(&comment).Error
	if err != nil {
		return nil, fmt.Errorf("查询表注释失败: %w", err)
	}
	result.Comment = comment

	return result, nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/db/ddl.go
git commit -m "feat: add GetTableDDL for DDL and structured schema info"
```

---

### Task 6: Metadata Parser (migrate from sql-exporter)

**Files:**
- Create: `internal/parser/meta.go`

- [ ] **Step 1: Copy and adapt parser/meta.go**

This is a direct migration from `sql-exporter/internal/parser/meta.go` — the only change is the module import path (no internal imports to change). Copy it verbatim:

```go
// internal/parser/meta.go
package parser

import (
	"regexp"
	"strings"
)

// ColumnMeta holds structured metadata parsed from a column alias.
type ColumnMeta struct {
	Tag    string            // e.g. "URL", "HTML"
	Params string            // raw parameter string
	Args   map[string]string // parsed structured arguments
}

// Column represents a parsed column definition.
type Column struct {
	RawName     string                 // original SQL column alias
	DisplayName string                 // display name with metadata stripped
	Meta        map[string]*ColumnMeta // metadata index, keyed by Tag
}

var metaRegex = regexp.MustCompile(`\[(\w+)(?:\(([^)]*)\))?\]`)

// ParseColumns parses metadata from raw column names.
func ParseColumns(rawColumns []string) []*Column {
	columns := make([]*Column, len(rawColumns))
	for i, raw := range rawColumns {
		columns[i] = parseColumn(raw)
	}
	return columns
}

func parseColumn(rawName string) *Column {
	col := &Column{
		RawName: rawName,
		Meta:    make(map[string]*ColumnMeta),
	}

	displayName := rawName
	matches := metaRegex.FindAllStringSubmatch(rawName, -1)

	for _, match := range matches {
		tag := match[1]
		params := ""
		if len(match) > 2 {
			params = match[2]
		}

		meta := &ColumnMeta{
			Tag:    tag,
			Params: params,
			Args:   parseArgs(tag, params),
		}
		col.Meta[tag] = meta

		displayName = strings.Replace(displayName, match[0], "", 1)
	}

	col.DisplayName = strings.TrimSpace(displayName)
	if col.DisplayName == "" {
		col.DisplayName = rawName
	}

	return col
}

func parseArgs(tag, params string) map[string]string {
	args := make(map[string]string)
	if params == "" {
		return args
	}

	switch tag {
	case "URL":
		parts := strings.Split(params, ",")
		if len(parts) >= 1 {
			args["expiry"] = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 && strings.TrimSpace(parts[1]) == "D" {
			args["download"] = "true"
		}
	case "HTML":
		parts := strings.SplitN(params, ",", 2)
		if len(parts) >= 1 {
			args["type"] = strings.TrimSpace(parts[0])
		}
		if len(parts) >= 2 {
			interaction := strings.TrimSpace(parts[1])
			args["interaction"] = interaction
			if len(interaction) >= 2 && interaction[1] == ':' {
				args["interactionType"] = string(interaction[0])
				rest := ""
				if len(interaction) > 2 {
					rest = interaction[2:]
				}
				if idx := strings.Index(rest, "->"); idx != -1 {
					args["hint"] = strings.TrimSpace(rest[:idx])
					args["bindColumn"] = strings.TrimSpace(rest[idx+2:])
				} else {
					args["hint"] = strings.TrimSpace(rest)
					args["bindToSelf"] = "true"
				}
			}
		}
	case "H":
		args["height"] = strings.TrimSpace(params)
	}

	return args
}

// HasMeta checks whether the column has the given metadata tag.
func (c *Column) HasMeta(tag string) bool {
	_, ok := c.Meta[tag]
	return ok
}

// GetMeta returns the metadata for the given tag, or nil.
func (c *Column) GetMeta(tag string) *ColumnMeta {
	return c.Meta[tag]
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/parser/meta.go
git commit -m "feat: migrate metadata parser from sql-exporter"
```

---

### Task 7: Exporter Interface + Excel Exporter (migrate)

**Files:**
- Create: `internal/exporter/exporter.go`
- Create: `internal/exporter/excel.go`

- [ ] **Step 1: Write exporter.go**

```go
// internal/exporter/exporter.go
package exporter

import "sql-query/internal/parser"

// Exporter is the interface for all export formats.
type Exporter interface {
	Export(columns []*parser.Column, data [][]*string) error
}
```

- [ ] **Step 2: Write excel.go**

Migrated from sql-exporter — only the import path changes (`sql-exporter/internal/parser` → `sql-query/internal/parser`).

```go
// internal/exporter/excel.go
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
```

- [ ] **Step 3: Commit**

```bash
git add internal/exporter/exporter.go internal/exporter/excel.go
git commit -m "feat: migrate exporter interface and Excel exporter from sql-exporter"
```

---

### Task 8: HTML Exporter + Template (migrate)

**Files:**
- Create: `internal/exporter/html.go`
- Create: `internal/exporter/templates/report.tmpl`

- [ ] **Step 1: Create template directory and copy template**

```bash
mkdir -p internal/exporter/templates
```

Copy `report.tmpl` verbatim from `/Users/mac/workspaces/github/sql-exporter/internal/exporter/templates/report.tmpl` — no changes needed.

```bash
cp /Users/mac/workspaces/github/sql-exporter/internal/exporter/templates/report.tmpl internal/exporter/templates/report.tmpl
```

- [ ] **Step 2: Write html.go**

Migrated from sql-exporter — only the import path changes.

```go
// internal/exporter/html.go
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
```

- [ ] **Step 3: Commit**

```bash
git add internal/exporter/html.go internal/exporter/templates/report.tmpl
git commit -m "feat: migrate HTML exporter and template from sql-exporter"
```

---

### Task 9: JSON Exporter (new)

**Files:**
- Create: `internal/exporter/json.go`

- [ ] **Step 1: Write json.go**

```go
// internal/exporter/json.go
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
```

- [ ] **Step 2: Commit**

```bash
git add internal/exporter/json.go
git commit -m "feat: add JSON exporter for query results"
```

---

### Task 10: Root Command (cmd/root.go)

**Files:**
- Create: `cmd/root.go`

- [ ] **Step 1: Write root.go**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"sql-query/internal/config"
	"sql-query/internal/db"
	"sql-query/internal/errutil"
)

var (
	envFile  string
	jsonFlag bool

	// Shared state set by PersistentPreRunE
	cfg      *config.Config
	database *gorm.DB
)

var rootCmd = &cobra.Command{
	Use:   "sql-query",
	Short: "MySQL 数据库结构查询与 SQL 导出工具",
	Long:  "sql-query 是一个 CLI 工具，提供 MySQL 数据库结构查询和 SQL 查询导出能力，专为 AI skills 设计。",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Load .env file
		if envFile != "" {
			if err := godotenv.Load(envFile); err != nil {
				errutil.Exit(errutil.ExitGenericError, "file_error",
					fmt.Sprintf(".env 文件加载失败: %s", err), jsonFlag)
			}
		}

		// Load config
		var err error
		cfg, err = config.Load()
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				err.Error(), jsonFlag)
		}

		// Connect to database
		fmt.Fprintln(os.Stderr, "连接数据库...")
		database, err = db.Connect(cfg.DBDSN)
		if err != nil {
			errutil.Exit(errutil.ExitConnectFailed, "connection_failed",
				fmt.Sprintf("数据库连接失败: %s", err), jsonFlag)
		}
		fmt.Fprintln(os.Stderr, "数据库连接成功")

		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&envFile, "env", "e", "", ".env 配置文件路径")
	rootCmd.PersistentFlags().BoolVar(&jsonFlag, "json", false, "以 JSON 格式输出")
}

// Execute runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(errutil.ExitGenericError)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/root.go
git commit -m "feat: add root command with shared config loading and DB connection"
```

---

### Task 11: Tables Subcommand (cmd/tables.go)

**Files:**
- Create: `cmd/tables.go`

- [ ] **Step 1: Write tables.go**

```go
// cmd/tables.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sql-query/internal/db"
	"sql-query/internal/errutil"
)

var tablesCmd = &cobra.Command{
	Use:   "tables",
	Short: "列出当前数据库的所有表",
	RunE: func(cmd *cobra.Command, args []string) error {
		tables, err := db.GetTables(database)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("查询表列表失败: %s", err), jsonFlag)
		}

		if jsonFlag {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(tables)
		}

		// Text output
		fmt.Printf("Found %d tables:\n", len(tables))
		for _, t := range tables {
			if t.TableComment != "" {
				fmt.Printf("  %-30s - %s\n", t.TableName, t.TableComment)
			} else {
				fmt.Printf("  %s\n", t.TableName)
			}
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tablesCmd)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/tables.go
git commit -m "feat: add tables subcommand to list database tables"
```

---

### Task 12: Table Subcommand (cmd/table.go)

**Files:**
- Create: `cmd/table.go`

- [ ] **Step 1: Write table.go**

```go
// cmd/table.go
package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"sql-query/internal/db"
	"sql-query/internal/errutil"
)

var tableCmd = &cobra.Command{
	Use:   "table <name>",
	Short: "查看指定表的 DDL 和结构信息",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		tableName := args[0]

		ddl, err := db.GetTableDDL(database, tableName)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("查询表结构失败: %s", err), jsonFlag)
		}
		if ddl == nil {
			errutil.Exit(errutil.ExitTableNotFound, "table_not_found",
				fmt.Sprintf("表 '%s' 不存在", tableName), jsonFlag)
		}

		if jsonFlag {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			enc.SetEscapeHTML(false)
			return enc.Encode(ddl)
		}

		// Text output: raw DDL
		fmt.Println(ddl.RawDDL)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(tableCmd)
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/table.go
git commit -m "feat: add table subcommand for DDL and schema inspection"
```

---

### Task 13: Query Subcommand (cmd/query.go)

**Files:**
- Create: `cmd/query.go`

- [ ] **Step 1: Write query.go**

```go
// cmd/query.go
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"sql-query/internal/db"
	"sql-query/internal/errutil"
	"sql-query/internal/exporter"
	"sql-query/internal/parser"
)

var (
	sqlFile    string
	outputFile string
	excelFlag  bool
	htmlFlag   bool
	workers    int
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "执行 SQL 查询并导出结果",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate output format
		formatCount := 0
		if excelFlag {
			formatCount++
		}
		if htmlFlag {
			formatCount++
		}
		if jsonFlag {
			formatCount++
		}
		if formatCount == 0 {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				"请指定输出格式：--excel、--html 或 --json", jsonFlag)
		}
		if formatCount > 1 {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				"只能选择一种输出格式：--excel、--html 或 --json", jsonFlag)
		}

		// Set default output file if needed
		if outputFile == "" {
			if excelFlag {
				outputFile = "output.xlsx"
			} else if htmlFlag {
				outputFile = "output.html"
			}
			// JSON with no -o: write to stdout (outputFile stays "")
		}

		// Read SQL
		sqlContent, err := readSQL()
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "file_error",
				fmt.Sprintf("读取 SQL 失败: %s", err), jsonFlag)
		}
		if sqlContent == "" {
			errutil.Exit(errutil.ExitGenericError, "invalid_argument",
				"未提供 SQL 内容，请使用 -f 指定文件或通过 stdin 输入", jsonFlag)
		}

		// Execute SQL
		fmt.Fprintln(os.Stderr, "执行 SQL 查询...")
		queryStart := time.Now()
		columns, data, err := db.Execute(database, sqlContent, cfg.QueryTimeout)
		if err != nil {
			errutil.Exit(errutil.ExitGenericError, "sql_syntax_error",
				fmt.Sprintf("执行 SQL 失败: %s", err), jsonFlag)
		}
		fmt.Fprintf(os.Stderr, "查询完成，共 %d 列 %d 行 (耗时 %v)\n",
			len(columns), len(data), time.Since(queryStart).Round(time.Millisecond))

		// Parse metadata
		parsedColumns := parser.ParseColumns(columns)

		// Note: S3 presigning is Phase 2 — skipped for now.
		// Log a warning if URL metadata is present.
		for _, col := range parsedColumns {
			if col.HasMeta("URL") {
				fmt.Fprintln(os.Stderr, "警告: 发现 [URL] 元数据但 S3 预签名尚未启用 (Phase 2)")
				break
			}
		}

		// Export
		var exp exporter.Exporter
		if excelFlag {
			fmt.Fprintf(os.Stderr, "导出 Excel -> %s\n", outputFile)
			exp = exporter.NewExcelExporter(outputFile)
		} else if htmlFlag {
			fmt.Fprintf(os.Stderr, "导出 HTML -> %s\n", outputFile)
			exp = exporter.NewHTMLExporter(outputFile)
		} else {
			if outputFile != "" {
				fmt.Fprintf(os.Stderr, "导出 JSON -> %s\n", outputFile)
			}
			exp = exporter.NewJSONExporter(outputFile)
		}

		if err := exp.Export(parsedColumns, data); err != nil {
			errutil.Exit(errutil.ExitGenericError, "file_error",
				fmt.Sprintf("导出失败: %s", err), jsonFlag)
		}

		fmt.Fprintln(os.Stderr, "完成")
		return nil
	},
}

func init() {
	queryCmd.Flags().StringVarP(&sqlFile, "file", "f", "", "SQL 文件路径（不指定则读取 stdin）")
	queryCmd.Flags().StringVarP(&outputFile, "output", "o", "", "输出文件路径")
	queryCmd.Flags().BoolVar(&excelFlag, "excel", false, "导出为 Excel 格式")
	queryCmd.Flags().BoolVar(&htmlFlag, "html", false, "导出为 HTML 格式")
	queryCmd.Flags().IntVarP(&workers, "workers", "w", runtime.NumCPU(), "并发处理数")
	rootCmd.AddCommand(queryCmd)
}

func readSQL() (string, error) {
	if sqlFile != "" {
		content, err := os.ReadFile(sqlFile)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// Check if stdin has data
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return "", nil // no stdin pipe
	}

	reader := bufio.NewReader(os.Stdin)
	var content []byte
	for {
		line, err := reader.ReadBytes('\n')
		content = append(content, line...)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}
	return string(content), nil
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/query.go
git commit -m "feat: add query subcommand for SQL execution and export"
```

---

### Task 14: Main Entry Point + Build Verification

**Files:**
- Create: `main.go`

- [ ] **Step 1: Write main.go**

```go
// main.go
package main

import "sql-query/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 2: Run go mod tidy and build**

```bash
cd /Users/mac/workspaces/github/sql-query
go mod tidy
go build -o sql-query .
```

Expected: builds successfully with no errors.

- [ ] **Step 3: Verify CLI help output**

```bash
./sql-query --help
./sql-query tables --help
./sql-query table --help
./sql-query query --help
```

Expected: each command shows proper usage and flags.

- [ ] **Step 4: Verify error handling without .env**

```bash
./sql-query tables 2>/dev/null; echo "exit code: $?"
```

Expected: exit code 1 (DB_DSN not configured).

```bash
./sql-query tables --json 2>/dev/null
```

Expected: `{"error":"invalid_argument","message":"环境变量 DB_DSN 未配置"}` on stdout.

```bash
./sql-query table nonexistent -e nonexistent.env 2>/dev/null; echo "exit code: $?"
```

Expected: exit code 1 (.env file not found).

- [ ] **Step 5: Commit**

```bash
git add main.go go.mod go.sum
git commit -m "feat: add main entry point and complete CLI build"
```

---

### Task 15: Integration Test with Real Database

This task is manual verification — run these commands against a real MySQL database.

- [ ] **Step 1: Create a test .env file**

Create `.env` (gitignored) with your MySQL connection:

```bash
echo 'DB_DSN=user:password@tcp(127.0.0.1:3306)/your_db?charset=utf8mb4&parseTime=True&loc=Local' > .env
echo '.env' >> .gitignore
```

- [ ] **Step 2: Test tables command**

```bash
./sql-query tables -e .env
./sql-query tables -e .env --json
```

Expected: text format shows `Found N tables:` with table names; JSON format shows array of `{"table_name":..., "table_comment":...}`.

- [ ] **Step 3: Test table command**

```bash
# Replace 'users' with an actual table name from step 2
./sql-query table users -e .env
./sql-query table users -e .env --json
```

Expected: text shows raw DDL; JSON shows structured object with columns, indexes, and ddl fields.

- [ ] **Step 4: Test table not found**

```bash
./sql-query table nonexistent_table_xyz -e .env 2>/dev/null; echo "exit code: $?"
./sql-query table nonexistent_table_xyz -e .env --json 2>/dev/null
```

Expected: exit code 3; JSON mode outputs `{"error":"table_not_found",...}`.

- [ ] **Step 5: Test query command**

```bash
echo "SELECT 1 AS id, 'hello' AS name" | ./sql-query query -e .env --json
echo "SELECT 1 AS id, 'hello' AS name" | ./sql-query query -e .env --excel -o test.xlsx
echo "SELECT 1 AS id, 'hello' AS name" | ./sql-query query -e .env --html -o test.html
```

Expected: JSON to stdout, Excel and HTML files created.

- [ ] **Step 6: Test empty result**

```bash
echo "SELECT 1 WHERE 1=0" | ./sql-query query -e .env --json
```

Expected: `[]` on stdout.

- [ ] **Step 7: Clean up test files and commit .gitignore**

```bash
rm -f test.xlsx test.html
git add .gitignore
git commit -m "chore: add .gitignore with .env"
```
