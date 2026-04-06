# Product Requirements Document: sql-query

**Version**: 1.1
**Date**: 2026-04-05
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100

---

## Executive Summary

`sql-query` 是一个独立的 CLI 工具，专为 AI skills 设计，提供 MySQL 数据库结构查询和 SQL 查询导出能力。工具支持三个核心子命令：查看数据库表列表、查看表 DDL、执行 SQL 并导出为 Excel/HTML/JSON。导出逻辑参考现有 `sql-exporter` 项目的元数据协议、增强型导出等能力。

---

## Problem Statement

**现状**：现有 `sql-exporter` 是单命令工具，仅支持"执行 SQL → 导出文件"的单一流程，无法查询数据库结构信息（表列表、DDL），且不支持 JSON 输出格式，不方便 AI agent 程序化调用。

**方案**：构建独立 `sql-query` CLI，提供结构化的子命令体系，支持文本和 JSON 双重输出模式，满足 AI skills 对数据库元信息查询和数据导出的需求。

---

## Success Metrics

- AI skills 能通过 `sql-query tables` 和 `sql-query table <name>` 获取数据库结构
- 支持 `--json` 输出，方便程序化解析
- `sql-query query` 完整复用 sql-exporter 的元数据协议和导出能力
- stdout 仅输出数据，日志走 stderr，支持管道操作

---

## User Personas

### Primary: AI Skill / Agent
- **角色**：自动化工具调用方
- **目标**：查询数据库结构、执行 SQL 并获取结构化结果
- **需求**：干净的 stdout 输出（JSON/文本），无干扰日志；错误时获取结构化错误信息

### Secondary: 开发者
- **角色**：手动执行 CLI 的开发人员
- **目标**：快速查看表结构、导出查询结果
- **需求**：人类可读的文本输出

---

## Functional Requirements

### 子命令 1: `sql-query tables`

**功能**：列出当前数据库所有表的名称和注释

**参数**：

| 参数 | 类型 | 说明 |
|---|---|---|
| `-e, --env` | string | .env 配置文件路径 |
| `--json` | bool | 以 JSON 格式输出 |

**文本输出**：

```
Found 3 tables:
  users      - 用户表
  orders     - 订单表
  products   - 商品表
```

**JSON 输出**：

```json
[
  {"table_name": "users", "table_comment": "用户表"},
  {"table_name": "orders", "table_comment": "订单表"},
  {"table_name": "products", "table_comment": "商品表"}
]
```

**SQL 实现**：

```sql
SELECT TABLE_NAME AS table_name,
       COALESCE(TABLE_COMMENT, '') AS table_comment
FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_TYPE = 'BASE TABLE'
ORDER BY TABLE_NAME
```

---

### 子命令 2: `sql-query table <name>`

**功能**：查看指定表的完整 DDL（含字段定义、索引、注释）

**参数**：

| 参数 | 类型 | 说明 |
|---|---|---|
| `<name>` | arg | 表名（必填） |
| `-e, --env` | string | .env 配置文件路径 |
| `--json` | bool | 以 JSON 格式输出 |

**文本输出**：直接打印 `SHOW CREATE TABLE` 的原生 DDL

```sql
CREATE TABLE `users` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `username` varchar(64) NOT NULL COMMENT '用户名',
  `email` varchar(128) DEFAULT NULL COMMENT '邮箱',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '状态：1启用 0禁用',
  `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB AUTO_INCREMENT=10001 DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

**JSON 输出**：DDL + 结构化字段/索引信息

```json
{
  "table_name": "users",
  "comment": "用户表",
  "ddl": "CREATE TABLE `users` (...) ENGINE=InnoDB ...",
  "columns": [
    {"name": "id", "type": "bigint(20) unsigned", "nullable": "NO", "default": null, "comment": "主键ID"},
    {"name": "username", "type": "varchar(64)", "nullable": "NO", "default": null, "comment": "用户名"},
    {"name": "email", "type": "varchar(128)", "nullable": "YES", "default": null, "comment": "邮箱"},
    {"name": "status", "type": "tinyint(4)", "nullable": "NO", "default": "1", "comment": "状态：1启用 0禁用"},
    {"name": "created_at", "type": "datetime", "nullable": "NO", "default": "CURRENT_TIMESTAMP", "comment": "创建时间"}
  ],
  "indexes": [
    {"name": "PRIMARY", "columns": "id", "is_unique": true, "is_primary": true, "index_type": "BTREE"},
    {"name": "uk_username", "columns": "username", "is_unique": true, "is_primary": false, "index_type": "BTREE"},
    {"name": "idx_status", "columns": "status", "is_unique": false, "is_primary": false, "index_type": "BTREE"}
  ]
}
```

**SQL 实现**：

```sql
-- 1. 原生 DDL
SHOW CREATE TABLE `table_name`;

-- 2. 结构化列信息
SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, COLUMN_COMMENT
FROM INFORMATION_SCHEMA.COLUMNS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
ORDER BY ORDINAL_POSITION;

-- 3. 结构化索引信息
SELECT INDEX_NAME, GROUP_CONCAT(COLUMN_NAME ORDER BY SEQ_IN_INDEX) AS COLUMNS,
       NOT NON_UNIQUE AS IS_UNIQUE, INDEX_NAME = 'PRIMARY' AS IS_PRIMARY, INDEX_TYPE
FROM INFORMATION_SCHEMA.STATISTICS
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?
GROUP BY INDEX_NAME, NON_UNIQUE, INDEX_TYPE;

-- 4. 表注释
SELECT TABLE_COMMENT FROM INFORMATION_SCHEMA.TABLES
WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = ?;
```

---

### 子命令 3: `sql-query query`

**功能**：执行 SQL 查询并导出结果，完整复用 sql-exporter 的元数据协议处理

**参数**：

| 参数 | 类型 | 说明 |
|---|---|---|
| `-e, --env` | string | .env 配置文件路径 |
| `-f, --file` | string | SQL 文件路径（不指定则读取 stdin） |
| `-o, --output` | string | 输出文件路径 |
| `--excel` | bool | 导出为 Excel |
| `--html` | bool | 导出为 HTML |
| `--json` | bool | 导出为 JSON（新增） |
| `-w, --workers` | int | 并发处理数（默认 CPU 核数） |

**处理流程**（参考 sql-exporter）：

1. 执行 SQL，获取列名和数据
2. 解析列别名中的元数据协议（`[URL(24h)]`、`[HTML(I,P:预览->ID)]` 等）
3. 并发处理 S3 预签名 URL（Phase 2，见 MVP 分期）
4. 按指定格式导出

**JSON 导出格式**：

```json
[
  {"用户名": "alice", "邮箱": "alice@example.com", "头像": "https://s3.../signed-url"},
  {"用户名": "bob", "邮箱": "bob@example.com", "头像": "https://s3.../signed-url"}
]
```

- key 使用元数据解析后的 DisplayName
- null 值输出 JSON null
- 无 `-o` 时写 stdout，有 `-o` 时写文件

---

## Error Handling

### 退出码规范

| 退出码 | 含义 | 示例场景 |
|--------|------|---------|
| 0 | 成功 | 正常执行完成 |
| 1 | 通用错误 | SQL 语法错误、参数无效、文件读写失败 |
| 2 | 连接失败 | 数据库连接超时、认证失败、DSN 配置错误 |
| 3 | 表不存在 | `table <name>` 指定的表不存在 |

### 错误输出规范

**文本模式**：错误信息输出到 stderr，stdout 无输出

```
# stderr
Error: table 'foo' does not exist in database 'mydb'
```

**JSON 模式（`--json`）**：stderr 输出人类可读错误，stdout 同时输出结构化错误 JSON

```
# stderr
Error: table 'foo' does not exist

# stdout
{"error": "table_not_found", "message": "table 'foo' does not exist in database 'mydb'"}
```

### 错误码对照表

| error 字段值 | 退出码 | 说明 |
|---|---|---|
| `connection_failed` | 2 | 数据库连接失败 |
| `table_not_found` | 3 | 表不存在 |
| `sql_syntax_error` | 1 | SQL 语法错误 |
| `invalid_argument` | 1 | 参数无效 |
| `file_error` | 1 | 文件读写失败 |
| `query_timeout` | 1 | 查询超时 |

### 边缘情况

| 场景 | 行为 |
|------|------|
| 空结果集 | 文本模式输出 "No rows returned"；JSON 模式输出 `[]` |
| .env 文件不存在 | 退出码 1，提示文件路径不存在 |
| stdin 无输入（query 命令无 -f 且无 stdin） | 退出码 1，提示需要提供 SQL |

---

## Performance Constraints

| 参数 | 默认值 | 配置方式 | 说明 |
|------|--------|---------|------|
| `QUERY_TIMEOUT` | 300 秒（5 分钟） | 环境变量 | SQL 查询超时时间，超时后终止查询并返回错误 |
| 导出行数 | 无限制 | — | 不限制导出行数 |
| `-w, --workers` | CPU 核数 | CLI 参数 | S3 预签名并发处理数（Phase 2） |

---

## Technical Architecture

### 项目结构

```
sql-query/
├── main.go                          # 入口
├── go.mod
├── cmd/
│   ├── root.go                      # 根命令 + 共享 flag + loadConfigAndConnect()
│   ├── tables.go                    # tables 子命令
│   ├── table.go                     # table <name> 子命令
│   └── query.go                     # query 子命令
├── internal/
│   ├── config/
│   │   └── config.go                # 环境变量配置加载（DB_DSN, S3_*）
│   ├── db/
│   │   ├── connect.go               # MySQL 连接（GORM）
│   │   ├── execute.go               # SQL 执行 + 数据读取
│   │   ├── tables.go                # 表列表查询
│   │   └── ddl.go                   # DDL 查询
│   ├── parser/
│   │   └── meta.go                  # 元数据协议解析
│   ├── processor/
│   │   └── pipeline.go              # S3 预签名并发处理（Phase 2）
│   ├── exporter/
│   │   ├── exporter.go              # Exporter 接口
│   │   ├── excel.go                 # Excel 导出
│   │   ├── html.go                  # HTML 导出
│   │   ├── json.go                  # JSON 导出（新）
│   │   └── templates/
│   │       └── report.tmpl          # HTML 模板
│   └── s3/
│       └── presigner.go             # S3 预签名（Phase 2）
└── .env.example
```

### 依赖

| 依赖 | 用途 |
|---|---|
| `github.com/spf13/cobra` | CLI 框架 |
| `gorm.io/gorm` + `gorm.io/driver/mysql` | MySQL 连接与查询 |
| `github.com/xuri/excelize/v2` | Excel 导出 |
| `github.com/aws/aws-sdk-go-v2` | S3 预签名（Phase 2） |
| `github.com/joho/godotenv` | .env 文件加载 |

### 从 sql-exporter 移植的模块

| 模块 | 参考来源 | 改动 |
|---|---|---|
| `internal/config/config.go` | `sql-exporter/internal/config/config.go` | 去掉 PG 相关 |
| `internal/db/connect.go` | `sql-exporter/internal/db/database.go` | 仅保留 MySQL 连接 |
| `internal/db/execute.go` | `sql-exporter/internal/db/database.go` | SQL 执行逻辑，日志改 stderr |
| `internal/parser/meta.go` | `sql-exporter/internal/parser/meta.go` | 完整移植 |
| `internal/processor/pipeline.go` | `sql-exporter/internal/processor/pipeline.go` | 完整移植，日志改 stderr（Phase 2） |
| `internal/exporter/*` | `sql-exporter/internal/exporter/*` | 完整移植 |
| `internal/s3/presigner.go` | `sql-exporter/internal/s3/presigner.go` | 完整移植（Phase 2） |

### 新增模块

| 模块 | 说明 |
|---|---|
| `cmd/root.go` | Cobra 根命令，PersistentFlags，loadConfigAndConnect() |
| `cmd/tables.go` | tables 子命令 |
| `cmd/table.go` | table 子命令 |
| `cmd/query.go` | query 子命令（参考 sql-exporter 的 cmd/root.go run 函数） |
| `internal/db/tables.go` | GetTables — INFORMATION_SCHEMA.TABLES |
| `internal/db/ddl.go` | GetTableDDL — SHOW CREATE TABLE + COLUMNS + STATISTICS |
| `internal/exporter/json.go` | JSON 导出器，实现 Exporter 接口 |

### 关键数据结构

```go
// internal/db/tables.go
type TableInfo struct {
    TableName    string `json:"table_name" gorm:"column:table_name"`
    TableComment string `json:"table_comment" gorm:"column:table_comment"`
}

// internal/db/ddl.go
type ColumnInfo struct {
    Name     string  `json:"name"`
    Type     string  `json:"type"`
    Nullable string  `json:"nullable"`
    Default  *string `json:"default,omitempty"`
    Comment  string  `json:"comment,omitempty"`
}

type IndexInfo struct {
    Name      string `json:"name"`
    Columns   string `json:"columns"`
    IsUnique  bool   `json:"is_unique"`
    IsPrimary bool   `json:"is_primary"`
    IndexType string `json:"index_type"`
}

type TableDDL struct {
    TableName string       `json:"table_name"`
    Comment   string       `json:"comment,omitempty"`
    Columns   []ColumnInfo `json:"columns"`
    Indexes   []IndexInfo  `json:"indexes"`
    RawDDL    string       `json:"ddl"`
}
```

---

## Design Constraints

- **仅支持 MySQL**
- **stdout 只输出数据**（表格 / JSON / DDL），日志走 stderr
- **SQL 注入防护**：表名用反引号包裹
- **JSON 导出**：无 `-o` 时写 stdout，有 `-o` 时写文件
- 元数据协议语法与 sql-exporter 完全一致

---

## MVP Scope & Phasing

### Phase 1: MVP（首版交付）

**包含：**
- `sql-query tables` — 列出表名和注释（文本 + JSON）
- `sql-query table <name>` — 查看表 DDL（文本 + JSON）
- `sql-query query` — 执行 SQL 并导出为 Excel / HTML / JSON
- 元数据协议解析（列别名中的 DisplayName 等）
- 完整的错误处理和退出码规范
- 查询超时控制（`QUERY_TIMEOUT` 环境变量）

**不包含：**
- S3 预签名 URL 处理（`[URL(24h)]` 协议暂不生效）
- `internal/s3/` 和 `internal/processor/` 模块

**MVP 定义**：三个子命令均可正常使用，AI Agent 能通过 JSON 模式获取结构化数据和错误信息。

### Phase 2: S3 预签名增强

- 移植 `internal/s3/presigner.go` 和 `internal/processor/pipeline.go`
- `query` 子命令支持 `[URL(24h)]` 元数据协议
- `-w, --workers` 参数生效
- S3 相关环境变量配置（`S3_*`）

### Future Considerations
- 支持更多数据库（PostgreSQL、SQLite）
- 交互式查询模式
- 查询结果分页

---

## Implementation Plan

### Step 1: 项目初始化
- `go mod init sql-query`
- 添加依赖

### Step 2: 移植基础模块
- config、db/connect、db/execute、parser（不含 s3、processor）

### Step 3: 移植导出模块
- exporter 接口、excel、html + 模板

### Step 4: 新增 db 查询
- `internal/db/tables.go` — GetTables
- `internal/db/ddl.go` — GetTableDDL

### Step 5: 新增 JSON 导出器
- `internal/exporter/json.go`

### Step 6: CLI 命令层 + 错误处理
- root.go、tables.go、table.go、query.go、main.go
- 统一错误处理：退出码、JSON 错误输出

---

## Verification

```bash
go build -o sql-query .

# 列出表
./sql-query tables -e .env
./sql-query tables -e .env --json

# 查看表结构
./sql-query table users -e .env
./sql-query table users -e .env --json

# 错误场景
./sql-query table nonexistent -e .env --json    # 退出码 3, JSON 错误
./sql-query tables -e nonexistent.env            # 退出码 1, 文件不存在

# 执行查询导出
./sql-query query -e .env -f query.sql --json -o out.json
./sql-query query -e .env -f query.sql --excel -o out.xlsx
./sql-query query -e .env -f query.sql --html -o out.html
echo "SELECT 1" | ./sql-query query -e .env --json   # stdin 输入
```
