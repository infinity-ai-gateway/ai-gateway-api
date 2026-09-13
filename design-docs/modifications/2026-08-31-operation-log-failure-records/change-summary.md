# 操作日志失败记录变更摘要

> 关联 issue：[ai-gateway-api#117](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/117)  
> 前置设计文档：`ai-gateway-api/design-docs/modifications/2026-08-31-operation-log/`

## 1. 背景

`ai-gateway-api` 在 2026-08-31-operation-log 变更中已为所有配置域接入操作日志，并在设计文档中明确：

> “失败操作一并记录 | 写操作失败时同样记录日志（`status = 2`），保留失败审计证据。”

但实际代码实现仅在写操作**成功**后调用 `OperationLogManager.Record()`。以 `ClusterManager.DeleteCluster` 为例，当集群仍被路由规则引用时，事务执行失败直接返回错误，操作日志表中不会留下任何记录，导致管理员无法通过 `GET /operation-logs` 追溯到这次失败的删除尝试。

## 2. 目标

- 让所有配置域的写操作在失败时也能生成一条 `status = 2` 的操作日志。
- 失败日志需携带简明的错误信息（`error_msg`），便于审计与排障。
- 保持现有成功日志行为不变，不引入新的 API 契约变更。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 涉及模块 | 所有已接入操作日志的配置域 Manager：`model/entity`、`model/api_key`、`model/iprovider`、`model/icluster_conf`、`model/iroute_conf`、`model/iprotocol`、`model/quota`、`model/rate_limit_policy`、`model/imodel_price`、`model/iauth` |
| 涉及接口 | 无接口契约变更；`GET /open-api/v1/operation-logs` 已支持按 `status` 过滤 |
| 数据迁移 | 无，复用现有 `operation_logs` 表的 `status` 与 `error_msg` 字段 |
| 数据面影响 | 无 |

## 4. 关键决策

| 决策 | 说明 |
|------|------|
| 统一 `recordXxxOperation` 签名 | 各域的私有记录函数增加 `err error` 入参，由函数内部根据 `err != nil` 决定 `Status` 与 `ErrorMsg`。 |
| 失败分支显式调用 | 在 Manager 的 Create / Update / Delete / Reset / Import / 授权变更等写操作返回错误前，调用记录函数并传入错误。 |
| 错误信息截断 | `error_msg` 字段为 `varchar(1024)`，超过长度时截断并追加省略标记，避免写入数据库时超长。 |
| 异步落盘不变 | 失败日志仍通过 `OperationLogManager.Record()` 进入异步缓冲队列，不阻塞业务返回。 |
| 最小可识别信息 | 失败日志至少包含操作动作、资源类型、资源 ID / 名称、失败原因；变更摘要（`change_summary`）在失败时尽可能保留入参或旧值。 |

## 5. 关联文档

- `ai-gateway-api/design-docs/modifications/2026-08-31-operation-log/change-summary.md`
- `ai-gateway-api/design-docs/modifications/2026-08-31-operation-log/api-changes.md`
- `ai-gateway-api/design-docs/modifications/2026-08-31-operation-log/design-changes.md`
- `ai-gateway-api/design-docs/modifications/2026-08-31-operation-log-failure-records/design-changes.md`
