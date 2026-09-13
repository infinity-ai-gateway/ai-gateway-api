# 操作日志失败记录设计变更说明

## 1. 现状问题

当前各配置域的操作日志记录函数固定将 `Status` 设置为 `ioperlog.StatusSuccess`，且不接收 `error` 入参，例如：

```go
// model/icluster_conf/cluster_operation_log.go
func (cm *ClusterManager) recordClusterOperation(
    ctx context.Context,
    action string,
    clusterID int64,
    clusterName string,
    before, after map[string]interface{},
) {
    entry := &ioperlog.OperationLogEntry{
        ...
        Status: ioperlog.StatusSuccess,
        ...
    }
    cm.operationLogManager.Record(ctx, entry)
}
```

Manager 的写操作仅在成功分支调用该函数：

```go
// model/icluster_conf/cluster.go
func (cm *ClusterManager) DeleteCluster(...) (err error) {
    err = cm.txn.AtomExecute(ctx, func(ctx context.Context) error {
        ...
    })
    if err != nil {
        return err   // 失败时无日志
    }
    cm.recordClusterOperation(...)
    return nil
}
```

这导致 [issue #117](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/117) 中描述的 `DELETE /clusters/{cluster_name}` 失败场景没有任何操作日志。

## 2. 设计变更

### 2.1 新增错误信息截断工具函数

在 `model/ioperlog` 中新增辅助函数，确保 `error_msg` 不超过数据库字段长度：

```go
// model/ioperlog/helper.go
func TruncateErrorMessage(err error, maxLen int) string
```

- `maxLen` 默认取 `1024`，与 `operation_logs.error_msg` 字段长度一致。
- 截断时保留末尾省略标记（如 `...`），使阅读者明确信息已被截断。
- 若 `err == nil` 返回空字符串。

### 2.2 修改各域记录函数签名

所有 `recordXxxOperation` 函数统一增加 `err error` 入参：

```go
func (cm *ClusterManager) recordClusterOperation(
    ctx context.Context,
    action string,
    clusterID int64,
    clusterName string,
    before, after map[string]interface{},
    err error,   // 新增
)
```

函数内部：

```go
status := ioperlog.StatusSuccess
errorMsg := ""
if err != nil {
    status = ioperlog.StatusFailed
    errorMsg = ioperlog.TruncateErrorMessage(err, 1024)
}

entry := &ioperlog.OperationLogEntry{
    ...
    Status:   status,
    ErrorMsg: errorMsg,
    ...
}

cm.operationLogManager.Record(ctx, entry)
```

涉及文件：

| 域 | 记录函数文件 |
|----|--------------|
| entity | `model/entity/operation_log.go` |
| api_key | `model/api_key/api_key_operation_log.go` |
| provider | `model/iprovider/provider_operation_log.go` |
| cluster | `model/icluster_conf/cluster_operation_log.go` |
| route / domain | `model/iroute_conf/operation_log.go` |
| certificate | `model/iprotocol/operation_log.go` |
| quota_plan | `model/quota/operation_log.go` |
| rate_limit_policy | `model/rate_limit_policy/operation_log.go` |
| model_price | `model/imodel_price/operation_log.go` |
| user / token | `model/iauth/operation_log.go` |

### 2.3 在 Manager 失败分支补充调用

对每个 Manager 的写操作，在返回错误前调用记录函数并传入 `err`。典型模式：

```go
err = cm.txn.AtomExecute(ctx, func(ctx context.Context) error { ... })
if err != nil {
    cm.recordClusterOperation(
        ctx,
        string(ioperlog.ActionDelete),
        cluster.ID,
        cluster.Name,
        clusterToMap(cluster),
        nil,
        err,
    )
    return err
}
cm.recordClusterOperation(
    ctx,
    string(ioperlog.ActionDelete),
    cluster.ID,
    cluster.Name,
    clusterToMap(cluster),
    nil,
    nil,
)
return nil
```

优先处理 [issue #117](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/117) 相关的 `ClusterManager.DeleteCluster`，随后批量处理其余 Manager：

| Manager | 文件路径 | 需补充失败日志的动作 |
|---------|----------|----------------------|
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

### 2.4 失败时变更摘要的处理

- **Create 失败**：`change_summary` 可仅包含请求入参（`after`），帮助定位因参数校验失败导致的错误。
- **Update 失败**：若已查询到旧值，则保留 `before`；否则仅记录请求入参。
- **Delete 失败**：保留被删除对象的旧值（`before`），与成功删除时一致。
- 所有写入 `change_summary` 的数据继续经过 `ioperlog.MaskSensitiveFields()` 脱敏。

## 3. 接口与数据模型影响

### 3.1 API 接口

无变化。`GET /open-api/v1/operation-logs` 已支持 `status` 参数（`1` 成功，`2` 失败）和 `error_msg` 字段。

### 3.2 数据表

无变化。复用 `operation_logs` 表的现有字段：

```sql
`status` tinyint(4) NOT NULL DEFAULT '1' COMMENT '操作结果：1=success, 2=failed',
`error_msg` varchar(1024) NOT NULL DEFAULT '' COMMENT '失败时的简要错误信息',
```

## 4. 系统设计文档更新

为保持 `design-docs/sys-design/` 与代码实现同步，进行以下更新：

- `sys-design/模型层设计文档.md` 第 4.25 节：补充失败日志的写入时机、`recordXxxOperation` 的 `err` 参数语义、以及 `ClusterManager.DeleteCluster` 等典型失败路径示例。
- `sys-design/数据库设计文档.md` 第 6.9 节：补充 `status` 与 `error_msg` 字段在失败审计中的用途说明。
- 新增 `sys-design/details/操作日志模块.md`：沉淀操作日志模块的总体架构、数据模型、写入流程、异步批量机制、脱敏规则、接入域、失败日志处理、测试策略与边界情况。
- 更新 `sys-design/summary.md`：将"操作日志模块"索引从指向 modifications 改为指向新的 `details/操作日志模块.md`。

## 5. 测试要求

- 为 `ClusterManager.DeleteCluster` 新增失败场景单元测试：构造 delete checker 返回错误，断言生成的日志 `Status == ioperlog.StatusFailed` 且 `ErrorMsg` 非空。
- 为 `EntityManager` / `EntityTypeManager` 补充失败场景单元测试。
- 其余域至少保证编译通过；时间允许时补充失败断言。
- 运行：

```bash
cd ai-gateway-api
make test-model
make test-model-cover-gate
```

确保 `model/` 语句覆盖率不低于 70%。

## 6. 风险与注意事项

- **避免循环**：记录失败日志时不应触发新的业务校验或写操作，仅做数据组装。
- **幂等性**：同一操作多次失败会产生多条失败日志，符合审计需求。
- **错误信息敏感内容**：当前直接记录 `err.Error()`；若后续有合规要求，可在 `TruncateErrorMessage` 中增加敏感词过滤。
- **队列压力**：失败日志数量通常远低于成功日志，异步缓冲队列压力可忽略。
