package errutil

import (
	"encoding/json"
	"fmt"
	"os"
)

// Exit codes
const (
	ExitOK            = 0
	ExitGenericError  = 1
	ExitConnectFailed = 2
	ExitTableNotFound = 3
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
