# Product Requirements Document: Conventional Commits Changelog

**Version**: 1.0
**Date**: 2026-04-07
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100

---

## Executive Summary

增强 GoReleaser 的 changelog 配置，按约定式提交（Conventional Commits）规范对 commits 进行分组展示。Release Notes 将自动按 Features、Bug Fixes、Refactoring 等类别组织，过滤掉 merge commits 等噪音，提升 GitHub Release 页面的可读性。

---

## Problem Statement

**现状**：当前 changelog 仅按时间排序展示所有 commits，不区分类型，阅读体验差。参见 v0.0.1-beta 的 Release Notes — 所有 commit 混在一起。

**方案**：在 `.goreleaser.yaml` 中配置 changelog groups，按 Conventional Commits 前缀分组，并使用 exclude 过滤无关 commits。

**业务影响**：用户和维护者可快速了解每个版本的新功能、修复和变更。

---

## Success Metrics

- Release Notes 按类别（Features / Bug Fixes / Refactoring / Others）分组展示
- merge commits 和 bot commits 被过滤
- 不符合约定式提交格式的 commit 归入 "Others" 分组

---

## User Stories & Acceptance Criteria

### Story 1: 分类展示 Changelog

**As a** Release 阅读者
**I want to** 看到按类别分组的变更日志
**So that** 能快速找到新功能和修复

**Acceptance Criteria:**
- [ ] `feat:` 开头的 commits 归入 "Features" 分组
- [ ] `fix:` 开头的 commits 归入 "Bug Fixes" 分组
- [ ] `refactor:` 开头的 commits 归入 "Refactoring" 分组
- [ ] `docs:`, `chore:`, `ci:`, `test:` 等归入 "Others" 分组
- [ ] merge commits 被过滤不展示

---

## Functional Requirements

### 修改文件
- `.goreleaser.yaml` — 替换 `changelog` 配置块

### Changelog Groups 配置

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
  exclude:
    - '^Merge '
```

### Out of Scope
- 自定义 commit message lint（commitlint）
- 自动版本号推断（semantic-release）

---

## Technical Constraints

- GoReleaser v2 `changelog.groups` 使用 Go 正则语法
- `exclude` 过滤 merge commits

---

## MVP Scope

单一改动：修改 `.goreleaser.yaml` 中 `changelog` 部分，无其他文件变更。

---

*This PRD was created through interactive requirements gathering with quality scoring.*
