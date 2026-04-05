package log

import (
	"fmt"
	"os"
	"strings"
)

// Level represents log severity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var level = LevelError

// SetLevel sets the global log level.
func SetLevel(l Level) {
	level = l
}

// ParseLevel converts a string to Level. Returns LevelError for unknown values.
func ParseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return LevelDebug
	case "info":
		return LevelInfo
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelError
	}
}

func Debug(format string, args ...interface{}) {
	if level <= LevelDebug {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func Info(format string, args ...interface{}) {
	if level <= LevelInfo {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func Warn(format string, args ...interface{}) {
	if level <= LevelWarn {
		fmt.Fprintf(os.Stderr, "[WARN] "+format+"\n", args...)
	}
}

func Error(format string, args ...interface{}) {
	if level <= LevelError {
		fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	}
}
