package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Validation holds the result of each security layer.
type Validation struct {
	L1AST      string `json:"l1_ast"`             // pass | rejected
	L1Reason   string `json:"l1_reason,omitempty"` // rejection reason (if rejected)
	L2Explain  string `json:"l2_explain"`          // pass | error | skipped
	L3ReadOnly string `json:"l3_readonly_tx"`      // pass | error | skipped
}

// Entry is a single audit log record written as one JSON line.
type Entry struct {
	Timestamp    string     `json:"timestamp"`
	Status       string     `json:"status"`                  // success | rejected | error
	SQL          string     `json:"sql"`
	DurationMs   int64      `json:"duration_ms"`
	Rows         int        `json:"rows,omitempty"`
	Columns      int        `json:"columns,omitempty"`
	Error        string     `json:"error,omitempty"`
	EnvFile      string     `json:"env_file,omitempty"`
	Database     string     `json:"database,omitempty"`
	User         string     `json:"user,omitempty"`
	OutputFormat string     `json:"output_format,omitempty"`
	OutputFile   string     `json:"output_file,omitempty"`
	Validation   Validation `json:"validation"`
}

// Write appends the entry as a single JSON line to the daily audit log file.
// File path: <dir>/query-audit-YYYY-MM-DD.log
// Errors are printed to stderr but never returned — audit must not break the main flow.
func (e *Entry) Write(dir string) {
	if dir == "" {
		dir = "."
	}

	filename := fmt.Sprintf("query-audit-%s.log", time.Now().Format("2006-01-02"))
	path := filepath.Join(dir, filename)

	data, err := json.Marshal(e)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] audit log marshal failed: %v\n", err)
		return
	}
	data = append(data, '\n')

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] audit log open failed: %v\n", err)
		return
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] audit log write failed: %v\n", err)
	}
}
