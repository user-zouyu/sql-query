package audit

import "strings"

// DSNInfo holds non-sensitive connection info extracted from a DSN string.
type DSNInfo struct {
	User     string
	Database string
}

// ParseDSN extracts user and database from a go-sql-driver/mysql DSN string.
// Never exposes the password.
func ParseDSN(dsn string) DSNInfo {
	var info DSNInfo

	// Extract user: everything before the first ':'
	if idx := strings.Index(dsn, ":"); idx > 0 {
		info.User = dsn[:idx]
	}

	// Extract database: between '/' and '?'
	if idx := strings.Index(dsn, "/"); idx >= 0 {
		rest := dsn[idx+1:]
		if qIdx := strings.Index(rest, "?"); qIdx >= 0 {
			info.Database = rest[:qIdx]
		} else {
			info.Database = rest
		}
	}

	return info
}
