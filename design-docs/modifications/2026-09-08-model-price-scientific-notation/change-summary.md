# model-prices 价格放开科学计数法与更高小数精度

## 1. 背景

v0.6「RMB 计费精度提高」迭代中，控制面导入的模型目录（`model-list.yaml`，450 个模型）出现大量 **10~12 位小数** 价格，例如 `output_cost_per_token: 7.6234102728e-08`、`input_cost_per_token: 4.141631732e-06`。这些价格是 `unit_price_usd_per_1M × group_ratio / 1e6 × 6.8` 的计算结果，`group_ratio` 有 6 位有效小数，无法在源头约简。

此前 issue-102 的方案（见 `2026-08-27-issue-102-eight-decimal-price-precision/`）为 `PriceMap` / `TierPriceMap` 增加自定义 `MarshalJSON`，强制价格以十进制表示法输出（如 `0.0000015`），不丢失 8 位小数的精度。该方案的两个前提已不再成立：

1. **精度需求超过 8 位小数**：12 位小数以十进制表示为 `0.000000076234102728`，位数靠人工数，读写极易出错；科学计数法 `7.6234102728e-08` 一目了然。YAML / JSON 语法均原生支持科学计数法。
2. **BFE 侧消费方式升级**：BFE 已改为价格保持 `float64`、请求计费时逐项 `quota.CalcCostUnits`（`round(用量 × (价格 × 1e8))`）再取整扣减（见 `bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/design-changes.md`），不再在配置加载期把价格截断为 1e-8 定点整数，超精度价格不再损失精度。

## 2. 目标

1. `prices` / `tier_prices` 的**输入**（OpenAPI 请求体、`model-list.yaml` 导入）显式支持科学计数法与十进制表示法，两者等价；
2. `prices` / `tier_prices` 的**输出**（OpenAPI 响应、InnerAPI 导出的 `cluster_conf.data`）不再强制十进制，允许标准编码器输出科学计数法；
3. 校验规则更新：非负校验不变，新增 float64 可精确表示的兜底上限；
4. 数据库无需迁移（价格本就按 `float64` 存储，科学计数法只是文本表示）；
5. 接口契约不变：字段名、数据类型（JSON number）均不变。

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api`（本文）；`bfe` 侧见 `bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/` |
| 主要文件 | `model/imodel_price/model_price.go`、`model/imodel_price/validate.go`、`design-docs/api-define/OpenAPI接口定义/model-prices.md` |
| 数据库 | 无需迁移；浮点数值本身存储不变 |
| 接口契约 | 不变；仅 JSON 文本表示方式与校验说明调整 |
| 数据面影响 | BFE 解析到的 `float64` 数值不变（科学计数法本就是合法 JSON number） |

## 4. 最终方案概览

**删除** `PriceMap` / `TierPriceMap` 的自定义 `MarshalJSON`（issue-102 引入），回归 Go 标准 `encoding/json` 编码：

- 输入解析（`yaml.v3` / `encoding/json`）本就支持科学计数法，**零改动**；
- 输出序列化由标准编码器决定：极小值输出为科学计数法（如 `7.6234102728e-08`），常规值输出为十进制，两者均为合法表示；
- `validate.go` 保留键名枚举与非负校验，新增 `|price × 1e8| < 2^53`（约 9e15）兜底校验，保证下游 BFE 浮点计费不溢出。

`model-prices.md` 中"支持 8 位及以上小数精度，JSON 序列化使用十进制表示法，不使用科学计数法"等描述同步更新为"科学计数法与十进制表示法均合法，数值按 float64 解析（有效数字约 15 位）"。

## 5. 预期收益与风险

| 项目 | 说明 |
|------|------|
| 收益 | 12 位小数价格可无损导入、导出与计费；目录文件（`model-list.yaml`）可直接由生成工具以科学计数法输出，无需二次展开为十进制 |
| 主要风险 | 导出配置文本中极小价格变为科学计数法，运维习惯需适应；下游消费方若对价格做**文本级**处理（如字符串比较、正则提取）需改为数值比较。已确认 BFE 按 `float64` 解析，无此问题 |
| 兼容性 | 数值语义不变；十进制输入完全兼容；科学计数法输入此前亦能被标准解析器接受（只是文档禁止），实际为文档解禁 |

## 6. 上线前检查清单

1. `go test ./model/imodel_price/...` 通过（含新增科学计数法解析与序列化用例）；
2. `make test-model-cover-gate` 通过；
3. InnerAPI 导出含 `7.6234102728e-08` 类价格的 `cluster_conf.data`，BFE `bfe -t test_conf` 加载成功；
4. `model-prices.md` 与代码行为一致。

## 7. 参考文档

- `2026-08-27-issue-102-eight-decimal-price-precision/`（前序方案，本次部分回退其序列化策略）
- `model/imodel_price/model_price.go`、`model/imodel_price/validate.go`、`model/imodel_price/import.go`
- `bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/design-changes.md`（BFE 侧配套改造）
- `document-ai-gateway/迭代系统设计/v0.6/RMB计费精度提高/`（迭代需求来源）
