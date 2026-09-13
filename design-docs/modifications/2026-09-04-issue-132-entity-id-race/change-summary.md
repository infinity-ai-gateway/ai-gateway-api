# Issue #132：Entity 自动 ID 生成竞态与删除后 ID 复用修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/132](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/132)

> 当前 Entity 未指定 ID 时，系统会查询全部现存 Entity，从 `entity-N` 中计算最大序号，然后生成 `entity-(N+1)`。该机制存在并发创建冲突、删除后复用 ID、ABA 身份混淆、污染关联状态、影响测试与运维可靠性等隐患。

## 2. 目标

1. 并发创建多个 Entity 时，所有请求获得不同 ID；
2. 删除当前最大编号 Entity 后，新建 Entity 不复用该 ID（序号单调不回退）；
3. 创建失败或事务回滚不会导致已发放序号被其他 Entity 重新使用；
4. 旧 Entity 的缓存、异步任务和清理操作不会影响后续新建 Entity；
5. `entity_id` 在数据库层具有唯一约束（已有 `uk_entity_id`，保持不变）；
6. **保留 `entity-{seq}` 的 ID 可读格式**，减少接口兼容性影响。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `endpoints/openapi_v1/entity/create.go`、`model/entity/`、`storage/rdb/entity/`、`storage/rdb/internal/dao/`、`stateful/container/`、`db_ddl.sql`、`db_ddl_sqlite.sql` |
| 数据库 | 新增 `entity_id_seq` 序列表 |
| 接口契约 | OpenAPI 请求/响应字段不变；仅 ID 分配机制变更 |
| 数据迁移 | 需要初始化 `entity_id_seq.next_seq` 为当前最大 `entity-N` 序号 + 1 |
| 数据兼容 | 旧数据 `entity_id` 全部保留，不做改写 |

## 4. 最终方案概览

**采用“数据库原子序列表”方案，与 Issue #80（API-Key ID 竞态修复，`2026-08-23-issue-80-apikey-id-race`）同一套已验证模式：**

- 新增单表 `entity_id_seq(name, next_seq)`，全局固定一行（`name = 'entity'`），维护下一个可用序号；已分配序号永久消耗，删除 Entity 后不回退。
- 序号分配采用与 `TAPIKeyIDSeqAllocate` 完全一致的最终实现（Issue #99 改进版）：MySQL 用单条 `INSERT ... ON DUPLICATE KEY UPDATE` + `LAST_INSERT_ID()` 原子分配，SQLite 用 `INSERT OR IGNORE` + `UPDATE` + 读回；避免了早期 CAS 重试在 MySQL 默认隔离级别下的锁等待问题。
- `generateEntityID`（`endpoints/openapi_v1/entity/create.go:127-146`）删除，改为调用新的 `EntityIDGenerator`；同时消除“每创建一个 Entity 全表拉取 + 逐条联表填充（N+3 次查询）”的性能问题。
- `entity-{seq}` 格式不变，兼容现有设计文档、下游 BFE/conf-agent 配置导出及前端展示。

> **备选方案说明：**
> - **UUID/ULID**：issue 备选方案，因会改变 `entity-N` 可读格式，未采用。
> - **复用内主键 `id`（AUTO_INCREMENT）派生序号**：`entities.id` 虽是单调不回退的自增列，但历史数据的 `entity-N` 与主键 `id` 并不对应，且删除后新分配的主键可能小于历史最大 N，需额外维护偏移/冲突重试，复杂度不低于序列表，未采用。

## 5. 预期收益与风险

| 项目 | 说明 |
|------|------|
| 收益 | 根除并发 ID 冲突（不再出现 500/友好重复错误之外的异常）；删除后 ID 不复用，消除 ABA 歧义；operation_logs、导出配置中对旧 `entity-N` 的引用从此可唯一对应历史资源；创建路径不再全表扫描 |
| 主要风险 | 新序列表需一次性迁移初始化；`next_seq` 必须正确初始化，否则可能与旧数据冲突（由 `uk_entity_id` 兜底拦截，但需运维核查） |
| 兼容性 | `uk_entity_id` 唯一索引保留；新旧 ID 格式一致；显式传入重复 `entity_id` 的 422 查重保护继续生效 |

## 6. 上线前必须执行的迁移

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

> 说明：
> - `next_seq` 必须初始化为当前最大序号 + 1，否则新分配序号可能与旧数据冲突；
> - 只处理 `entity-%` 规范格式的旧 ID；无非规范 ID 时 `COALESCE` 保证从 1 开始。

## 7. 参考文档

- [Issue #132](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/132)
- `design-docs/modifications/2026-08-23-issue-80-apikey-id-race/`（同模式已落地的 API-Key 修复，实现细节可直接参照）
- `ai-gateway-api/endpoints/openapi_v1/entity/create.go`（`generateEntityID`）
- `ai-gateway-api/model/entity/entity_manager.go`（`CreateEntity` / `DeleteEntity`）
- `ai-gateway-api/db_ddl.sql`（`uk_entity_id` 唯一索引）
- `design-docs/api-define/OpenAPI接口定义/entities.md`
