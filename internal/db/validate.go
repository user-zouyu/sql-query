package db

import (
	"fmt"
	"regexp"
	"strings"
)

// dangerousKeywords are SQL statement-level keywords that indicate write operations.
var dangerousKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER", "TRUNCATE",
	"CREATE", "REPLACE", "RENAME", "GRANT", "REVOKE",
	"LOCK", "UNLOCK", "CALL", "LOAD",
}

// dangerousPatterns are SQL clause patterns that could cause side effects.
var dangerousPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bINTO\s+OUTFILE\b`),
	regexp.MustCompile(`(?i)\bINTO\s+DUMPFILE\b`),
	regexp.MustCompile(`(?i)\bFOR\s+UPDATE\b`),
	regexp.MustCompile(`(?i)\bFOR\s+SHARE\b`),
	regexp.MustCompile(`(?i)\bLOCK\s+IN\s+SHARE\s+MODE\b`),
}

// statementKeywordRe matches the first keyword of a SQL statement,
// skipping leading whitespace, comments, and CTEs (WITH ... AS).
var statementKeywordRe = regexp.MustCompile(`(?i)^\s*(?:--[^\n]*\n\s*|/\*[\s\S]*?\*/\s*)*(WITH\b|SELECT\b|INSERT\b|UPDATE\b|DELETE\b|DROP\b|ALTER\b|CREATE\b|TRUNCATE\b|REPLACE\b|RENAME\b|GRANT\b|REVOKE\b|LOCK\b|UNLOCK\b|CALL\b|LOAD\b|SET\b|EXPLAIN\b|SHOW\b|DESCRIBE\b|DESC\b)`)

// ValidateReadOnly checks that the SQL is a safe read-only statement.
// Returns nil if safe, or an error describing why it was rejected.
func ValidateReadOnly(sql string) error {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return fmt.Errorf("SQL 为空")
	}

	// Find the first statement keyword
	match := statementKeywordRe.FindStringSubmatch(trimmed)
	if len(match) < 2 {
		return fmt.Errorf("无法识别的 SQL 语句类型")
	}

	keyword := strings.ToUpper(match[1])

	// Allow: SELECT, WITH (CTE), EXPLAIN, SHOW, DESCRIBE/DESC
	switch keyword {
	case "SELECT", "WITH", "EXPLAIN", "SHOW", "DESCRIBE", "DESC":
		// OK — continue to pattern checks below
	case "SET":
		// Only allow SET NAMES
		if matched, _ := regexp.MatchString(`(?i)^\s*SET\s+NAMES\b`, trimmed); matched {
			return nil
		}
		return fmt.Errorf("禁止执行 SET 语句（仅允许 SET NAMES）")
	default:
		return fmt.Errorf("禁止执行 %s 语句（仅允许 SELECT 查询）", keyword)
	}

	// Check for dangerous patterns within the query
	for _, pat := range dangerousPatterns {
		if pat.MatchString(trimmed) {
			return fmt.Errorf("SQL 包含不安全的子句: %s", pat.String())
		}
	}

	// Check for dangerous keywords used as sub-statements (e.g., SELECT ... ; DROP TABLE)
	// Split by semicolons and check each statement
	statements := splitStatements(trimmed)
	for i, stmt := range statements {
		if i == 0 {
			continue // first statement already validated above
		}
		stmtTrimmed := strings.TrimSpace(stmt)
		if stmtTrimmed == "" {
			continue
		}
		subMatch := statementKeywordRe.FindStringSubmatch(stmtTrimmed)
		if len(subMatch) >= 2 {
			subKeyword := strings.ToUpper(subMatch[1])
			switch subKeyword {
			case "SELECT", "WITH", "EXPLAIN", "SHOW", "DESCRIBE", "DESC":
				// OK
			default:
				return fmt.Errorf("禁止在多语句中执行 %s（仅允许 SELECT 查询）", subKeyword)
			}
		}
	}

	return nil
}

// splitStatements splits SQL by semicolons, respecting quoted strings.
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false

	for i := 0; i < len(sql); i++ {
		ch := sql[i]

		switch {
		case ch == '\'' && !inDoubleQuote && !inBacktick:
			inSingleQuote = !inSingleQuote
			current.WriteByte(ch)
		case ch == '"' && !inSingleQuote && !inBacktick:
			inDoubleQuote = !inDoubleQuote
			current.WriteByte(ch)
		case ch == '`' && !inSingleQuote && !inDoubleQuote:
			inBacktick = !inBacktick
			current.WriteByte(ch)
		case ch == ';' && !inSingleQuote && !inDoubleQuote && !inBacktick:
			statements = append(statements, current.String())
			current.Reset()
		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	return statements
}
