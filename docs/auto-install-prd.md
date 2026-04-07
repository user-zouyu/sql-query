# Product Requirements Document: Claude Code 自动安装提示词

**Version**: 1.0
**Date**: 2026-04-07
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100

---

## Executive Summary

为 `sql-query` 项目创建一个结构化的安装提示词文件（`docs/setup.md`），用户在 Claude Code 中引用该文件后，Claude 会自动完成 CLI 二进制下载和 skill 配置。同时在 README 中添加安装指引，提供 GitHub raw URL 供用户快速使用。

这将极大降低新用户的安装门槛 — 从手动下载、解压、配置环境变量的多步操作，简化为一句话触发自动安装。

---

## Problem Statement

**现状**：用户需要手动完成多个步骤才能使用 sql-query skill：
1. 从 GitHub Releases 下载对应平台的二进制
2. 解压并放到 PATH 中
3. 复制 SKILL.md 到项目或全局 `.claude/skills/` 目录
4. 配置 `SQL_QUERY_BIN` 和 `SQL_QUERY_ENV` 环境变量

**方案**：创建一个 Claude Code 可理解的结构化提示词 `docs/setup.md`，用户在 Claude Code 中通过 URL 引用即可触发自动安装流程。

**业务影响**：
- 新用户安装时间从 10+ 分钟降至 1 分钟
- 消除手动操作中的平台/架构选择错误
- 提升 skill 的传播和采纳率

---

## Success Metrics

- 用户在 Claude Code 中引用 setup.md URL 后，可自动完成安装
- 安装后 `sql-query version` 命令可正常输出版本信息
- skill 文件正确放置到 `.claude/skills/sql-query/SKILL.md`
- 支持 macOS（amd64/arm64）和 Linux（amd64/arm64）平台

---

## User Personas

### Primary: 开发者（sql-query 新用户）
- **角色**: 想在自己项目中使用 sql-query skill 的开发者
- **目标**: 快速安装并开始使用
- **痛点**: 手动安装步骤多、容易出错
- **技术水平**: 中级

---

## User Stories & Acceptance Criteria

### Story 1: 通过 Claude Code 自动安装

**As a** 开发者
**I want to** 在 Claude Code 中引用安装提示词 URL 后自动完成安装
**So that** 无需手动操作即可开始使用 sql-query skill

**Acceptance Criteria:**
- [ ] `docs/setup.md` 包含结构化的安装指令，Claude Code 可理解并执行
- [ ] 自动检测当前 OS（darwin/linux）和 Arch（amd64/arm64）
- [ ] 从 GitHub Releases 下载最新版本的对应平台二进制
- [ ] 解压并安装到正确位置
- [ ] Windows 平台提示不支持并给出手动安装指引

### Story 2: 支持项目级和全局安装

**As a** 开发者
**I want to** 选择安装到当前项目或全局
**So that** 可以按需配置

**Acceptance Criteria:**
- [ ] 提示用户选择安装范围（项目级 / 全局）
- [ ] 项目级：skill 和 binary 均放在 `.claude/skills/sql-query/`，binary 路径为 `.claude/skills/sql-query/scripts/sql-query`
- [ ] 全局：skill 和 binary 均放在 `~/.claude/skills/sql-query/`，binary 路径为 `~/.claude/skills/sql-query/scripts/sql-query`
- [ ] SKILL.md 中 `SQL_QUERY_BIN` 默认值更新为 skill 内部的 `scripts/sql-query` 相对路径
- [ ] 安装完成后显示后续配置提示（DB_DSN 配置方法）

### Story 3: README 安装指引

**As a** 开发者
**I want to** 在 README 中看到清晰的安装方法
**So that** 知道如何快速开始

**Acceptance Criteria:**
- [ ] README 中包含安装段落
- [ ] 提供 Claude Code 一键安装命令（引用 raw URL）
- [ ] 提供手动安装备选方案

---

## Functional Requirements

### Core Features

**Feature 1: `docs/setup.md` 安装提示词**
- 结构化的 Claude Code 指令
- 安装流程：
  1. 检测 OS 和 Arch（`uname -s`, `uname -m`）
  2. 询问安装范围（项目级 / 全局）
  3. 获取最新 release tag（`gh release list` 或 GitHub API）
  4. 构造下载 URL 并下载对应归档文件
  5. 创建 `{skill_dir}/scripts/` 目录，解压二进制到其中
  6. 下载 SKILL.md 到 `{skill_dir}/SKILL.md`
  7. 验证安装（运行 `{skill_dir}/scripts/sql-query version`）
  8. 显示后续配置说明（DB_DSN）

  其中 `{skill_dir}` 为：
  - 项目级：`.claude/skills/sql-query/`
  - 全局：`~/.claude/skills/sql-query/`

**Feature 2: README 安装段落**
- Claude Code 自动安装方式（一行命令）
- 手动安装方式（从 Releases 下载）
- 后续配置说明（DB_DSN）

### Out of Scope
- 数据库连接配置（DB_DSN）的自动化
- 版本升级检测
- Windows 平台自动安装
- Homebrew 安装方式（Phase 2）

---

## Technical Constraints

### Integration
- **GitHub Releases**: 二进制归档命名格式 `sql-query_{version}_{os}_{arch}.tar.gz`（Windows: `.zip`）
- **SKILL.md**: 从仓库 raw URL 下载：`https://raw.githubusercontent.com/user-zouyu/sql-query/main/.claude/skills/sql-query/SKILL.md`
- **setup.md** raw URL: `https://raw.githubusercontent.com/user-zouyu/sql-query/main/docs/setup.md`

### 平台映射
| `uname -s` | `uname -m` | 归档文件 OS | 归档文件 Arch |
|------------|-----------|------------|-------------|
| Darwin | arm64 | darwin | arm64 |
| Darwin | x86_64 | darwin | amd64 |
| Linux | aarch64 | linux | arm64 |
| Linux | x86_64 | linux | amd64 |

---

## MVP Scope

### Phase 1: MVP（本次实现）
- `docs/setup.md` 安装提示词（支持项目级/全局安装）
- README 安装段落

### Phase 2: 增强
- Homebrew 安装支持
- 版本升级检测
- `.claude/settings.json` 自动配置

---

## Risk Assessment

| Risk | Probability | Impact | Mitigation Strategy |
|------|------------|--------|---------------------|
| GitHub raw URL 访问受限（国内网络） | Medium | High | 提供手动安装备选方案 |
| Claude Code 不同版本对提示词理解差异 | Low | Medium | 使用明确的 bash 命令，减少歧义 |
| Release 归档命名格式变更 | Low | High | 提示词中硬编码命名模板，与 goreleaser 配置一致 |

---

## Dependencies & Blockers

**Dependencies:**
- GitHub Releases 已有可用版本（v0.0.1-beta / v0.0.2-beta 已存在）
- 仓库为 public（或用户有访问权限）

**Known Blockers:**
- 无

---

*This PRD was created through interactive requirements gathering with quality scoring.*
