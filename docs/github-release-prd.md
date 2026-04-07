# Product Requirements Document: GitHub Release 自动构建发布

**Version**: 1.0
**Date**: 2026-04-07
**Author**: Sarah (Product Owner)
**Quality Score**: 91/100

---

## Executive Summary

为 `sql-query` CLI 工具建立自动化发布流水线。开发者只需给 git 仓库打一个语义化版本 tag（如 `v1.0.0`），即可自动触发 GitHub Actions 完成测试、多平台构建、校验和生成、Homebrew formula 更新，并创建 GitHub Release。

这将消除手动构建和发布的繁琐流程，确保每次发布的一致性和可追溯性，同时通过 Homebrew tap 为 macOS 用户提供便捷的安装方式。

---

## Problem Statement

**现状**：`sql-query` 项目目前没有自动化发布流程。发布需手动构建各平台二进制文件、打包、计算校验和、创建 GitHub Release 并上传产物，过程繁琐且容易出错。

**方案**：使用 GitHub Actions + GoReleaser 实现 tag 触发的全自动构建发布流水线。

**业务影响**：
- 发布时间从数十分钟降至推送一个 tag（< 1 分钟人工操作）
- 消除手动构建导致的平台遗漏或校验和不一致问题
- 通过 Homebrew tap 降低 macOS 用户的安装门槛

---

## Success Metrics

**Primary KPIs:**
- 推送 tag 后 GitHub Release 自动创建，包含所有目标平台产物
- 测试失败时发布流程自动终止，不产出不完整的 Release
- macOS 用户可通过 `brew install user-zouyu/tap/sql-query` 安装

**Validation**: 通过推送测试 tag（如 `v0.0.1-test`）验证完整流程

---

## User Personas

### Primary: 项目维护者（开发者）
- **角色**: sql-query 开发者
- **目标**: 快速、可靠地发布新版本
- **痛点**: 手动构建多平台二进制文件耗时且易出错
- **技术水平**: 高级

### Secondary: 终端用户（CLI 使用者）
- **角色**: sql-query 的使用者
- **目标**: 便捷下载或安装对应平台的二进制文件
- **痛点**: 需要从源码编译或手动查找下载链接
- **技术水平**: 中级

---

## User Stories & Acceptance Criteria

### Story 1: Tag 触发自动发布

**As a** 项目维护者
**I want to** 推送语义化版本 tag 后自动构建并发布
**So that** 无需手动操作即可完成多平台发布

**Acceptance Criteria:**
- [ ] 推送 `v*.*.*` 格式 tag 时触发 GitHub Actions workflow
- [ ] 非 `v*.*.*` 格式的 tag 不触发 workflow
- [ ] Workflow 在 GoReleaser 构建前执行 `go test ./...`
- [ ] 测试失败时 workflow 终止，不创建 Release

### Story 2: 多平台二进制构建

**As a** 终端用户
**I want to** 在 GitHub Release 中找到我平台的预编译二进制
**So that** 无需安装 Go 工具链即可使用

**Acceptance Criteria:**
- [ ] 构建以下平台：linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
- [ ] 产物为压缩归档（Linux/macOS: tar.gz, Windows: zip）
- [ ] 包含 SHA256 校验和文件（checksums.txt）
- [ ] 归档中包含二进制文件和 LICENSE（如存在）

### Story 3: Homebrew 安装

**As a** macOS 用户
**I want to** 通过 Homebrew 安装 sql-query
**So that** 可以方便地安装和更新

**Acceptance Criteria:**
- [ ] GoReleaser 自动生成 Homebrew formula
- [ ] Formula 推送到 `user-zouyu/homebrew-tap` 仓库
- [ ] 用户可通过 `brew tap user-zouyu/tap && brew install sql-query` 安装
- [ ] Formula 包含正确的 description 和 homepage

### Story 4: 自动生成 Release Notes

**As a** 项目维护者
**I want to** Release Notes 从 commits 自动生成
**So that** 无需手动编写变更日志

**Acceptance Criteria:**
- [ ] Changelog 自动包含从上一个 tag 到当前 tag 的所有 commits
- [ ] Release 标题为 tag 名称（如 `v1.0.0`）

---

## Functional Requirements

### Core Features

**Feature 1: GitHub Actions Workflow**
- 文件路径: `.github/workflows/release.yml`
- 触发条件: `push tags: v*.*.*`
- 运行环境: `ubuntu-latest`
- 步骤:
  1. Checkout 代码（含完整 git history 用于 changelog 生成）
  2. 设置 Go 环境（版本与 go.mod 一致）
  3. 运行 `go test ./...`
  4. 执行 GoReleaser（使用 `goreleaser/goreleaser-action`）
- 所需 Secrets: `GITHUB_TOKEN`（自动提供，无需额外配置）

**Feature 2: GoReleaser 配置**
- 文件路径: `.goreleaser.yaml`
- 构建配置:
  - binary name: `sql-query`
  - 入口: `main.go`
  - 目标平台: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64
  - ldflags: 注入 version, commit, date 信息
- 归档配置:
  - Linux/macOS: tar.gz
  - Windows: zip
- Checksum: SHA256
- Changelog: 自动从 commits 生成
- Homebrew: Phase 2 实现（MVP 不包含）

**Feature 3: 版本信息注入**
- 通过 ldflags 在编译时注入版本信息
- 变量: version, commit hash, build date
- 可通过 `sql-query --version` 查看（如 cobra 已支持 version 命令）

### Out of Scope
- Docker 镜像构建和推送
- 自动 changelog 分类（如 Conventional Commits 分组）
- 签名（GPG/cosign）
- 自动 bump 版本号
- Linux 包管理器发布（apt/yum）

---

## Technical Constraints

### Performance
- 构建流程应在 5 分钟内完成
- 并行构建多平台二进制以加速

### Security
- `HOMEBREW_TAP_TOKEN` 存储为 GitHub Repository Secret，需 `repo` scope 的 PAT
- `GITHUB_TOKEN` 使用 Actions 自动提供的 token，需配置 `contents: write` 权限

### Integration
- **GitHub Actions**: 使用 `actions/checkout@v4`, `actions/setup-go@v5`, `goreleaser/goreleaser-action@v6`
- **GoReleaser**: 使用 v2 版本
- **Homebrew tap**: 独立仓库 `user-zouyu/homebrew-tap`，需预先创建

### Technology Stack
- Go 1.26.x（与 go.mod 一致）
- GoReleaser v2
- GitHub Actions

---

## MVP Scope & Phasing

### Phase 1: MVP（本次实现）
- GitHub Actions workflow（tag 触发 + 测试 + 构建发布）
- GoReleaser 配置（多平台构建 + checksum + changelog）
- 版本信息 ldflags 注入

### Phase 2: 增强（后续可选）
- Homebrew tap formula 自动更新
- Conventional Commits 分类 changelog
- GPG/cosign 签名
- Docker 镜像构建发布

### Future Considerations
- Linux 包管理器（apt/yum repo）发布
- Windows Scoop/Chocolatey 发布
- 自动化版本号管理

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation Strategy |
|------|------------|--------|---------------------|
| HOMEBREW_TAP_TOKEN 过期或权限不足 | Medium | High | 文档中说明 token 所需 scope，设置提醒 |
| Go 版本不兼容 | Low | High | workflow 中 Go 版本从 go.mod 提取，保持一致 |
| homebrew-tap 仓库不存在导致发布失败 | Medium | Medium | 在文档中说明前置步骤：创建 tap 仓库 |
| GoReleaser 配置错误导致构建失败 | Low | Medium | 可通过 `goreleaser check` 和 `goreleaser release --snapshot` 本地验证 |

---

## Dependencies & Blockers

**Dependencies:**
- 无（MVP 阶段仅使用 GitHub Actions 自动提供的 `GITHUB_TOKEN`）

**Known Blockers:**
- 无

---

## Appendix

### 前置准备清单（MVP）
- 无额外准备，`GITHUB_TOKEN` 由 GitHub Actions 自动提供

### Phase 2 前置准备（Homebrew tap）
1. 创建 GitHub 仓库 `user-zouyu/homebrew-tap`
2. 创建 PAT（Settings → Developer settings → Personal access tokens），勾选 `repo` scope
3. 在 `sql-query` 仓库 Settings → Secrets → Actions 中添加 `HOMEBREW_TAP_TOKEN`

### 发布操作流程
```bash
# 1. 确保代码已合并到 main
# 2. 打 tag 并推送
git tag v1.0.0
git push origin v1.0.0
# 3. 在 GitHub Actions 中查看构建进度
# 4. 构建完成后 GitHub Release 自动创建
```

### References
- [GoReleaser 文档](https://goreleaser.com/)
- [goreleaser-action](https://github.com/goreleaser/goreleaser-action)
- [GoReleaser Homebrew 配置](https://goreleaser.com/customization/homebrew/)

---

*This PRD was created through interactive requirements gathering with quality scoring to ensure comprehensive coverage of business, functional, UX, and technical dimensions.*
