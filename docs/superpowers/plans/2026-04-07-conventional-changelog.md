# Conventional Commits Changelog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** GoReleaser changelog 按约定式提交分组展示（Features / Bug Fixes / Refactoring / Others），过滤 merge commits

**Architecture:** 修改 `.goreleaser.yaml` 中 `changelog` 配置块，使用 `groups` 按正则匹配 Conventional Commits 前缀分组，使用 `exclude` 过滤 merge commits。

**Tech Stack:** GoReleaser v2

---

### Task 1: 配置 Conventional Commits Changelog 分组

**Files:**
- Modify: `.goreleaser.yaml:39-40`

- [ ] **Step 1: 修改 `.goreleaser.yaml` 的 changelog 配置**

将现有的：

```yaml
changelog:
  sort: asc
```

替换为：

```yaml
changelog:
  sort: asc
  groups:
    - title: "Features"
      regexp: '^.*?feat(\(.+\))?\!?:.+$'
      order: 0
    - title: "Bug Fixes"
      regexp: '^.*?fix(\(.+\))?\!?:.+$'
      order: 1
    - title: "Refactoring"
      regexp: '^.*?refactor(\(.+\))?\!?:.+$'
      order: 2
    - title: "Others"
      order: 999
  filters:
    exclude:
      - '^Merge '
```

- [ ] **Step 2: 验证配置有效性**

Run: `goreleaser check`
Expected: `1 configuration file(s) validated`

- [ ] **Step 3: 本地 snapshot 验证 changelog 分组**

Run: `GOROOT=/usr/local/go GOPATH=$HOME/go PATH="/usr/local/go/bin:$HOME/go/bin:$PATH" goreleaser release --snapshot --clean 2>&1 | tail -5`
Expected: `release succeeded`

然后检查生成的 changelog：
Run: `cat dist/CHANGELOG.md`
Expected: 输出中包含 `## Features`、`## Bug Fixes`、`## Refactoring`、`## Others` 等分组标题（视 commit 历史中实际存在的类型而定）

- [ ] **Step 4: 清理并提交**

```bash
rm -rf dist/
git add .goreleaser.yaml
git commit -m "feat(changelog): use Conventional Commits grouping in release notes"
```
