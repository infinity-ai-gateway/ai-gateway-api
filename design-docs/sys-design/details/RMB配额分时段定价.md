# RMB 配额分时段定价

## 1. 背景与目标

随着 DeepSeek 等模型提供商采用"高峰 / 空闲"分时段定价策略，BFE RMB 配额扣费需要具备按请求发生时刻匹配不同价格的能力。

本设计目标：

1. 支持按请求发生时刻匹配特定价格 tier（如 `peak`），未命中时 fallback 到默认价格。
2. 支持不同 provider 配置不同的时区和时段规则。
3. 结合 provider/cluster 分离，时段模板放在 `/providers`，分时段价格放在 `/model-prices`。
4. 保持对现有固定价格的向后兼容。
5. 运行时成本计算仍保持纯整数运算，不引入浮点误差。
6. `prices` / `tier_prices` 中的浮点价格 originally 在 JSON 序列化时使用十进制表示法（issue-102）；**v0.6 已改为科学计数法与十进制等价合法**，标准编码器输出，详见 `design-docs/modifications/2026-09-08-model-price-scientific-notation/`。

## 2. 核心概念

| 概念 | 说明 |
|------|------|
| **Tier** | 按时间维度划分的价格层级，如 `peak`（高峰）。请求发生时刻命中某个 tier 时，使用对应价格；否则 fallback 到默认 `prices`。 |
| **TimeRange** | 一个时段定义，包含 `weekdays`（星期几，0=周日）、`start` / `end`（HH:MM，左闭右开）。 |
| **Provider 时段模板** | 定义在 `/providers` 上的 `time_zone` 和 `tiers`，同一 provider 下所有模型共享。 |
| **Model tier 价格** | 定义在 `/model-prices` 上的 `tier_prices`，描述某个模型在某个 tier 下的价格。 |

## 3. 配置归属与下发链路

```
/provider deepseek
    ├── time_zone: Asia/Shanghai
    └── tiers: [peak]
    ├── model-prices deepseek-v4-pro chat
    │       ├── prices: 默认/空闲价
    │       └── tier_prices.peak: 高峰价
    └── model-prices deepseek-v4-flash chat
            └── ...

/cluster my-cluster
    ├── llm_config.provider: deepseek
    └── AIConf.ModelTable（导出时由 control-plane 把 provider 的 time_zone/tiers 与 model-prices 的 prices/tier_prices 拼接）
```

多个 cluster 引用同一个 provider 时，会各自得到一份相同的 `ModelTable` 数据；provider 的时段规则变更后，所有引用它的 cluster 在下一次配置导出时自动生效。

## 4. 数据模型

### 4.1 `/providers` 新增字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `time_zone` | string | 计算时段所使用的时区；默认 `Asia/Shanghai`；须为 IANA 时区名 |
| `tiers` | []PricingTier | 时段 tier 定义列表；**初期 `name` 只支持 `peak`** |

```json
{
  "name": "peak",
  "time_ranges": [
    { "weekdays": [1, 2, 3, 4, 5], "start": "09:00", "end": "12:00" },
    { "weekdays": [1, 2, 3, 4, 5], "start": "14:00", "end": "18:00" }
  ]
}
```

### 4.2 `/model-prices` 新增字段

| 字段 | 类型 | 说明 |
|------|------|------|
| `tier_prices` | object | tier name -> 价格表；**初期键名只支持 `peak`**；内部键名须为 `prices` 枚举 |

### 4.3 BFE `AIConf.ModelTable` 结构

```go
type TimeRange struct {
    Weekdays []int  // 0=周日，1=周一 ... 6=周六；为空表示每天
    Start    string // "HH:MM"
    End      string // "HH:MM"，必须 > Start；跨午夜请拆成两段
}

type PriceTier struct {
    Name       string      // 初期只支持 "peak"
    TimeRanges []TimeRange // 命中任意一个即属于该 Tier
}

// PriceMap maps price keys to float64 values.
// It marshals to JSON using decimal notation instead of scientific notation,
// so values like 0.0000015 remain readable in exported configs.
type PriceMap map[string]float64

// TierPriceMap maps tier names to PriceMap values with the same decimal notation.
type TierPriceMap map[string]map[string]float64

type ModelPrice struct {
    Provider            string
    Model               string
    BaseModel           string
    Mode                string
    Capabilities        []string
    SupportedParameters []string
    Limits              map[string]interface{}
    Prices              PriceMap     // 默认价格
    TierPrices          TierPriceMap // tier name -> 价格表
    Metadata            map[string]interface{}
}

type ModelTable struct {
    Currency string      // 固定 "RMB"
    TimeZone string      // 默认 "Asia/Shanghai"
    Tiers    []PriceTier // 时段定义
    Models   []ModelPrice
}
```

## 5. 控制面实现要点

### 5.1 接口层

- `PUT /providers/{provider_name}/pricing-tiers`：单独维护 provider 的高峰/闲时模板，支持 JSON / YAML 两种提交方式。
- `/providers` 的 CRUD 与读取接口均须支持 `time_zone` / `tiers` 字段。
- `/model-prices` 的 CRUD 与导入接口须支持 `tier_prices` 字段。

### 5.2 模型层

- `model/iprovider`：
  - `Provider` / `ProviderParam` 新增 `time_zone`、`tiers`。
  - 新增 `UpdatePricingTiers` 方法，负责解析、校验并更新时段模板。
  - `CreateProvider` / `UpdateProvider` 增加对 `tiers` 的校验。
- `model/imodel_price`：
  - `ModelPrice` / `ModelPriceParam` 新增 `tier_prices`。
  - CRUD 与 YAML 导入增加对 `tier_prices` 的校验（tier name、价格键名、非负价格）。
  - ~~`PriceMap` 与 `TierPriceMap` 实现自定义 `MarshalJSON`，使用 `strconv.FormatFloat(v, 'f', -1, 64)` 输出十进制表示法，避免 8 位小数价格被序列化为科学计数法。~~（v0.6 已删除自定义 `MarshalJSON`，回归标准编码，科学计数法与十进制等价合法；校验新增 `价格 × 1e8 < 2^53` 上限。）
- `model/icluster_conf/exporter.go`：
  - 导出 `AIConf` 时，根据 `cluster.llm_config.provider` 查询 Provider，将 `time_zone` / `tiers` 填入 `ModelTable`。
  - 按同一 `provider` 查询 `model_prices`，将 `prices` / `tier_prices` 填入对应 `ModelPrice`。
  - `Currency` 固定为 `"RMB"`。

### 5.3 存储层

- `providers` 表新增 `time_zone`（varchar，默认 `Asia/Shanghai`）、`tiers`（text / JSON）。
- `model_prices` 表新增 `tier_prices`（JSON）。

## 6. BFE 数据面行为

BFE 侧与 v0.4 方案基本一致，`AIConf.ModelTable` 结构未变，仅配置来源发生变化。~~为保持整条链路配置文本一致，BFE 侧同构的 `PriceMap` / `TierPriceMap` 也实现了自定义 `MarshalJSON`，使用相同的十进制表示法。~~（v0.6 起 BFE 删除该自定义序列化，价格保持 `float64`、计费时逐项 `quota.CalcCostUnits` 取整，见 `bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/`。）

### 6.1 加载阶段

- 解析 `TimeZone`，默认 `Asia/Shanghai`。
- 校验 `Tiers`：名字非空、**初期 `name` 只支持 `peak`**、时间格式正确、`Weekdays` 合法、同一 Tier 内时间不重叠。
- 将默认 `Prices` 和每个 tier 的价格表分别转成定点整数。

### 6.2 运行时时段匹配

```go
func (table *ModelTable) ActiveTierName(now time.Time) string {
    if table == nil || len(table.Tiers) == 0 {
        return ""
    }
    t := now.In(table.tz)
    wd := int(t.Weekday())
    hour, min := t.Hour(), t.Minute()
    cur := hour*60 + min

    for i := range table.Tiers {
        tier := &table.Tiers[i]
        for _, tr := range tier.TimeRanges {
            if len(tr.Weekdays) > 0 && !containsInt(tr.Weekdays, wd) {
                continue
            }
            start := parseHHMM(tr.Start)
            end := parseHHMM(tr.End)
            if start <= cur && cur < end {
                return tier.Name
            }
        }
    }
    return ""
}
```

### 6.3 运行时成本计算

- `TokenUsage` 增加 `CachedTokens`，用于命中缓存的输入 token 数。
- `calcCostUnits` 根据当前时间匹配 tier，再取对应价格。
- 缓存命中 / 未命中分别按 `cache_read_input_token_cost` 和 `input_cost_per_token` 计算。

## 7. 校验规则

### 7.1 `/providers`

1. `time_zone` 须为合法 IANA 时区名；为空时默认 `Asia/Shanghai`。
2. `tiers` 中每个 tier 必须包含非空 `name` 和至少一个 `time_range`。
3. **`tiers` 中每个 tier 的 `name` 初期只支持 `peak`**。
4. `time_ranges` 中 `weekdays` 元素必须在 0-6 之间；`start` / `end` 格式为 `HH:MM`，且 `end` > `start`。
5. 同一 tier 内部 `time_ranges` 不得重叠。

### 7.2 `/model-prices`

1. `tier_prices.<tier>.<key>` 必须是 `prices` 枚举中的合法键名，且值为非负数。
2. **`tier_prices` 的键名初期只支持 `peak`**。
3. `tier_prices` 中的 tier name 与 provider 的 `tiers` 不做强制引用校验；若引用了一个不存在的 tier name，可记录告警，但不阻塞写入。

## 8. 向后兼容

- `/providers` 不填 `time_zone` / `tiers` 时，BFE 的 `ModelTable.TimeZone` / `ModelTable.Tiers` 为空，行为与现在完全一致（固定价格）。
- `/model-prices` 不填 `tier_prices` 时，按默认 `Prices` 固定价格计费。
- 命中 tier 但该 tier 未配置某个价格键时，自动 fallback 到默认 `Prices` 中的对应键。
- `TokenUsage.UsedCost`、Lua 扣减逻辑、Redis 定点数存储都不需要改。

## 9. 风险与注意事项

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 浮点价格 JSON 序列化精度/可读性 | 默认 encoder 对 8 位小数会输出科学计数法，配置可读性差，中间层可能误解析 | ~~`ai-gateway-api` 与 `bfe` 的 `PriceMap` / `TierPriceMap` 均自定义 `MarshalJSON`，使用 `strconv.FormatFloat(v, 'f', -1, 64)` 强制十进制表示。~~（v0.6 起两种表示法均合法，按 float64 数值消费；中间层须按 JSON number 解析而非文本处理。） |
| tier name 首期约束 | 仅支持 `peak` | 设计时已通过 `Weekdays` 预留扩展能力，后续只需放开命名约束。 |
| provider 与 model-prices 的 tier 松耦合 | `tier_prices` 可能引用 provider 未定义的 tier | 可记录告警，但不做强制引用校验以保持灵活性。 |
| 时区解析依赖系统时区数据库 | 部署环境需包含 IANA 时区数据 | 使用 Go 标准库 `time.LoadLocation`，确保容器镜像包含时区数据。 |
| 高峰/空闲切换时刻的边界 | 时间区间左闭右开，切换瞬间价格突变 | 产品侧明确规则，BFE 按配置精确匹配。 |
| 多 cluster 共享 provider | 修改 provider 时段规则会影响所有引用 cluster | 更新时给出引用列表确认。 |

## 10. 实现状态

本设计已于 `2026-08-25` 完成编码并全量测试通过（`go build ./...`、`go test ./...`）。

后续于 `2026-08-27` 针对 Issue #102 补充了 8 位小数价格 JSON 序列化精度修复：为 `ai-gateway-api` 与 `bfe` 两侧的 `PriceMap` / `TierPriceMap` 增加自定义 `MarshalJSON`，使用十进制表示法替代科学计数法。

再后续于 `2026-09-08`（v0.6 RMB 计费精度提高）回退上述序列化定制：目录数据出现 10~12 位小数价格，十进制表示可读性更差，且 BFE 计费链路已改为浮点价格 + 逐项取整；两侧删除自定义 `MarshalJSON`，科学计数法与十进制等价合法，ai-gateway-api 校验新增 `价格 × 1e8 < 2^53` 上限。详见 `design-docs/modifications/2026-09-08-model-price-scientific-notation/`。

主要落地文件：

| 层次 | 文件 | 说明 |
|------|------|------|
| DDL | `db_ddl.sql`、`db_ddl_sqlite.sql` | 表结构扩展 |
| 存储层 | `storage/rdb/internal/dao/table_providers.go`、`table_model_prices.go` | DAO 字段 |
| 存储层 | `storage/rdb/provider/provider.go`、`storage/rdb/model_price/model_price.go` | JSON 序列化 |
| 模型层 | `model/iprovider/provider.go`、`validate.go` | provider 时区/时段模型与校验 |
| 模型层 | `model/imodel_price/model_price.go`、`validate.go` | model price tier 价格模型与校验；含 `PriceMap`/`TierPriceMap` 自定义 `MarshalJSON` |
| 导出层 | `model/icluster_conf/cluster.go`、`model/iroute_conf/exporter.go` | `AIConf.ModelTable` 拼接 |
| 接口层 | `endpoints/openapi_v1/provider/pricing_tiers.go` | `PUT /providers/{provider_name}/pricing-tiers` |
| BFE 数据面 | `bfe/bfe_config/bfe_cluster_conf/cluster_conf/cluster_conf_load.go` | 已有 `TimeZone`/`Tiers`/`TierPrices`/`ActiveTierName`；含 `PriceMap`/`TierPriceMap` 自定义 `MarshalJSON` |

BFE 数据面保持原有 v0.4 实现，仅配置来源由 ai-gateway-api 控制面按本设计拼接后下发。

## 11. 相关文档

- API 定义：`design-docs/api-define/OpenAPI接口定义/providers.md`、`design-docs/api-define/OpenAPI接口定义/model-prices.md`
- Inner API 定义：`design-docs/api-define/InnerAPI接口定义/server-data-conf.md`
- 修改说明：`design-docs/modifications/2026-08-25-rmb-tiered-pricing/`、`design-docs/modifications/2026-08-27-issue-102-eight-decimal-price-precision/`
- 上游设计来源：`document-ai-gateway/迭代系统设计/v0.5/计费能力扩展/bfe-rmb-tiered-pricing-design.md`
