# quota-plan 周期重置集成测试设计文档

## 1. 模块概述

quota-plan 周期重置模块负责在配额计划（`quota_plan`）跨越自然周/自然月边界时，自动将 API-Key / Entity 在 Redis 中的剩余配额重置为 `quota` 总量，并更新 `quota_plans.last_reset_at`。本期实现引入 Redis 分布式锁、Redis 原子 `SET` 以及 `last_reset_at` 条件更新，保证多实例部署下的幂等性。

由于真实调度周期为 1 分钟，集成测试通过新增的内部接口 `/inner-api/v1/quota/trigger-reset` 确定性触发调度任务，避免长时间等待。多实例场景通过 `testutil.StartServerWithSharedInfra` 让两个 `ai-gateway-api` 子进程共享同一个 miniredis 与同一个 SQLite 数据库文件，模拟生产环境多实例竞争分布式锁的行为。

## 2. 接口列表

| 编号 | 接口名称 | 方法 | 路径 | 说明 |
|------|----------|------|------|------|
| QR-1 | 触发周期重置 | POST | `/inner-api/v1/quota/trigger-reset` | 手动触发一次带分布式锁保护的配额周期重置任务 |
| QR-2 | 创建 API-Key | POST | `/open-api/v1/api-keys` | 创建测试用的 API-Key |
| QR-3 | 更新 API-Key 配额计划 | PATCH | `/open-api/v1/api-keys/{id}` | 为 API-Key 设置非无限配额计划 |
| QR-4 | 创建 Entity | POST | `/open-api/v1/entities` | 创建测试用的 Entity |
| QR-5 | 更新 Entity 配额计划 | PATCH | `/open-api/v1/entities/{id}` | 为 Entity 设置非无限配额计划 |

> 注：QR-2 ~ QR-5 为测试辅助接口，用于准备被测数据；核心断言通过直接读写 miniredis / SQLite 完成。

## 3. 测试用例统计

| 场景 | 测试用例数 |
|------|-----------|
| 跨周期自动重置（API-Key total_token） | 1 |
| 跨周期自动重置（Entity RMB 精度） | 1 |
| 同周期内不重复重置 | 1 |
| 已重置后再次触发保持幂等 | 1 |
| 多实例共享 Redis 时只有一个实例完成重置 | 1 |
| 锁释放后另一实例可再次获取锁（幂等） | 1 |
| **合计** | **6** |

## 4. 认证方式

测试环境配置 `SkipTokenValidate=true`，所有请求无需携带认证头。

## 5. 目录结构

```
quota_period_reset/
├── design.md
└── quota_period_reset_test.go
```

## 6. 触发周期重置

### 6.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | quota-plan 周期重置 |
| 接口名称 | 触发周期重置 |
| 方法 | POST |
| 路径 | `/inner-api/v1/quota/trigger-reset` |
| 说明 | 手动触发一次带 Redis 分布式锁保护的 `BalanceSyncManager.ResetExpiredBalances()`，不影响调度器下次定时执行 |

### 6.2 接口参数说明

#### 6.2.1 请求参数

无 Body 参数。

#### 6.2.2 返回数据字段

| 参数名 | 类型 | 说明 |
|--------|------|------|
| status | string | 固定返回 `"ok"` |

### 6.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| QR-1-001 | 跨周期 API-Key 月度 total_token 配额自动重置 | 正常流程 | 将 `last_reset_at` 拨到上月，触发重置后 Redis 恢复为 quota，`last_reset_at` 更新到当前月 |
| QR-1-002 | 跨周期 Entity 月度 RMB 配额自动重置精度 | 正常流程 / 精度 | 将 `last_reset_at` 拨到上月，触发重置后 Redis 恢复为 RMB quota，精度误差 ≤ 1e-5 |
| QR-1-003 | 同周期内不重复重置 | 幂等性 | `last_reset_at` 保持在当前月，触发重置后 Redis 与 `last_reset_at` 均不变 |
| QR-1-004 | 已重置后再次触发保持幂等 | 幂等性 | 首次重置后再调低 Redis，再次触发同一周期内不应二次重置 |
| QR-2-001 | 多实例共享 Redis 时只有一个实例实际完成重置 | 并发/锁 | 两个 ai-gateway-api 实例共享 Redis 与 DB，并发触发重置，结果一致且无数据损坏 |
| QR-2-002 | 锁释放后另一实例可再次获取锁并执行（幂等） | 并发/锁 | 首次重置后从另一实例再次触发，因 `last_reset_at` 已更新，不应二次重置 |

### 6.4 测试场景详细设计

#### 6.4.1 QR-1-001：跨周期 API-Key 月度 total_token 配额自动重置

##### 设计思路

验证当 API-Key 的 `quota_plan` 跨越自然月边界时，调度器会原子重置其 Redis 剩余量为 quota 总量，并更新 `quota_plans.last_reset_at`。

##### 前提数据准备

1. 创建 API-Key A，不指定 entity_id。
2. 通过 PATCH 为其配置 `quota_plan`：
   - `unlimited = false`
   - `quota = 1000000`
   - `unit = "total_token"`
   - `reset_period = "monthly"`
3. 通过 `ServerManager.SetQuotaRemaining(apiKeyAValue, 100000, "total_token")` 将 Redis 剩余量调低，模拟已消耗 900000。
4. 通过 `ServerManager.UpdateQuotaPlanLastResetAt(apiKeyAID, "api_key", previousMonthStart)` 将 `last_reset_at` 拨到上个月 1 日 00:00:00。

##### 执行步骤

1. 记录触发前 Redis 剩余量与 `last_reset_at`。
2. 发送 POST 请求到 `/inner-api/v1/quota/trigger-reset`。
3. 验证响应 `ErrNum = 200`。
4. 读取 Redis 剩余量，断言等于 `1000000`。
5. 读取 `quota_plans.last_reset_at`，断言落在当前自然月。

##### 请求参数

```json
{}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| status | "ok" | Equals |

**Redis / DB 校验**：

| 校验项 | 预期值 | 校验方式 |
|--------|--------|---------|
| API-Key A Redis 剩余量 | 1000000 | Equals |
| API-Key A `last_reset_at` | 当前自然月内 | After currentMonthStart |

---

#### 6.4.2 QR-1-002：跨周期 Entity 月度 RMB 配额自动重置精度

##### 设计思路

验证 Entity 的 RMB 配额在周期重置时，Redis 值能按 1e-8 元定点精度正确恢复为 quota 总量，且 `last_reset_at` 更新。

##### 前提数据准备

1. 创建 Entity-Type T（level=1）。
2. 创建 Entity B，类型为 T，不指定 parent_id。
3. 通过 PATCH 为其配置 `quota_plan`：
   - `unlimited = false`
   - `quota = 100.5`
   - `unit = "RMB"`
   - `reset_period = "monthly"`
4. 通过 `ServerManager.SetQuotaRemaining(entityID, 10.5, "RMB")` 将 Redis 剩余量调低。
5. 通过 `ServerManager.UpdateQuotaPlanLastResetAt(entityID, "entity", previousMonthStart)` 将 `last_reset_at` 拨到上个月。

##### 执行步骤

1. 记录触发前 Redis 剩余量与 `last_reset_at`。
2. 发送 POST 请求到 `/inner-api/v1/quota/trigger-reset`。
3. 验证响应 `ErrNum = 200`。
4. 读取 Redis 剩余量，断言在 `100.5 ± 1e-5` 范围内。
5. 读取 `quota_plans.last_reset_at`，断言落在当前自然月。

##### 请求参数

```json
{}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| status | "ok" | Equals |

**Redis / DB 校验**：

| 校验项 | 预期值 | 校验方式 |
|--------|--------|---------|
| Entity B Redis 剩余量 | 100.5 | InDelta(±1e-5) |
| Entity B `last_reset_at` | 当前自然月内 | After currentMonthStart |

---

#### 6.4.3 QR-1-003：同周期内不重复重置

##### 设计思路

验证当 `last_reset_at` 已处于当前自然月时，即使 Redis 剩余量较低，调度器也不会再次重置，从而保证同一周期内的幂等性。

##### 前提数据准备

1. 创建 API-Key C。
2. 通过 PATCH 为其配置 `quota_plan`：
   - `unlimited = false`
   - `quota = 500000`
   - `unit = "total_token"`
   - `reset_period = "monthly"`
3. 通过 `ServerManager.SetQuotaRemaining(apiKeyCValue, 100000, "total_token")` 将 Redis 剩余量调低。
4. 通过 `ServerManager.UpdateQuotaPlanLastResetAt(apiKeyCID, "api_key", currentMonthMid)` 将 `last_reset_at` 保持在当前自然月（例如本月 15 日 12:00:00）。

##### 执行步骤

1. 记录触发前 Redis 剩余量与 `last_reset_at`。
2. 发送 POST 请求到 `/inner-api/v1/quota/trigger-reset`。
3. 验证响应 `ErrNum = 200`。
4. 读取 Redis 剩余量，断言仍为 `100000`。
5. 读取 `quota_plans.last_reset_at`，断言与触发前一致（未变化）。

##### 请求参数

```json
{}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| status | "ok" | Equals |

**Redis / DB 校验**：

| 校验项 | 预期值 | 校验方式 |
|--------|--------|---------|
| API-Key C Redis 剩余量 | 100000 | Equals |
| API-Key C `last_reset_at` | 与触发前一致 | WithinDuration |

---

#### 6.4.4 QR-1-004：已重置后再次触发保持幂等

##### 设计思路

在 QR-1-001 已完成跨周期重置的基础上，再次调低 Redis 剩余量并触发重置，验证同一周期内不会二次重置，进一步确认 `last_reset_at` 条件更新的幂等效果。

##### 前提数据准备

1. 复用 QR-1-001 中已重置的 API-Key A。
2. 确认其 `last_reset_at` 已更新到当前自然月。
3. 通过 `ServerManager.SetQuotaRemaining(apiKeyAValue, 200000, "total_token")` 再次调低 Redis 剩余量。

##### 执行步骤

1. 记录再次触发前 Redis 剩余量（应为 200000）。
2. 发送 POST 请求到 `/inner-api/v1/quota/trigger-reset`。
3. 验证响应 `ErrNum = 200`。
4. 读取 Redis 剩余量，断言仍为 `200000`，未恢复为 quota。
5. 读取 `quota_plans.last_reset_at`，断言未变化。

##### 请求参数

```json
{}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| status | "ok" | Equals |

**Redis / DB 校验**：

| 校验项 | 预期值 | 校验方式 |
|--------|--------|---------|
| API-Key A Redis 剩余量 | 200000 | Equals |
| API-Key A `last_reset_at` | 与 QR-1-001 重置后一致 | WithinDuration |

---

## 7. 多实例分布式锁

### 7.1 测试目标

验证 `ai-gateway-api` 多实例部署时，`QuotaResetScheduler` 通过 Redis 分布式锁实现互斥：同一时刻只有一个实例执行 `ResetExpiredBalances()`；锁释放后，其他实例可继续获取锁并执行（此时因 `last_reset_at` 已更新而幂等跳过）。

### 7.2 共享基础设施

通过 `testutil.StartServerWithSharedInfra(sharedRedis, sharedDBPath)` 启动第二个实例：

- 第一个实例 A：创建独立的 miniredis 与 SQLite DB；
- 第二个实例 B：复用实例 A 的 miniredis 与 DB 文件路径；
- 两个实例监听不同端口，使用不同临时配置目录，但访问同一份 Redis 与 DB。

### 7.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| QR-2-001 | 多实例共享 Redis 时只有一个实例实际完成重置 | 并发/锁 | 并发触发 A/B 的 `/inner-api/v1/quota/trigger-reset`，验证 Redis 与 DB 最终状态一致且正确 |
| QR-2-002 | 锁释放后另一实例可再次获取锁并执行（幂等） | 并发/锁 | 首次重置后由 B 再次触发，验证 B 能获取锁但因 `last_reset_at` 已更新而不二次重置 |

### 7.4 测试场景详细设计

#### 7.4.1 QR-2-001：多实例共享 Redis 时只有一个实例实际完成重置

##### 设计思路

启动两个共享 Redis 与 DB 的 `ai-gateway-api` 实例，构造需要跨周期重置的 quota_plan，并发触发两个实例的 `/inner-api/v1/quota/trigger-reset`。由于分布式锁的存在，只有一个实例会实际执行重置；另一个实例会跳过。最终验证 Redis 剩余量被重置为 quota，`last_reset_at` 更新到当前月。

##### 前提数据准备

1. 启动实例 A（`StartServerWithSharedInfra(nil, "")`）。
2. 启动实例 B（`StartServerWithSharedInfra(smA.Redis, smA.DBPath)`）。
3. 将全局 HTTP 客户端指向实例 A 的 URL。
4. 创建 API-Key M，配置 `quota_plan`：
   - `unlimited = false`
   - `quota = 800000`
   - `unit = "total_token"`
   - `reset_period = "monthly"`
5. 通过 `ServerManager.SetQuotaRemaining(apiKeyMValue, 50000, "total_token")` 将 Redis 剩余量调低。
6. 通过 `ServerManager.UpdateQuotaPlanLastResetAt(apiKeyMID, "api_key", previousMonthStart)` 将 `last_reset_at` 拨到上个月。

##### 执行步骤

1. 启动两个 goroutine，分别向实例 A 和实例 B 的 `/inner-api/v1/quota/trigger-reset` 发送 POST 请求。
2. 等待两个请求均返回。
3. 验证两个响应的 HTTP 状态码均为 200（无论是否实际执行，触发接口都返回成功）。
4. 读取 Redis 剩余量，断言等于 `800000`。
5. 读取 `quota_plans.last_reset_at`，断言落在当前自然月。

##### 请求参数

```json
{}
```

##### 预期返回结果

**实例 A HTTP Status**：200  
**实例 B HTTP Status**：200

**Redis / DB 校验**：

| 校验项 | 预期值 | 校验方式 |
|--------|--------|---------|
| API-Key M Redis 剩余量 | 800000 | Equals |
| API-Key M `last_reset_at` | 当前自然月内 | After currentMonthStart |

---

#### 7.4.2 QR-2-002：锁释放后另一实例可再次获取锁并执行（幂等）

##### 设计思路

在 QR-2-001 已完成重置的基础上，由实例 B 再次触发重置。由于 `last_reset_at` 已更新到当前月，实例 B 即使获取到锁，也不会再次重置 Redis；该用例同时验证锁已被释放，另一实例能够成功获取锁。

##### 前提数据准备

1. 复用 QR-2-001 中已重置的 API-Key M。
2. 确认其 `last_reset_at` 已更新到当前自然月。
3. 通过 `ServerManager.SetQuotaRemaining(apiKeyMValue, 100000, "total_token")` 再次调低 Redis 剩余量。

##### 执行步骤

1. 向实例 B 的 `/inner-api/v1/quota/trigger-reset` 发送 POST 请求。
2. 验证响应 HTTP 状态码为 200。
3. 读取 Redis 剩余量，断言仍为 `100000`，未恢复为 quota。
4. 读取 `quota_plans.last_reset_at`，断言未变化。

##### 请求参数

```json
{}
```

##### 预期返回结果

**实例 B HTTP Status**：200

**Redis / DB 校验**：

| 校验项 | 预期值 | 校验方式 |
|--------|--------|---------|
| API-Key M Redis 剩余量 | 100000 | Equals |
| API-Key M `last_reset_at` | 与 QR-2-001 重置后一致 | WithinDuration |

---

## 8. 依赖与数据准备

1. 测试二进制 `ai-gateway-api.exe` 已由 `make build` 生成。
2. `testutil.StartServer()` 启动完整服务，初始化 SQLite 与 miniredis。
3. 多实例场景使用 `testutil.StartServerWithSharedInfra(sharedRedis, sharedDBPath)` 启动后续实例，使其与第一个实例共享同一个 miniredis 和同一个 SQLite 数据库文件：
   - 实例 A：`smA, _ := StartServerWithSharedInfra(nil, "")`，创建独立的 Redis 与 DB；
   - 实例 B：`smB, _ := StartServerWithSharedInfra(smA.Redis, smA.DBPath)`，复用实例 A 的 Redis 与 DB。
4. 测试前需创建 Entity-Type，再创建 Entity；API-Key 可直接创建。
5. 通过 `ServerManager.UpdateQuotaPlanLastResetAt` 直接写 SQLite，模拟跨周期/同周期场景。
6. 通过 `ServerManager.SetQuotaRemaining` / `GetQuotaRemaining` 直接读写 miniredis。

## 9. 注意事项

1. `/inner-api/v1/quota/trigger-reset` 仅供测试与运维排障，不走 OpenAPI 鉴权（受 `McUserProbe` 中间件保护，但测试环境 `SkipTokenValidate=true`）。
2. 触发接口复用生产调度器的分布式锁路径，因此会真实尝试获取 Redis 锁；单实例测试下总能获取锁。
3. 每个测试包使用独立进程与独立 miniredis，测试间互不干扰。
4. 测试用例共享一次服务启动；QR-1-004 依赖 QR-1-001 的副作用，需按顺序放在同一个 `t.Run` 链中执行。
5. `last_reset_at` 的周期判断基于服务器本地时区，测试构造的 `previousMonthStart` / `currentMonthMid` 需使用相同本地时区。
6. 多实例共享基础设施时，实例 A 创建的 miniredis 与 SQLite 文件由实例 A 的 `Shutdown()` 统一关闭或删除；后续实例在 `Shutdown()` 中不会重复释放共享资源。
7. 为避免同一测试进程内多实例复制二进制时触发 Windows 文件锁冲突，`testutil` 在复制的临时二进制文件名中追加 `time.Now().UnixNano()`，确保文件名唯一。
