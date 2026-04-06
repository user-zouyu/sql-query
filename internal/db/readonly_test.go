package db

import (
	"fmt"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	_ = godotenv.Load("../../.env.test")
	_ = godotenv.Load("../../.env")

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		fmt.Println("SKIP: DB_DSN not set")
		os.Exit(0)
	}

	var err error
	testDB, err = Connect(dsn)
	if err != nil {
		fmt.Printf("SKIP: cannot connect to MySQL: %v\n", err)
		os.Exit(0)
	}

	os.Exit(m.Run())
}

// ==================== L1: ValidateReadOnly (AST) ====================

func TestValidate_Allowed(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		// Basic SELECT
		{"SELECT 1", "SELECT 1"},
		{"SELECT from table", "SELECT * FROM users"},
		{"SELECT with WHERE", "SELECT id, username FROM users WHERE id = 1"},
		{"SELECT with JOIN", "SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id"},
		{"SELECT subquery", "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)"},
		{"SELECT COUNT", "SELECT COUNT(*) FROM users"},
		{"SELECT UNION", "SELECT 1 UNION SELECT 2"},
		{"SELECT UNION ALL", "SELECT 1 UNION ALL SELECT 2"},
		{"SELECT GROUP BY", "SELECT id, COUNT(*) FROM users GROUP BY id"},
		{"SELECT ORDER BY LIMIT", "SELECT * FROM users ORDER BY id LIMIT 10"},
		{"SELECT DISTINCT", "SELECT DISTINCT status FROM users"},

		// CTE
		{"CTE", "WITH cte AS (SELECT 1) SELECT * FROM cte"},
		{"CTE multiple", "WITH a AS (SELECT 1), b AS (SELECT 2) SELECT * FROM a, b"},

		// Comments
		{"line comment", "-- comment\nSELECT 1"},
		{"block comment", "/* comment */ SELECT 1"},

		// Functions
		{"NOW()", "SELECT NOW()"},
		{"VERSION()", "SELECT VERSION()"},
		{"COALESCE", "SELECT COALESCE(email, 'N/A') FROM users"},
		{"IF", "SELECT IF(status=1, 'on', 'off') FROM users"},
		{"CASE", "SELECT CASE WHEN id=1 THEN 'a' ELSE 'b' END FROM users"},

		// Trailing semicolon
		{"trailing semicolon", "SELECT 1;"},

		// Nested subquery
		{"deep subquery", "SELECT * FROM (SELECT * FROM (SELECT 1 AS n) a) b"},

		// INFORMATION_SCHEMA
		{"info schema", "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE()"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err != nil {
				t.Errorf("expected allowed, got: %v", err)
			}
		})
	}
}

func TestValidate_Blocked(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		// DML
		{"INSERT", "INSERT INTO users (username) VALUES ('x')"},
		{"UPDATE", "UPDATE users SET username = 'x' WHERE id = 1"},
		{"DELETE", "DELETE FROM users WHERE id = 1"},
		{"REPLACE", "REPLACE INTO users (id, username) VALUES (1, 'x')"},

		// DDL
		{"CREATE TABLE", "CREATE TABLE test_xxx (id INT)"},
		{"ALTER TABLE", "ALTER TABLE users ADD COLUMN foo INT"},
		{"DROP TABLE", "DROP TABLE users"},
		{"TRUNCATE", "TRUNCATE TABLE users"},
		{"RENAME TABLE", "RENAME TABLE users TO users_old"},
		{"CREATE INDEX", "CREATE INDEX idx_x ON users(username)"},

		// Non-query statements
		{"SHOW TABLES", "SHOW TABLES"},
		{"DESCRIBE", "DESCRIBE users"},
		{"EXPLAIN", "EXPLAIN SELECT * FROM users"},
		{"SET NAMES", "SET NAMES utf8mb4"},
		{"SET variable", "SET @a = 1"},
		{"SET GLOBAL", "SET GLOBAL read_only = 1"},

		// Privilege / Lock / Procedure
		{"GRANT", "GRANT SELECT ON testdb.* TO 'u'@'localhost'"},
		{"REVOKE", "REVOKE SELECT ON testdb.* FROM 'u'@'localhost'"},
		{"CALL", "CALL some_proc()"},
		{"LOAD DATA", "LOAD DATA INFILE '/tmp/x' INTO TABLE users"},

		// Locking clauses
		{"FOR UPDATE", "SELECT * FROM users FOR UPDATE"},
		{"FOR SHARE", "SELECT * FROM users FOR SHARE"},
		{"LOCK IN SHARE MODE", "SELECT * FROM users LOCK IN SHARE MODE"},
		{"UNION FOR UPDATE", "SELECT 1 UNION SELECT 2 FOR UPDATE"},

		// INTO OUTFILE / DUMPFILE
		{"INTO OUTFILE", "SELECT * FROM users INTO OUTFILE '/tmp/x'"},
		{"INTO DUMPFILE", "SELECT * FROM users INTO DUMPFILE '/tmp/x'"},

		// DoS functions
		{"SLEEP", "SELECT SLEEP(10)"},
		{"SLEEP in WHERE", "SELECT * FROM users WHERE SLEEP(1)"},
		{"BENCHMARK", "SELECT BENCHMARK(1, SHA1('x'))"},
		{"GET_LOCK", "SELECT GET_LOCK('k', 0)"},

		// Multi-statement injection
		{"SELECT;DROP", "SELECT 1; DROP TABLE users"},
		{"SELECT;INSERT", "SELECT 1; INSERT INTO users (username) VALUES ('x')"},

		// MySQL conditional comment injection
		{"conditional INSERT", "/*!50000 INSERT INTO users VALUES (1,'x') */"},

		// Unquoted non-ASCII aliases — rejected by parser (use backticks)
		{"Chinese alias unquoted", "SELECT 1 AS 用户名"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateReadOnly(tc.sql); err == nil {
				t.Errorf("expected blocked: %s", tc.sql)
			}
		})
	}
}

// ==================== L2+L3: Execute (EXPLAIN + READ ONLY TX) ====================

func TestExecute_Allowed(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"SELECT 1", "SELECT 1"},
		{"SELECT from table", "SELECT * FROM users LIMIT 1"},
		{"SELECT COUNT", "SELECT COUNT(*) AS cnt FROM users"},
		{"SELECT subquery", "SELECT * FROM users WHERE id IN (SELECT id FROM users)"},
		{"SELECT JOIN", "SELECT u.id FROM users u JOIN orders o ON u.id = o.user_id LIMIT 1"},
		{"SELECT UNION", "SELECT 1 AS n UNION SELECT 2"},
		{"info schema", "SELECT TABLE_NAME FROM INFORMATION_SCHEMA.TABLES WHERE TABLE_SCHEMA = DATABASE() LIMIT 1"},
		{"NOW()", "SELECT NOW()"},
		{"VERSION()", "SELECT VERSION()"},
		{"deep subquery", "SELECT * FROM (SELECT * FROM (SELECT 1 AS n) a) b"},
		{"large offset", "SELECT * FROM users LIMIT 1 OFFSET 999999"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cols, _, err := Execute(testDB, tc.sql, 10, 100)
			if err != nil {
				t.Errorf("expected success, got: %v", err)
				return
			}
			if len(cols) == 0 {
				t.Error("expected columns")
			}
		})
	}
}

func TestExecute_DML_Blocked(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"INSERT", "INSERT INTO users (username, email) VALUES ('hack', 'x@x')"},
		{"UPDATE", "UPDATE users SET username = 'hack' WHERE 1=1"},
		{"DELETE", "DELETE FROM users WHERE username = 'hack'"},
		{"REPLACE", "REPLACE INTO users (id, username, email) VALUES (99999, 'x', 'x@x')"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Execute(testDB, tc.sql, 10, 100)
			if err == nil {
				t.Errorf("expected error for DML: %s", tc.sql)
			}
		})
	}
}

func TestExecute_DDL_Blocked(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"CREATE TABLE", "CREATE TABLE test_hack (id INT)"},
		{"DROP TABLE", "DROP TABLE users"},
		{"ALTER TABLE", "ALTER TABLE users ADD COLUMN hack VARCHAR(100)"},
		{"TRUNCATE", "TRUNCATE TABLE users"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := Execute(testDB, tc.sql, 10, 100)
			if err == nil {
				t.Errorf("expected error for DDL: %s", tc.sql)
			}
		})
	}
}

// ==================== Full pipeline: L1 → L2 → L3 ====================

func TestFullPipeline(t *testing.T) {
	cases := []struct {
		name         string
		sql          string
		validatePass bool
		executePass  bool
	}{
		{"SELECT", "SELECT * FROM users LIMIT 1", true, true},
		{"CTE", "WITH c AS (SELECT 1) SELECT * FROM c", true, true},
		{"INSERT", "INSERT INTO users (username) VALUES ('x')", false, false},
		{"DROP TABLE", "DROP TABLE users", false, false},
		{"SHOW TABLES", "SHOW TABLES", false, false},
		{"FOR UPDATE", "SELECT * FROM users FOR UPDATE", false, false},
		{"SLEEP", "SELECT SLEEP(1)", false, false},
		{"multi-stmt", "SELECT 1; DROP TABLE users", false, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateReadOnly(tc.sql)
			if tc.validatePass && err != nil {
				t.Fatalf("L1: expected pass, got: %v", err)
			}
			if !tc.validatePass {
				if err == nil {
					t.Fatalf("L1: expected block, but passed")
				}
				return
			}
			_, _, err = Execute(testDB, tc.sql, 10, 100)
			if tc.executePass && err != nil {
				t.Errorf("L2+L3: expected pass, got: %v", err)
			}
			if !tc.executePass && err == nil {
				t.Errorf("L2+L3: expected block, but passed")
			}
		})
	}
}

// ==================== Data integrity ====================

func TestDataIntegrity(t *testing.T) {
	_, rows, err := Execute(testDB, "SELECT COUNT(*) AS cnt FROM users", 10, 100)
	if err != nil {
		t.Fatalf("users table not accessible: %v", err)
	}
	if len(rows) == 0 || rows[0][0] == nil {
		t.Fatal("unexpected empty result")
	}
	t.Logf("users table intact, row count: %s", *rows[0][0])
}
