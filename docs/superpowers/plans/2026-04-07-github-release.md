# GitHub Release 自动构建发布 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 推送 `v*.*.*` tag 时自动构建多平台二进制并发布 GitHub Release

**Architecture:** 三个文件：版本变量（`cmd/version.go`）通过 ldflags 注入，GoReleaser 配置（`.goreleaser.yaml`）定义构建矩阵和归档规则，GitHub Actions workflow（`.github/workflows/release.yml`）编排测试和发布流程。

**Tech Stack:** Go 1.26, GoReleaser v2, GitHub Actions

---

### Task 1: 添加版本信息变量和 version 子命令

**Files:**
- Create: `cmd/version.go`

这一步为二进制注入构建时版本信息。GoReleaser 通过 ldflags 在编译时设置这些变量。

- [ ] **Step 1: 创建 `cmd/version.go`**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func init() {
	rootCmd.AddCommand(versionCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "显示版本信息",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("sql-query %s (commit: %s, built: %s)\n", version, commit, date)
	},
}
```

- [ ] **Step 2: 验证编译和运行**

Run: `cd /Users/mac/workspaces/github/sql-query && go build -o sql-query . && ./sql-query version`
Expected: `sql-query dev (commit: none, built: unknown)`

- [ ] **Step 3: 验证 ldflags 注入**

Run: `go build -ldflags "-X sql-query/cmd.version=v0.0.1-test -X sql-query/cmd.commit=abc123 -X sql-query/cmd.date=2026-04-07" -o sql-query . && ./sql-query version`
Expected: `sql-query v0.0.1-test (commit: abc123, built: 2026-04-07)`

- [ ] **Step 4: 清理并提交**

```bash
rm -f sql-query
git add cmd/version.go
git commit -m "feat: add version subcommand with ldflags support"
```

---

### Task 2: 创建 GoReleaser 配置

**Files:**
- Create: `.goreleaser.yaml`

GoReleaser v2 配置文件，定义构建目标、归档格式、checksum 和 changelog 规则。

- [ ] **Step 1: 创建 `.goreleaser.yaml`**

```yaml
version: 2

builds:
  - id: sql-query
    main: .
    binary: sql-query
    env:
      - CGO_ENABLED=0
    ldflags:
      - -s -w
      - -X sql-query/cmd.version={{.Version}}
      - -X sql-query/cmd.commit={{.Commit}}
      - -X sql-query/cmd.date={{.Date}}
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64

archives:
  - id: default
    format: tar.gz
    format_overrides:
      - goos: windows
        format: zip
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

changelog:
  sort: asc

release:
  github:
    owner: user-zouyu
    name: sql-query
  name_template: "{{.Tag}}"
```

- [ ] **Step 2: 验证配置有效性**

Run: `cd /Users/mac/workspaces/github/sql-query && go install github.com/goreleaser/goreleaser/v2@latest && goreleaser check`
Expected: 输出 `config is valid` 或类似成功信息（无 error）

- [ ] **Step 3: 本地 snapshot 测试**

Run: `goreleaser release --snapshot --clean 2>&1 | tail -20`
Expected: 构建成功，`dist/` 目录下生成 5 个归档文件（linux_amd64, linux_arm64, darwin_amd64, darwin_arm64, windows_amd64）和 checksums.txt

- [ ] **Step 4: 验证产物**

Run: `ls dist/*.tar.gz dist/*.zip dist/checksums.txt`
Expected:
```
dist/sql-query_0.0.0-SNAPSHOT-xxx_darwin_amd64.tar.gz
dist/sql-query_0.0.0-SNAPSHOT-xxx_darwin_arm64.tar.gz
dist/sql-query_0.0.0-SNAPSHOT-xxx_linux_amd64.tar.gz
dist/sql-query_0.0.0-SNAPSHOT-xxx_linux_arm64.tar.gz
dist/sql-query_0.0.0-SNAPSHOT-xxx_windows_amd64.zip
dist/checksums.txt
```

- [ ] **Step 5: 清理并提交**

```bash
rm -rf dist/
echo "dist/" >> .gitignore
git add .goreleaser.yaml .gitignore
git commit -m "feat: add GoReleaser config for multi-platform builds"
```

---

### Task 3: 创建 GitHub Actions workflow

**Files:**
- Create: `.github/workflows/release.yml`

当推送 `v*.*.*` tag 时触发，先跑测试，再用 GoReleaser 构建发布。

- [ ] **Step 1: 创建目录**

Run: `mkdir -p /Users/mac/workspaces/github/sql-query/.github/workflows`

- [ ] **Step 2: 创建 `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - "v*.*.*"

permissions:
  contents: write

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Run tests
        run: go test ./...

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

- [ ] **Step 3: 提交**

```bash
git add .github/workflows/release.yml
git commit -m "ci: add GitHub Actions release workflow"
```

---

### Task 4: 端到端验证

这一步确认所有配置文件正确，本地 snapshot 构建正常。实际的 GitHub Release 需要推送 tag 后在 GitHub Actions 上验证。

- [ ] **Step 1: 最终 GoReleaser 校验**

Run: `cd /Users/mac/workspaces/github/sql-query && goreleaser check`
Expected: `config is valid`

- [ ] **Step 2: 最终 snapshot 构建**

Run: `goreleaser release --snapshot --clean 2>&1 | tail -5`
Expected: 构建成功，无错误

- [ ] **Step 3: 验证版本注入**

Run: `./dist/sql-query_linux_amd64_v1/sql-query version 2>/dev/null || ./dist/sql-query_darwin_arm64_v8.0/sql-query version 2>/dev/null || echo "check dist/ for correct binary path"`
Expected: 输出版本信息（snapshot 版本号）

- [ ] **Step 4: 清理**

Run: `rm -rf dist/`

- [ ] **Step 5: 推送代码到 main 分支**

```bash
git push origin main
```

- [ ] **Step 6: 创建测试 tag 验证完整流程**

```bash
git tag v0.1.0
git push origin v0.1.0
```

然后在 GitHub 上检查：
1. Actions 页面应出现 "Release" workflow 运行
2. 运行成功后，Releases 页面应出现 `v0.1.0` release
3. Release 中应包含 5 个归档文件 + checksums.txt
4. Release Notes 应自动生成
