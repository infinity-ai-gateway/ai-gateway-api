# quota-plan 定期重置多实例并发风险修复设计变更说明

> 对应风险分析报告：`document-ai-gateway/迭代系统设计/v0.6/quota-plan定期reset的风险调研/分析报告.md`

## 1. 概述

### 1.1 问题现象

`ai-gateway-api` 多实例部署时，每个实例都会在启动时初始化 `QuotaResetScheduler`（见 `stateful/container/rdb/components.go`），并以 1 分钟为周期独立执行 `BalanceSyncManager.ResetExpiredBalances`：

- 从 `quota_plans` 读取 `last_reset_at`；
- 判断 plan 是否跨越自然周/自然月边界；
- 若满足条件，重置该 plan 下所有 API-Key / Entity 的 Redis 剩余量，并更新 `quota_plans.last_reset_at`。

由于调度器之间没有互斥，多个实例可能同时判定某个 `quota_plan` 已跨越周期边界并执行重置，带来以下风险：

- **重复重置 Redis 导致已扣配额被回补**：当前 `ResetToQuota` 内部通过 `GetInt64 + IncrBy(delta)` 补偿写入，非原子；并发重置时可能把用户已消耗的量补回来，造成超额使用。
- **`last_reset_at` 更新竞争**：多个实例同时读取旧 `last_reset_at` 并判定需要重置，扩大了重复处理窗口。
- **Redis 与 DB 两步操作不一致**：重置 Redis 与更新 `quota_plans.last_reset_at` 不在同一个原子操作内，实例崩溃或网络抖动会导致两边状态不一致。

### 1.2 变更目标

1. 为 `QuotaResetScheduler` 引入基于 Redis 的分布式锁，保证全局同一时刻只有一个实例执行 `ResetExpiredBalances`。
2. 锁具备 TTL 和看门狗续期机制，实现故障转移：持有锁实例崩溃后，其他实例可在 TTL 到期后替补。
3. 释放锁时校验 token，避免复活实例误删他人锁。
4. 在 `ResetExpiredBalances` 更新 `quota_plans.last_reset_at` 时增加条件判断，实现最后一道幂等防线。
5. 本期不改动 API 契约、不新增数据表、不影响数据面导出格式。

### 1.3 变更范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 涉及模块 | `model/quota`、`model/quotacache`、`stateful/container/rdb` |
| 接口契约 | 无变化 |
| 数据迁移 | 无 |

---

## 2. 现状代码分析

### 2.1 调度器启动路径

```text
stateful/container/rdb/components.go
  └─ Init()
       └─ container.QuotaResetScheduler.Start()   // 每个实例都会执行
```

```go
// model/quota/scheduler.go
func (s *QuotaResetScheduler) run() {
    s.resetQuotasWithRecover()
    ticker := time.NewTicker(1 * time.Minute)
    for {
        select {
        case <-ticker.C:
            s.resetQuotasWithRecover()
        ...
        }
    }
}
```

### 2.2 到期重置

```go
// model/quota/balance_sync.go:ResetExpiredBalances
return m.txn.AtomExecute(ctx, func(ctx context.Context) error {
    // 1. 获取所有需要重置的配额计划（非无限配额且设置了 weekly/monthly）
    plans, err := m.planStorager.FetchQuotaPlanList(ctx, &QuotaPlanFilter{
        ResetPeriod: []string{"weekly", "monthly"},
        Unlimited:   lib.PBool(false),
    })
    // ...
    for _, plan := range plans {
        shouldReset := m.shouldResetByPeriod(plan.LastResetAt, *plan.ResetPeriod, now)
        if shouldReset {
            // 1. 重置该 plan 下所有 API-Key / Entity 的 Redis 使用量
            m.resetAPIKeysRedisUsage(ctx, *plan.ID)
            // 2. 更新 quota_plans.last_reset_at
            m.planStorager.UpdateQuotaPlan(ctx, &QuotaPlanFilter{ID: plan.ID},
                &QuotaPlanParam{LastResetAt: lib.PTime(now)})
        }
    }
    return nil
})
```

> 注：旧实现中还有 `BalanceSyncManager.SyncAllBalances`，用于每分钟把 Redis 剩余量汇总回写 `quota_balances` 表。该逻辑已在 `2026-08-26-quota-balance-sync-remove` 中移除，`quota_balances` 表也已删除，`last_reset_at` 已上移至 `quota_plans` 表。

### 2.3 Redis 重置方式

```go
// model/quotacache/redis.go
func (c *redisQuotaCache) ResetToQuota(ctx, key, quota, unit) error {
    return c.SetRemaining(ctx, key, quota, unit)
}

func (c *redisQuotaCache) SetRemaining(ctx, key, quota, unit) error {
    currentValue, _ := c.client.GetInt64(redisKey) // 可能返回 nil
    targetValue := PtrToRedisValue(quota, unit)
    delta := targetValue - currentValue
    _, err := c.client.IncrBy(redisKey, delta)
    return err
}
```

Redis 重置采用"读取当前值 + 计算差值 + IncrBy"的补偿方式，非原子，依赖读取后到写入前当前值不被其他写者修改。

---

## 3. 详细设计

### 3.1 核心原则

- **锁保护调度任务**：把 `ResetExpiredBalances` 放在锁下执行，避免多实例同时进入重置流程。
- **Redis 写入原子化**：定期重置时直接通过 Redis `SET` 把 key 覆盖为 quota 总量，替代 `GetInt64 + IncrBy(delta)` 的补偿写入，消除单条 Redis 写操作内部的 read-modify-write race。
- **先 Redis 后 DB**：单个 plan 的重置顺序为先原子写 Redis，再条件更新 `quota_plans.last_reset_at`；即使实例在写 DB 前崩溃，下次调度仍会再次触发，Redis 会被幂等地重新 SET 为 quota。
- **故障可转移**：锁带 TTL，崩溃实例持有的锁会自动过期；其他实例通过看门狗等待后可替补。
- **安全释放**：释放/续期时通过 Lua 脚本校验锁 value，防止误操作他人锁。
- **数据库条件更新兜底**：在锁正常持有的情况下，配合 `last_reset_at` 条件更新，可保证同一周期内同一 plan 只被重置一次。

### 3.2 新增 `LockClient` 抽象

在 `model/quotacache` 或 `lib/` 中新增分布式锁客户端抽象（如项目已有通用 Redis 锁实现，优先复用）：

```go
// DistributedLock 分布式锁客户端抽象
type DistributedLock interface {
    Acquire(ctx context.Context, key, token string, ttl time.Duration) (bool, error)
    Release(ctx context.Context, key, token string) error
    Renew(ctx context.Context, key, token string, ttl time.Duration) error
}
```

实现要求（由于项目使用的 `redis_client.Client` 接口没有 `SET NX EX`，全部通过 `NewScript/Run` 走 Lua 脚本实现）：

- `Acquire`：使用 Lua 脚本，仅当 key 不存在时才 `setex`。
- `Release`：使用 Lua 脚本，仅当 `get(key) == token` 时才执行 `del`。
- `Renew`：使用 Lua 脚本，仅当 `get(key) == token` 时才执行 `expire`。

```lua
-- acquire.lua
if redis.call("exists", KEYS[1]) == 0 then
    redis.call("setex", KEYS[1], tonumber(ARGV[2]), ARGV[1])
    return 1
else
    return 0
end
```

```lua
-- release.lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("del", KEYS[1])
else
    return 0
end
```

```lua
-- renew.lua
if redis.call("get", KEYS[1]) == ARGV[1] then
    return redis.call("expire", KEYS[1], tonumber(ARGV[2]))
else
    return 0
end
```

如项目已存在类似 `lib/redislock` 或 `lib/xredis` 的封装，应优先复用并扩展其接口，避免重复造轮子。

### 3.3 `QuotaResetScheduler` 加锁改造

在 `model/quota/scheduler.go` 中：

1. 为 `QuotaResetScheduler` 注入 `DistributedLock` 客户端和本实例 token。
2. 在 `resetQuotas` 方法开头尝试获取全局锁。
3. 获取成功后启动看门狗续期，再执行原有重置逻辑。
4. 任务完成或异常退出时停止看门狗并释放锁。

```go
type QuotaResetScheduler struct {
    txn            itxn.TxnStorager
    balanceSyncMgr BalanceSyncer
    lockClient     DistributedLock // 新增
    instanceToken  string          // 新增：本实例唯一 token
    stopCh         chan struct{}
}

func (s *QuotaResetScheduler) resetQuotas() {
    ctx := context.Background()
    lockKey := "quota:reset:scheduler:lock"
    ttl := 5 * time.Minute

    acquired, err := s.lockClient.Acquire(ctx, lockKey, s.instanceToken, ttl)
    if err != nil {
        stateful.AccessLogger.Warn("Failed to acquire quota scheduler lock: %v", err)
        return
    }
    if !acquired {
        stateful.AccessLogger.Info("Quota scheduler lock not acquired, skip")
        return
    }

    // 启动看门狗续期，避免任务耗时超过 TTL 导致锁被抢占
    stopRenew := s.startRenew(ctx, lockKey, ttl)
    defer stopRenew()

    defer func() {
        if err := s.lockClient.Release(ctx, lockKey, s.instanceToken); err != nil {
            stateful.AccessLogger.Warn("Failed to release quota scheduler lock: %v", err)
        }
    }()

    // 原有逻辑
    if err := s.balanceSyncMgr.ResetExpiredBalances(ctx); err != nil {
        stateful.AccessLogger.Error("Failed to reset expired balances: %v", err)
    }
}
```

看门狗实现示例：

```go
func (s *QuotaResetScheduler) startRenew(ctx context.Context, key string, ttl time.Duration) func() {
    done := make(chan struct{})
    go func() {
        ticker := time.NewTicker(ttl / 3)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                if err := s.lockClient.Renew(ctx, key, s.instanceToken, ttl); err != nil {
                    stateful.AccessLogger.Warn("Failed to renew quota scheduler lock: %v", err)
                    return
                }
            case <-done:
                return
            case <-ctx.Done():
                return
            }
        }
    }()
    return func() { close(done) }
}
```

### 3.4 条件更新兜底

在 `model/quota/balance_sync.go:ResetExpiredBalances` 中，更新 `quota_plans.last_reset_at` 时增加条件：

```go
// 原实现
_, err = m.planStorager.UpdateQuotaPlan(ctx, &QuotaPlanFilter{
    ID: plan.ID,
}, &QuotaPlanParam{
    LastResetAt: lib.PTime(now),
})
```

改进为带条件的更新（需要在 DAO / Filter 层支持）：

```sql
UPDATE quota_plans
SET last_reset_at = ?
WHERE id = ?
  AND (last_reset_at IS NULL OR last_reset_at < ?)
```

条件中的第三个 `?` 应为当前周期的开始时间（例如 `weekly` 时取本周一 00:00:00，`monthly` 时取本月 1 日 00:00:00）。这样即使锁失效导致两个实例先后执行，第二个实例也会因 `last_reset_at >= 周期开始时间` 而更新 0 行。

实现上可以在 `QuotaPlanFilter` 中新增 `LastResetAtBefore *time.Time`，并在 `UpdateQuotaPlan` 的 SQL 组装里把它作为 WHERE 条件。

> 注意：该条件依赖于 `shouldResetByPeriod` 与 SQL 条件使用一致的"周期开始时间"计算逻辑，避免时区或边界差异导致误判。
>
> 执行顺序：在锁保护下，对每个 plan 先原子 SET Redis（见 3.6），再条件更新 `last_reset_at`。若实例在 SET Redis 后、更新 DB 前崩溃，下次调度仍会因 `last_reset_at` 未更新而再次触发，Redis 会被幂等地重新 SET。

### 3.5 锁配置建议

| 参数 | 建议值 | 说明 |
|------|--------|------|
| 锁 key | `quota:reset:scheduler:lock` | 全局唯一 |
| 锁 value | `{instance_id}:{uuid}` | 释放/续期时校验身份 |
| TTL | 5~10 分钟 | 覆盖正常重置耗时 |
| 续期周期 | TTL / 3 | 留有足够余量应对网络延迟 |
| 释放方式 | Lua 脚本校验 token | 防止误删他人锁 |
| 兜底机制 | DB 条件更新 | `last_reset_at < 周期开始时间` 才更新 |

### 3.6 Redis 重置原子化

当前 `ResetToQuota` 复用 `SetRemaining`，采用"读取当前值 + 计算差值 + `IncrBy`"的补偿方式：

```go
func (c *redisQuotaCache) SetRemaining(ctx, key, quota, unit) error {
    currentValue, _ := c.client.GetInt64(redisKey)
    targetValue := PtrToRedisValue(quota, unit)
    delta := targetValue - currentValue
    _, err := c.client.IncrBy(redisKey, delta)
    return err
}
```

这在多实例并发重置时存在 race：实例 A 读取到 900 后，用户请求扣减到 800，实例 B 读取到 800 并计算 `delta=200`，最终把值补回 1000。

本期为 `QuotaCache` 新增一个专门用于**清零式重置**的原子方法：

```go
// QuotaCache 接口新增
type QuotaCache interface {
    // ... 原有方法
    ResetToQuotaAtomic(ctx context.Context, key string, quota *float64, unit *string) error
}
```

实现通过 Lua 脚本调用 Redis `SET`（因为 `redis_client.Client` 接口没有裸 `SET` 方法，但支持 `NewScript/Run`）：

```go
const setQuotaScriptSrc = `
redis.call('set', KEYS[1], ARGV[1])
return 1
`

func (c *redisQuotaCache) ResetToQuotaAtomic(ctx context.Context, key string, quota *float64, unit *string) error {
    redisKey := stateful.AIUsedQuotaKey(key)
    targetValue := golibquota.PtrToRedisValue(quota, unit)
    _, err := c.setQuotaScript.Run(redisKey, targetValue)
    return err
}
```

> 注：`setQuotaScript` 在 `NewRedisQuotaCache` 时通过 `client.NewScript` 创建并缓存，避免每次重置都创建 script 对象。

`model/quota/balance_sync.go` 中的 `resetAPIKeysRedisUsage` 改为调用 `ResetToQuotaAtomic`：

```go
for _, apiKey := range apiKeys {
    if apiKey.Key == nil || apiKey.KeyCreateAt == nil {
        continue
    }
    if err := m.quotaCache.ResetToQuotaAtomic(ctx, *apiKey.Key, plan.Quota, plan.Unit); err != nil {
        // 记录日志，继续处理下一个
        continue
    }
}

for _, entity := range entities {
    if entity.EntityID == nil {
        continue
    }
    if err := m.quotaCache.ResetToQuotaAtomic(ctx, *entity.EntityID, plan.Quota, plan.Unit); err != nil {
        continue
    }
}
```

#### 为什么用 SET

- 定期重置的语义就是"把剩余量恢复为 quota 总量"，不需要保留当前值。
- `SET` 是原子操作，不存在`GetInt64 + IncrBy(delta)` 内部"读-算-写"的 race。
- 与 `ApplyQuotaPlanChange` 的 `SetRemaining`（`IncrBy(delta)`）使用场景不同，后者用于"保留 used 的差额调整"，不应改为 `SET`。

#### 与锁和条件更新的配合

| 层级 | 作用 |
|------|------|
| 分布式锁 | 保证同一时刻只有一个实例进入 `ResetExpiredBalances`，避免多个实例同时遍历和写 Redis / DB。 |
| Redis 原子 SET | 消除单条 Redis 写操作内部的 race；相比 `IncrBy(delta)`，不会因为并发读到的中间值而错误地"补回"配额。 |
| DB 条件更新 | 锁正常工作时，保证同一周期内同一 plan 只被重置一次；锁异常失效时，作为最后一道防线避免重复更新 `last_reset_at`。 |

#### 剩余风险：崩溃在"SET Redis"与"更新 DB"之间

上述三层防护在**锁正常持有且实例不崩溃**时，可以完美保证每个 plan 每个周期只重置一次。但仍有一个极端窗口：

```text
T1: 实例 A 获取锁
T2: A 对 plan X 执行 ResetToQuotaAtomic（Redis 被 SET 为 quota）
T3: 用户请求消耗 100，Redis 剩余量变为 quota - 100
T4: A 在更新 quota_plans.last_reset_at 前崩溃
T5: 锁 TTL 到期，实例 B 获取锁
T6: B 执行 ResetExpiredBalances，发现 plan X 的 last_reset_at 仍是旧值
T7: B 再次对 plan X 执行 ResetToQuotaAtomic（Redis 被重新 SET 为 quota）
T8: T3 时刻用户消耗的 100 被抹掉
```

也就是说：**原子 `SET` 解决的是单条 Redis 写操作内的 race，但不能解决"一个实例已经 SET 过、另一个实例又来 SET"的跨实例重复重置问题。** 只要 Redis SET 和 DB 更新不能跨系统原子，这个窗口就存在。

#### 可选增强方案：Redis 周期标记（marker）+ Lua

如果业务对上述极端窗口零容忍，可以在 Redis 中引入"本周期已重置"标记，并通过 Lua 脚本把"检查标记"和"SET 剩余量"做成原子操作：

```lua
-- reset_quota.lua
-- KEYS[1]: marker key, KEYS[2]: remaining key
-- ARGV[1]: cycle_start, ARGV[2]: quota, ARGV[3]: marker_ttl
local currentCycle = redis.call("get", KEYS[1])
if currentCycle ~= false and currentCycle == ARGV[1] then
    return 0  -- 本周期已重置，跳过
end
redis.call("set", KEYS[2], ARGV[2])
redis.call("set", KEYS[1], ARGV[1])
redis.call("expire", KEYS[1], ARGV[3])
return 1  -- 本实例执行了重置
```

配合的 Go 接口：

```go
type QuotaCache interface {
    // ... 原有方法
    ResetToQuotaIfNeeded(ctx context.Context, key string, quota *float64, unit *string, cycleStart string) (bool, error)
}
```

`ResetExpiredBalances` 流程变为：

```go
for _, plan := range plans {
    if shouldReset {
        cycleStart := getCycleStart(now, *plan.ResetPeriod)
        performed := false
        for _, apiKey := range apiKeys {
            done, err := m.quotaCache.ResetToQuotaIfNeeded(ctx, *apiKey.Key, plan.Quota, plan.Unit, cycleStart)
            if err == nil && done { performed = true }
        }
        for _, entity := range entities {
            done, err := m.quotaCache.ResetToQuotaIfNeeded(ctx, *entity.EntityID, plan.Quota, plan.Unit, cycleStart)
            if err == nil && done { performed = true }
        }
        // 无论 performed 是 true（本实例重置）还是 false（A 已重置但 DB 未更新），
        // 都尝试条件更新 DB last_reset_at，确保长期标记一致。
        _, _ = m.planStorager.UpdateQuotaPlan(ctx,
            &QuotaPlanFilter{ID: plan.ID, LastResetAtBefore: &cycleStart},
            &QuotaPlanParam{LastResetAt: lib.PTime(now)})
    }
}
```

这样即使 A 在 SET Redis 后、更新 DB 前崩溃，B 看到 marker 已存在也不会再次 SET Redis，只会把 DB 的 `last_reset_at` 补上，T3 时刻的消耗被保留。

**引入该方案的代价**：

- 新增 marker key（如 `QUOTA_<key>:reset_marker`），需要管理过期时间。
- Lua 脚本需要访问两个 key；在 Redis Cluster 模式下，需要确保 marker key 与 remaining key 落在同一 slot（例如通过 hash tag `QUOTA_{<key>}` 和 `QUOTA_{<key>}:reset_marker`），否则 Lua 脚本无法保证原子执行。
- 如需修改现有 key 的 slot 计算方式，会涉及数据迁移或数据面兼容性问题。

**本期建议**：

- **默认采用"分布式锁 + 原子 SET + DB 条件更新"组合**，实现简单，能覆盖绝大多数生产场景。
- 在文档和风险表中明确记录"实例崩溃在 SET 与 DB 更新之间"这一小概率窗口。
- 如果后续业务验证该窗口不可接受，再引入 marker + Lua 的增强方案。

---

## 4. 涉及文件清单

| 文件 | 修改内容 |
|------|----------|
| `model/quotacache/lock.go`（新增） | 定义 `DistributedLock` 接口；基于 Redis Lua 脚本实现 `Acquire`/`Release`/`Renew`。 |
| `model/quotacache/quotacache.go` | `QuotaCache` 接口新增 `ResetToQuotaAtomic`。 |
| `model/quotacache/redis.go` | 实现 `ResetToQuotaAtomic`（Lua `SET`）；缓存 script 对象。 |
| `model/quota/scheduler.go` | 注入锁客户端和实例 token；`resetQuotas` 加锁、看门狗续期、执行原逻辑、释放锁。 |
| `model/quota/balance_sync.go` | `resetAPIKeysRedisUsage` 改用 `ResetToQuotaAtomic`；`ResetExpiredBalances` 更新 `quota_plans.last_reset_at` 时增加 `last_reset_at < 周期开始时间` 条件。 |
| `model/quota/quota_plan.go` | `QuotaPlanFilter` 新增 `LastResetAtBefore`。 |
| `storage/rdb/quota/quota_plan.go` | `quotaPlanFilterToParam` 映射 `LastResetAtBefore`。 |
| `storage/rdb/internal/dao/table_quota_plans.go` | `TQuotaPlanParam` 新增 `LastResetAtBefore`（`db:"last_reset_at,<"`）。 |
| `stateful/mock_redis.go` | 增强 `mockRedisScript.Run`，支持本期用到的 Lua 脚本（set、exists/setex、get/del、get/expire）。 |
| `stateful/container/rdb/components.go` | 初始化 `QuotaResetScheduler` 时传入锁客户端和实例 token。 |
| `model/quotacache/lock_test.go`（新增） | 锁的获取、释放、续期、token 隔离测试。 |
| `model/quotacache/redis_test.go`（新增） | `ResetToQuotaAtomic` 覆盖、nil 处理测试。 |
| `model/quota/scheduler_test.go` | 新增测试：锁未获取时跳过、锁获取后执行、异常时释放锁、看门狗续期行为。 |
| `model/quota/balance_sync_test.go` | 新增测试：条件更新 filter 携带 `LastResetAtBefore`。 |
| `model/api_key/mocks_test.go`、`model/entity/mocks_test.go` | `fakeQuotaCache` 补充 `ResetToQuotaAtomic` 方法。 |

---

## 5. 测试计划

### 5.1 单元测试

#### `model/quotacache` 锁客户端

1. **Acquire 成功**：key 不存在时 Lua 脚本通过 `setex` 创建并返回 `1`。
2. **Acquire 失败**：key 已存在且未过期时返回 `0`。
3. **Release 成功**：持有当前 token 时删除 key。
4. **Release 安全**：key 被其他 token 持有时不删除。
5. **Renew 成功**：持有当前 token 时重置过期时间。
6. **Renew 安全**：key 被其他 token 持有时不续期。

#### `model/quota/scheduler.go`

1. **未获取锁则跳过**：mock `Acquire` 返回 `false`，断言 `ResetExpiredBalances` 未被调用。
2. **获取锁后执行**：mock `Acquire` 返回 `true`，断言 `ResetExpiredBalances` 被调用，且任务完成后调用 `Release`。
3. **异常时释放锁**：任务 panic 时，defer 中仍调用 `Release`。
4. **看门狗续期**：任务执行时间跨越多个续期周期，断言 `Renew` 被调用。

#### `model/quotacache/redis.go`

1. **`ResetToQuotaAtomic` 成功覆盖**：调用后 Redis key 值变为 `PtrToRedisValue(quota, unit)`，且只发起一次 `SET`。
2. **`ResetToQuotaAtomic` 不依赖当前值**：预先写入一个较小值，再调用 `ResetToQuotaAtomic`，结果应直接为 quota，不会出现 delta 补偿错误。
3. **`ResetToQuotaAtomic` 与 `SetRemaining` 独立**：确认 `ApplyQuotaPlanChange` 等差额调整场景仍调用 `SetRemaining`（`IncrBy`），而定期重置场景调用 `ResetToQuotaAtomic`（`SET`）。

#### `model/quota/balance_sync.go`

1. **同一周期内不重复重置**：构造 `quota_plans.last_reset_at = 本周一 00:00:05` 的 plan，reset_period 为 `weekly`；调用 `ResetExpiredBalances` 时，`UpdateQuotaPlan` 应返回 0 行受影响，且不调用 Redis 重置。
2. **跨周期正常重置**：`quota_plans.last_reset_at = 上周日 23:59:59`，应调用 `ResetToQuotaAtomic` 重置 Redis，并更新 `last_reset_at`。
3. **先 Redis 后 DB 顺序**：mock `UpdateQuotaPlan` 返回错误，断言 Redis 已被 SET，且函数不因 DB 更新失败而回滚 Redis。

### 5.2 集成测试

1. **多实例只有一个执行**：启动两个 `ai-gateway-api` 实例，等待 2~3 个调度周期，断言同一分钟内只有一个实例的日志打印"Starting quota scheduler tasks"。
2. **锁持有者崩溃后替补**：实例 A 获取锁后手动 kill，实例 B 在锁 TTL 到期后的下一个调度周期成功获取锁并继续执行。

---

## 6. 风险与注意事项

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| 锁实现引入新的 Redis 依赖 | 如果项目已有通用 Redis 锁，应复用；否则需新增实现并测试 | 优先复用已有封装 |
| 看门狗 goroutine 泄漏 | panic 或任务退出时未正确停止续期 goroutine | 使用 `defer stopRenew()` 保证停止 |
| 任务执行时间超过 TTL | 看门狗续期失败或停止后锁过期 | TTL 设为平均耗时的 5~10 倍；配合 DB 条件更新兜底 |
| 条件更新与 `shouldResetByPeriod` 不一致 | 两者对"周期开始时间"计算不同可能导致漏重置或重复重置 | 统一使用同一工具函数计算周期开始时间 |
| Redis 与 DB 时钟不同步 | `last_reset_at` 使用 DB `now()`，而调度器使用实例本地时间 | 建议统一使用数据库时间或 NTP 校准 |
| 锁 key 与其他环境冲突 | 多个环境共用同一 Redis 时 key 可能冲突 | 可在 key 前增加环境前缀，如 `{env}:quota:reset:scheduler:lock` |
| `ResetToQuotaAtomic` 被误用到差额调整场景 | `ApplyQuotaPlanChange` 依赖 `SetRemaining` 的 `IncrBy(delta)` 保留 used，若误用 `SET` 会丢失使用量 | 仅允许 `resetAPIKeysRedisUsage` 调用 `ResetToQuotaAtomic`；`ApplyQuotaPlanChange` 继续调用 `SetRemaining` |
| 崩溃在 SET Redis 与更新 DB 之间 | 持有锁实例 A 重置 Redis 后、更新 `last_reset_at` 前崩溃；实例 B 获得锁后因 DB 仍是旧值而再次重置 Redis，可能抹掉 A 重置后产生的合法消耗 | 该窗口概率较低；如业务零容忍，引入 Redis marker + Lua 原子检查（见 3.6 可选增强方案） |

---

## 7. 后续可优化（本期不做）

- **细粒度锁**：未来 `quota_plan` 数量增多后，将全局锁拆分为按 `quota_plan_id` 的细粒度锁 `quota:reset:plan:{plan_id}:lock`。
- **调度器 jitter**：引入首次执行随机 jitter，降低多个实例同时竞争锁的概率。
- **监控与告警**：为锁获取失败、任务耗时、重置失败 plan 数等指标增加监控。
- **分离调度与执行**：长期可将"到期判断"和"实际重置"拆分为独立 worker 或 leader 选举机制。
