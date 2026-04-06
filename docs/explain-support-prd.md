# Product Requirements Document: 支持 EXPLAIN 查询 & 移除 L2 预检

**Version**: 1.0
**Date**: 2026-04-06
**Author**: Sarah (Product Owner)
**Quality Score**: 93/100

---

## Executive Summary

用户需要通过 `EXPLAIN SELECT ...` 查看查询执行计划以优化 SQL 性能，但当前 L1 AST 验证将 EXPLAIN 视为非 SELECT 语句而拒绝。

同时，L2 EXPLAIN 预检层（`execute.go` 中在执行前发送 `EXPLAIN <sql>` 来拦截 DDL）在 AST 验证引入后已变得冗余——AST 在应用层就能完整拦截所有非 SELECT 语句，L2 只增加了一次无意义的网络往返。

本次改动：允许 `EXPLAIN SELECT/UNION` 通过 L1 验证，同时移除 L2 EXPLAIN 预检。防御架构从三层简化为两层（L1 AST + L2 READ ONLY 事务），更简洁、更快。

---

## Problem Statement

**当前情况**:
1. `EXPLAIN SELECT * FROM users` 被 L1 拒绝（"禁止执行 EXPLAIN 语句"），DBA 无法查看执行计划
2. L2 EXPLAIN 预检对每次查询多一次网络往返，但 AST 已能完整拦截 DDL/DML，L2 成为纯开销

**解决方案**:
1. L1 AST 验证中允许 `*sqlparser.ExplainStmt`，但仅当其内部语句是 SELECT/UNION 时
2. 移除 `execute.go` 中的 EXPLAIN 预检代码（第 39-46 行）

**业务影响**: DBA 可查看执行计划；每次查询少一次 MySQL 往返

---

## Success Metrics

**Primary KPIs:**
- `EXPLAIN SELECT ...` 正常返回执行计划
- `EXPLAIN INSERT/UPDATE/DELETE/DROP` 仍被 L1 拒绝
- 查询延迟降低（移除 EXPLAIN 预检的网络往返）
- L2 READ ONLY 事务仍正常拦截 DML

---

## User Stories & Acceptance Criteria

### Story 1: 查看查询执行计划

**As a** DBA
**I want to** 执行 `EXPLAIN SELECT * FROM users`
**So that** 我能查看 MySQL 的查询执行计划，优化慢查询

**Acceptance Criteria:**
- [ ] `EXPLAIN SELECT * FROM users` 返回执行计划结果
- [ ] `EXPLAIN SELECT ... UNION SELECT ...` 正常工作
- [ ] `WITH cte AS (...) EXPLAIN SELECT ...` — 如果 Vitess 支持则允许
- [ ] `EXPLAIN FORMAT=JSON SELECT ...` 正常工作（如 Vitess 支持）
- [ ] `EXPLAIN ANALYZE SELECT ...` 正常工作（如 Vitess 支持）

### Story 2: 拒绝 EXPLAIN 非只读语句

**As a** 安全审计员
**I want to** `EXPLAIN INSERT/UPDATE/DELETE/DROP` 仍被拒绝
**So that** 攻击者不能通过 EXPLAIN 绕过安全检查

**Acceptance Criteria:**
- [ ] `EXPLAIN INSERT INTO users ...` 被 L1 拒绝
- [ ] `EXPLAIN UPDATE users SET ...` 被 L1 拒绝
- [ ] `EXPLAIN DELETE FROM users` 被 L1 拒绝
- [ ] `EXPLAIN DROP TABLE users` — 被 L1 拒绝（AST 解析失败或类型不匹配）

### Story 3: 移除 L2 EXPLAIN 预检

**As a** 开发者
**I want to** 移除 execute.go 中的 EXPLAIN 预检
**So that** 减少一次无用的网络往返，简化代码

**Acceptance Criteria:**
- [ ] `execute.go` 中删除 `EXPLAIN` 预检代码（第 39-46 行）
- [ ] READ ONLY 事务保留（仍作为 L2 兜底）
- [ ] 审计日志中 `validation` 字段移除 `l2_explain`，仅保留 `l1_ast` 和 `l2_readonly_tx`

---

## Functional Requirements

### 改动 1: validate.go — 允许 EXPLAIN SELECT

在 `checkReadOnly` 中新增对 `*sqlparser.ExplainStmt` 的处理：

```go
case *sqlparser.ExplainStmt:
    // EXPLAIN 只允许包含只读 SELECT
    return checkReadOnly(node.Statement)  // 递归检查内部语句
```

同时 `checkDangerousNodes` Walk 需要能穿透 ExplainStmt 内部节点（Walk 天然支持）。

### 改动 2: execute.go — 删除 EXPLAIN 预检

删除第 39-46 行的 EXPLAIN 预检代码。更新注释，说明防御架构现在是 L1 AST + L2 READ ONLY TX。

### 改动 3: audit 日志字段调整

`internal/audit/audit.go` 的 `Validation` struct：
- 移除 `L2Explain` 字段
- 将 `L3ReadOnly` 重命名为 `L2ReadOnly`
- `cmd/query.go` 中对应更新

### Out of Scope
- EXPLAIN 结果的格式化/美化展示
- EXPLAIN ANALYZE 的特殊处理（如果 Vitess 不支持则暂不处理）

---

## Technical Constraints

### Vitess Parser 兼容性
- 需确认 `sqlparser.ExplainStmt` 的结构，找到内部 Statement 字段
- 需确认 `EXPLAIN FORMAT=JSON SELECT ...` 是否能被解析

### 安全
- EXPLAIN 本身在 MySQL 中是只读的（不执行 DML），但为防万一仍在 L1 限制只允许 EXPLAIN + SELECT
- READ ONLY 事务作为最后兜底

---

## MVP Scope

- 允许 `EXPLAIN SELECT` 通过 L1
- 拒绝 `EXPLAIN INSERT/UPDATE/DELETE`
- 移除 L2 EXPLAIN 预检
- 更新审计日志字段

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| Vitess ExplainStmt 结构与预期不同 | Low | Medium | 先 `go doc` 确认结构再编码 |
| 移除 L2 后 DDL 漏过 | Low | High | AST 已完整拦截 DDL + READ ONLY TX 兜底 |

---

*This PRD was created through interactive requirements gathering with quality scoring.*
