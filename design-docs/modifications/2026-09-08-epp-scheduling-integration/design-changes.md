# EPP 调度对接：ai-gateway-api 控制面设计变更说明

> 本文档只描述 `ai-gateway-api` 控制面为 EPP 调度对接所做的改造。EPP 侧设计见 `document-ai-gateway/EPP分析/ai-gateway-epp/` 系列文档，BFE 侧改造见 `bfe/docs/zh_cn/modifications/2026-09-06-epp-ai-gateway-integration/design-changes.md`。上游改造方案见 `document-ai-gateway/迭代系统设计/v0.6/EPP支持/ai-gateway-api/ai-gateway-api改造方案-EPP调度对接.md`。

## 1. 概述

### 1.1 变更目标

1. cluster 新增显式 `balance_mode` 参数（默认 `WRR`，可选 `EPP`），作为 EPP 模式的 OpenAPI 配置入口。
2. cluster 新增 `epp_config` 字段（**简化用户形态**：调度档位 + 调优参数 + 流控，导出时编译为完整 `EndpointPickerConfig`），`balance_mode=EPP` 时必填并生效、`WRR` 时可保留休眠（切换模式不丢配置），随 `/clusters` 配置（替代上游方案的独立 CRUD 域与原始结构透出）。
3. 新增 `epp_data` 统一下发端点：epp_config 与 assignment 全量视图合并导出（单 topic、单 version 快照）。
4. 新增 OpenAPI `/epp-pool`（单例，`GET` + `PATCH` 全量替换），维护 EPP 实例池（实例组 + 实例列表），替代上游方案中的 InnerAPI 注册/心跳方式。
5. 新增 EPP 实例组与分配模型，EPPAddr 生成由静态派生改为分配驱动。
6. 新增 OpenAPI 管理面（epp-assignments 查询与手工覆写）。

### 1.2 可复用基础（代码事实，已核实）

| 能力 | 位置 | 说明 |
|------|------|------|
| InnerAPI 框架 | `endpoints/innerapi_v1/endpoints.go:33-57` | 9 个导出端点已就绪，endpoint→xreq.Convert→Manager 模式 |
| 版本化下发框架 | `model/iversion_control/version_control.go:84-111` | `ExportConfig(ctx, topic, generator)` 事务内生成→MD5 签名→与 `config_versions` 表比对，变了才发新 version；新 topic 零框架改动 |
| token 认证 | `endpoints/middleware/user_probe.go:26-48` | `Authorization: Token xxx`，逐端点 `Authorizer` 沿用 `FeatureRoute + ActionExport` |
| 单例池接口模式 | `endpoints/openapi_v1/bfe_pool/update.go:31-36`、`design-docs/api-define/OpenAPI接口定义/alb-pool.md` | `/alb-pool` 单例资源：`GET` 详情 + `PATCH` 全量替换实例列表；`/epp-pool` 直接复用该模式 |
| EPP 静态建模 | `model/icluster_conf/pool.go:129-143` | `ProductPoolRoleEPP`、`EPPServer{Domain,Port,Endpoints}`、`EPPEndpoint{IP,Port}` |
| EPPAddr 生成 | `model/icluster_conf/cluster.go:1183-1184,1362-1382` | `buildEPPAddrsFromSubClusters` 从 EPPServer 派生 `GslbBasic.EPPAddr` |
| cluster_table 导出 | `model/icluster_conf/exporter.go:44-110` | EPP 从中获取 model server 实例列表（本方案不变更） |

### 1.3 缺口

| 缺口 | 现状 |
|------|------|
| cluster 无显式 EPP 开关 | `BalanceMode` 由 SubCluster 的 pool `Role` 派生（`cluster.go:295-302`），而设置 `Role=EPP` 的旧 `/instance-pools` 端点已移除（`endpoints/openapi_v1/product_pool/endpoints.go` 注册表为空），无任何 OpenAPI 可达路径；`POST /clusters` 创建的 pool 固定 `Role=COMMON`（`cluster.go:450`） |
| EPP 实例列表无 OpenAPI 配置入口 | 唯一能写 `pools.epp_server`（`EPPServer{Domain,Port,Endpoints}`）的是旧 `/instance-pools` 端点（`product_pool/create.go:53` 的 `epp_server` 参数），已随上述端点一并移除；现有 `PATCH /alb-pool`（`bfe_pool/update.go:31-36`）只更新内置 BFE pool 的 `Instances`，不触碰 `EPPServer`。EPPAddr 现由 `pools.epp_server` 派生（`cluster.go:1367`），该数据现已无 OpenAPI 可写 |
| EPP 下发通道 | `epp_config` Go 代码零出现（已核实），assignment 下发不存在 |
| EPP 实例组/分配模型 | 不存在。EPP 实例当前只是 pool 内的静态配置（EPPServer），无实例组、无角色、无序 |
| `EPPAddr` 有序主备语义 | 字段已生成但无语义文档、无按序分配 |
| 失效检测/再平衡 | 无 |

---

## 2. cluster `balance_mode` 与 `epp_config` 字段（P0）

### 2.1 `balance_mode` 设计

cluster 新增显式 `balance_mode` 字段（OpenAPI JSON 字段名 `balance_mode`），替代现状的 SubCluster Role 派生：

| 项 | 设计 |
|----|------|
| 存储 | `clusters` 表新增 `balance_mode` varchar 列，默认 `'WRR'`；`model/icluster_conf.Cluster` 结构体新增同名字段 |
| 枚举 | `WRR`（默认）、`EPP` |
| OpenAPI | `POST /clusters` 与 cluster 更新接口新增可选参数；GET/List 响应返回该字段（契约见 `api-changes.md` §3.2） |
| 导出 | `getBalanceMode()`（`cluster.go:295-302`）改为读取显式字段，直接决定 `GslbBasic.BalanceMode`（`cluster.go:1170`） |

`getBalanceMode()` 简化为直接读取显式字段（列默认 `'WRR'`，字段恒有值），删除 Role 派生逻辑——`balance_mode` 是 EPP 模式的唯一判定来源。新列带默认值，存量数据无需迁移。

### 2.2 `epp_config` 设计（简化用户形态）

| 项 | 设计 |
|----|------|
| 存储 | `clusters` 表新增 `epp_config` JSON 列（可空），存**用户形态的简化配置**（保留原始字段，未显式设置的 key 不落盘，以区分 `cache_affinity` 缺省"跟随档位"与显式覆盖）；`Cluster` 结构体新增 `EppConfig *EppConfigSimplified` 字段 |
| 形状 | **简化用户形态**：`scheduling_profile`（档位）+ `cache_affinity` / `kv_cache_utilization_max`（调优）+ `prefix_cache_affinity` / `session_affinity_enabled` + `session_affinity_header`（亲和特性开关）+ `flow_control`（流控），字段定义见 `api-changes.md` §3.2.1；不直接暴露 llm-d `EndpointPickerConfig` 插件声明细节 |
| OpenAPI | 随 `POST/PUT /clusters` 一并传入、校验、更新；GET/List 响应原样返回用户形态（契约见 `api-changes.md` §3.2） |
| 校验 | `balance_mode=EPP` ⇒ **必填**并生效；`balance_mode=WRR` ⇒ 可选（传入则保留并做格式校验，不编译、不导出，休眠）；字段级校验（枚举、数值范围、秒数取值范围）全部 api 侧强制 |
| 编译 | epp_data generator 在导出时把简化配置**确定性编译**为完整 `EndpointPickerConfig`（注入 `cluster-table-discovery`、`utilization-filter`、scorer、`max-score-picker`，按档位/参数展开权重与流控段）；编译规则固化在代码模板中、单测覆盖，正常路径出厂即合法 |
| 生效 | 仅 `balance_mode=EPP` 时编译下发；`WRR` 时保留休眠，EPP ↔ WRR 切换不丢配置 |
| 停用 | 将 `balance_mode` 改回 `WRR` 即可（无需清空 `epp_config`） |
| 高级模式 | **本期不提供**原样透传；如未来出现自定义插件链需求，届时再开放 |

**为什么不直接暴露 `EndpointPickerConfig`**：那是 llm-d 的内部实现抽象（插件实例、pluginRef 引用图、DAG 层序、Quantity 格式），用户侧配置它意味着理解整套插件体系，且 api 只能做 JSON Schema 结构校验，引用错误要到 EPP 编译期才暴露。改为"档位 + 调优参数"后：用户表达成本从"组装插件链"降到"选一个档位、填几个数字"；api 从"做不了引用校验"变成"配置出厂即合法"（模板自生成、引用/DAG 由构造保证）；EPP 侧消费格式不变。档位语义（latency-first / balanced / throughput-first）与 scorer 权重映射见 `api-changes.md` §3.2.1 编译规则表。

**为什么不采用上游方案的独立建表 + 独立 CRUD 域**：epp_config 是 per-cluster 配置且与 `balance_mode` 强耦合——EPP 模式必须有调度配置，非 EPP 模式不应有。挂在 `/clusters` 下可在 cluster 写入时原子强制该校验，杜绝"EPP cluster 无调度配置"的悬空状态与"非 EPP cluster 残留 EPP 配置"的孤儿数据；管理面也收敛到单一资源。变更面分析：epp_config 的变更几乎总是与 cluster 创建/模式变更同步发生，独立 CRUD 带来的灵活性与它引入的跨资源一致性成本不对等。

### 2.3 创建 / 更新流程

- **创建（`POST /clusters`）**：
  - `balance_mode` 缺省 / `WRR`：`epp_config` 可选（传入则保留休眠）；创建路径与现状一致（`Role=COMMON` 实例池）。
  - `balance_mode=EPP`：`epp_config` 必填；创建的 pool `Role=EPP`，复用现有 EPP 分支校验（`cluster.go:498` 单 subcluster 约束、`cluster.go:854` EPP 跳过实例池校验）；**池仍同步 provider `instance_pool` 实例**（与更新路径一致；`Role=EPP` 仅标记类型）——这些实例供两处消费：① cluster_table 导出（EPP 的 `cluster-table-discovery` 靠它发现推理后端，§1.2"EPP 从中获取 model server 实例列表（本方案不变更）"）；② EPP 全挂时 BFE 回退本地 WRR 的兜底后端。`BalanceMode=EPP` 下 BFE 不做本地 WRR，实例列表不参与正常均衡；`llm_config` 校验照常。
- **更新**：
  - `WRR → EPP`：须同时提供合法 `epp_config`（此前休眠保留的配置可继续用，也可一并更新）；即时 bump server_data_conf version。
  - `EPP → WRR`：仅改 `balance_mode` 即可，`epp_config` 与分配记录保留休眠，再切回 `EPP` 时继续生效（休眠保留的 `epp_config` 非空仍须保持字段合法——校验与 `balance_mode` 无关）。
- **变更影响面**：epp_config / assignment 下发范围、EPPAddr 生成路径（§4.3）、server_data_conf 导出校验（EPP 模式 cluster 无分配时拒绝）统一以 `cluster.balance_mode == EPP` 判定。
- **实现修正（2026-09-08，SC28 五组件集成测试 TC-07 暴露）**：provider 实例池同步钩子 `ProviderInstancePoolSyncer`（`model/icluster_conf/cluster.go`）原实现对 `Role=EPP` 的 subcluster 实例池**跳过同步**，导致 `PATCH /providers` 修改 `instance_pool`（如某实例 `weight=0` 摘流）后，EPP cluster 的池实例不更新、cluster_table 导出仍旧值，EPP `cluster-table-discovery` 与 BFE 兜底 WRR 均感知不到摘流——与本节"池仍同步 provider 实例（与更新路径一致）"的设计意图矛盾（创建路径已对齐，更新路径漏改）。已修复为 EPP 池同样同步，并同步修正单测（`skips EPP sub-clusters` → `syncs EPP sub-clusters`）。

---

## 3. epp_data 统一下发（P0，epp_config + assignment 合并）

- **端点**：`GET /inner-api/v1/configs/epp_data/config?version=...`（接口契约见 `api-changes.md` §2.1）。
- **topic**：新增 `ConfigTopicEppData`，复用 `ExportConfig` 框架（version 格式 `20060102150405`，未变化返回 `Data: null`，与现有接口行为一致）。
- **形状**：

```json
{
    "Version": "20260906120000",
    "Config": {
        "epp_config": { "cluster-a": { "...": "编译后的 EndpointPickerConfig（由简化配置确定性展开）" } },
        "assignment": {
            "cluster-a": { "primary": "epp-a", "standby": "epp-b" },
            "cluster-c": { "primary": "epp-c", "standby": null }
        }
    }
}
```

- **epp_config 段**：数据源为 `clusters` 表 `balance_mode=EPP` 的 cluster 的 `epp_config` 列（§2.2，用户形态简化配置），generator 在导出时编译为完整 `EndpointPickerConfig`（编译规则见 `api-changes.md` §3.2.1）。OpenAPI 校验保证 EPP 模式 cluster 配置必填，导出结果中 EPP cluster 必然有配置。
- **assignment 段（全量视图）**：`map[cluster名]{primary, standby}`，**所有 EPP 实例返回完全相同的内容**；范围仅含 `balance_mode=EPP` 的 cluster（`EPP → WRR` 后残留的分配记录休眠保留、不进导出）；`primary`/`standby` 为 `/epp-pool` 实例 id，单实例组时 `standby` 为 `null`；数据源为 `epp_assignments` + `epp_instances` 的读时 join。
- **EPP 侧消费**：以自身 `-instance-id` 逐 cluster 匹配 `primary`/`standby` 得出角色；未命中跳过；`epp_config` 有而 `assignment` 无条目的 cluster = 未分配异常态（EPP 本地告警）。

**为什么合并单端点、单 topic**（与上游"两个端点 + 两个独立 topic"不同）：

1. **原子快照**：cluster 创建、模式变更、手工覆写往往同时改变两段配置，单 topic 天然保证 EPP 拿到同一 version 的 epp_config 与 assignment，消除跨 topic 版本偏移导致的角色/配置错配窗口。
2. **代价可忽略**：数据量极小（两段 × 几十个 cluster），任一段变化全量重发的增量成本可忽略，"变更频率不同"不构成拆分理由。
3. **实现简化**：generator 单事务内生成两段、一次 MD5 签名；EPP 轮询端点减半。

---

## 4. EPP 实例组与分配模型（P0）

### 4.1 数据模型（新增表）

```
epp_instances:  id, host, port, group_name, 创建/更新时间；id 池内全局唯一；UNIQUE(host, port)（池内地址唯一）
epp_assignments: cluster(唯一), group_name, primary_instance_id
```

说明：

- 实例**无** `status` / `last_heartbeat` 字段——存活感知在 BFE 侧（EPPAddr 连接滞回），api 侧不做存活标记（§5）。
- `clusters` 表仅新增 `balance_mode`、`epp_config` 两列（§2），其余不动；cluster ↔ EPP 实例组的关联全部走 `epp_assignments`。
- 不复用 `pools` 表存 EPP 实例：pool/SubCluster 机制保留给 BFE 实例池，新实例池独立存储，避免两套语义纠缠。
- 分配**只存主**：`epp_assignments` 仅记 `primary_instance_id`，standby 为读时展开（同组除 primary 外的实例），避免双写不一致；管理面全量视图（`GET /api/v1/epp-assignments`）即两者的读时 join，不一致时标记 degraded（契约见 `api-changes.md` §3.3.1）。

### 4.2 分配器（第一期简化，与《EPP主备池化部署方案》一致）

- 实例组固定 2 实例（测试环境允许 1 实例组/单实例：所有 cluster 仅有主、无备）。
- 分配输入：实例组列表（来自 `/epp-pool` 配置的 `epp_instances`）、cluster 列表（`balance_mode=EPP`）；输出写入 `epp_assignments`。
- 分配变更即时生效——下次 server_data_conf 导出时自然带出。
- 约束校验：
  - 生产组必须 2 实例完整；
  - 主备同地址拒绝；
  - `balance_mode=EPP` 的 cluster 无有效分配（无记录或记录悬空）时，导出 server_data_conf **降级导出**：该 cluster 的 `BalanceMode` 置为 `WRR`、不生成 `EPPAddr`，同时 ai-gateway-api 输出 **error 级日志**（含 cluster 名与降级原因）。降级后 BFE 以**本地 WRR** 调度该 cluster 池中的 provider 实例（请求仍可服务，仅失去 EPP 智能调度），属显式降级而非静默错误；单 cluster 降级不阻塞整份 server_data_conf 下发。分配恢复后下轮导出自动回到 `EPP`。

#### 4.2.1 分配是自动生成的

`epp_assignments` 由**分配器自动生成**，`PUT /api/v1/epp-assignments/{cluster}` 手工覆写作为运维兜底（故障转移确认、特殊布局）。管理面 `GET /api/v1/epp-assignments` 视图与 epp_data 的 assignment 段的数据来源即分配器输出 + 手工覆写结果。

**触发时机（P0）**：

| 触发点 | 行为 |
|--------|------|
| cluster 创建为 `balance_mode=EPP`，或 `WRR → EPP` 变更 | 无分配记录 → 自动分配（算法见 4.2.2） |
| `/epp-pool` PATCH 后分配悬空（primary 实例已被移出池） | 自动修复：组仍存在 → 同组剩余实例中重选 primary（不换组）；组已不存在（或规模不满足部署形态要求）→ **跨组重分配**（对整个池重跑贪心算法选新组）；池无可分配候选组 → 清除分配（未分配态，导出时降级 `WRR` + error 日志，容量恢复后自动修复路径重新分配） |
| P2 失效检测信号就绪后 | 叠加失效实例触发的主动翻转（先翻备观察再确认），算法届时扩展 |

#### 4.2.2 自动分配算法（P0：贪心 + 确定性 tie-break）

```
输入：实例池（组→实例列表，来自 epp_instances）、现有分配（epp_assignments）、目标 cluster

1. 候选组过滤：实例数满足部署形态要求（生产=2，测试≥1）
2. 选组：组负载 = 组内各实例"作为 primary 承担的 cluster 数"之和
        → 选负载最小的组；并列取组名字典序最小（组间均衡）
3. 选主：组内选"作为 primary 承担的 cluster 数最少"的实例
        → 并列取实例 id 字典序最小（组内均衡）
4. 写入 epp_assignments（cluster 唯一键 upsert）
```

设计取舍：

- **确定性**：全程字典序 tie-break，无随机；结果可重放、可单测；同一输入多次执行结果一致。
- **无新增并发控制**：cluster 创建/更新路径本身串行，`/epp-pool` PATCH 触发的修复量小且幂等；P0 不为分配器引入额外分布式锁（如有并发分配竞争，唯一键约束兜底，失败方重试读取后再分配）。
- **互为主备是合法输出**：同组两实例在不同 cluster 间互为主备（cluster-a 主=epp-a、cluster-b 主=epp-b）是该算法的自然结果——分配以 cluster 为单位，不做组↔cluster 一对一约束。
- 反亲和（同 host 不共主备/不共组）为 P2 约束，届时作为选组打分项加入。

### 4.3 EPPAddr 生成逻辑改造

`model/icluster_conf/cluster.go:1362-1382` 的 `buildEPPAddrsFromSubClusters` 被替换为"按 `epp_assignments` 查 cluster 的 `{primary, standby}` 生成有序 `EPPAddr`"（`[0]`=主、`[1]`=备）：`cluster.balance_mode == EPP` 且有有效分配时按分配生成；**无有效分配时降级导出**——该 cluster `BalanceMode` 置为 `WRR`、不生成 `EPPAddr`，同时输出 error 级日志（含 cluster 名与原因），不阻塞整份 server_data_conf 下发。实例地址由 `epp_instances` 的 host/port 经 `net.JoinHostPort` 拼接。

---

## 5. EPP 实例池管理：`/epp-pool`（P0，替代注册/心跳）

上游方案采用 InnerAPI `register` / `heartbeat` 自注册方式；本期**改为 OpenAPI `/epp-pool` 全量配置**，模式对齐 `/alb-pool`（单例资源、`GET` 详情 + `PATCH` 全量替换，接口契约见 `api-changes.md` §3.1）。

### 5.1 设计要点

| 项 | 设计 |
|----|------|
| 资源形态 | 单例池，池名由配置项提供（建议 `RunTime.DefaultEPPInstancePoolName`，默认值如 `EPP.pool`），请求不传 `name` |
| 数据形状 | `{"name": ..., "groups": [{"name": "g1", "instances": [{"id": "epp-a", "host": "10.0.0.1", "port": 9002}]}]}`（host/port 分字段，与项目 Instance 模型风格一致；IPv6 字面量不带括号） |
| 存储 | 展开写入 `epp_instances` 表（id、host、port、group_name） |
| 写入方式 | `PATCH` 全量替换：部署流程在实例变更（扩缩容、换机）后调用；不提供增量接口 |
| 校验 | 组名非空唯一；实例 id 池内全局唯一；`(host, port)` 组合池内全局唯一（host 为 Hostname 或 IP，port 为合法端口）；生产环境每组恰 2 实例（测试环境允许单实例组，校验强度由部署形态配置项控制） |
| 下发拼接 | 生成 EPPAddr 时以 `net.JoinHostPort(host, strconv.Itoa(port))` 拼为 `host:port`（IPv6 自动加括号），BFE `EPPAddr` 的消费格式不变 |

### 5.2 为什么用静态配置替代自注册

- **与既有管理面一致**：`/alb-pool` 已确立"单例池 + 全量替换"的运维模式，EPP 实例池照此办理，管理面行为可预期，无需新增"实例自发写库"的反向通道及幂等、鉴权、时钟同步等一系列问题。
- **实例列表是部署事实**：EPP 实例由部署系统拉起/下线，列表权威来源是部署流程而非进程自身；PATCH 全量替换天然幂等，与部署系统的 reconcile 循环契合。
- **api 侧无存活管理负担**：自注册方案需要心跳续约、失联阈值、Scheduler 分布式锁等一整套机制；改为静态配置后 P0 全部不需要。实例存活与 failover 由 BFE 侧 EPPAddr 连接滞回判断驱动，api 只保证配置与分配视图稳定下发。
- **保留演进空间**：若 P2 需要 api 侧主动失效检测，可在此基础上叠加心跳或 BFE 健康上报，信号来源届时另定，不与本期耦合。

### 5.3 与上下游的衔接

- **对 EPP**：实例身份即池中实例 id，EPP 以 `-instance-id` 启动参数（默认 hostname）在 epp_data 的 assignment 全量视图中定位自身角色；id 不在池中则未命中任何角色（fail-static，保留本地已知角色）。
- **对分配器**：分配输入直接读 `epp_instances`（§4.2）；`/api/v1/epp-assignments/{cluster}` PUT 手工覆写时校验 primary 存在于对应组实例列表。
- **对导出**：实例池变更不 bump `ConfigTopicEppData` topic（cluster→role 映射未变）；EPPAddr 由 `epp_assignments` 驱动，实例池变更仅在导致分配悬空修复时才间接影响导出。

---

## 6. 再平衡与失效检测（P2）

- **P2 主动失效检测**：失效实例触发的自动分配翻转（先翻备观察健康再确认）+ 加组扩容与组间再平衡（再平衡不设独立端点，随失效检测/扩容流程内触发或经 `PUT /api/v1/epp-assignments/{cluster}` 手工覆写）+ 反亲和分配约束。
- **前提**：失效检测/就绪信号来源届时在"EPP 上报通道 / 心跳 / BFE 健康回调"中统一设计——`/epp-pool` 静态配置模式下 api 侧无存活数据，P2 需先补信号通道。原 P1 的"就绪感知翻转""双活跃观测"依赖该信号通道，一并顺延至信号通道确定后评估。

---

## 7. 数据库变更

| 表 | 变更 | 文件 |
|----|------|------|
| `clusters` | 新增 `balance_mode` 列（varchar，默认 `'WRR'`，取值 `WRR`/`EPP`）；新增 `epp_config` 列（JSON text，可空） | `db_ddl.sql`、`db_ddl_sqlite.sql` |
| `epp_instances` | 新增（id，host，port，group_name，创建/更新时间；`UNIQUE(host, port)` 池内地址唯一） | 同上 |
| `epp_assignments` | 新增（cluster 唯一键，group_name，primary_instance_id） | 同上 |

`pools.epp_server` 列保留（本期无消费方，后续版本清理）。无 migration 框架，全量 DDL 手工同步；新列带默认值/可空，存量数据无需迁移。

---

## 8. 涉及文件清单

| 文件 | 改造点 |
|------|--------|
| `endpoints/innerapi_v1/endpoints.go` | 注册 `epp_data/config` 端点 |
| `endpoints/innerapi_v1/epp_data/`（新增） | epp_data 导出 handler（epp_config + assignment 两段生成） |
| `endpoints/openapi_v1/epp_pool/`（新增） | `/epp-pool` GET/PATCH（模式照 `bfe_pool/`） |
| `endpoints/openapi_v1/epp_assignments/`（新增） | OpenAPI assignments 查询与手工覆写 |
| `endpoints/openapi_v1/product_cluster/create.go`、`update_basic.go` | `/clusters` 请求体新增 `balance_mode`、`epp_config` 参数与校验 |
| `model/iversion_control/version_control.go` | 复用，新增 `ConfigTopicEppData` topic |
| `model/icluster_conf/cluster.go:295-302` | `getBalanceMode()` 简化为直读 `balance_mode` 字段（默认 `WRR`），删除 Role 派生逻辑 |
| `model/icluster_conf/cluster.go:450` | 创建流程按 `balance_mode` 生成 `Role=COMMON`/`Role=EPP` 的 pool |
| `model/icluster_conf/cluster.go:1362-1382` | EPPAddr 生成改为分配驱动（旧 EPPServer 派生逻辑删除） |
| `model/iroute_conf/exporter.go:67-166` | server_data_conf 导出接入分配处理（EPP 模式 cluster 无有效分配时降级为该 cluster `BalanceMode=WRR` + error 日志，不阻塞整份下发） |
| `model/epp_pool/`（新增）+ `storage/rdb/epp_pool/`（新增） | 实例池读写、分配器、epp_data generator（含简化 epp_config → EndpointPickerConfig 编译模板） |
| `lib/validate/validate.go` | `balance_mode` 枚举、`epp_config` 简化字段校验（枚举/范围/duration，随 balance_mode 条件必填规则） |
| `db_ddl.sql` / `db_ddl_sqlite.sql` | `clusters` 新增两列 + 两张新表 |

---

## 9. 测试计划

### 9.1 验收标准（checkable）

1. EPP 实例以其 `-instance-id`（与 `/epp-pool` 配置一致）经 InnerAPI 拉取到 epp_data（epp_config + assignment 全量视图）与 cluster_table，version 增量生效（重复拉取返回 `Data: null`）；所有实例拿到的 assignment 内容一致。
2. `POST /clusters` 携带 `balance_mode=EPP` + 合法 `epp_config` 创建的 cluster：server_data_conf 中 `BalanceMode=EPP`、`EPPAddr = [主, 备]`（分配器自动分配）；epp_data 导出包含该 cluster 的 epp_config（编译后形态，与档位/参数预期一致）与 assignment 条目；缺 `epp_config` 时创建返回 422。
3. epp_config 变更（`PUT /clusters`）后，EPP 在下一同步周期拿到新配置并热加载生效（EPP 侧责任，本方案提供下发）。
4. `EPP → WRR` 变更：仅改 `balance_mode` 即成功；导出 `BalanceMode=WRR`、EPPAddr 为空；`epp_config` 与分配记录保留（GET 回读不变、不进 epp_data 导出，非空配置仍须字段校验通过）；再 `WRR → EPP` 时原配置与原分配继续生效。

### 9.2 单元测试

- `getBalanceMode()`：直读 `balance_mode` 字段；缺省/空值为 `WRR`；Role 派生逻辑已删除、无引用。
- cluster 创建/更新校验：`balance_mode=EPP` 缺 `epp_config` 拒绝；`epp_config` 非空即做简化字段校验（枚举、范围、秒数取值范围），**与 `balance_mode` 无关**（含 WRR 休眠保留与 `EPP → WRR` 随带场景）；休眠语义（EPP→WRR 保留、切回继续生效、不进导出）。
- cluster 创建：`balance_mode=EPP` 生成 `Role=EPP` pool 且**池同步 provider 实例**（cluster_table 导出与兜底 WRR 依赖；BFE 在 EPP 模式下不做本地 WRR）；缺省与显式 `WRR` 走现状路径。
- `/epp-pool` PATCH：全量替换语义、id 唯一性校验、`(host, port)` 组合唯一性校验、host/port 格式校验、组规模校验（生产 2 实例 / 测试允许单实例）、空组拒绝；PATCH 后 GET 回读一致；PATCH 触发分配悬空自动修复。
- epp_data generator：epp_config 段数据源为 `clusters` 表、EPP cluster 必有配置；简化配置 → EndpointPickerConfig 编译正确性（档位权重映射、cache_affinity 覆盖、prefix/session 亲和 scorer 条件注入与固定权重、flow_control 展开、featureGates 追加、固定插件注入）；assignment 段 cluster→{primary, standby} 读时 join 正确、单实例组 `standby=null`；version 不变返回 `Data: null`。
- 分配器：生产组 2 实例完整性校验、主备同地址拒绝、贪心选组选主的确定性（同输入同输出、负载均衡断言）、EPP 模式 cluster 无有效分配时导出降级（`BalanceMode=WRR` + error 日志，不阻塞整份下发）。
- EPPAddr 生成：分配驱动有序输出 `[主, 备]`、host/port 经 `net.JoinHostPort` 拼接；EPP 模式 cluster 无有效分配时导出降级不报错。

### 9.3 集成测试

- OpenAPI：`PATCH /epp-pool`（配置实例组）→ `POST /clusters`（`balance_mode=EPP` + `epp_config`）→ `GET /epp-assignments`（验证自动分配）→ server_data_conf 导出有序 EPPAddr、epp_data 导出包含两段配置。
- InnerAPI：epp_data 增量拉取（重复拉取返回 `Data: null`）；assignment 全量视图对所有实例一致。

---

## 10. 风险与注意事项

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| /epp-pool 配置与部署事实漂移 | 静态配置模式下，实例实际下线但未更新 /epp-pool 时，api 侧无感知 | 部署流程负责 reconcile（变更后 PATCH）；BFE 侧 EPPAddr 滞回容错；P2 补失效检测信号通道 |
| cluster 更新面变大 | `balance_mode`/`epp_config` 并入 `/clusters` 后，cluster 写入的校验与冲突面增大 | 两字段校验内聚在 cluster 写入路径；epp_config 校验为简化字段（枚举/范围/duration），错误信息指明字段路径 |
| epp_config 与 balance_mode 不一致 | 独立 CRUD 模式下可能出现的悬空/孤儿配置 | 本期已消除：单一写入口 + 条件必填校验（EPP⇒必填、WRR⇒可选休眠保留；非空一律做字段校验），写入时原子强制 |
| 双 EPP 判定来源 | `balance_mode` 字段与 pool `Role` 并存，可能不一致（如手工改库） | `balance_mode` 是唯一判定来源（`getBalanceMode()` 直读字段，Role 派生已删除）；OpenAPI 是唯一写入口，创建/更新时保持两者一致（`balance_mode=EPP` ⇔ pool `Role=EPP`） |
| 分配变更与 EPP 实际角色的时序 | 分配即时变更但 EPP 下一同步周期才应用 | P0 由 BFE 侧滞回吸收短暂不一致；若 P2 引入 api 主动翻转，届时结合信号通道设计"先同步后翻转"约束 |
| 合并 topic 的全量重发 | 任一段变化会 bump version，另一段随之重发 | 数据量极小，代价可忽略；换来的是两段配置的原子快照 |
| server_data_conf 降级导出 | EPP 模式 cluster 无有效分配时导出行为需明确定义 | 降级导出：该 cluster `BalanceMode=WRR`、不生成 EPPAddr，同时输出 error 级日志（含 cluster 名与原因）；不阻塞整份下发，分配恢复后自动回到 `EPP`；降级期间该 cluster 无后端可调度，需靠 error 日志 + `unassigned_clusters` 监控告警发现 |
