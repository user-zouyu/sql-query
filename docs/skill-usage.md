# /sql Skill 使用手册

`/sql` 是一个 Claude Code skill，让你可以在对话中直接用自然语言查询 MySQL 数据库。它基于 `sql-query` CLI 工具，强制只读模式，安全地探索数据库结构和执行查询。

## 前置条件

1. **sql-query 二进制文件** — 项目根目录已编译好，或在 PATH 中
2. **`.env` 配置文件** — 包含 `DB_DSN` 数据库连接串

```bash
# 编译 sql-query（如果还没有）
cd /path/to/sql-query
go build -o sql-query .

# 准备 .env
cp .env.example .env
# 编辑 .env，填入数据库连接信息
```

## 配置环境变量

Skill 通过两个环境变量找到 `sql-query` 和数据库配置：

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `SQL_QUERY_BIN` | `sql-query` 二进制文件路径 | `sql-query`（从 PATH 查找） |
| `SQL_QUERY_ENV` | `.env` 配置文件路径 | 无（必填） |

### 方式一：Claude Code settings.json（推荐）

在项目目录下创建 `.claude/settings.json`：

```json
{
  "env": {
    "SQL_QUERY_BIN": "/path/to/sql-query",
    "SQL_QUERY_ENV": "/path/to/.env"
  }
}
```

优点：配置跟着项目走，不同项目可以连不同的数据库，不污染全局环境。

### 方式二：Shell 环境变量

在 `~/.zshrc` 或 `~/.bashrc` 中添加：

```bash
export SQL_QUERY_BIN="/path/to/sql-query"
export SQL_QUERY_ENV="/path/to/.env"
```

### 方式三：自动发现

如果不配置环境变量，skill 会尝试在项目目录下寻找 `sql-query` 二进制和 `.env` 文件。找不到时会提示你手动指定路径。

## 使用方式

### 基本用法

在 Claude Code 中输入 `/sql` 加上你的问题：

```
/sql 有哪些表
/sql 查一下订单最多的用户
/sql 这个用户有多少积分
```

也可以不加 `/sql` 前缀，直接描述数据库相关的问题，skill 会自动触发：

```
帮我看看数据库里有哪些表
查一下邮箱是 alice@example.com 的用户信息
```

### 使用示例

#### 探索数据库结构

```
/sql 有哪些表
/sql 看看 users 表的结构
/sql orders 表有哪些字段和索引
```

#### 业务数据查询

```
/sql 查一下订单最多的用户邮箱
/sql 最近 7 天注册了多少用户
/sql 哪个商品卖得最好
```

#### 数据调查

```
/sql 查一下用户 12345 的所有订单
/sql p10@aa.aa 这个用户有多少个相框
/sql 有多少人领取了积分卡，有人领取多次吗
```

#### 帮写 SQL

```
/sql 给我写一个查询用户相框数量的 SQL
/sql 帮我写一个统计每日订单量的 SQL
```

#### 导出数据

```
/sql 把上个月的订单数据导出成 Excel
/sql 导出用户活跃度报表为 HTML
```

## 支持的输出格式

| 格式 | 说明 | 适用场景 |
|------|------|---------|
| JSON | 结构化数据，默认输出到终端 | 数据检查、小数据量查看 |
| Excel | `.xlsx` 文件 | 报表导出、数据分析 |
| HTML | 带分页和暗色主题的网页 | 大数据量浏览、分享 |

## 安全机制

### 只读模式

Skill 强制只读——所有 SQL 在执行前会被验证，只允许 `SELECT` 和 `WITH`（CTE）语句。

以下操作会被拒绝：
- 写入：`INSERT`、`UPDATE`、`DELETE`、`REPLACE`
- 结构变更：`CREATE`、`ALTER`、`DROP`、`TRUNCATE`、`RENAME`
- 权限操作：`GRANT`、`REVOKE`
- 锁操作：`LOCK`、`UNLOCK`、`FOR UPDATE`、`FOR SHARE`
- 数据导出：`INTO OUTFILE`、`INTO DUMPFILE`
- 其他：`CALL`、`LOAD`、`SET`（`SET NAMES` 除外）

### 凭据保护

数据库连接信息存储在 `.env` 文件中，skill 不会在对话中显示密码、DSN 或任何凭据。

## 工作流程

当你提出数据库问题时，skill 会按以下流程处理：

```
1. 探索结构  →  列出表、查看相关表的字段和索引
2. 编写 SQL  →  根据表结构写出正确的查询，展示给你确认
3. 执行查询  →  运行 SQL 并获取结果
4. 呈现结果  →  格式化为表格，总结关键发现，提供后续建议
```

Skill 会：
- 先查表结构再写 SQL，确保字段名正确
- 利用字段注释理解业务含义（如「状态：1启用 0禁用」）
- 自动加 `LIMIT` 防止拉取过多数据
- 用中文别名让结果更易读
- 结果较大时建议导出为 Excel 或 HTML

## 常见问题

### SQL_QUERY_ENV 未设置怎么办？

Skill 会提示你指定 `.env` 文件路径。建议通过 `.claude/settings.json` 配置一次，后续自动生效。

### 能否连接多个数据库？

可以。准备多个 `.env` 文件（如 `.env.prod`、`.env.staging`），切换时修改 `SQL_QUERY_ENV` 指向不同的文件即可。

### 查询超时怎么办？

默认超时 300 秒。可以在 `.env` 中调整：

```
QUERY_TIMEOUT=600
```

### 遇到 collation 错误？

不同表的字符集可能不一致（如 `utf8mb4_unicode_ci` vs `utf8mb4_0900_ai_ci`）。Skill 会自动在 JOIN 条件中添加 `COLLATE` 子句解决。

### 能修改数据吗？

不能。Skill 严格只读。如需写入操作，请使用其他工具或直接连接数据库客户端。
