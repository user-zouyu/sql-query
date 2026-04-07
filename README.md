# sql-query

MySQL 数据库查询 CLI 工具，专为 Claude Code skill 设计。支持只读 SQL 查询、多格式导出（JSON/Excel/HTML）、S3 预签名 URL。

## 安装

### Claude Code 自动安装（推荐）

在 Claude Code 中发送以下内容：

```
请按照 https://raw.githubusercontent.com/user-zouyu/sql-query/main/docs/setup.md 的指引安装 sql-query
```

Claude 会自动检测你的平台，下载 CLI 二进制并配置 skill。

### 手动安装

1. 从 [GitHub Releases](https://github.com/user-zouyu/sql-query/releases) 下载对应平台的二进制
2. 解压到 `.claude/skills/sql-query/scripts/` 目录
3. 下载 [SKILL.md](https://raw.githubusercontent.com/user-zouyu/sql-query/main/.claude/skills/sql-query/SKILL.md) 到 `.claude/skills/sql-query/`

### 配置数据库

创建 `.env` 文件：

```
DB_DSN=user:password@tcp(host:3306)/database?charset=utf8mb4
```

在 `.claude/settings.json` 中设置：

```json
{
  "env": {
    "SQL_QUERY_ENV": "/path/to/.env"
  }
}
```

## 使用

在 Claude Code 中使用 `/sql-query` 命令，然后用自然语言描述你想查询的内容。

## 功能

- **只读安全**：双层防护（Vitess AST 验证 + MySQL READ ONLY 事务）
- **多格式导出**：JSON、Excel、HTML
- **S3 预签名 URL**：自动将 bucket:key 转为可访问的预签名链接
- **中文友好**：支持中文别名（需反引号包裹）

## License

MIT
