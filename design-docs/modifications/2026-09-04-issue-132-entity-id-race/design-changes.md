# Issue #132：Entity ID 生成竞态与复用——设计变更说明

## 1. 当前问题定位

### 1.1 竞态与复用发生点

```text
endpoints/openapi_v1/entity/create.go:127
    generateEntityID(ctx)
        └── EntityManager.FetchEntityList(ctx, &EntityFilter{})  // 全表拉取
            └── 逐条 populateAssociatedData（quota_plan / rate_limit / route_rules 联表）
                └── 扫描 max(entity-N) → 返回 entity-{max+1}
```

- 该函数在事务外执行，两个并发请求可能同时读到相同的 `max`，生成同一个 ID；
- 删除最大编号 Entity 后 `max` 回落，新实体复用已删除的 `entity-N`（ABA 问题）；
- `operation_logs`、下游 BFE/conf-agent 导出配置中残留的旧 `entity-N` 引用指向新资源，产生歧义。

### 1.2 现有防线及缺口

| 防线 | 现状 | 缺口 |
|------|------|------|
| `uk_entity_id` 唯一索引 | 存在（`db_ddl.sql` / `db_ddl_sqlite.sql`） | 并发冲突时第二个请求拿到原始 DB 错误（未翻译为友好业务错误），表现为 500 |
| 应用层查重 | endpoint 层按 type+name 查重 | 不覆盖并发同 ID；无锁、无 CAS |
| 删除清理 | `DeleteEntity` 事务内删 quota_plan / rate_limit_policy / route_rules，提交后清 Redis 配额与限流 key | operation_logs 按 ID 引用但不清理；ABA 场景下旧日志指向新资源 |

### 1.3 附带性能问题

`FetchEntityList` 每次都会 `populateAssociatedData` 逐条联表查询（`entity_manager.go` FetchEntityList 内部），即每创建一个 Entity，ID 生成的代价是全表 + 3N 次查询。实体数量增长后不可接受。修复 ID 生成后此路径不再被创建流程使用。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| **A. 数据库原子序列表（推荐）** | 新增 `entity_id_seq`，CAS 原子分配序号 | 保留 `entity-N` 格式；序号永久消耗不复用；与 Issue #80 修复同一模式，已有落地经验 | 需新增表/DAO；需一次性迁移 | **采用** |
| B. 复用自增主键 `id` | 插入拿 AUTO_INCREMENT 主键，同一事务回写 `entity_id = entity-{id}` | 不新增表 | 历史 `entity-N` 与主键 id 不对应；删除后新主键可能小于历史最大 N，需偏移维护与冲突重试；SQLite 下 `AUTOINCREMENT` 语义差异 | 未采用 |
| C. UUID/ULID | 移除序号生成，模型层直接生成 UUID | 改动最小，无竞态 | 改变可读格式；前端/文档/下游需同步 | 未采用（issue 备选，格式要求排除） |

## 3. 推荐方案：数据库原子序列表

### 3.1 新增表结构

#### MySQL（`db_ddl.sql`）

```sql
CREATE TABLE IF NOT EXISTS `entity_id_seq` (
  `name`     varchar(32) NOT NULL,
  `next_seq` bigint      NOT NULL DEFAULT 1,
  PRIMARY KEY (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Entity ID 序号分配表';
```

#### SQLite（`db_ddl_sqlite.sql`）

```sql
CREATE TABLE IF NOT EXISTS `entity_id_seq` (
  `name`     varchar(32) NOT NULL PRIMARY KEY,
  `next_seq` bigint      NOT NULL DEFAULT 1
);
```

> 说明：与 `api_key_id_seq` 按 `product_name` 分桶不同，Entity 序号是全局的，固定一行 `name = 'entity'`。预留 `name` 主键是为了表结构可复用（未来其他域可共表）。

### 3.2 序号分配语义

- `next_seq` 表示下一个可用序号；分配即消耗，事务回滚也不退回（满足验收标准 3）。
- 分配流程（与 `TAPIKeyIDSeqAllocate` 相同的最终实现，即 Issue #99 改进版）：
  1. 短事务内：
     - **MySQL**：`INSERT INTO entity_id_seq (name, next_seq) VALUES (?, LAST_INSERT_ID(2)) ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1)`，随后 `SELECT LAST_INSERT_ID()` 读回新值；分配结果 = 读回值 - 1。单条语句原子完成初始化或自增，避免 CAS 重试在 REPEATABLE READ 下的锁等待（Issue #99）。
     - **SQLite**：`INSERT OR IGNORE` 确保行存在，`UPDATE ... SET next_seq = next_seq + 1`，再读回 `next_seq`；分配结果 = 读回值 - 1。
  2. 生成的 ID：`fmt.Sprintf("entity-%d", seq)`。

### 3.3 DAO 层设计

新增 `storage/rdb/internal/dao/table_entity_id_seq.go`：

```go
const tEntityIDSeqTableName = "entity_id_seq"

type TEntityIDSeq struct {
    Name    string `db:"name"`
    NextSeq int64  `db:"next_seq"`
}

// TEntityIDSeqAllocate 原子分配并返回下一个可用序号（兼容 MySQL/SQLite）。
func TEntityIDSeqAllocate(dbCtx lib.DBContexter, name string) (int64, error)
```

实现与 `storage/rdb/internal/dao/table_api_key_id_seq.go` 的 `TAPIKeyIDSeqAllocate` 保持同一策略（MySQL 单语句原子分配 / SQLite INSERT OR IGNORE + UPDATE），复用同包已有的 `isMySQL` 辅助函数。

### 3.4 模型层接口与实现

新增 `model/entity/id_generator.go`：

```go
// EntityIDGenerator 提供原子分配 Entity ID 的能力。
type EntityIDGenerator interface {
    Generate(ctx context.Context) (string, error)
}
```

新增 `storage/rdb/entity/id_generator.go`：`RDBEntityIDGenerator`，内部调用 `dao.TEntityIDSeqAllocate(dbCtx, "entity")`，通过 `dbCtxFactory` 开启独立短事务，格式化为 `entity-%d` 返回。实现参照 `storage/rdb/api_key/id_generator.go`。

### 3.5 Endpoint 层改造

`endpoints/openapi_v1/entity/create.go`：

1. 删除 `generateEntityID` 函数（含 `fmt.Sscanf` 全表扫描逻辑）；
2. `param.EntityID` 为空时改为：

```go
generatedID, err := container.EntityIDGenerator.Generate(req.Context())
if err != nil {
    return nil, xerror.WrapModelError(err) // 参照 Issue #80 使用的错误包装
}
param.EntityID = &generatedID
```

### 3.6 容器初始化

- `stateful/container/components.go`：新增 `var EntityIDGenerator entity.EntityIDGenerator`；
- `stateful/container/rdb/components.go`：`container.EntityIDGenerator = entityStorage.NewRDBEntityIDGenerator(container.DBContextFactory)`（注入方式参照 `RDBAPIKeyIDGenerator`）。

### 3.7 模型层 `CreateEntity` 行为

- 显式传入 `entity_id` 的查重保护保留（422 Duplicate），防止绕过生成器的调用；
- `uk_entity_id` 唯一索引作为最终一致性防线保留；如实现统一的唯一冲突错误翻译（1062 → 友好业务错误），属可选增强，不在本次必需范围。

## 4. 数据迁移

### 4.1 初始化序列值

```sql
-- MySQL
INSERT INTO entity_id_seq (name, next_seq)
SELECT 'entity', COALESCE(MAX(CAST(SUBSTRING(entity_id, 8) AS UNSIGNED)) + 1, 1)
FROM entities
WHERE entity_id LIKE 'entity-%';

-- SQLite
INSERT INTO entity_id_seq (name, next_seq)
SELECT 'entity', COALESCE(MAX(CAST(SUBSTR(entity_id, 8) AS INTEGER)) + 1, 1)
FROM entities
WHERE entity_id LIKE 'entity-%';
```

> 说明：只处理 `entity-%` 规范格式的旧 ID；无非规范 ID 时从 1 开始（实际生成时若冲突会被 `uk_entity_id` 拦截，概率极低，建议运维排查）。

### 4.2 升级步骤

1. 执行 DDL 创建 `entity_id_seq` 表（MySQL 与 SQLite 分别执行）；
2. 执行迁移 SQL 初始化 `next_seq`；
3. 滚动升级 `ai-gateway-api` 实例；
4. 升级后新创建的 Entity 使用原子序号。

## 5. 涉及文件清单

| 文件 | 修改内容 |
|------|----------|
| `db_ddl.sql` | 新增 `entity_id_seq` 表 |
| `db_ddl_sqlite.sql` | 新增 `entity_id_seq` 表 |
| `storage/rdb/internal/dao/table_entity_id_seq.go` | 新增序列表 DAO + `TEntityIDSeqAllocate` |
| `storage/rdb/internal/dao/table_entity_id_seq_test.go` | DAO 分配并发单元测试 |
| `storage/rdb/entity/id_generator.go` | 新增 `RDBEntityIDGenerator` 实现 |
| `model/entity/id_generator.go` | 新增 `EntityIDGenerator` 接口 |
| `stateful/container/components.go` | 新增 `EntityIDGenerator` 全局变量 |
| `stateful/container/rdb/components.go` | 初始化并注入 `RDBEntityIDGenerator` |
| `endpoints/openapi_v1/entity/create.go` | 删除 `generateEntityID`，改用 `container.EntityIDGenerator.Generate` |
| `model/entity/entity_manager_test.go` | 保留显式重复 ID 查重测试 |
| `test/integration/tests/entity/create/create_test.go` | 新增 `TestEntity_CreateAutoID`：自动 ID 格式断言 + 连续创建单调递增 |
| `test/integration/tests/entity/delete/delete_test.go` | 新增 E-6-004：删除最大编号后重建不复用 ID |
| `design-docs/modifications/2026-09-04-issue-132-entity-id-race/` | 本方案文档 |

## 6. 测试计划

### 6.1 单元测试

1. **DAO 并发分配**：N 个 goroutine（如 50~100）并发调用 `TEntityIDSeqAllocate`，断言序号唯一且从 1 连续递增（参照 `TestTAPIKeyIDSeqAllocate_Concurrent`）；
2. **删除/回滚不回收序号**：分配后不回写任何数据，继续分配应得到递增序号；
3. **Endpoint 流程**：mock `EntityIDGenerator` 返回固定值，断言 `EntityCreateAction` 透传该 ID；
4. **模型层回归**：显式重复 `entity_id` 仍返回 422。

### 6.2 集成测试

1. **E-1-101 自动生成 ID 格式**：创建未指定 ID 的 Entity，断言返回 ID 符合 `entity-<正整数>` 格式；
2. **E-1-102 连续创建 ID 单调递增**：串行创建 5 个 Entity，断言序号严格递增；
3. **E-6-004 删除最大编号后不复用**：创建 → 删除 → 重建，断言新 ID 序号大于被删 ID；
4. 全量回归 `entity` 相关集成测试（create / delete / update / list）。

> 关于并发集成测试：与 Issue #80 相同，集成测试环境为 SQLite 文件库且驱动 busy 等待会阻塞至请求超时，多连接并发写会活锁（该问题在旧实现下同样存在，属环境限制而非本次引入）；并发正确性由 DAO 层单元测试（50 goroutine）覆盖，生产环境为 MySQL，由原子单语句分配保证。
> 另注意：集成测试启动的是项目根目录预编译的 `ai-gateway-api.exe`，修改代码后必须先重新编译，否则测试跑的是旧二进制。

### 6.3 验收对照（issue 验收标准）

| 验收标准 | 验证方式 |
|----------|----------|
| 并发创建获得不同 ID | DAO 并发单测 + 集成测试并发创建 |
| 删除最大编号后不复用 | 集成测试：delete max → create → 断言新 ID 更大 |
| 失败/回滚不回收已发放 ID | DAO 单测：分配后不落库，下次分配递增 |
| 旧缓存/异步任务不影响新资源 | 由序号不回退保证；删除路径已有 Redis 清理，回归验证 |
| `entity_id` 数据库唯一约束 | `uk_entity_id` 已存在，保持不变 |

## 7. 风险与缓解

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| 迁移时 `next_seq` 初始化错误 | 初始化 SQL 漏跑或旧数据含非规范 ID | `uk_entity_id` 兜底拦截冲突；上线检查清单强制核对 `next_seq` |
| 序号表成为写入热点 | 所有创建更新同一行 | Entity 创建频率远低于 API-Key；MySQL 行锁下 CAS 冲突概率低，外层重试兜底 |
| SQLite 高并发锁竞争 | 集成测试环境限制 | 与 Issue #80 相同处理：并发正确性由 DAO 单测覆盖，生产为 MySQL |
| 生成器故障导致创建不可用 | DB 连接/事务异常 | 返回 500 并记录日志，不影响已有数据 |
| 操作日志中的旧 `entity-N` 引用 | ABA 消除后历史引用天然指向唯一资源 | 无需清理 operation_logs（审计要求保留），序号不回退保证不再产生歧义 |

## 8. 上线前检查清单

1. **执行 DDL**：在目标数据库创建 `entity_id_seq` 表；
2. **执行迁移 SQL**：按第 4.1 节初始化 `next_seq`，并核对值 = 当前最大 `entity-N` + 1；
3. **验证**：并发创建若干 Entity 无错误、ID 连续；删除最大编号 Entity 后重建，确认不复用；
4. **灰度发布**：先升级一个实例观察，确认无异常后全量滚动；
5. **回滚预案**：回滚代码后旧 `generateEntityID` 逻辑恢复（仅文档层面，如需回滚需同时回滚代码与保留序列表，避免序号回退复用）。
