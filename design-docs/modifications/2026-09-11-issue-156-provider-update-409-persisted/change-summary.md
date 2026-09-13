# Issue #156：Provider 更新返回 409 但修改已生效（假事务 + 先写后校验）修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/156](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/156)

> SC1201-TC052：从 provider 移除一个已被 cluster 引用的模型，返回 409，但实际更新已生效。

复现：

1. 创建 provider，含模型 `controlled-model-a/b`；
2. 创建 cluster，`llm_config.models` 引用这两个模型；
3. PATCH provider 移除 `controlled-model-b` → 返回 409 `Conflict: ... is referenced by cluster ...`；
4. 重新查询 provider → `controlled-model-b` 已被移除。

期望：409 时更新应被拒绝，provider 保持原值。

影响：引用完整性校验形同虚设——调用方收到拒绝信号但配置已被破坏，cluster 引用了 provider 中已不存在的模型，数据面行为不可预期；且 409 响应具有误导性。

## 2. 根因（本地 v0.0.9 代码核实，行号已按当前代码校准）

两层缺陷叠加：

### L1（执行层，系统性）：AtomExecute 事务是空转的

- dao 统一执行通道 `storage/rdb/internal/dao/internal/curd.go` 的 `QueryOne/QueryList/Create/Update/Delete` 全部调用 `dbCtx.Conn().ExecContext/QueryContext`；
- `DBContext.Conn()`（`lib/xdb.go:42`）返回连接池 `*sql.DB`（autocommit），接口签名（`DBContexter.Conn() *sql.DB`）根本无法返回 `*sql.Tx`；`dbCtx` 仅作为 `context.Context` 载体（超时/SQL 日志）；
- 而 `AtomExecute`（`storage/rdb/txn/txn.go:36-43`）→ `dbCtxFactory(ctx, lib.OpenTxn())` → `lib.RDBTxnExecute`（`lib/xdb.go:95`）确实 `Begin` 了 `dc.tx` 并把同一个 `dc` 传给业务闭包——但 dao 层语句走 `dc.Conn()`（裸连接池）即时持久化，报错后 `commitOrRollback` 回滚的是一个**零语句的空事务**；
- 推论：全站 `AtomExecute` 多步写路径均无原子性保证，"先写后校验 + 回滚兜底"模式整体失效。

### L2（流程层）：写在前、校验在后

`ProviderManager.UpdateProvider`（`model/iprovider/provider.go:196-270`）闭包顺序：

1. :207 `FetchProvider` 取 `existing`；
2. :219-221 捕获 `origInstancePool/origKeys/origModels`；
3. :223 **`storager.UpdateProvider` 落库（此时已 autocommit 持久化）**；
4. :230-239 计算 `clusterConfigChanged`；
5. :241-258 构造 `newProvider` 快照并顺序执行 3 个 hook（endpoint `update.go:60-62` 注入：`ProviderInstancePoolSyncer` → `ProviderKeyRefChecker` → `ProviderModelRefChecker`）；
6. `ProviderModelRefChecker`（`model/icluster_conf/cluster.go:992`，#106 引入）检出引用冲突返回 409——但落库早已完成。

校验逻辑本身正确（409 检出无误），错的是 L1 使回滚兜底失效，L2 的写后校验把暴露窗口放大为必现。

### 连带影响（同根因，随 P1 一并消除）

- `ProviderKeyRefChecker`（`cluster.go:948`，移除被引用 key）同病；
- `ProviderInstancePoolSyncer`（`cluster.go:903`，级联写引用 cluster 的 sub-cluster pool）有副作用且位于 hook 链**首位**——中途失败时 provider 已改、级联半途，产生部分写入；
- 其他 `AtomExecute` 多步路径（CreateProvider 查重+插入、DeleteProvider 引用检查+删除、cluster 级联等）同样无原子保证。

## 3. 修复策略（P1 + P2，已确认）

| 项 | 内容 | 说明 |
|----|------|------|
| **P1** | 执行层根治：dao 执行入口绑定真实事务，`AtomExecute` 成为真事务 | 本文档实施。**不实施 P0 重排**——真事务落地后，"先写后校验 + 回滚兜底"即正确，409 时 provider 写入随事务回滚，#156 消除；同时全站多步写路径获得原子性 |
| **P2** | 回归测试：事务行为单测 + 集成测试 + design.md 登记 | 钉死"校验失败 = 零写入"契约 |

不实施 P0（UpdateProvider 重排 / hook 签名拆分）的理由：P1 后该改动无必要收益，却引入 manager 签名变更与调用方改造；hook 顺序在真事务下也安全（校验、级联、落库同属一个事务，失败整体回滚）。

## 4. 目标

1. PATCH provider 因引用冲突返回 409 时，provider 记录**零变更**（事务回滚）；
2. 移除被引用 key 的 409、Syncer 级联中途失败等场景同样整体回滚，无部分写入；
3. `AtomExecute` 成为真事务：闭包内读见闭包内写（读己之写），失败整体回滚，成功才提交；
4. 无事务路径（`OpenTxn` 之外的普通 dao 调用）行为完全不变；
5. 回归验证：E2E SC1201-TC052 重跑 PASSED；SC2101-TC034 d1 臂补"409 后回读未变"断言应转 PASSED。

## 5. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `lib/xdb.go`（`SqlExecutor` 抽象 + `DBContext.Execer()` + `DBContexter` 接口扩展）；`storage/rdb/internal/dao/internal/curd.go`（四函数改走 Execer）；其余直接调用 `dbCtx.Conn()` 的 dao 表文件同步改造 |
| 接口契约 | OpenAPI/InnerAPI 不变；仅持久化语义从"autocommit 逐步生效"修正为"事务内原子生效" |
| 风险面 | 全站 dao 调用点（grep `.Conn()` 全量改造，重点回归：UpdateProvider 三 hook、Create/DeleteProvider、cluster 级联、quota/操作日志批量写）；真事务持有锁时间变长，需关注并发与死锁（保持现有访问顺序，本次不调整并发结构） |
| 不在范围 | hook 顺序重排（P0，已否决）；历史已被部分写入破坏的数据不回溯 |
| 数据迁移 | 无 |

## 6. 最终方案（P1）

### 6.1 SqlExecutor 抽象（lib/xdb.go）

`*sql.DB` 与 `*sql.Tx` 均实现 `ExecContext/QueryContext/QueryRowContext`，抽取公共接口：

```go
type SqlExecutor interface {
    ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
    QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
    QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
}
```

### 6.2 DBContext 分流执行入口

```go
// Execer 返回当前有效的 SQL 执行器：事务内返回 *sql.Tx，否则返回连接池 *sql.DB。
func (ctx *DBContext) Execer() SqlExecutor {
    if ctx.tx != nil {
        return ctx.tx
    }
    return ctx.conn
}
```

`DBContexter` 接口（`lib/xdb.go:23-26`）增加 `Execer() SqlExecutor`；`Conn() *sql.DB` 保留（`BeginTrans`、`Transaction` 等仍需要）。

### 6.3 dao 层改造

- `curd.go` 的 `QueryOne/QueryList/Create/Update/Delete`：`dbCtx.Conn().ExecContext(...)` → `dbCtx.Execer().ExecContext(...)`（Query 同理）；
- 其余直接使用 `dbCtx.Conn()` 的 dao 表文件同步改造（当前 grep 命中：`table_api_key_id_seq.go`、`table_entity_id_seq.go`、`table_model_prices.go`、`table_operation_logs.go`、`table_products.go` 等，实施时全量核对）；
- SQL 日志记录（`stateful.SQLRecord`）逻辑不变。

### 6.4 事务语义变化说明

- `AtomExecute`：闭包内全部读写经 `dc.Execer()` 走 `dc.tx`——读己之写、失败整体回滚、成功提交；`UpdateProvider` 的落库、校验 hook 的读、Syncer 的级联写同属一个事务，任一失败（含 409）整体回滚，#156 消除；
- 嵌套 `AtomExecute`（代码实证，修正初版结论）：生产 factory（`stateful/config_database.go:140-143`）每次调用都**新建** DBContext，不从 ctx 复用已有事务；Syncer 直接调 `poolStorager.UpdatePool`（`cluster.go:935`）不经过嵌套 `AtomExecute`；epp_pool 等多处 `AtomExecute` 均为顺序调用。因此 `RDBTxnExecute` 的 `dc.tx != nil` 复用分支（`lib/xdb.go:97-101`）当前是**死路径**，内层提前 commit 的隐患不会触发，语义不变；
- 非事务路径：`tx == nil` 时 `Execer()` 返回连接池，行为与现状完全一致；
- 操作日志为异步批量落库（`Record` 仅入队，不经过 dao 事务），主事务回滚不会产生日志残留问题，无需改动；
- `commitOrRollback` 对 "invalid connection" 直接返回 err 不回滚的既有分支保持不变；
- **连带修复（P1 范围内）**：factory 中 `dc.BeginTrans()` 的错误当前被忽略（`config_database.go:142`），P1 实施时改为返回错误。

### 6.5 副作用评估（P1 落地后的行为变化，代码实证）

| 副作用 | 实证结论 | 影响面 |
|--------|----------|--------|
| 行锁持有变长 | **成立**。单事务持锁最长路径是 `ImportModelPrices`（`model/imodel_price/model_price.go:204`）：replace = 全表 delete + 逐条 insert，merge = 全表读 + 逐行 update/insert，整个批次一个事务。其次 Provider PATCH + Syncer 级联：provider 行锁 + pool 行锁，持锁覆盖 `FetchClusterList` 全表扫描与逐 pool 更新 | 并发导入互斥串行化；单批执行超 `innodb_lock_wait_timeout`（默认 50s）报错回滚；并发 PATCH 同一 provider / 共享 pool 锁等待 |
| 读一致性变化 | 事务内 dao 读走 tx：RR 快照 + 读己之写。三个 hook 的引用检查（纯读 `FetchClusterList`）不再读到并发事务中间态——对 #156 语义更正确。普通 SELECT 不加行锁，无额外锁风险 | hook 校验、merge 导入的全表读 |
| 回滚覆盖面扩大 | 几十个 `AtomExecute` 调用点（iauth、icluster_conf、epp_pool、quota、entity、api_key、route_rules…）从假原子变真原子：以往"外层失败但部分写已 autocommit 残留"被回滚。已抽查 `allocateAndUpsert` 重试路径（`assignment.go:117-132`）：失败后重读走新 DBContext（无 tx），读到回滚后状态，语义正确 | 个别若隐含依赖部分写入的路径会暴露，由全量集成测试兜住 |
| 连接占用 | 事务期间独占一个连接直至 commit/rollback；池上限由 conf `MaxOpenConns` 控制（`config_database.go:78`） | 导入长事务占 1 个连接，正常配置无感 |

**导入分批的取舍**：若导入批次可能很大，后续可考虑每 N 条一个事务分批提交——但这改变导入的原子语义（部分批次已提交），属于独立变更，**不进 #156**。

## 7. 测试（P2）

### 7.1 事务行为单测（sqlite 真实链路）

model 层 fake storager 无法覆盖事务行为，新增 sqlite 内存库 + 真实 `RDBTxnStorager` + 真实 storager 的链路测试（模式同 `storage/rdb/provider/provider_test.go`）：

1. **核心锚点**：建 provider（2 模型）+ cluster 引用 → 经 `AtomExecute` 链路先更新 provider、后执行返回 409 的校验 hook → 断言事务回滚、provider models 原值未变（对应 SC1201-TC052）；
2. 校验通过场景：更新提交生效；
3. 非事务路径：`OpenTxn` 之外的 dao 调用即时生效（回归 6.4 语义 3）；
4. 读己之写：事务内写入后同一事务内查询可见；
5. 连带修复验证：factory 中 `BeginTrans` 失败时 `AtomExecute` 返回错误而非继续执行闭包。

### 7.2 集成测试（test/integration，真实二进制）

`tests/provider` 模块新增（登记 design.md）：

- 创建 provider（2 模型）+ cluster 引用 → PATCH 移除被引用模型 → 断言 409 且 GET provider models 原值未变；
- 对照组：移除未被引用模型 → 200 且生效。

### 7.3 E2E

- SC1201-TC052 重跑 PASSED；
- 建议为 SC2101-TC034 d1 臂补"409 后回读 provider models 未变"断言（部署前该断言应 FAILED_PRODUCT，作为 P1 的验证门槛）。

## 8. 回归验证

1. 本地：`go build ./...`、`go vet ./...`、`go test ./...`（重点：`storage/rdb/...`、`model/...`）、model 覆盖率门禁（≥70%）通过；
2. 集成测试全量：`test/integration` 各模块（provider、clusters、api_key、entity、operation_log、model_price 等）串行运行通过；
3. 部署后 E2E：SC1201-TC052、SC2101-TC034、SC1201-TC051（provider 正常更新 retention）重跑 PASSED。

## 9. 后续观察

- 真事务落地后关注 MySQL 行锁持有时间与死锁日志：`SHOW ENGINE INNODB STATUS` 死锁段、`performance_schema.data_lock_waits`、lock wait timeout 错误日志；重点路径即 6.5 表格中的两条——model_price 批量导入（最长批事务）与 Provider PATCH + cluster 级联。如出现锁竞争再评估 hook 重排（原 P0）或导入分批作为优化手段，均作为独立变更；
- `RDBTxnExecute` 的嵌套复用语义当前为死路径（6.4），补单测钉死现状语义即可，不做改造；
- factory 中 `BeginTrans` 错误吞掉的问题随 P1 一并修复。

## 10. 实施记录（已完成）

### 代码改动

| 文件 | 改动 |
|------|------|
| `lib/xdb.go` | 新增 `SqlExecutor` 接口（`*sql.DB`/`*sql.Tx` 均实现）；`DBContext.Execer()` 按 `tx != nil` 分流；`DBContexter` 接口增加 `Execer()` |
| `stateful/config_database.go` | factory 中 `BeginTrans()` 错误不再吞掉，失败时返回错误 |
| `storage/rdb/internal/dao/internal/curd.go` | `QueryOne/QueryList/Create/Update/Delete` 五通道 `Conn()` → `Execer()` |
| `storage/rdb/internal/dao/table_providers.go`、`table_model_prices.go`、`table_operation_logs.go`、`table_products.go` | 绕过 curd 的原生 SQL 同步改走 `Execer()` |
| `storage/rdb/model_price/model_price.go` | `DeleteAllModelPrices`（replace 导入路径）、`ListProviders` 改走 `Execer()` |
| `storage/rdb/internal/dao/table_api_key_id_seq.go`、`table_entity_id_seq.go` | 保持 `Conn()` 不变，注释说明有意为之（独立小事务、仅接口层调用） |

### 测试

- 单测：`lib/xdb_test.go` 新增 `TestDBContextExecer`（池/事务分流）；`storage/rdb/provider/provider_txn_test.go` 新增 4 个 sqlite 真实链路锚点（更新后校验失败回滚、提交生效、读己之写、回滚丢弃插入）；
- 集成测试：`tests/provider/instance_pool_sync/rollback_test.go` 新增 PV-SYNC-1-004（409 且 GET models 原值未变）/ PV-SYNC-1-005（对照组 200 生效），已登记 `tests/provider/design.md`（§10.2 用例表 + §10.3.4/10.3.5 详细设计，统计 53 → 55）；
- sys-design：`存储层设计文档.md` 新增 §5.2.1「dao 语句的事务路由（issue #156）」。

### 验证结果

- `go build ./...`、`go vet ./...`、`go test ./...` 全量通过；
- 集成测试 `tests/provider/instance_pool_sync` 全包通过（含新增 2 例与既有 PV-SYNC-1-002/003）；
- 全量集成测试串行通过。
