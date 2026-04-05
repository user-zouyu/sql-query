# sql-query 使用手册

`sql-query` 是一个 MySQL 数据库结构查询与 SQL 导出 CLI 工具，专为 AI Agent 设计。支持三个子命令：查看表列表、查看表 DDL、执行 SQL 导出。

## 安装

```bash
go build -o sql-query .
```

## 快速开始

```bash
# 1. 配置数据库连接
cp .env.example .env
# 编辑 .env，填入 DB_DSN

# 2. 查看所有表
./sql-query tables -e .env

# 3. 查看表结构
./sql-query table users -e .env

# 4. 执行查询
echo "SELECT * FROM users LIMIT 10" | ./sql-query query -e .env --json
```

## 配置

### 环境变量

通过 `.env` 文件或系统环境变量配置：

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `DB_DSN` | 是 | — | MySQL 连接字符串 |
| `QUERY_TIMEOUT` | 否 | `300` | SQL 查询超时（秒） |
| `LOG_LEVEL` | 否 | `error` | 日志级别：debug/info/warn/error |

**DB_DSN 格式：**

```
user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
```

### 全局参数

| 参数 | 说明 |
|------|------|
| `-e, --env <path>` | 指定 .env 配置文件路径 |
| `--json` | 以 JSON 格式输出（默认文本格式） |
| `--log-level <level>` | 日志级别，优先级高于 `LOG_LEVEL` 环境变量 |

## 命令详解

### tables — 列出所有表

```bash
sql-query tables -e .env [--json]
```

**文本输出：**

```
Found 3 tables:
  orders                              5 rows - 订单表
  products                            3 rows - 商品表
  users                               4 rows - 用户表
```

**JSON 输出：**

```json
{
  "count": 3,
  "tables": [
    {"table_name": "orders", "table_comment": "订单表", "table_rows": 5},
    {"table_name": "products", "table_comment": "商品表", "table_rows": 3},
    {"table_name": "users", "table_comment": "用户表", "table_rows": 4}
  ]
}
```

> `table_rows` 来自 INFORMATION_SCHEMA，InnoDB 表为近似值。

---

### table — 查看表结构

```bash
sql-query table <表名> -e .env [--json]
```

**文本输出：** 原生 DDL（`SHOW CREATE TABLE` 结果）

```sql
CREATE TABLE `users` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '主键ID',
  `username` varchar(64) NOT NULL COMMENT '用户��',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户表';
```

**JSON 输出：** DDL + 结构化字段和索引信息

```json
{
  "table_name": "users",
  "comment": "用户表",
  "ddl": "CREATE TABLE `users` ...",
  "columns": [
    {"name": "id", "type": "bigint unsigned", "nullable": "NO", "default": null, "comment": "主键ID"},
    {"name": "username", "type": "varchar(64)", "nullable": "NO", "default": null, "comment": "用户名"}
  ],
  "indexes": [
    {"name": "PRIMARY", "columns": "id", "is_unique": true, "is_primary": true, "index_type": "BTREE"},
    {"name": "uk_username", "columns": "username", "is_unique": true, "is_primary": false, "index_type": "BTREE"}
  ]
}
```

---

### query — 执行 SQL 查询并导出

```bash
# 从 stdin 读取 SQL
echo "SELECT * FROM users" | sql-query query -e .env --json

# 从文件读取 SQL
sql-query query -e .env -f query.sql --json

# 导出到文件
sql-query query -e .env -f query.sql --excel -o report.xlsx
sql-query query -e .env -f query.sql --html -o report.html
sql-query query -e .env -f query.sql --json -o data.json
```

**参数：**

| 参数 | 说明 |
|------|------|
| `-f, --file <path>` | SQL 文件路径，不指定则读取 stdin |
| `-o, --output <path>` | 输出��件路径（Excel/HTML 默认 output.xlsx/output.html，JSON 默认 stdout） |
| `--excel` | 导出为 Excel |
| `--html` | 导出为 HTML（带分页、暗色主题） |
| `--json` | 导出为 JSON |
| `-w, --workers <n>` | S3 预签名并发数（Phase 2，当前无效） |

三种格式必须且只能选一种。

**JSON 输出示例：**

```json
[
  {"username": "alice", "email": "alice@example.com"},
  {"username": "bob", "email": null}
]
```

- key 使用列的显示名称（元数据协议解析后）
- SQL NULL 输出为 JSON `null`
- 无 `-o` 时写 stdout，有 `-o` 时写文件

**元数据协议：**

SQL 列别名中可嵌入元数据指令，影响 Excel/HTML 导出的渲染行为：

```sql
SELECT
  id,
  avatar `[URL(24h)][HTML(I)] 头像`,        -- S3 预签名 + 图片渲染
  video  `[URL(24h,D)][HTML(V,P:预览)] 视频`, -- 下载模式 + 视频预览
  name   `[HTML(L)] 链接`                    -- 超链接渲染
FROM users
```

| 指令 | 说明 |
|------|------|
| `[URL(duration)]` | S3 预签名，如 `[URL(24h)]`（Phase 2） |
| `[URL(duration,D)]` | S3 预签名 + 触发浏览器下载 |
| `[HTML(I)]` | 渲染为图片 |
| `[HTML(V)]` | 渲染为视频 |
| `[HTML(L)]` | 渲染为超链接 |
| `[HTML(I,P:提示->列名)]` | 图片预览，绑定到指定列 |
| `[H(120px)]` | 限制图片/视频高度 |

## 输出规范

### stdout / stderr 分离

- **stdout**：仅输出数据（表格文本、JSON、DDL）
- **stderr**：日志信息（由 `--log-level` 控制）

适合管道操作：

```bash
# JSON 输出直接传给 jq
./sql-query tables -e .env --json | jq '.tables[].table_name'

# 结果写入文件
./sql-query table users -e .env --json > schema.json
```

### 日志级别

| 级别 | 内容 |
|------|------|
| `error`（默认） | 仅错误信息 |
| `warn` | + 警告（如 URL 元数据未启用） |
| `info` | + 连接状态、查询进度、导出路径 |
| `debug` | + SQL 内容、逐行读取进度 |

```bash
# ��默模式（默认，适合 AI Agent）
./sql-query tables -e .env --json

# 调试模式
./sql-query tables -e .env --log-level debug

# 通过环境变量设置
LOG_LEVEL=info ./sql-query tables -e .env
```

## 退出码

| 退出码 | 含义 | 场景 |
|--------|------|------|
| 0 | 成功 | — |
| 1 | 通用错误 | SQL 语法错误、参数无效、文件读写失败、查询超时 |
| 2 | 连接失败 | 数据库连接超时、认证失败 |
| 3 | 表不存在 | `table <name>` 指定的表不存在 |

### JSON 模式错误输出

`--json` 模式下，错误同时输出到 stderr（人类可读）和 stdout（结构化 JSON）：

```bash
$ ./sql-query table nonexistent -e .env --json 2>/dev/null
{"error":"table_not_found","message":"表 'nonexistent' 不存在"}

$ echo $?
3
```

错误码对照：

| error 字段 | 退出码 | 说明 |
|---|---|---|
| `connection_failed` | 2 | 数据库连接失败 |
| `table_not_found` | 3 | 表不存在 |
| `sql_syntax_error` | 1 | SQL 语法错误 |
| `invalid_argument` | 1 | 参数无效 |
| `file_error` | 1 | 文件读写失败 |
| `query_timeout` | 1 | 查询超时 |
| `s3_presign_failed` | 1 | S3 预签名失败 |

## S3 预签名

当 SQL 列别名包含 `[URL(duration)]` 元数据时，`query` 命令自动将 `bucket:key` 值转换为带时效的预签名 URL。

### 配置

在 `.env` 中添加 S3 凭据（仅在使用 `[URL]` 列时需要）：

```env
# AWS S3
S3_ACCESS_KEY=your-aws-access-key
S3_SECRET_KEY=your-secret-key
S3_REGION=us-west-1

# 阿里云 OSS（需要 S3_ENDPOINT）
S3_ACCESS_KEY=LTAI5tXXXXXXXXXXX
S3_SECRET_KEY=your-secret-key
S3_REGION=cn-hangzhou
S3_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com
```

### 使用示例

```sql
-- 数据库中 avatar 列存储格式: bucket:key (如 my-bucket:images/photo.png)
SELECT
  username,
  avatar `[URL(24h)][HTML(I)] 头像`
FROM users
```

```bash
# JSON: 头像字段变为预签名 URL
echo "SELECT username, avatar \`[URL(24h)] 头像\` FROM users" | \
  ./sql-query query -e .env --json

# Excel: URL 自动变为可点击的超链接
echo "SELECT username, avatar \`[URL(24h)] 头像\` FROM users" | \
  ./sql-query query -e .env --excel -o users.xlsx

# 下载模式: [URL(24h,D)] 触发浏览器下载而非预览
echo "SELECT file_path \`[URL(24h,D)] 文件\` FROM attachments" | \
  ./sql-query query -e .env --excel -o files.xlsx
```

### 并发控制

使用 `-w` 参数控制预签名并发数（默认 CPU 核数）：

```bash
echo "..." | ./sql-query query -e .env --json -w 4
```

### 错误处理

任何一个单元格的预签名失败，整个命令退出（exit code 1）：

```bash
$ echo "SELECT 'bad:key' \`[URL(24h)] f\`" | ./sql-query query -e .env --json 2>/dev/null
{"error":"s3_presign_failed","message":"S3 预签名失败: 行 0 列 0: ..."}
```

## 完整示例

```bash
# 列出所有表（含行数）
./sql-query tables -e .env --json | jq '.tables[] | "\(.table_name): \(.table_rows) rows"'

# 查看表结构，提取字段列表
./sql-query table orders -e .env --json | jq '[.columns[].name]'

# 联表查询导出 JSON
echo "
SELECT u.username, u.email,
       COUNT(o.id) AS order_count,
       SUM(o.amount) AS total
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id
" | ./sql-query query -e .env --json

# 导出 Excel 报表
./sql-query query -e .env -f monthly_report.sql --excel -o report.xlsx

# 导出 HTML 报表（带分页、暗色主题）
./sql-query query -e .env -f dashboard.sql --html -o dashboard.html
```
