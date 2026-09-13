# 操作日志（Operation Log）变更摘要

> 对应设计文档：`document-ai-gateway/迭代系统设计/v0.6/操作日志/操作日志设计方案.md`

## 1. 背景

`ai-gateway-api` 当前仅记录 HTTP access log，用于请求链路排障，但缺少面向配置变更的结构化审计能力。当 entity、api-key、provider、route 等配置被修改后，无法快速回答“谁在什么时间做了什么修改、修改了哪些字段、结果如何”等问题，给故障追溯和安全审计带来困难。

## 2. 目标

- 对 `ai-gateway-api` 控制面所有会产生写操作的配置域记录操作日志，并持久化到数据库。
- 提供结构化查询接口，支持按操作人、资源类型、资源 ID、动作类型、时间范围等维度检索。
- 对 api-key token、密码、私钥等敏感字段进行脱敏，避免日志泄露。
- 采用异步批量写入，尽量降低对主业务请求的延迟影响。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 涉及模块 | `model/ioperlog`、`storage/rdb/ioperlog`、`endpoints/openapi_v1/operation_log`、各配置域 Manager |
| 涉及接口 | 新增 `GET /open-api/v1/operation-logs`；各既有写接口内部接入操作日志记录 |
| 数据迁移 | 新增 `operation_logs` 表，无历史数据迁移 |
| 数据面影响 | 无 |

## 4. 关键决策

| 决策 | 说明 |
|------|------|
| Manager 层主动记录 | 由业务 Manager 在写操作成功后构造 `OperationLogEntry`，可携带资源 ID、名称、变更摘要等业务语义，优于纯 Middleware 解析。 |
| 异步批量落库 | `OperationLogManager` 通过内存缓冲 + 后台 worker 批量写入数据库，默认 200 条或 5 秒触发一次 INSERT。 |
| 全量域一期接入 | 第一期即覆盖 entity、entity_type、api_key、provider、cluster、route、domain、certificate、quota_plan、rate_limit_policy、model_price、user、token 等全部配置域。 |
| 失败操作一并记录 | 写操作失败时同样记录日志（`status = 2`），保留失败审计证据。 |
| 敏感字段统一脱敏 | 提供 `maskSensitiveFields` 工具函数，对 token、密码、私钥等字段进行掩码或排除。 |
| 查询接口仅对管理员开放 | 先按系统管理员权限控制；待权限体系重构后，再按 entity 维度细化。 |

## 5. 关联文档

- `document-ai-gateway/迭代系统设计/v0.6/操作日志/操作日志设计方案.md`
- `ai-gateway-api/design-docs/modifications/2026-08-31-operation-log/api-changes.md`
- `ai-gateway-api/design-docs/modifications/2026-08-31-operation-log/design-changes.md`
