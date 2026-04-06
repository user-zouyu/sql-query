package db

import (
	"fmt"
	"strings"

	"vitess.io/vitess/go/vt/sqlparser"
)

// dangerousFunctions are MySQL functions that can cause DoS or side effects.
var dangerousFunctions = map[string]struct{}{
	"sleep":     {},
	"benchmark": {},
	"get_lock":  {},
}

// parser is the shared Vitess SQL parser instance.
var parser = sqlparser.NewTestParser()

// ValidateReadOnly parses the SQL into an AST and verifies it is a safe,
// read-only SELECT query. Returns nil if safe, or an error describing why
// it was rejected.
//
// Checks performed:
//   - Only a single statement is allowed
//   - Statement must be SELECT (including UNION / CTE)
//   - No locking clauses (FOR UPDATE / FOR SHARE / LOCK IN SHARE MODE)
//   - No INTO OUTFILE / INTO DUMPFILE
//   - No dangerous functions (SLEEP, BENCHMARK, GET_LOCK)
//
// Note: unquoted non-ASCII identifiers (e.g. SELECT 1 AS 用户名) must use
// backticks (SELECT 1 AS `用户名`) since the Vitess parser requires it.
func ValidateReadOnly(sql string) error {
	sql = strings.TrimSpace(sql)
	if sql == "" {
		return fmt.Errorf("SQL 为空")
	}

	// Reject multiple statements (defense in depth — driver also blocks this)
	pieces, err := parser.SplitStatementToPieces(sql)
	if err != nil {
		return fmt.Errorf("SQL 解析失败: %w", err)
	}
	if len(pieces) > 1 {
		return fmt.Errorf("禁止执行多条语句（仅允许单条 SELECT）")
	}

	// Parse into AST — unparseable SQL is rejected (safe default)
	stmt, err := parser.Parse(sql)
	if err != nil {
		return fmt.Errorf("SQL 解析失败: %w", err)
	}

	// Only allow read-only SELECT statements
	if err := checkReadOnly(stmt); err != nil {
		return err
	}

	// Walk the entire AST to check for dangerous functions, lock functions,
	// and locking/INTO clauses in subqueries
	return checkDangerousNodes(stmt)
}

// checkReadOnly verifies the top-level statement is a read-only SELECT/UNION.
func checkReadOnly(stmt sqlparser.Statement) error {
	switch node := stmt.(type) {
	case *sqlparser.Select:
		return checkSelectSafe(node)
	case *sqlparser.Union:
		return checkUnionSafe(node)
	default:
		return fmt.Errorf("禁止执行 %s 语句（仅允许 SELECT 查询）", sqlparser.ASTToStatementType(stmt).String())
	}
}

// checkSelectSafe checks a single SELECT node for locking and INTO clauses.
func checkSelectSafe(sel *sqlparser.Select) error {
	if sel.Lock != sqlparser.NoLock {
		return fmt.Errorf("禁止使用锁子句: %s", sel.Lock.ToString())
	}
	if sel.Into != nil {
		return fmt.Errorf("禁止使用 INTO 子句")
	}
	return nil
}

// checkUnionSafe recursively validates both sides of a UNION.
func checkUnionSafe(union *sqlparser.Union) error {
	if union.Lock != sqlparser.NoLock {
		return fmt.Errorf("禁止使用锁子句: %s", union.Lock.ToString())
	}
	if union.Into != nil {
		return fmt.Errorf("禁止使用 INTO 子句")
	}
	if err := checkTableStmtSafe(union.Left); err != nil {
		return err
	}
	return checkTableStmtSafe(union.Right)
}

// checkTableStmtSafe dispatches to the appropriate checker for TableStatement.
func checkTableStmtSafe(stmt sqlparser.TableStatement) error {
	switch s := stmt.(type) {
	case *sqlparser.Select:
		return checkSelectSafe(s)
	case *sqlparser.Union:
		return checkUnionSafe(s)
	default:
		return fmt.Errorf("禁止执行 %T 语句（仅允许 SELECT 查询）", stmt)
	}
}

// checkDangerousNodes walks the entire AST to reject:
//   - Dangerous functions (SLEEP, BENCHMARK) via *FuncExpr
//   - Advisory lock functions (GET_LOCK, RELEASE_LOCK, etc.) via *LockingFunc
//   - Locking clauses in subqueries (FOR UPDATE inside a derived table)
func checkDangerousNodes(stmt sqlparser.Statement) error {
	var reason string

	_ = sqlparser.Walk(func(node sqlparser.SQLNode) (bool, error) {
		switch n := node.(type) {
		case *sqlparser.FuncExpr:
			name := strings.ToLower(n.Name.String())
			if _, bad := dangerousFunctions[name]; bad {
				reason = fmt.Sprintf("禁止调用危险函数: %s", n.Name.String())
				return false, nil
			}
		case *sqlparser.LockingFunc:
			reason = fmt.Sprintf("禁止调用锁函数: %s", sqlparser.String(n))
			return false, nil
		case *sqlparser.Select:
			// Catch FOR UPDATE / FOR SHARE in subqueries
			if n.Lock != sqlparser.NoLock {
				reason = fmt.Sprintf("禁止使用锁子句: %s", n.Lock.ToString())
				return false, nil
			}
			if n.Into != nil {
				reason = "禁止使用 INTO 子句"
				return false, nil
			}
		}
		return true, nil
	}, stmt)

	if reason != "" {
		return fmt.Errorf("%s", reason)
	}
	return nil
}
