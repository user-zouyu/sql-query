# Product Requirements Document: S3 预签名 URL 处理 (Phase 2)

**Version**: 1.0
**Date**: 2026-04-05
**Author**: Sarah (Product Owner)
**Quality Score**: 92/100
**依赖**: sql-query Phase 1 已完成

---

## Executive Summary

为 `sql-query query` 子命令增加 S3 预签名 URL 处理能力。当 SQL 列别名包含 `[URL(duration)]` 元数据时，自动将数据库中存储的 `bucket:key` 值转换为带时效的预签名 URL。支持所有 S3 兼容存储（AWS S3、阿里云 OSS、MinIO、腾讯 COS 等），通过 Worker Pool 并发处理以支持大数据量导出。

这是 sql-query 的 Phase 2 增强功能，Phase 1 中对 `[URL]` 元数据仅输出警告，本阶段补全该能力。

---

## Problem Statement

**现状**：Phase 1 的 `sql-query query` 在遇到 `[URL(24h)]` 元数据时仅打印警告，数据库中的 `bucket:key` 原始值直接输出，无法在报表中预览图片/视频，也无法作为有效 URL 供 AI Agent 使用。

**方案**：移植 sql-exporter 的 S3 预签名模块，在查询结果导出前将所有 `[URL]` 标记列的值转换为预签名 URL。

**业务影响**：
- Excel/HTML 报表中的图片和视频可直接预览和下载
- AI Agent 获取的 JSON 数据中包含可访问的临时 URL
- 与 sql-exporter 的元数据协议完全兼容

---

## Success Metrics

- `[URL(24h)]` 元数据生效，`bucket:key` 被替换为预签名 URL
- `[URL(24h,D)]` 的下载模式（Content-Disposition: attachment）正常工作
- Excel 中预签名 URL 自动转为超链接，HTML 中图片/视频可预览
- JSON 输出中的 URL 可直接访问
- 任何一个预签名失败时整体报错退出（exit code 1）
- 并发处理性能：1000 个 URL 的签名时间 < 30 秒（取决于网络）

---

## Functional Requirements

### 核心功能：S3 预签名 URL 生成

**触发条件**：SQL 列别名中包含 `[URL(duration)]` 或 `[URL(duration,D)]` 元数据

**输入格式**：数据库中存储的 `bucket:key` 字符串，如：
```
magic-frame:image/019a6193f44a7465bb1c8b32886e7a14.png
my-bucket:videos/demo.mp4
```

**输出格式**：带时效的预签名 URL，如：
```
https://my-bucket.s3.us-west-1.amazonaws.com/image/xxx.png?X-Amz-Algorithm=...&X-Amz-Expires=86400&X-Amz-Signature=...
```

**元数据参数解析**（已由 Phase 1 的 parser 完成）：

| 元数据 | 示例 | 解析结果 |
|--------|------|---------|
| `[URL(24h)]` | 24 小时有效期 | `expiry=24h, download=false` |
| `[URL(15m)]` | 15 分钟有效期 | `expiry=15m, download=false` |
| `[URL(1h30m)]` | 1.5 小时有效期 | `expiry=1h30m, download=false` |
| `[URL(24h,D)]` | 24 小时 + 下载模式 | `expiry=24h, download=true` |

**下载模式**：当 `D` 标记存在时，预签名 URL 附加 `ResponseContentDisposition=attachment; filename="原始文件名"` 参数，触发浏览器下载而非在线预览。

---

### 并发处理：Worker Pool

**处理流程**：

1. 扫描所有列，找出包含 `[URL]` 元数据的列索引
2. 遍历所有行，为每个非空的 URL 列单元格创建 Task
3. 启动 Worker Pool（`-w` 参数控制并发数，默认 CPU 核数）
4. 每个 Worker 从 Task Channel 取任务，调用 S3 预签名 API
5. 签名成功：原地更新 data 数组中的值
6. 签名失败：**整体失败**，返回错误，进程退出（exit code 1）

**进度日志**（`--log-level info`）：
```
[INFO] 发现 2 个 URL 列需要预签名处理
[INFO] 共 500 个单元格需要签名
[INFO] S3 预签名处理中 (workers: 8)...
[INFO] 签名进度: 100/500 (20.0%)
[INFO] 签名进度: 500/500 (100.0%)
[INFO] 预签名处理完成 (耗时 12.3s)
```

---

### 环境变量配置

| 变量 | 必填 | 默认值 | 说明 |
|------|------|--------|------|
| `S3_ACCESS_KEY` | 是* | — | S3 Access Key ID |
| `S3_SECRET_KEY` | 是* | — | S3 Secret Access Key |
| `S3_REGION` | 是* | — | S3 Region，如 `us-west-1`、`cn-hangzhou` |
| `S3_ENDPOINT` | 否 | — | 自定义 Endpoint（阿里云 OSS、MinIO 等必填） |

*仅在查询结果包含 `[URL]` 元数据时必填。无 `[URL]` 列时不检查 S3 配置。

**阿里云 OSS 配置示例**：
```env
S3_ACCESS_KEY=LTAI5tXXXXXXXXXXX
S3_SECRET_KEY=XXXXXXXXXXXXXXXXXXXXXXXX
S3_REGION=cn-hangzhou
S3_ENDPOINT=https://oss-cn-hangzhou.aliyuncs.com
```

**AWS S3 配置示例**：
```env
S3_ACCESS_KEY=your-aws-access-key
S3_SECRET_KEY=your-aws-secret-key
S3_REGION=us-west-1
# S3_ENDPOINT 留空则使用 AWS 默认
```

---

## Error Handling

### 失败策略：整体失败

任何一个单元格的预签名失败，**整体报错退出**：

```
Error: S3 预签名失败: 行 15 列 3: bucket 'xxx' 不存在
```

JSON 模式：
```json
{"error": "s3_presign_failed", "message": "S3 预签名失败: 行 15 列 3: bucket 'xxx' 不存在"}
```

退出码：1（通用错误）

### 常见错误场景

| 场景 | 错误信息 | 原因 |
|------|---------|------|
| S3 未配置 | `存在 [URL] 元数据但未配置 S3：需要 S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION` | .env 中缺少 S3 配置 |
| bucket:key 格式错误 | `无效的 S3 路径格式，期望 'bucket:key'，实际: xxx` | 数据库中存的值格式不对 |
| 签名失败 | `S3 预签名失败: 行 X 列 Y: Access Denied` | 凭据无权限或 bucket 不存在 |
| 过期时间格式错误 | `无效的过期时间格式: abc` | `[URL(abc)]` 中的 duration 无法解析 |

---

## Technical Architecture

### 新增/修改模块

| 模块 | 动作 | 说明 |
|------|------|------|
| `internal/config/config.go` | 修改 | 增加 S3 配置字段和 `HasS3Config()` 方法 |
| `internal/s3/presigner.go` | 新增 | 从 sql-exporter 移植，S3 预签名器 |
| `internal/processor/pipeline.go` | 新增 | 从 sql-exporter 移植，Worker Pool 并发处理 |
| `cmd/query.go` | 修改 | 在导出前调用 processor.Process()，替换警告逻辑 |

### 从 sql-exporter 移植

直接移植以下文件，仅修改 import 路径和日志输出（`fmt.Printf` → `log.Info`/`log.Debug`）：

- `sql-exporter/internal/s3/presigner.go` → `internal/s3/presigner.go`
- `sql-exporter/internal/processor/pipeline.go` → `internal/processor/pipeline.go`

### Config 变更

```go
// internal/config/config.go — 新增字段
type Config struct {
    DBDSN        string
    QueryTimeout int

    // S3 配置（Phase 2）
    S3AccessKey string
    S3SecretKey string
    S3Region    string
    S3Endpoint  string // 可选，用于兼容 OSS/MinIO
}

func (c *Config) HasS3Config() bool {
    return c.S3AccessKey != "" && c.S3SecretKey != "" && c.S3Region != ""
}
```

### query.go 变更

替换现有的 `[URL]` 警告逻辑为实际的预签名处理：

```go
// 替换原来的警告代码
if urlCount > 0 {
    if !cfg.HasS3Config() {
        errutil.Exit(errutil.ExitGenericError, "invalid_argument",
            "存在 [URL] 元数据但未配置 S3：需要 S3_ACCESS_KEY, S3_SECRET_KEY, S3_REGION", jsonFlag)
    }
    log.Info("S3 预签名处理中 (workers: %d)...", workers)
    if err := processor.Process(cfg, parsedColumns, data, workers); err != nil {
        errutil.Exit(errutil.ExitGenericError, "s3_presign_failed",
            fmt.Sprintf("S3 预签名失败: %s", err), jsonFlag)
    }
}
```

### 依赖新增

```
github.com/aws/aws-sdk-go-v2
github.com/aws/aws-sdk-go-v2/credentials
github.com/aws/aws-sdk-go-v2/service/s3
```

---

## Implementation Plan

### Step 1: Config 扩展
- 在 `config.go` 中增加 S3 字段和 `HasS3Config()` 方法

### Step 2: 移植 S3 预签名器
- 复制 `sql-exporter/internal/s3/presigner.go`
- 修改 import 路径（`sql-exporter` → `sql-query`）

### Step 3: 移植并发处理器
- 复制 `sql-exporter/internal/processor/pipeline.go`
- 修改 import 路径
- 日志改用 `log.Info`/`log.Debug`
- 失败策略改为整体失败（原实现是保留原始值继续）

### Step 4: 修改 query 命令
- 替换 `[URL]` 警告为实际的 `processor.Process()` 调用
- 添加 S3 配置缺失的错误处理

### Step 5: 验证

```bash
# 构建
go build -o sql-query .

# 测试预签名（需要真实 S3/OSS 凭据）
echo "SELECT avatar \`[URL(24h)][HTML(I)] 头像\` FROM users" | \
  ./sql-query query -e .env --json

# 测试下载模式
echo "SELECT file_path \`[URL(24h,D)] 文件\` FROM attachments" | \
  ./sql-query query -e .env --excel -o report.xlsx

# 测试缺少 S3 配置时的错误
echo "SELECT avatar \`[URL(24h)] 头像\` FROM users" | \
  ./sql-query query -e .env.no-s3 --json
# 期望: exit code 1, {"error":"invalid_argument","message":"存在 [URL] 元数据但未配置 S3..."}

# 测试无 URL 列时不需要 S3 配置
echo "SELECT * FROM users" | \
  ./sql-query query -e .env.no-s3 --json
# 期望: 正常输出，无 S3 相关错误
```

---

## Out of Scope

- 多 S3 Endpoint 支持（不同 bucket 对应不同存储服务）
- S3 上传功能
- 预签名 URL 缓存
- 自定义 S3 签名版本（统一使用 v4）
