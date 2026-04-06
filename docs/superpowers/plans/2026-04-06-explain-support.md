# EXPLAIN Support & Remove L2 Pre-check Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Allow `EXPLAIN SELECT` queries and remove the redundant L2 EXPLAIN pre-check, simplifying the defense architecture to L1 AST + L2 READ ONLY transaction.

**Architecture:** `validate.go` gains a `*sqlparser.ExplainStmt` case that recursively validates the inner statement is a safe SELECT. `execute.go` drops the EXPLAIN pre-check (lines 39-46). Audit `Validation` struct loses `L2Explain`, renames `L3ReadOnly` to `L2ReadOnly`.

**Tech Stack:** Go, Vitess sqlparser (already imported).

**PRD:** `docs/explain-support-prd.md`

---

### Task 1: Allow EXPLAIN SELECT in AST validation

**Files:**
- Modify: `internal/db/validate.go:64-73`

- [ ] **Step 1: Add ExplainStmt case to checkReadOnly**

In `internal/db/validate.go`, replace the `checkReadOnly` function (lines 64-74) with:

```go
// checkReadOnly verifies the top-level statement is a read-only SELECT/UNION,
// or an EXPLAIN wrapping a read-only SELECT/UNION.
func checkReadOnly(stmt sqlparser.Statement) error {
	switch node := stmt.(type) {
	case *sqlparser.Select:
		return checkSelectSafe(node)
	case *sqlparser.Union:
		return checkUnionSafe(node)
	case *sqlparser.ExplainStmt:
		// EXPLAIN is allowed only when its inner statement is a read-only SELECT.
		return checkReadOnly(node.Statement)
	default:
		return fmt.Errorf("禁止执行 %s 语句（仅允许 SELECT 查询）", sqlparser.ASTToStatementType(stmt).String())
	}
}
```

- [ ] **Step 2: Verify it compiles**

Run: `/usr/local/go/bin/go build ./internal/db/`
Expected: no errors

- [ ] **Step 3: Verify EXPLAIN SELECT is allowed**

Quick smoke test by writing a temporary main or using the existing binary:

```bash
/usr/local/go/bin/go build -o /tmp/sq-test . && echo "EXPLAIN SELECT * FROM users" | /tmp/sq-test query -e .env.test --json --log-level error 2>&1 | head -5
```

Expected: JSON output with EXPLAIN result rows (id, select_type, table, etc.), no "sql_rejected" error.

- [ ] **Step 4: Verify EXPLAIN INSERT is still blocked**

```bash
echo "EXPLAIN INSERT INTO users (username) VALUES ('x')" | /tmp/sq-test query -e .env.test --json --log-level error 2>&1
```

Expected: `SQL 安全校验失败: 禁止执行 INSERT 语句（仅允许 SELECT 查询）`

- [ ] **Step 5: Commit**

```bash
git add internal/db/validate.go
git commit -m "feat: allow EXPLAIN SELECT in AST validation"
```

---

### Task 2: Remove L2 EXPLAIN pre-check from execute.go

**Files:**
- Modify: `internal/db/execute.go:14-46`

- [ ] **Step 1: Remove EXPLAIN pre-check and update comment**

In `internal/db/execute.go`, replace lines 14-46 (the function comment and the EXPLAIN block) with:

```go
// Execute runs a SQL query inside a read-only transaction.
// The read-only transaction (START TRANSACTION READ ONLY) is enforced by MySQL —
// any write attempt (INSERT/DELETE/DROP etc.) will be rejected by the database engine.
// Each cell is *string (nil = SQL NULL). timeoutSec <= 0 means no timeout.
// maxRows <= 0 means no limit.
func Execute(gormDB *gorm.DB, sqlContent string, timeoutSec int, maxRows int) ([]string, [][]*string, error) {
	sqlDB, err := gormDB.DB()
	if err != nil {
		return nil, nil, err
	}

	ctx := context.Background()
	var cancel context.CancelFunc
	if timeoutSec > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	// Start a read-only transaction — MySQL will reject DML (INSERT/UPDATE/DELETE)
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, sqlContent)
```

This removes the entire EXPLAIN pre-check block (old lines 39-46) and goes straight from `tx.Rollback()` to `tx.QueryContext`.

- [ ] **Step 2: Verify it compiles**

Run: `/usr/local/go/bin/go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/db/execute.go
git commit -m "refactor: remove L2 EXPLAIN pre-check from execute

AST validation (L1) now fully handles statement type checking.
The EXPLAIN pre-check was redundant — it added a network round-trip
for every query with no additional safety benefit."
```

---

### Task 3: Simplify audit Validation struct (3-layer → 2-layer)

**Files:**
- Modify: `internal/audit/audit.go:12-17`
- Modify: `cmd/query.go:97-131`

- [ ] **Step 1: Update Validation struct**

In `internal/audit/audit.go`, replace the Validation struct (lines 12-17) with:

```go
// Validation holds the result of each security layer.
type Validation struct {
	L1AST      string `json:"l1_ast"`             // pass | rejected
	L1Reason   string `json:"l1_reason,omitempty"` // rejection reason (if rejected)
	L2ReadOnly string `json:"l2_readonly_tx"`      // pass | error | skipped
}
```

- [ ] **Step 2: Update cmd/query.go audit field references**

In `cmd/query.go`, replace the rejected block (around lines 99-109) — change `L2Explain` and `L3ReadOnly` references:

Find:
```go
			entry.Validation.L1AST = "rejected"
			entry.Validation.L1Reason = err.Error()
			entry.Validation.L2Explain = "skipped"
			entry.Validation.L3ReadOnly = "skipped"
```

Replace with:
```go
			entry.Validation.L1AST = "rejected"
			entry.Validation.L1Reason = err.Error()
			entry.Validation.L2ReadOnly = "skipped"
```

Then find the error block (around lines 117-121):

Find:
```go
			entry.Validation.L2Explain = "error"
			entry.Validation.L3ReadOnly = "skipped"
```

Replace with:
```go
			entry.Validation.L2ReadOnly = "error"
```

Then find the success block (around lines 126-127):

Find:
```go
		entry.Validation.L2Explain = "pass"
		entry.Validation.L3ReadOnly = "pass"
```

Replace with:
```go
		entry.Validation.L2ReadOnly = "pass"
```

- [ ] **Step 3: Update the comment in cmd/query.go**

Find:
```go
		// Execute SQL — EXPLAIN pre-check + READ ONLY transaction
```

Replace with:
```go
		// Execute SQL — READ ONLY transaction
```

- [ ] **Step 4: Verify it compiles**

Run: `/usr/local/go/bin/go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/audit/audit.go cmd/query.go
git commit -m "refactor(audit): simplify validation from 3-layer to 2-layer

Remove L2Explain field, rename L3ReadOnly to L2ReadOnly to reflect
the new architecture: L1 AST validation + L2 READ ONLY transaction."
```

---

### Task 4: End-to-end verification

**Files:** none (testing only)

- [ ] **Step 1: Build the binary**

```bash
/usr/local/go/bin/go build -o sql-query .
```

- [ ] **Step 2: Test EXPLAIN SELECT works**

```bash
export AUDIT_LOG_DIR=/tmp/audit-test && mkdir -p $AUDIT_LOG_DIR
echo "EXPLAIN SELECT * FROM users" | ./sql-query query -e .env.test --json --log-level error
```

Expected: JSON output with EXPLAIN rows (id, select_type, table, type, etc.)

- [ ] **Step 3: Test EXPLAIN FORMAT=JSON SELECT works**

```bash
echo "EXPLAIN FORMAT=JSON SELECT * FROM users WHERE id = 1" | ./sql-query query -e .env.test --json --log-level error
```

Expected: JSON output (single row with EXPLAIN column containing JSON query plan)

- [ ] **Step 4: Test EXPLAIN INSERT is blocked**

```bash
echo "EXPLAIN INSERT INTO users (username) VALUES ('x')" | ./sql-query query -e .env.test --json --log-level error 2>&1
```

Expected: `SQL 安全校验失败: 禁止执行 INSERT 语句`

- [ ] **Step 5: Test EXPLAIN DELETE is blocked**

```bash
echo "EXPLAIN DELETE FROM users" | ./sql-query query -e .env.test --json --log-level error 2>&1
```

Expected: `SQL 安全校验失败: 禁止执行 DELETE 语句`

- [ ] **Step 6: Test normal SELECT still works**

```bash
echo "SELECT COUNT(*) AS total FROM users" | ./sql-query query -e .env.test --json --log-level error
```

Expected: `[{"total": "4"}]`

- [ ] **Step 7: Verify audit log has 2-layer validation**

```bash
jq '.validation' $AUDIT_LOG_DIR/query-audit-$(date +%Y-%m-%d).log | head -20
```

Expected: `l1_ast` and `l2_readonly_tx` fields only, no `l2_explain`.

- [ ] **Step 8: Test injection via EXPLAIN is blocked**

```bash
echo "EXPLAIN SELECT SLEEP(10)" | ./sql-query query -e .env.test --json --log-level error 2>&1
```

Expected: `SQL 安全校验失败: 禁止调用危险函数: SLEEP`

- [ ] **Step 9: Clean up**

```bash
rm -rf /tmp/audit-test
```
