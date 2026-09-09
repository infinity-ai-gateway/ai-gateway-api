# epp_data 接口

## 1. 接口信息

| 项目 | 值 | 说明 |
|------|------|------|
| 含义 | 导出 EPP 配置（epp_config + assignment 全量视图，合并单端点） | 供 EPP 实例拉取调度配置与实例组分配视图；单 topic（`ConfigTopicEppData`），两段配置同一 version 快照 |
| 端点 | `/configs/epp_data/config` | - |
| Method | GET | - |
| 鉴权 | `FeatureRoute + ActionExport` | - |

## 2. 请求参数

**Query 参数**

| 参数名 | 类型 | 必填 | 说明 | 合法性条件 |
|--------|------|------|------|------------|
| version | string | 否 | 上次返回的版本号，用于增量同步 | 可选；无强制格式/长度校验；为空或未传时按首次拉取处理 |

**请求示例**

```shell
curl -X GET "http://api-server:port/inner-api/v1/configs/epp_data/config?version=00010101000000" \
  -H "Authorization:Token TOKEN_STRING"
```

## 3. 返回数据结构

### 3.1 顶层结构

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

| 字段 | 类型 | 说明 |
|------|------|------|
| Version | string | 配置版本号，格式 `20060102150405` |
| Config | object | EPP 配置，含 `epp_config` 与 `assignment` 两段 |

### 3.2 Config.epp_config 段

`epp_config` 为 `map[cluster名]EndpointPickerConfig`，由 OpenAPI 写入的简化 `epp_config`（见 OpenAPI 接口定义 [clusters.md](../OpenAPI接口定义/clusters.md)）在导出时**确定性编译**而来（编译规则见 `design-docs/modifications/2026-09-08-epp-scheduling-integration/api-changes.md` §3.2.1），出厂即合法。

| 字段 | 类型 | 说明 |
|------|------|------|
| \<cluster名\> | object | 编译后的完整 `EndpointPickerConfig`，key 为集群名称 |

范围为全部 `balance_mode=EPP` 的 cluster。OpenAPI 校验保证 EPP 模式 cluster 的 `epp_config` 必填，因此导出结果中 EPP cluster 必然有配置。

### 3.3 Config.assignment 段（全量视图）

`assignment` 为 `map[cluster名]{primary, standby}`，**所有 EPP 实例返回完全相同的内容**（同一 version 快照）。

| 字段 | 类型 | 说明 |
|------|------|------|
| \<cluster名\>.primary | string | 主实例 id，为 `/epp-pool` 中配置的实例 id |
| \<cluster名\>.standby | string \| null | 备实例 id；单实例组（无备）时为 `null` |

**EPP 侧消费方式**：以自身 `-instance-id`（须与 `/epp-pool` 中某实例 id 一致）逐 cluster 匹配——`primary == 本实例 id` → 本实例为该 cluster 的 primary；`standby == 本实例 id` → standby；均未命中 → 跳过该 cluster。全量视图的附带收益：EPP 可获知同组 peer 实例 id（如备 Cell 建联、双活跃自检）。

**异常态**：`epp_config` 中存在但 `assignment` 中无条目的 cluster = 未分配。EPP 侧应本地告警、不为该 cluster 建 cell；api 侧导出 server_data_conf 时该 cluster 降级为 `WRR` 并输出 error 日志（见 [server-data-conf.md](./server-data-conf.md) §3.3）。

## 4. 配置未变化返回示例

```json
{
    "ErrNum": 200,
    "ErrMsg": "success",
    "Data": null,
    "WorkMode": "ModeNormal"
}
```

---
