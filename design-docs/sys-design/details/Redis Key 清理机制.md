# Redis Key 清理机制

## 1. 背景与目标

### 1.1 背景

`ai-gateway-api` 在管理 API-Key、Entity 及其配额计划（quota-plan）和限流策略（rate-limit）时，运行时会向 Redis 写入大量状态 Key：

- **Quota Key**：记录 API-Key / Entity 的实时剩余额度；
- **Rate-Limit Key**：记录每条限流规则在当前窗口内的已消耗 Token 数 / 请求数。

当前控制面只负责这些 Key 的创建与更新，**没有任何地方主动删除已经失效的 Key**。随着 API-Key、Entity 被删除，或 rate-limit 规则被移除/变更，Redis 中会残留大量无用 Key，导致：

- 内存无法回收；
- 后续同名规则/资源复用时可能“继承”旧额度或旧计数。

### 1.2 目标

在以下场景发生时，由控制面主动清理对应 Redis Key：

1. API-Key 删除；
2. Entity 删除；
3. API-Key 更新导致 rate-limit 规则被删除；
4. Entity 更新导致 rate-limit 规则被删除。

同时明确：配额 plan 的常规变更（改配额、改单位、切换 unlimited）**不会**删除 Quota Key，只会覆盖其值。

---

## 2. Redis Key 格式

### 2.1 Quota Key

控制面定义（`stateful/config_redis.go`）：

```go
func AIUsedQuotaKey(key string) string {
    return fmt.Sprintf("QUOTA_%s", key)
}
```

实际 Redis Key：

| 归属对象 | Key 示例 |
|----------|----------|
| API-Key | `QUOTA_<api-key-token>` |
| Entity | `QUOTA_<entity-id>` |

BFE 侧直接使用控制面导出的 `QuotaPlan.RedisKey` 字段。

### 2.2 Rate-Limit Key

控制面导出时生成（`model/rate_limit_policy/rate_limit_policy_manager.go`）：

```go
RedisKey: fmt.Sprintf("RL_TPM_rlp-%d_%s", policyID, rule.Name)
RedisKey: fmt.Sprintf("RL_RPM_rlp-%d_%s", policyID, rule.Name)
```

其中 `policyID` 为限流策略的数据库 ID，`name` 为规则名称（在同一 policy 内唯一且不可变）。

BFE 侧在最终访问 Redis 时会拼接前缀：

```go
func buildRedisKey(policyId string, suffix string) string {
    return fmt.Sprintf("%s_%s_%s", "default_bfe", policyId, suffix)
}
```

因此 BFE 实际写入/读取的 Key 形如：

```text
default_bfe_rlp-<policyID>_RL_TPM_rlp-<policyID>_<name>
default_bfe_rlp-<policyID>_RL_RPM_rlp-<policyID>_<name>
```

> 并发限流 Key（`default_bfe_rlp-<policyID>_con`）由 BFE 自行构造，控制面不感知，不在本机制清理范围内。该 Key 已有 300 秒 TTL 并会在每次请求时刷新。

---

## 3. 触发场景与清理范围

### 3.1 API-Key 删除

**触发点**：`DELETE /open-api/v1/api-keys/{id}` 数据库事务提交成功后。

**清理范围**：

- 该 API-Key 自身的 Quota Key：`QUOTA_<api-key-token>`
- 该 API-Key 直接绑定的 `RateLimitPolicyID` 导出的全部 Rate-Limit Key

**不清理**：

- 所属 Entity 的 Quota Key
- 所属 Entity 及其父 Entity 的 Rate-Limit Key

原因：Entity 未被删除，仍可能被其他 API-Key 引用。

### 3.2 Entity 删除

**触发点**：`DELETE /open-api/v1/entities/{id}` 数据库事务提交成功后。

**约束**：当前 Entity 删除前会校验其下是否还有 API-Key 或子 Entity；若有则拒绝删除。

**清理范围**：

- 该 Entity 自身的 Quota Key：`QUOTA_<entity-id>`
- 该 Entity 自身 `RateLimitPolicyID` 导出的全部 Rate-Limit Key

**不清理**：

- 父 Entity 的 Quota Key
- 父 Entity 的 Rate-Limit Key

原因：父 Entity 未被删除，仍可能被其他子 Entity 或 API-Key 引用。

### 3.3 API-Key 更新导致 rate-limit 规则被删除

**触发点**：`PATCH /open-api/v1/api-keys/{id}` / `PUT /open-api/v1/api-keys/{id}` 数据库事务提交成功后。

**清理范围**：当前 API-Key 直接绑定的 `RateLimitPolicyID` 中，旧规则列表存在但新规则列表不存在的规则对应的 Rate-Limit Key。

**匹配方式**：按规则 `name` 精确匹配。

### 3.4 Entity 更新导致 rate-limit 规则被删除

**触发点**：`PATCH /open-api/v1/entities/{id}` / `PUT /open-api/v1/entities/{id}` 数据库事务提交成功后。

**清理范围**：当前被更新的 Entity 的 `RateLimitPolicyID` 中，旧规则列表存在但新规则列表不存在的规则对应的 Rate-Limit Key。

**不清理**：父 Entity 的 Rate-Limit Key。

---

## 4. 配额 plan 变更不触发 Quota Key 删除

根据 `model/quota/quota_plan_manager.go` 的 `adjustQuota` 实现：

| 变更类型 | 行为 | 是否删除 Quota Key |
|----------|------|-------------------|
| 有限配额 → 无限配额 | `ResetToQuota(ctx, key, sentinel, unit)` | 否 |
| 无限配额 → 有限配额 | `ResetToQuota(ctx, key, newQuota, unit)` | 否 |
| 单位 `total_token` ↔ `RMB` | `ResetToQuota(ctx, key, newQuota, unit)` | 否 |
| 配额总额变化 | `SetRemaining(ctx, key, remaining, unit)` | 否 |

因此，配额 plan 的常规变更**不会**产生“BFE 旧配置访问已删除 Quota Key”的问题。只有在 API-Key / Entity 被删除时，才会触发 Quota Key 清理。

---

## 5. 配置下发时序与对 BFE 的影响

### 5.1 配置下发时序

1. 控制面接收删除/更新请求，先写数据库；
2. 控制面返回 API 响应；
3. 控制面立即删除对应 Redis Key；
4. `conf-agent` 定期轮询 InnerAPI 拉取最新配置；
5. `conf-agent` 将新配置写入 BFE 并触发热加载；
6. BFE 使用新配置处理后续请求。

因此，控制面的 Redis Key 删除一定发生在 BFE 感知到配置变更之前。

### 5.2 Quota Key 提前删除的影响

BFE 在请求转发前调用 `plan.HasBalance`：

```go
current, err := client.GetInt64(q.RedisKey)
if err != nil {
    return false, 0, err
}
```

若 Quota Key 已被删除，`GetInt64` 返回 Redis 空值错误，BFE 返回 `CodeInternalQuotaError`。

但由于 Quota Key 的删除只发生在 API-Key / Entity 删除场景，而被删除的资源本不应再被使用，因此实际业务影响有限，主要表现为错误码不够准确。

### 5.3 Rate-Limit Key 提前删除的影响

BFE 的 TPM/RPM 限流使用 Lua 脚本访问 Redis。若 Key 已被删除，脚本会将其视为该窗口内无任何消耗，请求被放行（相当于限流窗口被重置）。

这会导致旧配置窗口内短暂限流失效、流量突增。窗口长度等于 `conf-agent` 轮询周期 + BFE 热加载时间，通常为秒级，可接受。

---

## 6. 实现方案：控制面立即清理

### 6.1 核心设计

- 在 `APIKeyManager.DeleteAPIKey`、`EntityManager.DeleteEntity` 完成数据库事务提交后，立即删除对应的 Quota Key 和 Rate-Limit Key。
- 在 `APIKeyManager.UpdateAPIKey` / `FullUpdateAPIKey`、`EntityManager.UpdateEntity` / `FullUpdateEntity` 中，若 rate-limit 规则发生删除/变更，立即删除旧规则对应的 Rate-Limit Key。
- quota-plan 常规变更不触发 Quota Key 清理。

### 6.2 扩展 `QuotaCache` 接口

在 `model/quotacache/quotacache.go` 中新增：

```go
type QuotaCache interface {
    GetRemaining(ctx context.Context, key string, unit *string) (float64, error)
    SetRemaining(ctx context.Context, key string, quota *float64, unit *string) error
    ResetToQuota(ctx context.Context, key string, quota *float64, unit *string) error
    ResetToQuotaAtomic(ctx context.Context, key string, quota *float64, unit *string) error
    DeleteKeys(ctx context.Context, keys []string) error
}
```

`DeleteKeys` 内部使用 Redis Pipeline 批量执行 `UNLINK`（小 Key 可直接 `DEL`）。

### 6.3 Rate-Limit Key 生成与清理

#### 6.3.1 用 `name` 作为规则稳定标识

原方案按数组下标生成 Redis Key：

```go
RedisKey: fmt.Sprintf("RL_TPM_rlp-%d_%d", policyID, idx)
```

导致规则删除/顺序调整时难以精确清理单条规则。本次机制将 rate-limit 规则的 `name` 提升为 policy 内唯一且不可变的标识，并用 `name` 生成 Redis Key：

```go
RedisKey: fmt.Sprintf("RL_TPM_rlp-%d_%s", policyID, rule.Name)
```

#### 6.3.2 `name` 合法性条件

- 必填、非空、长度 1-128 字符（已有）；
- 同一 `RateLimitPolicy` 内唯一（已有）；
- 字符集限制为 `[a-zA-Z0-9_-]`（新增）；
- 创建后不可修改（新增）。

#### 6.3.3 更新时规则匹配

- 请求体中存在、数据库中也存在的 `name`：视为修改该规则（`model`、`window_minutes` 等可变更，`name` 不可变）；
- 请求体中存在、数据库中不存在的 `name`：视为新增规则；
- 数据库中存在、请求体中不存在的 `name`：视为删除规则，触发该规则 Redis Key 的清理。

### 6.4 涉及文件

| 文件 | 修改内容 |
|------|----------|
| `model/quotacache/quotacache.go` | `QuotaCache` 接口新增 `DeleteKeys` |
| `model/quotacache/redis.go` | 实现 `DeleteKeys` |
| `model/api_key/api_key.go` | 删除/更新时清理 Quota Key 与 Rate-Limit Key |
| `model/entity/entity_manager.go` | 删除/更新时清理 Quota Key 与 Rate-Limit Key |
| `model/rate_limit_policy/rate_limit_policy.go` | `ExportTPMConfig` / `ExportRPMConfig` 保持 `RedisKey` 字段；生成逻辑改为按 `name` |
| `model/rate_limit_policy/rate_limit_policy_manager.go` | 导出时按 `name` 生成 `RedisKey`；提供规则 diff 辅助方法 |
| `lib/validate/validate.go` | 收紧 `name` 字符集；更新时校验 `name` 不可修改 |

---

## 7. 风险与补偿

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| Rate-Limit 短暂失效 | BFE 旧配置在热加载前仍可能访问已删除的 Rate-Limit Key，导致限流失效 | 窗口短暂，可接受 |
| Redis 压力过大 | 大批量 `UNLINK` 触发 Redis 主从同步压力 | 控制单次删除数量，分批执行 |
| 误删活跃 Key | 代码逻辑错误导致不该删除的 Key 被删除 | 删除前校验 Key 确实属于被删除/变更的资源；误删后可通过重置配额或等待 Rate-Limit 窗口自然恢复 |
| 历史 Key 残留 | 改造前按数组下标生成的旧 Key 不会被自动清理 | 运维可手动清理，或等待其因 TTL 自然过期； rate-limit Key 本身具备 TTL |

---

## 8. 相关文档

- `details/限流策略与导出.md`
- `details/配额余额同步机制.md`
- `api-define/OpenAPI接口定义/00-common.md`
