# Claude Code 自动安装提示词 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 创建 `docs/setup.md` 安装提示词和 README，让用户在 Claude Code 中一键安装 sql-query CLI 和 skill

**Architecture:** 三个改动：(1) 修改 SKILL.md 的 `SQL_QUERY_BIN` 默认路径指向 `scripts/sql-query`；(2) 创建 `docs/setup.md` 安装提示词，包含平台检测、下载、安装的完整 bash 指令；(3) 创建 README.md 安装指引。

**Tech Stack:** Markdown, Bash (curl/tar), GitHub Releases API

---

### Task 1: 修改 SKILL.md 的 SQL_QUERY_BIN 默认路径

**Files:**
- Modify: `.claude/skills/sql-query/SKILL.md:25-27`

binary 现在放在 skill 目录下的 `scripts/sql-query`，SKILL.md 需要更新默认路径，使用相对于 skill 目录的路径。

- [ ] **Step 1: 修改 SKILL.md 的 Setup 段落**

将现有的：

```markdown
## Setup

The `sql-query` binary and `.env` config path come from environment variables:

- `SQL_QUERY_BIN`: path to the `sql-query` binary (default: `sql-query` on PATH)
- `SQL_QUERY_ENV`: path to the `.env` file containing `DB_DSN` (required)

At the start of every invocation, verify the env path exists:

```bash
# Resolve paths
SQL_BIN="${SQL_QUERY_BIN:-sql-query}"
SQL_ENV="${SQL_QUERY_ENV}"
```
```

替换为：

```markdown
## Setup

The `sql-query` binary and `.env` config path come from environment variables:

- `SQL_QUERY_BIN`: path to the `sql-query` binary (default: auto-detected from skill's `scripts/` directory)
- `SQL_QUERY_ENV`: path to the `.env` file containing `DB_DSN` (required)

At the start of every invocation, resolve binary path and verify the env path exists:

```bash
# Resolve binary path: env var > skill scripts dir > PATH
if [ -n "${SQL_QUERY_BIN}" ]; then
  SQL_BIN="${SQL_QUERY_BIN}"
elif [ -x "$(dirname "$0")/../scripts/sql-query" ] 2>/dev/null; then
  SQL_BIN="$(dirname "$0")/../scripts/sql-query"
elif [ -x ".claude/skills/sql-query/scripts/sql-query" ]; then
  SQL_BIN=".claude/skills/sql-query/scripts/sql-query"
elif [ -x "${HOME}/.claude/skills/sql-query/scripts/sql-query" ]; then
  SQL_BIN="${HOME}/.claude/skills/sql-query/scripts/sql-query"
else
  SQL_BIN="sql-query"
fi
SQL_ENV="${SQL_QUERY_ENV}"
```
```

**注意**：上面的 shell 脚本在 Claude Code 执行 skill 时并不会真正运行。它是给 Claude 的指引 — Claude 会按照这个逻辑依次检查路径。所以实际上更简洁的做法是用自然语言描述优先级。

让我重新设计这个段落，用 Claude 能理解的自然语言优先级：

将 Setup 段落替换为：

```markdown
## Setup

The `sql-query` binary and `.env` config path come from environment variables:

- `SQL_QUERY_BIN`: path to the `sql-query` binary
- `SQL_QUERY_ENV`: path to the `.env` file containing `DB_DSN` (required)

At the start of every invocation, resolve paths:

```bash
# Resolve binary: env var → project skill dir → global skill dir → PATH
SQL_BIN="${SQL_QUERY_BIN:-.claude/skills/sql-query/scripts/sql-query}"
# If project-level binary doesn't exist, try global
if [ ! -x "$SQL_BIN" ]; then
  SQL_BIN="${HOME}/.claude/skills/sql-query/scripts/sql-query"
fi
# Final fallback to PATH
if [ ! -x "$SQL_BIN" ]; then
  SQL_BIN="sql-query"
fi
SQL_ENV="${SQL_QUERY_ENV}"
```
```

- [ ] **Step 2: 验证 SKILL.md 语法正确**

Run: `head -30 .claude/skills/sql-query/SKILL.md`
Expected: Setup 段落包含新的路径解析逻辑

- [ ] **Step 3: 提交**

```bash
git add .claude/skills/sql-query/SKILL.md
git commit -m "feat(skill): update SQL_QUERY_BIN default to scripts/ directory"
```

---

### Task 2: 创建 docs/setup.md 安装提示词

**Files:**
- Create: `docs/setup.md`

这是核心交付物 — Claude Code 可执行的安装提示词。用户在 Claude Code 中通过 URL 引用触发。

- [ ] **Step 1: 创建 `docs/setup.md`**

```markdown
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

✅ **sql-query 安装完成！**

**安装位置：**
- CLI: `${SKILL_DIR}/scripts/sql-query`
- Skill: `${SKILL_DIR}/SKILL.md`

**下一步：配置数据库连接**

创建 `.env` 文件（放在项目目录或其他安全位置）：

```
DB_DSN=user:password@tcp(host:3306)/database?charset=utf8mb4
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

---
```

- [ ] **Step 2: 验证 setup.md 文件内容正确**

Run: `wc -l docs/setup.md`
Expected: 文件存在且行数合理（~90-100 行）

- [ ] **Step 3: 提交**

```bash
git add docs/setup.md
git commit -m "feat: add Claude Code auto-install prompt (docs/setup.md)"
```

---

### Task 3: 创建 README.md

**Files:**
- Create: `README.md`

项目目前没有 README。创建一个包含项目简介和安装指引的 README。

- [ ] **Step 1: 创建 `README.md`**

```markdown
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
```

- [ ] **Step 2: 验证 README 内容**

Run: `head -20 README.md`
Expected: 显示项目标题和安装指引

- [ ] **Step 3: 提交**

```bash
git add README.md
git commit -m "docs: add README with installation guide"
```

---

### Task 4: 端到端验证

验证安装提示词中的每个关键命令都能正常工作。

- [ ] **Step 1: 验证平台检测**

Run: `uname -s && uname -m`
Expected: `Darwin` + `arm64`（或你当前平台的输出）

- [ ] **Step 2: 验证版本获取**

Run: `curl -sL "https://api.github.com/repos/user-zouyu/sql-query/releases/latest" | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//'`
Expected: 输出最新 tag（如 `v0.0.2-beta`）

- [ ] **Step 3: 验证下载和安装流程**

```bash
# 模拟项目级安装
SKILL_DIR="/tmp/test-sql-query-install/.claude/skills/sql-query"
VERSION="v0.0.2-beta"
SEMVER="0.0.2-beta"
OS="darwin"
ARCH="arm64"

mkdir -p "${SKILL_DIR}/scripts"
curl -sL "https://github.com/user-zouyu/sql-query/releases/download/${VERSION}/sql-query_${SEMVER}_${OS}_${ARCH}.tar.gz" -o /tmp/sql-query.tar.gz
tar -xzf /tmp/sql-query.tar.gz -C "${SKILL_DIR}/scripts/" sql-query
chmod +x "${SKILL_DIR}/scripts/sql-query"
curl -sL "https://raw.githubusercontent.com/user-zouyu/sql-query/main/.claude/skills/sql-query/SKILL.md" -o "${SKILL_DIR}/SKILL.md"
```

Expected: 无错误

- [ ] **Step 4: 验证安装结果**

Run: `"${SKILL_DIR}/scripts/sql-query" version && ls -la "${SKILL_DIR}/SKILL.md" "${SKILL_DIR}/scripts/sql-query"`
Expected: 版本信息输出正常，两个文件都存在

- [ ] **Step 5: 清理测试目录**

Run: `rm -rf /tmp/test-sql-query-install /tmp/sql-query.tar.gz`

- [ ] **Step 6: 提交所有改动并推送**

```bash
git push origin main
```
