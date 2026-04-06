# Product Requirements Document: 查询审计日志

**Version**: 1.0
**Date**: 2026-04-06
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100

---

## Executive Summary

sql-query 工具目前执行 SQL 后没有任何执行记录留存。当需要事后排查"谁在什么时候查了什么数据"或"是否有人尝试注入攻击"时，缺乏审计能力。

本功能在每次 `query` 命令执行时，自动将执行记录以 JSONL 格式追加到按天滚动的日志文件中。记录内容涵盖 SQL 原文、执行结果、耗时、安全校验详情等，支持 grep/jq 快速查询。无论执行成功还是被安全拦截，均记录在案。

---

## Problem Statement

**当前情况**: 查询执行后无痕迹，无法回溯执行历史、排查数据泄露嫌疑、统计使用频率

**解决方案**: 在 query 命令执行链路中嵌入审计日志，JSONL 格式按天归档

**业务影响**: 建立完整审计链路，满足安全合规要求，为异常行为检测提供数据基础

---

## Success Metrics

**Primary KPIs:**
- 100% 查询覆盖率：所有成功/失败/拦截的查询均有记录
- 日志可查询性：支持 `jq` / `grep` 在 5 秒内定位特定记录
- 零性能影响：日志写入不显著增加查询耗时（<5ms overhead）

---

## User Personas

### Primary: 运维/DBA
- **Goals**: 事后排查谁查了什么数据，是否有注入尝试
- **Pain Points**: 目前无任何执行记录
- **Technical Level**: 高级，熟悉 jq/grep

---

## User Stories & Acceptance Criteria

### Story 1: 自动记录每次查询

**As a** 运维人员
**I want to** 每次 query 执行后自动写入审计日志
**So that** 事后可以回溯所有查询历史

**Acceptance Criteria:**
- [ ] 成功执行的 SELECT 写入日志
- [ ] 被 L1 AST 验证拦截的 SQL 写入日志（status=rejected, 含拦截原因）
- [ ] 被 L2 EXPLAIN / L3 READ ONLY TX 拦截的 SQL 写入日志（status=error）
- [ ] 日志写入失败不影响主流程（仅 stderr 警告）

### Story 2: 按天归档

**As a** 运维人员
**I want to** 日志按天分文件存储
**So that** 单个文件不会过大，方便按日期查找

**Acceptance Criteria:**
- [ ] 文件名格式: `query-audit-YYYY-MM-DD.log`
- [ ] 日志目录通过 .env 中 `AUDIT_LOG_DIR` 配置（默认当前目录）
- [ ] 每行一个 JSON 对象（JSONL 格式），方便 `jq` 处理

### Story 3: 使用 jq 查询审计日志

**As a** 运维人员
**I want to** 用 jq/grep 快速过滤日志
**So that** 能快速定位特定查询或异常

**Acceptance Criteria:**
- [ ] `jq 'select(.status=="rejected")' query-audit-*.log` 能过滤出所有被拦截的查询
- [ ] `jq 'select(.duration_ms > 1000)' query-audit-*.log` 能找出慢查询
- [ ] `grep "DROP" query-audit-*.log` 能找到注入尝试

---

## Functional Requirements

### 日志字段定义

```jsonc
{
  // 基础信息
  "timestamp": "2026-04-06T15:30:45.123+08:00",  // RFC3339 with ms
  "status": "success",                             // success | rejected | error
  "sql": "SELECT * FROM users LIMIT 10",           // SQL 原文
  "duration_ms": 42,                               // 执行耗时（毫秒）
  "rows": 10,                                      // 返回行数（成功时）
  "columns": 5,                                    // 返回列数（成功时）
  "error": null,                                   // 错误信息（失败/拦截时）

  // 环境信息
  "env_file": ".env.test",                         // 使用的 .env 文件路径
  "database": "testdb",                            // 数据库名（从 DSN 解析）
  "user": "sqlquery",                              // 连接用户（从 DSN 解析）

  // 导出信息
  "output_format": "json",                         // json | excel | html
  "output_file": "output.xlsx",                    // 输出文件路径（无则为 null/stdout）

  // 验证详情
  "validation": {
    "l1_ast": "pass",                              // pass | rejected
    "l1_reason": null,                             // 拦截原因（被拦截时）
    "l2_explain": "pass",                          // pass | error | skipped
    "l3_readonly_tx": "pass"                       // pass | error | skipped
  }
}
```

### 字段说明

| 字段 | 类型 | 何时填充 | 说明 |
|------|------|----------|------|
| `status` | string | 始终 | `success`: 查询成功; `rejected`: L1 拦截; `error`: L2/L3/执行失败 |
| `duration_ms` | int | 始终 | 从验证开始到结束的总耗时 |
| `rows` / `columns` | int | 仅 success | 被拦截时为 0 或不填 |
| `error` | string | 仅 rejected/error | 错误/拦截原因原文 |
| `user` / `database` | string | 始终 | 从 DSN 中解析，**不记录密码** |
| `validation.l1_ast` | string | 始终 | L1 验证结果 |
| `validation.l2_explain` | string | L1 通过时 | L1 被拦截时为 `skipped` |
| `validation.l3_readonly_tx` | string | L2 通过时 | 同上 |

### 配置

| 环境变量 | 默认值 | 说明 |
|----------|--------|------|
| `AUDIT_LOG_DIR` | `.`（当前目录） | 日志文件目录 |

### Out of Scope
- 日志查询子命令（用 jq/grep 即可）
- 日志轮转/清理（运维层面用 logrotate 解决）
- 日志加密或签名
- 远程日志推送（syslog/ELK）

---

## Technical Constraints

### Performance
- 日志写入使用 append-only 模式，每次 open → write → close，避免持有文件句柄
- 写入耗时控制在 <5ms，不阻塞查询返回

### Security
- **DSN 中的密码不得出现在日志中**，仅记录 user 和 database
- 日志文件权限建议 0640

### Integration
- 嵌入 `cmd/query.go` 的 RunE 函数中
- 在 `ValidateReadOnly` 之前开始计时
- 在查询结束（无论成功/失败/拦截）后写入日志
- 新增 `internal/audit` 包

---

## MVP Scope

### Phase 1: MVP
- JSONL 审计日志，按天分文件
- 记录全部四类信息（基础/环境/导出/验证）
- 成功 + 拦截 + 失败全量记录
- `AUDIT_LOG_DIR` 环境变量配置

### Future Considerations
- `audit` 子命令：查询/统计日志
- 日志推送到远程（syslog/webhook）
- 日志签名防篡改

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation |
|------|------------|--------|------------|
| 日志写入失败影响主流程 | Low | High | 日志写入错误仅 stderr 警告，不阻断查询 |
| 日志文件过大 | Medium | Low | 按天分文件 + 文档说明 logrotate |
| 敏感信息泄露到日志 | Low | High | 仅从 DSN 提取 user/database，不记录密码 |

---

*This PRD was created through interactive requirements gathering with quality scoring.*
