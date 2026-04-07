# sql-query 自动安装

你是一个安装助手。请按照以下步骤安装 sql-query CLI 工具和 Claude Code skill。

## Step 1: 检测平台

运行以下命令检测当前操作系统和架构：

```bash
uname -s && uname -m
```

根据输出确定平台参数：

| uname -s | uname -m | OS     | ARCH  |
|----------|----------|--------|-------|
| Darwin   | arm64    | darwin | arm64 |
| Darwin   | x86_64   | darwin | amd64 |
| Linux    | aarch64  | linux  | arm64 |
| Linux    | x86_64   | linux  | amd64 |

如果是 Windows 或其他不支持的平台，告知用户需要手动从 GitHub Releases 下载：https://github.com/user-zouyu/sql-query/releases

## Step 2: 选择安装范围

询问用户选择安装方式：

1. **项目级安装**（推荐）：安装到当前项目的 `.claude/skills/sql-query/` 目录，仅在当前项目可用
2. **全局安装**：安装到 `~/.claude/skills/sql-query/` 目录，所有项目可用

根据选择设置安装目录：
- 项目级：`SKILL_DIR=".claude/skills/sql-query"`
- 全局：`SKILL_DIR="${HOME}/.claude/skills/sql-query"`

## Step 3: 获取最新版本号

```bash
curl -sL "https://api.github.com/repos/user-zouyu/sql-query/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'
```

将输出保存为 VERSION 变量（如 `v0.0.2-beta`）。去掉 `v` 前缀得到 SEMVER（如 `0.0.2-beta`）。

## Step 4: 下载并安装

```bash
# 创建目录
mkdir -p "${SKILL_DIR}/scripts"

# 下载归档文件（OS 和 ARCH 从 Step 1 确定）
curl -sL "https://github.com/user-zouyu/sql-query/releases/download/${VERSION}/sql-query_${SEMVER}_${OS}_${ARCH}.tar.gz" -o /tmp/sql-query.tar.gz

# 解压二进制到 scripts 目录
tar -xzf /tmp/sql-query.tar.gz -C "${SKILL_DIR}/scripts/" sql-query

# 设置可执行权限
chmod +x "${SKILL_DIR}/scripts/sql-query"

# 清理
rm -f /tmp/sql-query.tar.gz
```

## Step 5: 安装 Skill 文件

```bash
curl -sL "https://raw.githubusercontent.com/user-zouyu/sql-query/main/.claude/skills/sql-query/SKILL.md" -o "${SKILL_DIR}/SKILL.md"
```

## Step 6: 验证安装

```bash
"${SKILL_DIR}/scripts/sql-query" version
```

预期输出类似：`sql-query 0.0.2-beta (commit: xxx, built: xxx)`

## Step 7: 安装完成

安装成功后，向用户展示以下信息：

---

**sql-query 安装完成！**

**安装位置：**
- CLI: `${SKILL_DIR}/scripts/sql-query`
- Skill: `${SKILL_DIR}/SKILL.md`

**下一步：配置数据库连接**

参考 [.env.example](https://raw.githubusercontent.com/user-zouyu/sql-query/main/.env.example) 创建 `.env` 文件（放在项目目录或其他安全位置）：

```bash
# 数据库连接（必填）
DB_DSN=user:password@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local

# 查询超时（秒，默认 300）
# QUERY_TIMEOUT=300

# S3 预签名配置（仅在使用 [URL] 元数据时需要）
# S3_ACCESS_KEY=your-access-key
# S3_SECRET_KEY=your-secret-key
# S3_REGION=us-west-1
# S3_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com

# 审计日志目录（默认当前目录）
# AUDIT_LOG_DIR=/var/log/sql-query
```

然后设置环境变量 `SQL_QUERY_ENV` 指向该文件。可以在 `.claude/settings.json` 中配置：

```json
{
  "env": {
    "SQL_QUERY_ENV": "/path/to/.env"
  }
}
```

配置完成后，在 Claude Code 中使用 `/sql-query` 命令即可开始查询数据库。
