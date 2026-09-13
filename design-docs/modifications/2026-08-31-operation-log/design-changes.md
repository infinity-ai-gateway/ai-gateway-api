# 操作日志（Operation Log）设计变更说明

> 对应设计文档：`document-ai-gateway/迭代系统设计/v0.6/操作日志/操作日志设计方案.md`

## 1. 新增数据表

### 1.1 `operation_logs`

```sql
DROP TABLE IF EXISTS `operation_logs`;
CREATE TABLE `operation_logs` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `log_id` varchar(64) NOT NULL COMMENT '请求唯一标识，与 access log 中的 LogID 一致',
  `operator_type` tinyint(4) NOT NULL DEFAULT '0' COMMENT '操作者类型：0=user, 1=token',
  `operator_id` bigint(20) NOT NULL DEFAULT '0' COMMENT '操作者在对应表中的主键 ID',
  `operator_name` varchar(255) NOT NULL DEFAULT '' COMMENT '操作者名称',
  `action` varchar(32) NOT NULL COMMENT '操作动作：create/update/delete/reset/...',
  `resource_type` varchar(64) NOT NULL COMMENT '资源类型：entity/api_key/provider/...',
  `resource_id` varchar(255) NOT NULL DEFAULT '' COMMENT '被操作资源业务 ID',
  `resource_name` varchar(512) NOT NULL DEFAULT '' COMMENT '被操作资源名称',
  `resource_parent_id` varchar(255) NOT NULL DEFAULT '' COMMENT '资源父级业务 ID',
  `status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '操作结果：1=success, 2=failed',
  `error_msg` varchar(1024) NOT NULL DEFAULT '' COMMENT '失败时的简要错误信息',
  `change_summary` mediumtext COMMENT '变更摘要 JSON（脱敏后）',
  `request_path` varchar(512) NOT NULL DEFAULT '' COMMENT '请求路径',
  `request_method` varchar(16) NOT NULL DEFAULT '' COMMENT '请求方法',
  `client_ip` varchar(64) NOT NULL DEFAULT '' COMMENT '客户端 IP',
  `user_agent` varchar(512) NOT NULL DEFAULT '' COMMENT 'User-Agent',
  `created_at` datetime NOT NULL COMMENT '操作发生时间',
  PRIMARY KEY (`id`),
  KEY `idx_operator` (`operator_type`, `operator_id`),
  KEY `idx_resource` (`resource_type`, `resource_id`),
  KEY `idx_action` (`action`),
  KEY `idx_created_at` (`created_at`),
  KEY `idx_log_id` (`log_id`),
  KEY `idx_resource_parent` (`resource_parent_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='AI 网关配置操作日志表';
```

同步更新 `db_ddl.sql` 与 `db_ddl_sqlite.sql`。

## 2. 新增模块

### 2.1 `model/ioperlog`

新增操作日志业务管理模块：

- `types.go`：定义 `OperationLogEntry`、`OperationLogFilter`、`OperationLogQueryResult` 等类型。
- `storager.go`：定义 `OperationLogStorager` 接口（`BatchCreate`、`List`）。
- `manager.go`：定义 `OperationLogManager`：
  - `Record(ctx, entry)`：异步提交日志到内存缓冲通道。
  - `RecordSync(ctx, entry)`：同步写入，用于队列满降级或关键路径。
  - `QueryLogs(ctx, filter)`：查询操作日志。
  - 后台 worker：按批量大小（默认 200 条）或时间窗口（默认 5 秒）批量 INSERT。

### 2.2 `storage/rdb/ioperlog`

- `operation_log.go`：实现 `OperationLogStorager`，调用底层 DAO。
- `storage/rdb/internal/dao/table_operation_logs.go`：新增 `TOperationLog*` 辅助函数。

### 2.3 `endpoints/openapi_v1/operation_log`

- `list.go`：实现 `GET /open-api/v1/operation-logs` handler。
- `endpoints.go`：注册 endpoint。
- `convert.go`：响应结构转换。

## 3. 既有模块修改

### 3.1 容器初始化

`stateful/container/container.go`：

- 初始化 `OperationLogStorager` 与 `OperationLogManager`。
- 将 `OperationLogManager` 注入到各配置域 Manager。

### 3.2 各配置域 Manager 接入

在所有会产生写操作的 Manager 中，事务提交后调用 `OperationLogManager.Record()`：

| Manager | 文件路径 | 记录时机 |
|---------|----------|----------|
| EntityManager | `model/entity/entity_manager.go` | Create / Update / Delete |
| EntityTypeManager | `model/entity/entity_type_manager.go` | Create / Update / Delete |
| APIKeyManager | `model/api_key/api_key.go` | Create / Update / Delete |
| ProviderManager | `model/iprovider/provider.go` | Create / Update / Delete |
| ClusterManager | `model/icluster_conf/cluster.go` | Create / Update / Delete |
| RouteRuleManager | `model/iroute_conf/route_rule.go` | Create / Update / Delete |
| DomainManager | `model/iroute_conf/domain.go` | Create / Update / Delete |
| CertificateManager | `model/iprotocol/certificate.go` | Create / Update / Delete |
| QuotaPlanManager | `model/quota/quota_plan_manager.go` | Create / Update / Delete / Reset |
| RateLimitPolicyManager | `model/rate_limit_policy/rate_limit_policy_manager.go` | Create / Update / Delete |
| ModelPriceManager | `model/imodel_price/model_price.go` | Create / Update / Delete / Import |
| AuthManager | `model/iauth/authentication.go` / `authorization.go` | User / Token 的 Create / Update / Delete / 授权变更 |

### 3.3 权限模型

`model/iauth/features.go`：

- 新增 `FeatureOperationLog Feature = "OperationLog"`。
- 在 `ScopeSystem` 权限映射中授予 `actionAll`。

## 4. 敏感字段脱敏

### 4.1 脱敏规则

| 字段类型 | 脱敏方式 |
|----------|----------|
| `api_key.token` | 掩码，保留前 4 位和后 4 位，中间用 `****` 替代 |
| `password` / `secret` | 完全掩码为 `******` 或不记录 |
| `private_key` / 证书私钥 | 不记录具体内容，仅标记“已更新” |
| `session_key` / 用户密码 | 不记录 |

### 4.2 脱敏工具函数

建议在 `model/ioperlog` 或 `lib/` 中提供：

```go
func MaskSensitiveFields(resourceType string, data map[string]interface{}) map[string]interface{}
```

各 Manager 在构造 `change_summary` 前调用该函数，确保写入数据库前已完成脱敏。

## 5. 异步写入与降级策略

### 5.1 异步策略

- 缓冲通道默认容量 4096 条。
- 批量大小 200 条或每 5 秒触发一次 INSERT。
- 写入失败重试 3 次，仍失败则记录 error log。
- 进程退出时 flush 缓冲区剩余日志。

### 5.2 降级策略

当缓冲队列满时，根据配置选择：

- `sync`：同步写入数据库（推荐生产初期使用，确保审计不丢失）。
- `discard`：丢弃新日志并告警（稳定后可选用）。
- `expand`：临时扩容缓冲（需设上限）。

### 5.3 监控指标

建议新增 Prometheus 指标：

- `operation_log_buffered_total`
- `operation_log_dropped_total`
- `operation_log_insert_duration_ms`
- `operation_log_insert_failed_total`

## 6. 导出前置准备

第一期完成操作日志导出能力的字段标准化：

- 确定 CSV / JSON 导出字段顺序与列名。
- 明确 `change_summary` 在导出时的展开/压缩策略（建议默认压缩为 JSON 字符串）。
- 约定时间字段导出格式（建议 ISO 8601 或 Unix 时间戳）。

第三期在此基础上实现实际的导出接口与任务。

## 7. 测试要求

- `model/ioperlog` 语句覆盖率需达到 70% 以上。
- 验证异步缓冲、批量写入、队列满降级、优雅关闭 flush 等行为。
- 验证所有接入域的 Manager 在写操作后正确调用 `OperationLogManager.Record()`。
- 验证敏感字段脱敏效果。
