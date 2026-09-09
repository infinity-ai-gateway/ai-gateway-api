# EPP 调度对接（ai-gateway-api 控制面改造）变更摘要

## 1. 背景

目标架构为 **ai-gateway-epp（多 cluster 调度器）+ BFE（ext-proc 接入）**。在该架构下，ai-gateway-api 需要承担三个新职责：

1. **EPP 配置下发**：新增 `epp_data` InnerAPI，统一下发 epp_config（调度配置）与 assignment（实例角色全量视图），version 增量同步。
2. **EPP 实例组分配与主备下发**：EPP 以"实例组（每组 2 实例互为主备）"为部署单元，ai-gateway-api 为每个 cluster 分配 `{group, primary}`，并翻译为 BFE 消费的 `GslbBasic.EPPAddr` 有序列表（`[0]`=主、`[1]`=备），随 server_data_conf 下发。
3. **EPP 实例池管理**：实例与实例组由 OpenAPI `/epp-pool` 全量配置（参考 `/alb-pool` 单例模式），替代 InnerAPI 注册/心跳方式。

上游改造方案见 `document-ai-gateway/迭代系统设计/v0.6/EPP支持/ai-gateway-api/ai-gateway-api改造方案-EPP调度对接.md`。BFE 侧改造见 `bfe/docs/zh_cn/modifications/2026-09-06-epp-ai-gateway-integration/design-changes.md`。EPP 侧改造见 `ai-gateway-epp/docs/zh_cn/modifications/2026-09-08-epp-scheduling-integration/`。

## 2. 目标

- 在 OpenAPI `/clusters` 上新增 `balance_mode` 参数（默认 `WRR`，可选 `EPP`），作为 cluster 开启 EPP 模式的显式配置入口（现状为 SubCluster Role 派生，且设置 Role=EPP 的入口已被移除，无 OpenAPI 可达路径）。
- 在 OpenAPI `/clusters` 上新增 `epp_config` 字段（per-cluster，**简化用户形态**：调度档位 + 少量一等公民调优参数，api 导出时确定性编译为 llm-d `EndpointPickerConfig`）：`balance_mode=EPP` 时**必填**并生效；`WRR` 时可保留但休眠（不编译、不导出），EPP ↔ WRR 切换不丢配置；不引入独立的 epp-picker-config CRUD 域；本期不提供原样透传的高级模式。
- 新增 OpenAPI `/epp-pool`（`GET` 详情 + `PATCH` 全量替换），维护 EPP 实例池（实例组 + 实例列表），**替代** InnerAPI `register` / `heartbeat` 自注册方式。
- 新增 `GET /inner-api/v1/configs/epp_data/config` 导出端点：epp_config 与 assignment **合并下发**（单 topic、单 version 快照），assignment 为全量视图（所有 EPP 实例返回相同内容，由 EPP 按自身 id 选取角色）。
- 新增 EPP 实例组/分配数据模型（`epp_instances`、`epp_assignments` 两张新表），`EPPAddr` 生成由"静态派生"改为"分配驱动"（旧 EPPServer 派生逻辑删除）。
- 新增 OpenAPI 管理面：`epp-assignments` 查询与手工覆写。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要模块 | `endpoints/innerapi_v1/`、`endpoints/openapi_v1/`（新增 epp-pool / epp-assignments 域）、`model/iversion_control`、`model/icluster_conf`、`model/iroute_conf`、`model/epp_pool/`（新增）、`storage/rdb/epp_pool/`（新增） |
| 接口契约 | InnerAPI 新增 1 端点（`epp_data/config`）；OpenAPI 新增 `/epp-pool`（2 端点）、`epp-assignments`（2 端点）；既有 `/clusters` 新增可选字段 `balance_mode` 与 `epp_config`（缺省行为不变） |
| 数据库 | 新增 `epp_instances`、`epp_assignments` 两张表；`clusters` 表新增 `balance_mode` 列（默认 `WRR`）与 `epp_config` 列（JSON，可空） |
| BFE 影响 | server_data_conf 中 EPP 模式 cluster 的 `GslbBasic.EPPAddr` 变为有序主备列表（`[0]`=主、`[1]`=备） |
| EPP 影响 | EPP 经单端点增量拉取 epp_config + assignment 全量视图（以 `-instance-id` 匹配 primary/standby 得出自身角色）；prefix/session 亲和状态为 EPP 本地内存（LRU/binding），**无外部存储依赖**，failover 后重新收敛；无 EPP → api 方向的上报义务。EPP 无全局内存上限配置，内存边界靠容器/Pod 限额兜底；prefix 亲和 LRU 每后端有界（容量 = 后端 GPU block 数（autoTune）或默认 31250 blocks，约 5MB/后端），随集群后端数线性增长，是 EPP 内存主要消费者 |

## 4. 关键决策

| 决策 | 说明 |
|------|------|
| cluster 显式 `balance_mode` 参数 | OpenAPI `/clusters` 新增 `balance_mode`（默认 `WRR`，枚举 `WRR`/`EPP`），替代现状的 SubCluster Role 派生（`getBalanceMode`）。EPP 模式不再是"绑了 EPP pool 的副作用"，而是 cluster 的一等配置；epp_config / assignment 下发范围、EPPAddr 生成都以该字段为准。 |
| epp_config 随 `/clusters` 配置（替代独立 CRUD 域），采用简化用户形态 | 与上游方案"独立建表 + 独立 CRUD"不同：epp_config 是 per-cluster 配置且与 `balance_mode` 强耦合（EPP⇒必填并生效、WRR⇒可保留休眠），随 cluster 生命周期由 `POST/PUT /clusters` 一并创建、校验、更新，存储于 `clusters.epp_config` 列（JSON），GET 回读与写入一致。与上游方案"原样透出 llm-d `EndpointPickerConfig`"也不同：那是插件实例/引用图/DAG 层的内部实现抽象，用户侧无法正确组装、api 侧只能做结构校验；改为"档位（latency-first/balanced/throughput-first）+ 少量一等公民参数（cache_affinity、kv_cache_utilization_max、prefix_cache_affinity、session_affinity_enabled、session_affinity_header、flow_control）"，由 api 在 epp_data 导出时**确定性编译**为完整 `EndpointPickerConfig`（固定注入 cluster-table-discovery / utilization-filter / scorer / max-score-picker / openai-parser，档位映射 scorer 权重），出厂即合法。本期不提供原样透传的高级模式。校验在 cluster 写入时强制，杜绝"EPP cluster 无调度配置"的悬空状态；生效语义 = `balance_mode=EPP` 时编译下发、`WRR` 时休眠保留（停用 EPP 调度只需改回 `WRR`，无需清空配置）。 |
| epp_config 与 assignment 合并单端点下发 | 与上游方案"两个端点 + 两个独立 topic"不同：合并为 `GET /configs/epp_data/config`、单 topic `ConfigTopicEppData`。理由：数据量极小，全量重发代价可忽略；cluster 创建/分配变更往往同时改变两段，单 topic 天然保证两者同一 version 快照，消除跨 topic 版本偏移；EPP 轮询端点减半。 |
| assignment 全量下发（替代 per-instance 视图） | 与上游方案"按 `instance` 参数返回本实例视图"不同：所有 EPP 实例返回**完全相同**的 assignment（cluster → {primary, standby}），由 EPP 以 `-instance-id` 匹配得出自身角色。优点：全部实例共享同一 version 快照；api 无需校验实例身份，`instance` 参数、"未知实例 4xx"等边缘消失；EPP 顺带获知同组 peer，为备 Cell 建联预留信息。 |
| `/epp-pool` 替代实例自注册 | 不引入 InnerAPI register/heartbeat：EPP 实例与实例组由 OpenAPI `/epp-pool` 全量配置（单例资源，参考 `/alb-pool` 的 `GET` + `PATCH` 全量替换模式）。实例列表是部署事实的静态登记，由部署流程在实例变更后调用 PATCH 维护；实例身份即池中实例 id，EPP 以 `-instance-id` 启动参数（与池中 id 一致）在 assignment 全量视图中定位自身。 |
| P0 阶段 api 不感知实例存活 | 无心跳、无失联标记、无就绪上报：实例存活与 failover 由 BFE 侧 EPPAddr 连接滞回判断驱动，api 侧只保证配置与分配视图稳定下发。上游方案的 `assignment/report` 端点同步取消（其消费者均以 api 主动翻转为前提，与本设计不符；P2 如需要就绪/健康信号，届时在"上报通道 / 心跳 / BFE 健康回调"中统一设计）。 |
| 复用 ExportConfig 版本框架 | 新 topic `ConfigTopicEppData` 走既有"生成→MD5 签名→变化才发新 version"机制，零框架改动。 |
| EPPAddr 改为分配驱动 | `buildEPPAddrsFromSubClusters` 静态派生逻辑替换为按 `epp_assignments` 生成有序 `[主, 备]`；EPP 模式 cluster 无有效分配时导出降级为该 cluster `BalanceMode=WRR` + error 日志，不阻塞整份配置下发。 |
| 分配自动生成 + 手工覆写兜底 | `epp_assignments` 由分配器自动生成：cluster 进入 EPP 模式时按"贪心 + 确定性 tie-break"算法自动选组（组间负载均衡）选主（组内负载均衡）；`/epp-pool` 变更导致分配悬空时自动修复。`PUT /api/v1/epp-assignments/{cluster}` 仅作运维干预入口。算法详见 `design-changes.md` §4.2.2。 |
| 实例组固定 2 实例（测试环境允许单实例组） | 与《EPP主备池化部署方案》一致；单实例组 cluster 仅有主、无备，主失联时 BFE 侧降级本地均衡。 |
| EPP 模式 cluster 无有效分配时导出降级 | 原方案为导出校验拒绝（整份生成失败）；改为单 cluster 降级：`BalanceMode` 置为 `WRR`、不生成 `EPPAddr`，同时输出 error 级日志（含 cluster 名与原因），不阻塞整份 server_data_conf 下发。 |

## 5. 分期落地

| 期 | 内容 |
|----|------|
| P0 | cluster `balance_mode` + `epp_config` 字段 + `/epp-pool` 实例池管理 + epp_data 统一下发 + 实例组/分配模型 + EPPAddr 有序生成 + server-data-conf.md §3.3 文档（约 1 周） |
| P1 | （预留；视 P0 运行情况定） |
| P2 | 自动失效检测翻转（信号来源届时在"上报通道 / 心跳 / BFE 健康回调"中统一设计）、加组扩容、反亲和分配约束（视运行情况） |

## 6. 关联文档

- 上游改造方案：`document-ai-gateway/迭代系统设计/v0.6/EPP支持/ai-gateway-api/ai-gateway-api改造方案-EPP调度对接.md`
- 接口变更：本目录 `api-changes.md`
- 设计变更：本目录 `design-changes.md`
- `/alb-pool` 参考：`design-docs/api-define/OpenAPI接口定义/alb-pool.md`（单例 + 全量替换模式）
- 关联分析文档（`document-ai-gateway/EPP分析/`）：《ai-gateway-epp/EPP配置定义说明-picker_config.md》、《ai-gateway-epp/server-data-conf修改方案-EPPAddr字段语义.md》、《EPP改造方案讨论/EPP主备池化部署方案.md》
- EPP 侧消费实现参考：`ai-gateway-epp/pkg/poller/assignment.go`、`ai-gateway-epp/pkg/assignment/types.go`
