# Issue #142：导出配置版本号同秒碰撞楔死——设计变更说明

## 1. 当前问题定位

### 1.1 版本推进路径

```text
inner-api 导出请求（如 GET /inner-api/v1/configs/tls_conf/server_data_conf）
    └── VersionControlManager.ExportConfig            model/iversion_control/version_control.go:84
        └── 每请求全量重建导出内容（ConfigGenerator）
        └── DataWithoutVersion.UpdateVersion(ZeroVersion)
        └── Sign(DataWithoutVersion)                  // MD5 内容签名
        └── storager.UpsertConfigLastExportedVersion  storage/rdb/version_control/version_control.go:37
            ├── TConfigVersionOne(name, ORDER BY version DESC LIMIT 1)   // :43
            ├── 若 DataSign == 当前签名 → 返回旧版本（短路）              // :51
            └── CalculateVersion() = time.Now().Format("20060102150405") // :55
                └── TConfigVersionCreate（plain INSERT）                 // :60
```

### 1.2 缺陷机制链（同秒两次内容变更）

1. **秒级版本串**：同一秒内两次"内容已变"的导出计算出相同版本串；
2. **plain INSERT**：产生同 `(name, version)` 重复行，两行 DataSign 分别对应变更前/后内容；
3. **平局读取**：`ORDER BY version DESC LIMIT 1` 在重复行中取哪行由执行计划决定，稳定但任意；
4. **sign 短路**：若钉住行的 DataSign 恰等于当前重建内容签名，直接返回旧版本号、不再插入——版本推进被永久短路；内容每轮新鲜，版本号恒定，直到某次无关变更使签名失配才解开（假自愈）；
5. **500 风险**：一旦加上唯一索引而 INSERT 未处理冲突，duplicate key 将使导出请求整单 500（`UpsertConfigLastExportedVersion` 当前不捕获该错误）。

### 1.3 触发条件与影响

- 触发条件：**内容变更的导出轮询与上一个版本行的插入落在同一墙钟秒**；亚秒级连续变更（E2E create-export 收敛 → PATCH → PATCH 全程 <2s）必然触发，分钟级手工操作不会触发（排查时若以手工单步验证会得出"无问题"的误导结论）。
- 影响：版本号楔死期间按版本轮询收敛的消费者无限挂起（SC2101-TC018 六连失败，TC034/TC005 同窗口被拖累）；版本号失去唯一标识语义；数据面内容多数时序下仍可收敛，核心危害在版本通道。

## 2. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| **A. 唯一约束 + 冲突重试 + 版本抬升（推荐）** | `UNIQUE(name, version)`；插入冲突时版本 +1s 重试（上限若干次） | 彻底消除重复行；改动集中在存储层；版本串格式不变 | 存量重复行需先清理才能加索引 | **采用**（叠加方案 C） |
| B. 毫秒精度 + 序列号版本串 | `CalculateVersion()` 引入同 topic 严格单调分量 | 消除碰撞根源 | 版本串格式 14 位 → 17 位，conf-agent 输出目录命名、消费方判等存在兼容性风险 | 未采用（issue 备选，格式兼容性排除） |
| C. 确定性读取（辅助） | 读取改为 `ORDER BY version DESC, id DESC` | 消除平局不确定性，一行代码 | 不消除重复行与 sign 短路，不能单独作为修复 | **与 A 叠加采用** |

## 3. 推荐方案详细设计

### 3.1 存储层改造（核心）

`storage/rdb/version_control/version_control.go` 的 `UpsertConfigLastExportedVersion` 重写为：

```go
func (vcs *VersionControlStorager) UpsertConfigLastExportedVersion(ctx context.Context, css *iversion_control.ExportData) (string, error) {
    dbCtx, err := vcs.dbCtxFactory(ctx)
    if err != nil {
        return "", err
    }

    // 1. 确定性读取最新行（方案 C：平局取 id 最大即最晚插入的行）
    latest, err := dao.TConfigVersionOne(dbCtx, &dao.TConfigVersionParam{
        Name:    &css.Topic,
        OrderBy: lib.PString("version DESC, id DESC"),
    })
    if err != nil {
        return "", err
    }

    // 2. sign 短路：最新行即该 topic 当前真实内容（重复行消除后语义恢复正确）
    if latest != nil && latest.DataSign == css.DataSignWithoutVersion {
        return latest.Version, nil
    }

    // 3. 单调版本计算：候选 = max(当前秒, 最新版本 + 1s)
    version := iversion_control.Version(time.Now())
    if latest != nil && version <= latest.Version {
        if t, err := time.ParseInLocation(versionLayout, latest.Version, time.Local); err == nil {
            version = iversion_control.Version(t.Add(time.Second))
        }
    }

    // 4. 插入，duplicate key 走重试路径（不再 500）
    for i := 0; i < maxDuplicateRetry; i++ {
        _, err = dao.TConfigVersionCreate(dbCtx, &dao.TConfigVersionParam{
            Name:     &css.Topic,
            DataSign: &css.DataSignWithoutVersion,
            Version:  &version,
        })
        if err == nil {
            return version, nil
        }
        if !lib.DuplicateEntryError(err) {
            return "", err
        }
        // 并发插入抢先：重读最新行，版本再 +1s 后重试
        latest, err = dao.TConfigVersionOne(dbCtx, /* 同上 */)
        if err != nil {
            return "", err
        }
        if latest == nil {
            return "", err // 理论不可达（冲突行必存在），防御性返回
        }
        if t, perr := time.ParseInLocation(versionLayout, latest.Version, time.Local); perr == nil {
            version = iversion_control.Version(t.Add(time.Second))
        } else {
            return "", err // 非预期版本格式，保留原始冲突错误
        }
    }
    return "", err // 重试耗尽：返回最后一次冲突错误，优于静默写坏数据
}
```

要点说明：

- `versionLayout = "20060102150405"`。版本串为等长定宽数字串，字符串比较与时间比较等价，`version <= latest.Version` 即为"候选版本不晚于库内最新版本"。
- `maxDuplicateRetry` 建议 3：正常路径（第 3 步单调抬升）已消除同进程同秒碰撞，重试仅兜并发导出交错/多实例部署的残余竞态。
- 重试路径中若重读到的最新行 `DataSign == css.DataSignWithoutVersion`（并发请求已为本内容建过版本），可直接返回 `latest.Version` 提前收敛，避免无意义插入。
- `lib.DuplicateEntryError`（`lib/xdb.go:130-144`）已覆盖 MySQL `Error 1062: Duplicate entry` 与 SQLite `UNIQUE constraint failed`，无需新增错误识别逻辑。
- `ExportData.CalculateVersion()`（`model/iversion_control/version_control.go:57`）保留不动：模型层接口不变，最终版本号由存储层单调化后写回 `css.version`（`ExportConfig` 的 `UpdateVersion(lrv.version)` 流程不变）。

### 3.2 DDL 变更

#### MySQL（`db_ddl.sql`）

```sql
CREATE TABLE `config_versions` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `name` varchar(255) NOT NULL,
  `data_sign` varchar(255) NOT NULL,
  `version` varchar(255) NOT NULL,
  `created_at` datetime NOT NULL,
  `updated_at` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name_version` (`name`, `version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
```

#### SQLite（`db_ddl_sqlite.sql`）

```sql
CREATE TABLE config_versions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  data_sign TEXT NOT NULL,
  version TEXT NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (name, version)
);
```

> 说明：`version` 列保持 `varchar(255)`（只存 14 位时间串，不改类型以免惊动既有部署的 ORM/工具链）；唯一约束只依赖 `(name, version)` 组合。新装环境随 DDL 自动获得唯一约束；存量环境需先执行第 4 节清理迁移。

## 4. 数据迁移（存量环境上线前必须执行）

### 4.1 清理重复版本行（保留每组 `(name, version)` 中 id 最大即最晚插入的行）

```sql
-- MySQL（多表 DELETE，保留 id 最大行）
DELETE v1 FROM config_versions v1
INNER JOIN config_versions v2
  ON v1.name = v2.name AND v1.version = v2.version AND v1.id < v2.id;

-- SQLite
DELETE FROM config_versions WHERE id NOT IN (
  SELECT MAX(id) FROM config_versions GROUP BY name, version);
```

> 清理前可用 issue 提供的佐证 SQL 评估影响面：
> `SELECT name, version, COUNT(*) AS c FROM config_versions GROUP BY name, version HAVING c > 1;`
> 同时建议比对同版本两行的 `data_sign` 确认签名矛盾（即楔死证据）。

### 4.2 添加唯一约束

```sql
-- MySQL
ALTER TABLE `config_versions` ADD UNIQUE KEY `uk_name_version` (`name`, `version`);

-- SQLite（若原表未重建，采用 CREATE UNIQUE INDEX 等价生效）
CREATE UNIQUE INDEX IF NOT EXISTS `uk_name_version` ON config_versions (name, version);
```

### 4.3 升级步骤

1. 执行 4.1 清理 SQL（先SELECT佐证、再DELETE，建议备份或低峰执行）；
2. 执行 4.2 添加唯一约束；
3. 滚动升级 `ai-gateway-api` 实例（新代码对无唯一索引的旧库同样安全：无索引时 INSERT 不冲突，单调抬升逻辑仍生效）；
4. 升级后复跑 SC2101-TC018 / `tc018_repro.py` 实证。

> 顺序说明：代码与索引的升级顺序可互换——代码先行时单调抬升已消除新重复行产生；索引先行时靠清理 SQL 保证加索引成功。二者均兼容混合部署窗口。

## 5. 涉及文件清单

| 文件 | 修改内容 |
|------|----------|
| `db_ddl.sql` | `config_versions` 表新增 `UNIQUE KEY uk_name_version (name, version)` |
| `db_ddl_sqlite.sql` | `config_versions` 表新增 `UNIQUE (name, version)` |
| `storage/rdb/version_control/version_control.go` | `UpsertConfigLastExportedVersion` 重写：确定性读取、单调版本计算、duplicate key 重试 |
| `storage/rdb/version_control/version_control_test.go` | 新增（当前目录无测试文件）：同秒双变更单调递增、内容回跳再变更再推进、sign 短路不变、duplicate key 重试路径 |
| `design-docs/modifications/2026-09-07-issue-142-export-version-freeze/` | 本方案文档 |

> 说明：`model/iversion_control/version_control.go`、`storage/rdb/internal/dao/table_config_versions.go` 无需修改（DAO 的 `OrderBy` 本就透传字符串，`, id DESC` 直接可用）。

## 6. 测试计划

### 6.1 单元测试（新增 `storage/rdb/version_control/version_control_test.go`）

1. **同秒双变更单调递增**：mock/真实 DAO 连续两次内容不同的导出（第二次冻结时间戳在同一秒），断言第二次返回的版本 = 第一次 + 1s；
2. **内容回跳再变更**：内容 A → B → A，断言版本号 V1 < V2 < V3 且第三次不再复用 V1（验证 sign 短路只发生在内容真未变时）；
3. **sign 短路保留**：内容未变重复导出，断言返回同一版本且不产生新行；
4. **duplicate key 重试**：注入第一次 `TConfigVersionCreate` 返回 duplicate 错误（fake storager/DAO 错误桩），断言重试后版本 +1s 且最终成功；
5. **非冲突错误直通**：注入非 duplicate 错误，断言原样返回、不重试。

### 6.2 集成 / E2E 验证

1. **SC2101-TC018 requeue**：修复部署后复跑，亚秒 configure→remove 连发 + 版本收敛判据应通过；
2. **`tc018_repro.py` 重跑**（issue 附带，QA 环境 1/1 复现脚本）：期望输出 `VERDICT: CONVERGED - freeze NOT reproduced`；
3. **手工对照**：分钟级间隔两次 PATCH，版本正常推进、conf-agent 生成文件正常更新（回归不退化）。

### 6.3 验收对照（issue 期望结果）

| 验收标准 | 验证方式 |
|----------|----------|
| 每次内容变更产生新的严格递增 Version | 单元测试 1/2 + SC2101-TC018 |
| 消费方按版本轮询能可靠感知变更 | SC2101-TC018 收敛判据 + tc018_repro.py |
| 不再产生同 `(name, version)` 重复行 | 唯一约束 + 单元测试 4；QA 库佐证 SQL 修复后返回空 |
| duplicate key 不导致导出 500 | 单元测试 4/5 |

## 7. 风险与缓解

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| 存量重复行导致加索引失败 | QA 等环境已存在楔死产生的重复行 | 第 4.1 节清理 SQL 先行；SELECT 佐证后再 DELETE；低峰执行 |
| 并发导出交错（读最新→插入窗口） | 两请求读到同一最新行，候选版本相同 | 唯一索引拦截后到者；重试路径 +1s 重读重插；上限 3 次 |
| 多实例部署下时钟回拨/漂移 | NTP 回拨使 `time.Now()` 小于库内最新版本 | 单调抬升逻辑天然免疫（候选永远 ≥ 最新版本 + 1s，不依赖本地时钟前进） |
| 重试耗尽 | 极端竞争下 3 次重试均冲突 | 返回错误并记录日志；导出请求失败可重入（每请求全量重建，无半状态），优于静默楔死 |
| 清理误删版本行 | 重复行清理保留 id 最大行，若执行计划误判... | 清理语句按 `(name, version)` 分组保留 MAX(id)，与修复后 `ORDER BY version DESC, id DESC` 读取语义一致；清理前备份 |

## 8. 上线前检查清单

1. **佐证**（可选）：在目标库执行重复行 SELECT，确认影响范围；
2. **清理**：执行第 4.1 节 DELETE（保留 MAX(id) 行）；
3. **加约束**：执行第 4.2 节 `ALTER TABLE` / `CREATE UNIQUE INDEX`；
4. **升级**：滚动升级 `ai-gateway-api`；
5. **验证**：SC2101-TC018 requeue 通过；`tc018_repro.py` 输出 CONVERGED；重复行佐证 SQL 返回空；
6. **回滚预案**：回滚代码即可（旧代码在无唯一索引库上行为不变；若已加唯一索引，旧代码同秒碰撞时 INSERT 会 500 而非楔死——错误显式化，可监控发现，重复行不再产生）。
