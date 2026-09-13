# quota-plan 定期重置多实例并发风险修复变更摘要

## 1. 背景

`ai-gateway-api` 支持对 `quota_plan` 设置按自然周（`weekly`）或自然月（`monthly`）自动重置配额余额。该逻辑由进程内的 `QuotaResetScheduler` 每分钟驱动，调用 `BalanceSyncManager.ResetExpiredBalances`：

- 从 `quota_plans` 读取 `last_reset_at`；
- 判断 plan 是否跨越周期边界；
- 若满足条件，则重置该 plan 下所有 API-Key / Entity 的 Redis 剩余量，并更新 `quota_plans.last_reset_at`。

在典型生产部署中，`ai-gateway-api` 会启动 3 个实例，且每个实例都会独立启动自己的调度器。由于当前没有任何互斥机制，多个实例可能并发执行同一重置任务，存在 Redis 配额被重复覆盖、`quota_plans.last_reset_at` 更新竞争等风险。

## 2. 目标

- 保证同一时刻只有一个 `ai-gateway-api` 实例执行 `ResetExpiredBalances`。
- 把定期重置的 Redis 写操作改为原子 `SET`，消除 `GetInt64 + IncrBy(delta)` 单条写操作内的 race；对于跨实例重复重置问题，通过分布式锁和 DB 条件更新兜底。
- 即使持有锁的实例崩溃，其他实例也能在锁 TTL 到期后替补继续执行。
- 在分布式锁之外，通过数据库条件更新实现最后一道幂等防线，避免同一周期内对同一 plan 重复重置。
- 尽量减小改动范围，不影响现有接口契约、数据模型和导出格式。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要模块 | `model/quota`、`model/quotacache`、`stateful/container/rdb` |
| 涉及接口 | 无 OpenAPI / InnerAPI 变更，纯后台调度逻辑 |
| 接口契约 | 不变 |
| 数据迁移 | 无 |

## 4. 关键决策

| 决策 | 说明 |
|------|------|
| 采用 Redis 分布式锁 | 复用现有 Redis 客户端，通过 `SET key value NX EX <ttl>` 实现全局互斥，改动集中在 `QuotaResetScheduler` 层。 |
| 锁带 TTL + 看门狗续期 | 锁设置 5~10 分钟 TTL，任务执行期间由看门狗每 TTL/3 续期，既保证故障转移，又避免任务耗时超过 TTL 导致锁被抢占。 |
| 释放锁使用 Lua 校验 | 通过比对锁 value 是否为本实例 token，防止原实例复活后误释放其他实例持有的锁。 |
| Redis 写入原子化 | 定期重置时直接使用 Redis `SET` 覆盖为 quota 总量，替代 `GetInt64 + IncrBy(delta)`，避免单条 Redis 写内部的 read-modify-write race。 |
| 数据库条件更新兜底 | 在 `ResetExpiredBalances` 更新 `quota_plans.last_reset_at` 时增加 `last_reset_at < 周期开始时间` 条件，作为锁失效后的最后一道幂等防线。 |
| 锁粒度先全局后细化 | 本期使用全局锁 `quota:reset:scheduler:lock`；未来 plan 数量增多后可按 `quota:reset:plan:{plan_id}:lock` 拆分为细粒度锁。 |

## 5. 关联文档

- 风险分析来源：`document-ai-gateway/迭代系统设计/v0.6/quota-plan定期reset的风险调研/分析报告.md`
- 详细设计：`design-changes.md`
