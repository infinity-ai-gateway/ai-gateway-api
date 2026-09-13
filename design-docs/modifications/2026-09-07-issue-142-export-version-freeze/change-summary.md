# Issue #142：导出配置版本号同秒碰撞楔死修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/142](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/142)

> inner-api `GET /inner-api/v1/configs/tls_conf/server_data_conf` 的 Version 字段是所有消费方（conf-agent、BFE 配置生成、自动化验证）判断"配置是否变更"的唯一信号。当两次配置变更落在同一墙钟秒内时，`t_config_version`（实际表名 `config_versions`）会产生同 `(name, version)` 重复行且 DataSign 相互矛盾；此后版本读取 `ORDER BY version DESC LIMIT 1` 钉住其中一行，若其签名恰等于当前内容签名，版本推进被永久短路——导出内容每轮重建都是新的，但版本号不再变化，直到某次无关的新变更才解开。同秒连续变更是 E2E 测试的常规节奏，也是运维脚本/GitOps 类自动化可能出现的模式。

现象：E2E 用例 SC2101-TC018 六连失败；版本号楔死期间按版本号等待收敛的消费者无限挂起（观测到 120.1s / 202 轮不推进，解楔事件为 cleanup 的 DELETE 变更，属假自愈）；同一版本号先后承载不同内容，版本号失去"标识唯一配置状态"的语义。

## 2. 目标

1. 每次配置内容变更都产生新的、同 topic 内严格递增的 Version；
2. `config_versions` 表不存在同 `(name, version)` 重复行（数据库层唯一约束兜底）；
3. 并发/同秒导出请求不再出现 duplicate key 导致的导出整单 500；
4. 版本串格式保持 `20060102150405`（14 位秒级时间串）不变，conf-agent 等消费方（以版本号命名输出目录、按版本号判等）零改动；
5. 修复后 SC2101-TC018（亚秒 configure→remove 连发 + 版本收敛判据）可通过。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `model/iversion_control/version_control.go`、`storage/rdb/version_control/version_control.go`、`storage/rdb/internal/dao/table_config_versions.go`、`db_ddl.sql`、`db_ddl_sqlite.sql` |
| 数据库 | `config_versions` 表新增 `UNIQUE(name, version)`；需先清理存量重复行 |
| 接口契约 | Inner API 请求/响应字段不变，Version 格式与语义不变（仅保证单调递增） |
| 数据迁移 | 存量环境需执行重复行清理 + 唯一索引创建（见 design-changes.md 第 4 节） |
| 数据兼容 | 旧版本行全部保留，不做改写；新增版本行与同 topic 既有最大版本保持单调 |

## 4. 最终方案概览

**采用 issue 建议的"方案 A + 方案 C"组合，改动集中在存储层：**

- **唯一约束（方案 A 核心）**：`config_versions` 增加 `UNIQUE(name, version)`（MySQL 唯一索引 / SQLite 唯一索引），从数据库层根除重复版本行。
- **单调版本计算（方案 A 的"version + 1s"落地）**：`UpsertConfigLastExportedVersion` 插入前读取最新版本行，候选版本取 `max(当前秒, 最新版本 + 1s)`（等长定宽时间串可直接字符串比较），同秒第二次变更天然获得 +1s 的版本号，从源头消除碰撞。
- **duplicate key 重试路径**：插入冲突（两个并发导出请求同时读到同一最新行等残余竞态）用现成的 `lib.DuplicateEntryError`（`lib/xdb.go:138`，已覆盖 MySQL 1062 与 SQLite UNIQUE constraint）识别，重读最新行、版本再 +1s 后重试，上限若干次；不再整单 500。
- **确定性读取（方案 C）**：最新行查询改为 `ORDER BY version DESC, id DESC`，消除平局行选取的执行计划不确定性。

> **备选方案说明：**
> - **方案 B（毫秒精度 + 序列号版本串）**：可彻底消除碰撞，但改变版本串格式（14 位 → 17 位），conf-agent 以版本号命名输出目录、消费方按版本号判等，存在兼容性风险，本次不采用。若未来版本串需承载更高频率变更，可再评估。
> - **仅方案 C**：只消除平局不确定性，不消除重复行与 sign 短路，不能单独作为修复（与 issue 结论一致）。

## 5. 机制链与修复点对照

| # | 缺陷环节（源码定位） | 修复措施 |
|---|----------------------|----------|
| 1 | `CalculateVersion()` = `Version(time.Now())`，秒级时间串（`model/iversion_control/version_control.go:57-60`） | 存储层在秒级时间串基础上按"最新版本 + 1s"抬升，保证同 topic 单调递增；格式不变 |
| 2 | `TConfigVersionCreate` 为 plain INSERT，无冲突处理（`storage/rdb/internal/dao/table_config_versions.go:80`） | 表上新增 `UNIQUE(name, version)`；插入冲突走重试路径（`lib.DuplicateEntryError` 识别），不再 500 |
| 3 | `TConfigVersionOne(Name, OrderBy="version DESC")` 平局任取（`storage/rdb/version_control/version_control.go:43-46`） | 改为 `ORDER BY version DESC, id DESC`，取到同版本中最晚插入的行 |
| 4 | sign 短路：钉住行签名 == 当前内容签名则永久返回旧版本（`storage/rdb/version_control/version_control.go:51-53`） | 重复行消除 + 确定性读取后，最新行签名即该 topic 当前真实内容签名，短路逻辑恢复正确语义：内容未变返回旧版本（预期行为），内容已变必然插入新版本 |

## 6. 预期收益与风险

| 项目 | 说明 |
|------|------|
| 收益 | 版本号恢复"标识唯一配置状态"语义；按版本轮询收敛的消费者（E2E / conf-agent / GitOps 脚本）不再无限挂起；同秒连发变更不再产生矛盾重复行或 500 |
| 主要风险 | 存量环境存在重复行，直接加唯一索引会失败；需先执行清理 SQL（design-changes.md 第 4 节提供） |
| 兼容性 | 版本串格式不变；既有消费方无需升级；`ExportConfig` 事务语义（`model/iversion_control/version_control.go:84-110`）不变 |
| 残余竞态 | 两个并发导出请求可能交错"读最新 + 插入"，由唯一索引 + 重试路径兜底，重试上限内可收敛；极端重试耗尽返回错误优于静默写坏数据 |

## 7. 回归验证

1. **现成回归用例**：E2E SC2101-TC018（亚秒 configure→remove 连发 + 240 轮版本收敛判据），修复部署后 requeue 复跑即可实证；
2. **复现脚本**：issue 附带的 `tc018_repro.py`（6 个 HTTP 请求，QA 环境 1/1 复现）可在修复后重跑，期望 `VERDICT: CONVERGED`；
3. **DB 佐证 SQL**：修复前可用 issue 提供的 `GROUP BY name, version HAVING COUNT(*) > 1` 确认重复行；修复并清理后该查询应返回空。

## 8. 参考文档

- [Issue #142](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/142)（含完整机制链分析、复现脚本、DB 佐证 SQL）
- `ai-gateway-api/model/iversion_control/version_control.go`（`ExportConfig` / `CalculateVersion` / `Sign`）
- `ai-gateway-api/storage/rdb/version_control/version_control.go`（`UpsertConfigLastExportedVersion`）
- `ai-gateway-api/storage/rdb/internal/dao/table_config_versions.go`（`TConfigVersionOne` / `TConfigVersionCreate`）
- `ai-gateway-api/lib/xdb.go:130-144`（`DuplicateEntryError`，MySQL/SQLite 双兼容）
- `design-docs/modifications/2026-09-04-issue-132-entity-id-race/`（同类型"并发/竞态 + 唯一约束兜底"修复的落地范例）
