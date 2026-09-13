# EPP 调度对接：API 接口变更说明

## 1. 变更范围

| 接口类型 | 变更内容 |
|----------|----------|
| InnerAPI | 新增 1 端点：`/configs/epp_data/config`（epp_config + assignment 合并导出）；server_data_conf 响应中 `GslbBasic.EPPAddr` 语义变为有序主备列表 |
| OpenAPI | 新增 `/epp-pool`（GET/PATCH，替代实例自注册）、`epp-assignments`（2 端点查询与手工覆写）；既有 `/clusters` 新增可选字段 `balance_mode`（默认 `WRR`，可选 `EPP`）与 `epp_config`（EPP 模式必填） |
| 既有接口 | 无破坏式变更；`/clusters` 仅新增可选字段（缺省行为与现状一致）；`pools.epp_server` 列保留（本期无消费方，后续版本清理） |

新 InnerAPI 端点沿用通用约定（`InnerAPI接口定义/01-common.md`）：`Authorization: Token xxx`、`version` 增量同步（未变化返回 `Data: null`）、version 格式 `20060102150405`、错误响应包 `{ErrNum, ErrMsg, ...}`。鉴权为 `FeatureRoute + ActionExport`。

---

## 2. InnerAPI 新增端点

注册于 `endpoints/innerapi_v1/endpoints.go`，清单编号续 `InnerAPI接口定义/02-interface-list.md`：

| 序号 | 接口路径 | Method | 功能描述 | 特殊参数 | 鉴权 |
|---|---|---|---|---|---|
| 10 | `/configs/epp_data/config` | GET | 导出 EPP 配置（epp_config + assignment 全量视图，合并单端点） | `version` | `FeatureRoute + ActionExport |

单 topic：`ConfigTopicEppData`（`model/iversion_control/version_control.go` 的 `ExportConfig` 框架，新 topic 零框架改动）。

> **说明**：上游方案中的三个 InnerAPI 端点**不再实现**——`/epp/instances/register`、`/epp/instances/{id}/heartbeat` 由 OpenAPI `/epp-pool` 全量配置替代（§3.1），api 侧不做实例存活管理（详见 `design-changes.md` §5）；`/configs/epp_data/assignment/report` 取消，其消费者（就绪感知翻转、双活跃观测）均以 api 主动翻转分配为前提，与本设计（failover 由 BFE 滞回驱动）不符，P2 如需要就绪/健康信号届时统一设计。上游的 epp_config 与 assignment 两个导出端点合并为本端点（见 §2.1）。

### 2.1 10. epp_data 导出（epp_config + assignment 合并）

请求：`GET /inner-api/v1/configs/epp_data/config?version=00010101000000`

返回：

```json
{
    "ErrNum": 200,
    "ErrMsg": "success",
    "Data": {
        "Version": "20260906120000",
        "Config": {
            "epp_config": {
                "cluster-a": {
                    "featureGates": ["flowControl"],
                    "plugins": [
                        { "name": "ep-discover", "type": "cluster-table-discovery", "parameters": { "clusterName": "cluster-a" } },
                        { "name": "util-filter", "type": "utilization-filter",
                          "parameters": { "conditions": [ { "metric": "kv-cache-utilization", "maxValue": 0.9 } ] } },
                        { "name": "kv-scorer", "type": "kv-cache-utilization-scorer", "parameters": {} },
                        { "name": "max-score", "type": "max-score-picker", "parameters": {} }
                    ],
                    "schedulingProfiles": [
                        { "name": "default",
                          "plugins": [
                            { "pluginRef": "util-filter" },
                            { "pluginRef": "kv-scorer", "weight": 1.0 },
                            { "pluginRef": "max-score" }
                          ] }
                    ],
                    "dataLayer": { "discovery": { "endpoints": { "pluginRef": "ep-discover" } } },
                    "flowControl": { "defaultRequestTTL": "30s", "noEndpointRequestTTL": "10m" }
                }
            },
            "assignment": {
                "cluster-a": { "primary": "epp-a", "standby": "epp-b" },
                "cluster-b": { "primary": "epp-b", "standby": "epp-a" },
                "cluster-c": { "primary": "epp-c", "standby": null }
            }
        }
    },
    "WorkMode": "ModeNormal"
}
```

**`epp_config` 段**：

- `Config.epp_config` 为 `map[cluster名]EndpointPickerConfig`，由 OpenAPI 写入的简化 `epp_config`（§3.2）在导出时**确定性编译**而来（编译规则见 §3.2.1），出厂即合法。
- 范围：全部 `balance_mode=EPP` 的 cluster。OpenAPI 校验保证 EPP 模式 cluster 的 `epp_config` 必填（§3.2），因此导出结果中 EPP cluster 必然有配置。

**`assignment` 段（全量视图）**：

- `Config.assignment` 为 `map[cluster名]{primary, standby}`，**所有 EPP 实例返回完全相同的内容**（同一 version 快照）。
- `primary` / `standby` 为 `/epp-pool` 中配置的实例 id；单实例组（无备）时 `standby` 为 `null`。
- **EPP 侧消费方式**：以自身 `-instance-id`（须与 `/epp-pool` 中某实例 id 一致）逐 cluster 匹配——`primary == 本实例 id` → 本实例为该 cluster 的 primary；`standby == 本实例 id` → standby；均未命中 → 跳过该 cluster。
- 全量视图的附带收益：EPP 可获知同组 peer 实例 id（如备 Cell 建联、双活跃自检，后续如需）。
- 异常态：`epp_config` 中存在但 `assignment` 中无条目的 cluster = 未分配（server_data_conf 导出会拒绝该状态），EPP 侧应本地告警、不为该 cluster 建 cell。

**合并下发的理由**：数据量极小（两段配置 × 几十个 cluster），单 topic 全量重发代价可忽略；cluster 创建、模式变更、分配变更往往同时改变两段，单 topic 天然保证同一 version 快照，消除跨 topic 版本偏移；EPP 轮询端点减半。

### 2.2 server_data_conf：`EPPAddr` 有序主备语义

按《server-data-conf修改方案-EPPAddr字段语义.md》§2 草案，在 `InnerAPI接口定义/server-data-conf.md` 新增 §3.3：`GslbBasic.EPPAddr` 为有序列表，`[0]`=主、`[1]`=备。EPP 模式 cluster 无有效分配时**降级导出**：该 cluster `BalanceMode` 置为 `WRR`、不生成 `EPPAddr`，同时输出 error 级日志（含 cluster 名与原因），不阻塞整份 server_data_conf 下发。生成逻辑改造见 `design-changes.md` §4.3。

---

## 3. OpenAPI 变更（管理面）

沿用 openapi_v1 现有鉴权与 `xreq` 请求/响应模式。

### 3.1 /epp-pool（EPP 实例池管理，替代实例自注册）

单例资源，模式对齐 `/alb-pool`（参考 `design-docs/api-define/OpenAPI接口定义/alb-pool.md`）：池名由配置项提供（建议 `RunTime.DefaultEPPInstancePoolName`，默认值如 `EPP.pool`），请求中无需传入 `name`。

**实例 id 约定（部署形态）**：EPP 以 StatefulSet 部署，实例 id = **Pod hostname**（EPP 的 `-instance-id` 缺省值即 hostname，同组副本共享完全相同的启动参数，无需按实例差异化配置）；`/epp-pool` 登记时实例 id 取 pod 名（如 `epp-0`/`epp-1`），部署流程 reconcile 池时从 StatefulSet pod 名生成 PATCH 内容。前提：StatefulSet 保证 hostname 稳定唯一（不可用随机名的 Deployment）；非 K8s 部署显式传 `-instance-id`。

**数据模型**：

```json
{
    "name": "EPP.pool",
    "groups": [
        {
            "name": "g1",
            "instances": [
                { "id": "epp-a", "host": "10.0.0.1", "port": 9002 },
                { "id": "epp-b", "host": "10.0.0.2", "port": 9002 }
            ]
        }
    ]
}
```

| 端点 | Method | 说明 |
|---|---|---|
| `/api/v1/epp-pool` | GET | 获取 EPP 实例池详情（实例组 + 实例列表） |
| `/api/v1/epp-pool` | PATCH | 全量替换实例池（实例组 + 实例列表）；部署流程在实例变更后调用 |

**字段与约束**：

| 字段 | 类型 | 必填 | 说明 | 合法性条件 |
|------|------|------|------|------------|
| `groups` | []Group | Y | 实例组列表 | 至少 1 个元素；组名非空、唯一 |
| `groups[].instances` | []Instance | Y | 组内实例列表 | 至少 1 个元素 |
| `groups[].instances[].id` | string | Y | 实例 id（EPP 以 `-instance-id` 启动参数对应此值） | 非空；**池内全局唯一** |
| `groups[].instances[].host` | string | Y | 实例主机名或 IP（IPv6 字面量不带括号） | 非空；类型为 [Hostname](./00-common.md#公共参数类型) 或 IP |
| `groups[].instances[].port` | int | Y | 实例端口 | 类型为 [Port](./00-common.md#公共参数类型) |
| 地址唯一性 | - | - | 实例地址唯一性 | `(host, port)` 组合**池内全局唯一**（等价于旧 host:port 唯一语义） |
| 组规模 | - | - | 每组实例数 | 生产环境每组恰 2 实例（主备）；测试环境允许单实例组（仅主、无备）；由部署形态配置项控制校验强度 |

**执行逻辑（PATCH）**：校验 → 全量替换 `epp_instances` 表 → 返回更新后的池详情。实例池变更不直接 bump `ConfigTopicEppData` topic（实例增减不改变 cluster→role 映射）。

**存储与下发**：`epp_instances` 表存 `host`、`port` 两列；生成 EPPAddr 时以 `net.JoinHostPort(host, strconv.Itoa(port))` 拼接为 `host:port`（IPv6 自动加括号），BFE 消费格式不变。

**实例状态说明**：实例无 `status`/`last_heartbeat` 字段——存活感知在 BFE 侧（EPPAddr 连接滞回），api 侧不做存活标记（详见 `design-changes.md` §5）。

### 3.2 既有 `/clusters` 接口变更：`balance_mode` 与 `epp_config`

**变更原因**：

- 现状 cluster 的 `BalanceMode` 由 SubCluster 的 pool `Role` 派生（`model/icluster_conf/cluster.go:295-302` 的 `getBalanceMode`），而设置 `Role=EPP` 的旧 `/instance-pools` 端点已被移除（`endpoints/openapi_v1/product_pool/endpoints.go` 注册表为空），**没有任何 OpenAPI 可以为 cluster 开启 EPP**。
- EPP 调度配置（epp_config）是 per-cluster 配置且与均衡模式强耦合：EPP 模式必须有调度配置，非 EPP 模式不应有。因此不引入独立的 epp-picker-config CRUD 域，**随 `/clusters` 一并配置**，在 cluster 写入时强制校验。

**变更内容**：`POST /clusters` 与 cluster 更新接口（`PUT /clusters/{name}`）请求体新增两个可选字段：

| 参数名 | 类型 | 必填 | 说明 | 合法性条件 |
|--------|------|------|------|------------|
| `balance_mode` | string | 条件必填 | 集群均衡模式。`WRR`：BFE 本地加权轮询；`EPP`：由 EPP 调度器接管后端选择 | 枚举：`WRR`（默认）、`EPP` |
| `epp_config` | object | 条件必填 | EPP 调度配置（**简化用户形态**：调度档位 + 少量调优参数，见 §3.2.1）；api 导出时确定性编译为完整 `EndpointPickerConfig` | `balance_mode=EPP` 时**必填**且通过字段校验；`balance_mode=WRR` 时可选（传入则保留并做格式校验，但不编译、不导出，见下"休眠语义"） |

请求体示例（EPP 模式 cluster）：

```json
{
    "name": "cluster-a",
    "llm_config": { "provider": "p1", "models": ["m1"] },
    "balance_mode": "EPP",
    "epp_config": {
        "scheduling_profile": "balanced",
        "kv_cache_utilization_max": 0.9,
        "flow_control": {
            "max_requests": 1000,
            "queue_ttl": 30,
            "no_endpoint_queue_ttl": 600,
            "enable_eviction": false
        }
    }
}
```

**取值规则**：

| `balance_mode` | `epp_config` | 行为 |
|----------------|-----------------|------|
| 缺省 / `WRR` | 缺省 | 与现状完全一致：创建 `Role=COMMON` 实例池，导出 `GslbBasic.BalanceMode=WRR`，不进 EPP 下发范围 |
| 缺省 / `WRR` | 携带非空值 | 允许：保留并做格式校验，但不编译、不导出（休眠），再切回 `EPP` 时继续生效 |
| `EPP` | 必填 | 导出 `GslbBasic.BalanceMode=EPP`；`GslbBasic.EPPAddr` 由实例组分配模型生成（有序 `[主, 备]`，见 `design-changes.md` §4.3）；epp_config 随集群写入即生效（epp_data topic 在下一次导出时由 MD5 签名机制自然 bump）；纳入 assignment 下发范围；无有效分配时导出**降级**：该 cluster `BalanceMode` 置为 `WRR`、不生成 `EPPAddr`，同时输出 error 级日志 |

**约束与校验**：

- `balance_mode=EPP` 的 cluster 不创建 `Role=COMMON` 的 BFE 实例池（与 WRR 池区分），但**池实例列表照常同步 provider `instance_pool`**——EPP 的 cluster-table-discovery 经 cluster_table 导出发现推理后端，EPP 故障时 BFE 回退本地 WRR 也依赖该列表；`BalanceMode=EPP` 下 BFE 不做本地 WRR。`llm_config` 校验照常（provider/models/keys 与模式无关）。
- `balance_mode` 更新规则：允许 `WRR → EPP`（须同时提供合法 `epp_config`）；`EPP → WRR` 仅改 `balance_mode` 即可——`epp_config` 与分配记录保留（休眠：不编译、不导出），再切回 `EPP` 时原配置与原分配继续生效（分配如已悬空由 `/epp-pool` 变更时的自动修复处理）。**校验规则：`epp_config` 非空即须通过字段校验，与 `balance_mode` 无关**（休眠保留的配置同样保持合法）。
- 生效语义：`epp_config` 仅在 `balance_mode=EPP` 时编译下发；停用 EPP 调度 = 将 `balance_mode` 改回 `WRR`，**无需清空** `epp_config`。
- 响应（GET/List）中返回 `balance_mode` 与 `epp_config` 字段。

**缺省语义**：两字段均可选，缺省即 `WRR`、无 epp_config；新列带默认值/可空，存量数据无需迁移。

#### 3.2.1 `epp_config` 字段详细说明

`epp_config` 采用**简化用户形态**设计：不直接暴露 llm-d `EndpointPickerConfig` 的插件声明细节，用户以"调度档位 + 少量一等公民调优参数"表达意图，由 ai-gateway-api 在导出时**确定性编译**为完整 `EndpointPickerConfig`（编译规则见下表，出厂即合法）。本期不提供原样透传的高级模式；如未来出现自定义插件链需求，届时再开放。

**命名**：字段名取 `epp_config` 而非 llm-d 的 `picker_config`——简化形态已不再是 picker 配置的原始结构，叫 `picker_config` 会把实现命名泄漏到用户侧；`epp_config` 与 `balance_mode=EPP` 语义对称（开启 EPP ⇔ 提供 EPP 配置），也为未来容纳非 picker 类的 per-cluster EPP 设置（如超时、重试）预留空间。存储列、导出段、Go 字段同名（`clusters.epp_config` / `epp_config` 段 / `EppConfig`）。

**字段说明**：

| 字段 | 类型 | 必填 | 默认值 | 说明 | 合法性条件 |
|------|------|------|--------|------|------------|
| `scheduling_profile` | string | 否 | `balanced` | 调度策略档位：调度激进程度/优化目标的预设组合 | 枚举：`latency-first`（低延迟优先：队列等待权重最高）、`balanced`（均衡）、`throughput-first`（吞吐优先：KV cache 亲和最高） |
| `cache_affinity` | string | 否 | `medium` | scorer 权重覆盖项，显式指定 KV cache 亲和强度 | 枚举：`low` / `medium` / `high`；缺省（未显式设置）= `medium`，语义为**跟随 `scheduling_profile`，不覆盖其权重**；显式设置后覆盖 `scheduling_profile` 的 scorer 权重 |
| `prefix_cache_affinity` | bool | 否 | `true` | 前缀缓存亲和（`prefix-cache-scorer`）：相同 prompt 前缀的请求经前缀哈希匹配**尽力收敛**到同一后端，提升 KV cache 复用；零参数、自主学习（`approx-prefix-cache` producer）；**软亲和，不保证一定命中匹配后端**（见下"亲和语义"） | bool |
| `session_affinity_enabled` | bool | 否 | `false` | 会话亲和开关（`session-affinity-scorer`，session_id 策略）：开启后同一 session 的请求**尽力粘住**同一后端（有 binding 状态，亲和强度高于 prefix；但仍受 utilization-filter 与加权总分影响，非硬路由），绑定端点摘除后自动迁移重粘 | bool |
| `session_affinity_header` | string | 条件必填 | - | session id 来源请求头（如 `x-session-id`），scorer 从该 header 解析 session id；`enabled=false` 时保留但不生效（休眠） | `session_affinity_enabled=true` 时**必填**（非空 HTTP header 名）；`enabled=false`/缺省时可保留（做格式校验，不生效） |
| `kv_cache_utilization_max` | float | 否 | `0.9` | 端点过滤阈值：KV cache 利用率超过该值的端点被过滤 | `(0, 1]` |
| `flow_control` | object | 否 | - | 流控参数；缺省则不下发流控段（EPP 用系统默认） | 见下表 |

**`flow_control` 子字段**：

| 字段 | 类型 | 必填 | 默认值 | 说明 | 合法性条件 |
|------|------|------|--------|------|------------|
| `max_requests` | int | 否 | 不限 | 全局并发上限（跨全部优先级带）；`-1` 为显式"不限"，用于在更新中明确恢复不限（消除"缺省"与"保留原值"的合并语义二义） | 取值 `>0` 或 `-1`；缺省表示不限 |
| `queue_ttl` | int | 否 | 缺省由 EPP 决定（llm-d 默认 60 秒） | 池**有端点**时的排队预算（**单位：秒**），超期以可重试背压错误拒绝 | 取值 ≥ 0 的整数；`0` 为显式禁用驱逐 |
| `no_endpoint_queue_ttl` | int | 否 | 跟随 `queue_ttl` | 池**无端点**（冷启动扩容）时的排队预算（**单位：秒**），regime 切换时重新起算 | 同 `queue_ttl` |
| `enable_eviction` | bool | 否 | `false` | 需求驱动驱逐：高优先级被饱和阻塞时终止负优先级在飞请求回收容量 | bool |

**流控实现说明**（llm-d flow controller，`flowcontrol/`）：全部由 EPP 进程内实现、状态为本地内存，无外部存储——请求先进入 per-pool（≈per cluster）**优先级队列**（并发安全 heap，按优先级带排序，超 TTL 后台 sweep 拒绝），单 goroutine processor 每周期检查在飞计数是否达 `max_requests` ceiling，有空位才调度到后端，响应完成归还额度；`enable_eviction` 开启后 HoL 阻塞时可经 ext_proc ImmediateResponse 驱逐负优先级在飞请求。failover 切换后队列与在飞计数不保留、新 primary 从零开始（排队中请求由 BFE 侧断连/重试处理），与亲和状态冷启动同理。各优先级带另有 band 级限额，本期不暴露、用系统默认。

**缺省语义与存储约定**：`cache_affinity` 缺省值为 `medium`，语义为"跟随 `scheduling_profile`、不覆盖其权重"；仅当用户在请求中**显式设置** `low` / `medium` / `high` 时才作为覆盖项生效（显式 `medium` = 覆盖为 (0.6, 0.6)，与缺省行为不同）。为区分这两种状态，存储保留用户原始 JSON——未显式携带的字段不落盘、GET 回读与写入一致；默认值只体现在导出编译时。

**编译规则**（api 导出时把上述简化配置确定性展开为 `EndpointPickerConfig`，规则固化在代码中并单测覆盖）：

| 简化字段 | 编译为 |
|----------|--------|
| 固定部分 | 注入 `cluster-table-discovery` 端点发现插件（`clusterName` 取本 cluster）、`utilization-filter`（阈值取 `kv_cache_utilization_max`）、`kv-cache-utilization-scorer` + `queue-scorer` 两个 scorer、`max-score-picker` picker、`openai-parser` |
| `scheduling_profile` → scorer 权重 (kv, queue) | `latency-first` → (0.2, 1.0)；`balanced` → (1.0, 0.5)；`throughput-first` → (1.0, 0.2) |
| `cache_affinity` → scorer 权重 (kv, queue)（覆盖 profile，仅在显式设置时生效；缺省 `medium` 不覆盖，跟随 `scheduling_profile`） | `low` → (0.2, 1.0)；`medium` → (0.6, 0.6)；`high` → (1.0, 0.2) |
| `prefix_cache_affinity=true`（缺省即 true） | scorer 链追加 `prefix-cache-scorer`（权重固定 1.0）；`approx-prefix-cache` producer 为 EPP 默认注册，无需配置；`false` 则不注入 |
| `session_affinity_enabled=true` 时 | scorer 链追加 `session-affinity-scorer`（strategy=session_id，`sessionIdConfig.sources=[{header: <session_affinity_header>}]`，权重固定 1.0）；`false`/缺省不注入；无 session header 的请求 scorer 弃权，不受影响 |

**亲和类 scorer 与档位权重的关系**：`prefix-cache-scorer` / `session-affinity-scorer` 是**特性开关**（开/关 + 固定权重 1.0），与档位权重正交——`scheduling_profile` / `cache_affinity` 只调节 (kv, queue) 两个基础 scorer 的权重，不影响亲和 scorer。亲和 scorer 均为软打分：无前缀匹配 / 无 session header 时弃权，由基础 scorer 决定选端点，因此固定权重 1.0 对普通流量无副作用。

**亲和语义（重要，避免误解为硬路由）**：调度为"加权求和 + 最高分选取"（llm-d `scheduler_profile.go`：端点总分 = Σ scorer输出[0,1] × 权重，`max-score-picker` 取总分最高者，同分确定性轮转让位），且 filter 先于打分执行。因此前缀/session 亲和是**尽力收敛**，以下情况会打破亲和、把请求调度到非匹配后端：

- 匹配后端被 `utilization-filter` 过滤（KV cache 利用率超 `kv_cache_utilization_max`）——淘汰后请求必然去其他端点；
- 匹配率不高（只匹配 prompt 一小段），而另一后端 kv/queue 加权总分反超；
- 首次请求（无匹配数据）或 EPP 重启亲和数据未重建时，所有端点亲和分均为 0，退化为纯 kv/queue 打分。

这是期望行为：亲和让高匹配后端"通常胜出"，负载/利用率因素仍可打断亲和，避免热点后端被拖死。需要强粘性的场景应使用 `session_affinity_enabled`（有 binding 状态，比 prefix 稳定），而非依赖 prefix 收敛。

**亲和状态存放**：prefix 亲和索引是 EPP 实例**本地内存**中的 per-endpoint LRU（approx-prefix-cache producer 自学习写入，scorer 从 endpoint attribute 读取），**无需 Redis 等外部存储**。一个 cluster 同一时刻仅 primary EPP 做调度决策，本地索引即完备；failover/重启后新 primary 冷启动，经少量请求重新收敛（ai-gateway-epp SC11 已验证），与软亲和语义一致。会话亲和 binding 状态同为 EPP 本地内存。

**运行时指标来源**：kv/queue scorer 消费的利用率与队列指标由 EPP **周期性 HTTP 抓取各推理后端 `/metrics`（Prometheus 格式）**获得（`RefreshMetricsInterval` 刷新、本地内存缓存），ai-gateway-epp `injectDefaults` 默认自动注入指标 source/extractor，**同样无需外部存储**。某后端指标不可用时相应得分中性化（kv 得分视为 1.0、filter 不生效），调度正常退化。EPP 全链路的对外依赖仅两类无状态拉取：控制面配置（epp_data / cluster_table）与数据面指标（后端 /metrics）。
| `flow_control` 存在时 | 生成 `flowControl` 段：`max_requests`→`maxRequests`、`queue_ttl`→`defaultRequestTTL`、`no_endpoint_queue_ttl`→`noEndpointRequestTTL`、`enable_eviction`→`enableEviction`；`featureGates` 追加 `flowControl`。秒数由 api 在编译时转为 Go duration 下发（如 `30` → `30s`、`600` → `10m0s`）。特例：`max_requests` 缺省或为 `-1`（不限）时**不生成** `maxRequests` 字段（llm-d 缺省即不限） |

**校验分工**：

| 阶段 | 校验方 | 行为 |
|------|--------|------|
| 简化字段校验（枚举、数值范围、秒数取值范围） | ai-gateway-api | 不合法拒绝提交（422），全部 api 侧可强制 |
| 编译产物结构 | ai-gateway-api generator（确定性模板，单测覆盖） | 模板自身保证引用完整、DAG 无环，正常路径不出现编译失败 |
| 深度校验（插件存在性、层序约束等） | EPP 编译期 | 仅作为防御兜底；万一失败该 cluster 沿用旧引擎，不影响其他 cluster |

### 3.3 域：epp-assignments（分配查询与手工覆写）

| 端点 | Method | 说明 |
|---|---|---|
| `/api/v1/epp-assignments` | GET | 分配全量视图（per-cluster 展开 + 未分配/空闲清单，见下） |
| `/api/v1/epp-assignments/{cluster}` | PUT | 手工覆写单个 cluster 的分配 `{group_name, primary_instance_id}`（运维干预入口，触发 server_data_conf version bump） |

说明：分配由**分配器自动生成**（触发时机与算法见 `design-changes.md` §4.2：cluster 进入 EPP 模式时自动选组选主、`/epp-pool` 变更后自动修复悬空分配），本域提供查询视图与手工覆写（运维兜底）；分配输入（实例组列表）来自 `/epp-pool`（§3.1）；手工覆写校验 `primary_instance_id` 必须存在于对应组的实例列表中，且与 standby 实例不同地址。

#### 3.3.1 GET `/api/v1/epp-assignments` 响应详情

请求：`GET /api/v1/epp-assignments`（可选查询参数 `cluster=<name>` 过滤单个 cluster）

成功返回示例：

```json
{
    "ErrNum": 200,
    "ErrMsg": "success",
    "Data": {
        "clusters": [
            {
                "cluster": "cluster-a",
                "group": "g1",
                "primary":   { "id": "epp-a", "host": "10.0.0.1", "port": 9002 },
                "standby":   { "id": "epp-b", "host": "10.0.0.2", "port": 9002 }
            },
            {
                "cluster": "cluster-b",
                "group": "g1",
                "primary":   { "id": "epp-b", "host": "10.0.0.2", "port": 9002 },
                "standby":   { "id": "epp-a", "host": "10.0.0.1", "port": 9002 }
            },
            {
                "cluster": "cluster-c",
                "group": "g2",
                "primary":   { "id": "epp-c", "host": "10.0.0.3", "port": 9002 },
                "standby":   null
            }
        ],
        "unassigned_clusters": ["cluster-d", "cluster-e"],
        "idle_groups": ["g3"]
    }
}
```

**字段说明**：

| 字段 | 类型 | 说明 |
|------|------|------|
| `clusters` | []object | 全部 `balance_mode=EPP` 的 cluster 的分配展开视图（含未分配 cluster，见下） |
| `clusters[].cluster` | string | cluster 名 |
| `clusters[].group` | string | 分配的实例组名；未分配时为 `null` |
| `clusters[].primary` | object \| null | 主实例；未分配时为 `null`。结构同 /epp-pool 实例：`{id, host, port}` |
| `clusters[].standby` | object \| null | 备实例；**读时展开**：`epp_assignments` 只存 `primary_instance_id`，standby = 同组中除 primary 外的实例；单实例组或未分配时为 `null` |
| `unassigned_clusters` | []string | `balance_mode=EPP` 但 `epp_assignments` 无有效记录的 cluster 名列表。这些 cluster 导出 server_data_conf 时**降级为 `WRR`**（api 同时输出 error 日志），属需尽快处理的异常态，单独列出便于巡检 |
| `idle_groups` | []string | 实例池中未承担任何 cluster 分配的组名列表（组内实例均未作为 primary 出现；备角色不计入占用）。扩容新组后、完成分配前会出现在此列表 |

**语义规则**：

- 视图是 `epp_assignments`（cluster→{group, primary}）与 `epp_instances`（/epp-pool）的**读时 join**，不引入冗余存储；两者不一致时（如 primary 指向的实例已被从池中移除）该 cluster 标记为异常：`primary` 返回 null 并计入 `unassigned_clusters`，同时在 `clusters[]` 该条目标记 `"degraded": true`。
- `clusters` 默认返回全部 EPP 模式 cluster（含未分配）；传 `cluster=<name>` 时只返回该 cluster（`balance_mode != EPP` 的 cluster 返回 404 语义错误）。
- 实例在多个 cluster 间互为主备（如 `epp-a`/`epp-b` 同时担任 cluster-a 与 cluster-b 的主备）是**合法**的——分配以 cluster 为单位，不做组↔cluster 的一对一约束。

#### 3.3.2 PUT `/api/v1/epp-assignments/{cluster}` 请求详情

请求体：

```json
{
    "group_name": "g1",
    "primary_instance_id": "epp-a"
}
```

| 字段 | 类型 | 必填 | 说明 | 合法性条件 |
|------|------|------|------|------------|
| `group_name` | string | Y | 目标实例组 | 必须存在于 `/epp-pool` 当前实例池中 |
| `primary_instance_id` | string | Y | 主实例 id | 必须存在于 `group_name` 组的实例列表中 |

执行逻辑：校验 → 写入/更新 `epp_assignments`（cluster 唯一键 upsert）→ bump server_data_conf version（EPPAddr 下次导出即带出；epp_data topic 在下一次导出时由 MD5 签名机制自然 bump）→ 返回更新后的该 cluster 分配视图（结构同 §3.3.1 `clusters[]` 元素）。

约束：生产组覆写后应保持每组 2 实例（standby 自动为组内另一实例）；`EPP → WRR` 反向变更（§3.2）后分配记录休眠保留，再切回 `EPP` 时继续生效（如分配已悬空由自动修复处理）。

---

## 4. 文档变更

| 位置 | 变更 |
|------|------|
| `design-docs/api-define/InnerAPI接口定义/` | 新增 `epp-data.md`（合并端点契约，格式照 `cluster-table.md`）；`server-data-conf.md` 增加 §3.3；更新 `02-interface-list.md`、`README.md` 索引 |
| `design-docs/api-define/OpenAPI接口定义/` | 新增 `epp-pool.md`（格式照 `alb-pool.md`）、`epp-assignments.md`；`clusters.md` 增加 `balance_mode`、`epp_config` 字段定义与校验规则 |

---

## 5. 边界语义

| 场景 | 行为 |
|------|------|
| 单实例测试组 | EPPAddr 仅 `[主]`（`standby: null`）；主失联时 BFE 降级本地均衡（api 侧无动作） |
| EPP 模式 cluster 无有效分配 | assignment 段无该 cluster 条目；server_data_conf 导出时该 cluster **降级为 `WRR`**（不生成 EPPAddr），同时 api 输出 error 级日志，不阻塞整份下发 |
| `BalanceMode != EPP` 的 cluster | 不进 epp_config / assignment（EPP 侧对 unknown pool 告警，属 EPP 侧职责） |
