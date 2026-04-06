package db

import (
	"testing"
)

// TestBypass_CommentInjection tests that AST parsing is immune to comment tricks.
func TestBypass_CommentInjection(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		// MySQL conditional comments — AST parser handles these correctly
		{"conditional INSERT", "/*!50000 INSERT INTO users VALUES (1,'x') */"},
		{"conditional DROP", "/*!50000 DROP TABLE users */"},
		{"wrapped DML", "/*! INSERT INTO users (username) VALUES ('x') */"},

		// Comment between INTO and OUTFILE — AST sees INTO node, no regex needed
		{"INTO comment OUTFILE", "SELECT * FROM users INTO /* comment */ OUTFILE '/tmp/x'"},

		// Comment before dangerous keyword
		{"comment then DROP", "-- safe\nDROP TABLE users"},
		{"block comment then INSERT", "/* safe */ INSERT INTO users VALUES (1)"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err == nil {
				t.Errorf("should be blocked: %s", tc.sql)
			} else {
				t.Logf("blocked: %v", err)
			}
		})
	}
}

// TestBypass_DosFunctions verifies dangerous functions are blocked even in subqueries.
func TestBypass_DosFunctions(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"SLEEP top level", "SELECT SLEEP(0)"},
		{"SLEEP in WHERE", "SELECT * FROM users WHERE SLEEP(1)=0"},
		{"SLEEP in subquery", "SELECT * FROM users WHERE id = (SELECT SLEEP(1))"},
		{"BENCHMARK", "SELECT BENCHMARK(1, SHA1('x'))"},
		{"GET_LOCK", "SELECT GET_LOCK('k', 0)"},
		{"GET_LOCK in CASE", "SELECT CASE WHEN GET_LOCK('k',0) THEN 1 END"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err == nil {
				t.Errorf("should be blocked: %s", tc.sql)
			} else {
				t.Logf("blocked: %v", err)
			}
		})
	}
}

// TestBypass_LockingVariants verifies all locking clause forms are blocked.
func TestBypass_LockingVariants(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"FOR UPDATE", "SELECT * FROM users FOR UPDATE"},
		{"FOR SHARE", "SELECT * FROM users FOR SHARE"},
		{"LOCK IN SHARE MODE", "SELECT * FROM users LOCK IN SHARE MODE"},
		{"FOR UPDATE in subquery SELECT", "SELECT * FROM (SELECT * FROM users FOR UPDATE) t"},
		{"UNION FOR UPDATE", "SELECT 1 UNION SELECT 2 FOR UPDATE"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err == nil {
				t.Errorf("should be blocked: %s", tc.sql)
			} else {
				t.Logf("blocked: %v", err)
			}
		})
	}
}

// TestBypass_MultiStatement verifies multi-statement injection is blocked.
func TestBypass_MultiStatement(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"SELECT;DROP", "SELECT 1; DROP TABLE users"},
		{"SELECT;INSERT", "SELECT 1; INSERT INTO users VALUES (1)"},
		{"SELECT;SELECT (still blocked)", "SELECT 1; SELECT 2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err == nil {
				t.Errorf("should be blocked: %s", tc.sql)
			} else {
				t.Logf("blocked: %v", err)
			}
		})
	}
}

// TestBypass_MultiStatementDriver verifies go-sql-driver blocks multi-statements.
func TestBypass_MultiStatementDriver(t *testing.T) {
	sqlDB, err := testDB.DB()
	if err != nil {
		t.Fatalf("cannot get *sql.DB: %v", err)
	}

	_, err = sqlDB.Exec("SELECT 1; INSERT INTO users (username) VALUES ('hack')")
	if err == nil {
		var count int
		sqlDB.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'hack'").Scan(&count)
		if count > 0 {
			t.Errorf("CRITICAL: multiStatements enabled, INSERT executed")
			sqlDB.Exec("DELETE FROM users WHERE username = 'hack'")
		}
	} else {
		t.Logf("driver blocked: %v", err)
	}
}

// TestBypass_StringWithSemicolon ensures semicolons inside strings don't cause false rejects.
func TestBypass_StringWithSemicolon(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"semicolon in string", "SELECT * FROM users WHERE username = 'a;b'"},
		{"semicolon in alias", "SELECT 1 AS `a;b`"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err != nil {
				t.Errorf("false reject: %v", err)
			}
		})
	}
}

// TestBypass_ChineseAlias verifies that unquoted Chinese aliases are rejected
// (Vitess parser limitation) and backtick-quoted ones work fine.
func TestBypass_ChineseAlias(t *testing.T) {
	// Unquoted — must be rejected (parse error, safe default)
	if err := ValidateReadOnly("SELECT 1 AS 用户名"); err == nil {
		t.Error("unquoted Chinese alias should be rejected")
	}

	// Backtick-quoted — should pass
	if err := ValidateReadOnly("SELECT 1 AS `用户名`"); err != nil {
		t.Errorf("backtick Chinese alias should pass: %v", err)
	}

	// Chinese in string value — should pass
	if err := ValidateReadOnly("SELECT * FROM users WHERE username = '张三'"); err != nil {
		t.Errorf("Chinese in string value should pass: %v", err)
	}
}
