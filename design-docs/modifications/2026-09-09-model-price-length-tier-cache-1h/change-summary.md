# model-prices 长度分档价格键与 1h 缓存写价格键补全

## 1. 背景

新模型目录导入需求中，价格键与既有导入链路不兼容。长度分档键不做 `above_<N>k_tokens` 动态模式，直接硬编码枚举 200k/256k/272k/512k 四档（8 个键）。

## 2. 不兼容项与策略

| # | 不兼容项 | 影响面 | 策略 |
|---|---|---|---|
| 1 | `input_cost_per_image_token`、`input/output_cost_per_audio_token` 不在 `ValidPriceKeys`（BFE 已支持） | api 拒绝；无法下发 | api 补键，导出链路原样透传，BFE 零改动 |
| 2 | 长度分档键：api 仅认识 200k；BFE 全不认识 | 两端少计费 | **硬编码四档**（200k/256k/272k/512k），api 枚举校验，BFE 新增长度分档计费 |
| 3 | `cache_creation_input_token_cost_1h` 两端均不认识 | 两端少计费（1h 缓存写按 5m 价计） | 两端新增键 + BFE usage 解析 TTL 拆分 + 访问日志字段 |

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api`（本文）；BFE 侧配套改造见 BFE 仓库修改记录 |
| 主要文件 | `model/imodel_price/validate.go`、`design-docs/api-define/OpenAPI接口定义/model-prices.md`、`model/icluster_conf/cluster_test.go` |
| 接口契约 | `prices` / `tier_prices` 价格键枚举扩充 |
| 数据库 | 无 DDL 变更；prices 以 JSON 文本存储 |
| 数据面影响 | BFE 侧同步改造（常量、负价校验、长度分档计费、1h usage 拆分、访问日志 `ai_cache_write_1h_tokens`），端到端键对齐后方可生效 |

## 4. 最终方案

### 4.1 `ValidPriceKeys` 补键（`model/imodel_price/validate.go`）

硬编码枚举，含既有 200k 档共 4 档 + 1h 缓存写价 + 3 个对齐键：

```go
// 对齐 BFE（BFE 已支持，api 缺失）
"input_cost_per_image_token":         true,
"input_cost_per_audio_token":         true,
"output_cost_per_audio_token":        true,

// 新增：1h TTL 缓存写价
"cache_creation_input_token_cost_1h": true,

// 长度分档（硬编码四档；既有 200k 保留）
"input_cost_per_token_above_256k_tokens":  true,
"output_cost_per_token_above_256k_tokens": true,
"input_cost_per_token_above_272k_tokens":  true,
"output_cost_per_token_above_272k_tokens": true,
"input_cost_per_token_above_512k_tokens":  true,
"output_cost_per_token_above_512k_tokens": true,
```

键按原名字符串存入 `PriceMap`，`model/icluster_conf/cluster.go` 的 map 透传使新键自动到达 BFE ModelTable，**导出链路无需改动**（补一条透传单测断言）。

### 4.2 OpenAPI 文档

`design-docs/api-define/OpenAPI接口定义/model-prices.md`：更新价格键枚举表；metadata 一节明确"仅 `source`/`notes`"。

## 5. 测试

- `validate_test.go`：三个对齐键、`_1h` 键、256k/272k/512k 六条硬编码档键通过校验；`checkPricePrecision` 对新键生效；
- `icluster_conf/cluster_test.go`：新键导出透传断言。

## 6. 风险与兼容性

| 项目 | 说明 |
|------|------|
| 兼容性 | 未配置新键的模型走原有代码路径，计费结果与现状一致；存量数据无影响 |
| 回滚 | api 补键仅为枚举扩充，回滚无风险；prices 以 JSON 文本存储，无 DDL 变更 |
| 升级路径 | 新增长度档位在两端枚举中各加一对常量（档位超过 2-3 个时再泛化为动态模式） |

## 7. 实施顺序

1. **补键**：4.1 `ValidPriceKeys` 枚举扩充（`model/imodel_price/validate.go`）；
2. **OpenAPI 文档**：4.2 更新 `model-prices.md` 价格键枚举表；
3. **测试**：§5 单测（`validate_test.go`、`icluster_conf/cluster_test.go`）。

## 8. 参考文档

- `model/imodel_price/validate.go`
- `design-docs/api-define/OpenAPI接口定义/model-prices.md`
- `model/icluster_conf/cluster.go`（价格键导出透传）

---

## 9. 实施记录（2026-09-09）

已按本方案完成实施：

| 变更 | 文件 | 说明 |
|------|------|------|
| `ValidPriceKeys` 补键 | `model/imodel_price/validate.go` | 新增 3 个 BFE 对齐键、`cache_creation_input_token_cost_1h`、256k/272k/512k 六条长度分档键，共 10 键 |
| 校验单测 | `model/imodel_price/validate_test.go` | `TestValidateModelPriceNewPriceKeys`：10 个新键在 `prices` 与 `tier_prices.peak` 中均通过校验（含 `checkPricePrecision`），未知档位键（如 `above_999k_tokens`）仍被拒绝 |
| 导出透传单测 | `model/icluster_conf/cluster_test.go` | 断言 `cache_creation_input_token_cost_1h`、`input_cost_per_token_above_256k_tokens` 等新键经导出到达 BFE ModelTable |
| OpenAPI 文档 | `design-docs/api-define/OpenAPI接口定义/model-prices.md` | `prices` 键名枚举表补齐 10 个新键（1h 缓存写价、四档长度分档、image/audio token 键） |

验证：`go test ./model/imodel_price/... ./model/icluster_conf/...` 全量通过。导出链路为 `PriceMap` map 透传，新键自动到达 BFE ModelTable，无需改动 `model/icluster_conf/cluster.go`。
