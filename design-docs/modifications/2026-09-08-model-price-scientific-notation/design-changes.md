# model-prices 科学计数法与高精度价格设计变更说明

## 1. 当前问题定位

### 1.1 问题表象

`model-list.yaml`（450 个模型，由计费目录自动生成）中大量价格为 10~12 位小数，以科学计数法书写：

```yaml
models:
  - provider: example-provider
    model: qwen2.5-omni-7b
    base_model: qwen2.5-omni-7b
    mode: chat
    prices:
      input_cost_per_token: 6.0168984e-09
      output_cost_per_token: 7.6234102728e-08
```

而接口定义文档（`design-docs/api-define/OpenAPI接口定义/model-prices.md`）规定价格"支持 8 位及以上小数精度，JSON 序列化使用十进制表示法（如 `0.0000015`），不使用科学计数法"。文档与真实数据脱节：

1. `7.6234102728e-08` 展开为十进制是 `0.000000076234102728`，人工读写极易数错位数；
2. 此前 issue-102 方案在 `PriceMap` / `TierPriceMap` 上定制的十进制 `MarshalJSON`，会把这类值序列化为超长十进制串，配置文本可读性差；
3. BFE 侧旧的"加载期转 1e-8 定点整数"逻辑会把 `7.6234102728e-08` 截断为 `7`（相对误差 8.1%）——该问题由 BFE 侧改造（浮点价格 + 逐项取整）解决，本文不重复描述，见 `bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/design-changes.md`。

### 1.2 根因

- 表示层：issue-102 为避免科学计数法被误读为精度丢失，选择了"十进制 only"策略；当精度需求超过 8 位小数后，该策略的可读性优势反转为其最大缺点。
- 语法层：YAML（`gopkg.in/yaml.v3`）与 JSON（`encoding/json`）均原生支持科学计数法数字字面量，当前限制 purely 是策略选择，不存在技术障碍。

### 1.3 为什么现有测试没暴露

`TestParseModelListYAMLEightDecimalPlaces` 覆盖的是十进制 8 位小数（`0.00000005`）的解析与序列化；目录文件一直未被纳入 ai-gateway-api 的测试资产，科学计数法输入路径（解析其实一直可用）从未被显式测试。

## 2. 目标

1. 输入（OpenAPI 请求体、YAML 导入）显式支持科学计数法与十进制表示法；
2. 输出（OpenAPI 响应、InnerAPI 导出）回归标准 JSON 编码，允许科学计数法；
3. 校验规则：键名枚举、非负、必填等不变；新增 float64 表示上限兜底；
4. 数据库与接口契约（字段名、JSON number 类型）不变。

## 3. 方案对比

| 方案 | 思路 | 优点 | 缺点 | 结论 |
|------|------|------|------|------|
| A. 删除自定义 `MarshalJSON`，回归标准编码（推荐） | 回退 issue-102 的序列化定制 | 改动最小（净删代码）；标准行为、无维护成本；输入解析本就支持科学计数法 | 极小值输出为科学计数法，运维需适应 | **采用** |
| B. 保留十进制 `MarshalJSON`，仅文档解禁输入 | 输入放开、输出维持十进制 | 输出格式不变 | 12 位小数十进制串可读性差；与"两种表示法等价"的语义自相矛盾（输出永远是十进制） | 不采用 |
| C. 价格改字符串 / decimal 类型 | 契约层面支持任意精度 | 彻底避免浮点 | 改动 OpenAPI 契约、数据库、BFE 解析，影响面大；float64 的 15 位有效数字已覆盖现有数据 | 不采用 |

> 注：issue-102 解决的原始问题（"科学计数法被误读为精度丢失"）随 BFE 计费链路改为浮点计算后自然消解——`1.5e-6` 与 `0.0000015` 解析为同一 float64，扣减精度由逐项取整保证。

## 4. 详细改动

### 4.1 `model/imodel_price/model_price.go`

删除 `PriceMap.MarshalJSON` 与 `TierPriceMap.MarshalJSON`（及不再使用的 `bytes` / `strconv` import），类型定义与注释保留并更新：

```go
// PriceMap is a map of price keys to their numeric values (yuan per unit).
// Prices may use more than 8 decimal places; both decimal and scientific
// notation are accepted on input and may appear in JSON output.
type PriceMap map[string]float64

// TierPriceMap is a map of tier names to PriceMap values.
type TierPriceMap map[string]map[string]float64
```

序列化行为变化（标准 `encoding/json` 对 float64 使用最短表示）：

| 值 | 旧输出（十进制 MarshalJSON） | 新输出（标准编码） |
|----|------------------------------|--------------------|
| `0.000002` | `0.000002` | `2e-6` |
| `0.0000015` | `0.0000015` | `1.5e-6` |
| `7.6234102728e-08` | `0.000000076234102728` | `7.6234102728e-08` |
| `0.03` | `0.03` | `0.03` |

两种表示法解析后均为同一 float64，语义不变。

### 4.2 `model/imodel_price/import.go`

**无需改动**。`yaml.v3` 原生把 `6.0168984e-09` 解析为 float64。`ParseModelListYAML` 的既有流程（解析 → `ValidateModelPrice` → 入库）不变。

### 4.3 `model/imodel_price/validate.go`

`ValidateModelPrice` 中：

- 键名枚举校验、非负校验、`prices` 必填、`peak` tier 限制：**全部保留**；
- 新增兜底校验：`|v × 1e8| >= 2^53`（约 9e15）时拒绝。价格经 BFE 计费时以 `用量 × (价格 × 1e8)` 参与浮点运算，float64 整数精确表示上限为 2^53；该校验保证下游计算不溢出。正常价格（< 1 元/token）远不会触发，仅防御异常输入：

```go
for k, v := range m.Prices {
    if !ValidPriceKeys[k] {
        return xerror.WrapParamErrorWithMsg("invalid price key: %s", k)
    }
    if v < 0 {
        return xerror.WrapParamErrorWithMsg("price %s must be >= 0", k)
    }
    // Guard the downstream float64 cost computation
    // (usage * (price * 1e8)) against losing integer exactness beyond 2^53.
    if v*quota.RmbPrecision >= (1<<53) {
        return xerror.WrapParamErrorWithMsg("price %s exceeds the maximum representable precision", k)
    }
}
```

`tier_prices` 内层价格加同样校验。

### 4.4 接口定义文档 `model-prices.md` 同步

| 位置 | 修改 |
|------|------|
| 字段说明 `prices` / `tier_prices` 行 | "支持 8 位及以上小数精度，JSON 序列化使用十进制表示法……不使用科学计数法" → "支持科学计数法与十进制表示法；按 float64 解析（有效数字约 15 位）；必须为非负数" |
| 1.2 节"价格精度与 JSON 序列化" | 重写：两种表示法等价；精度上限来自 float64；导出文本可能出现科学计数法 |
| 2.2 节 YAML 字段说明 | `prices` / `tier_prices` 说明同步放开 |
| 第 4 节校验规则 5 | "支持 8 位及以上小数精度" → "支持科学计数法与十进制表示法；单价格 × 1e8 不得超过 2^53" |

## 5. 数据迁移

无需迁移。价格字段数据库类型不变（`float64` / `REAL`），科学计数法仅是 JSON/YAML 文本层的表示方式，入库数值不变。

## 6. 测试计划

### 6.1 单元测试（`model/imodel_price/`）

1. **新增** `TestParseModelListYAMLScientificNotation`：
   - 解析含 `7.6234102728e-08`、`4.141631732e-06` 的 YAML；
   - 断言 `Prices` 中 float64 值与字面量相等；
   - `json.Marshal` 回写后 `json.Unmarshal` 往返一致（数值层面）。
2. **改造** `TestParseModelListYAMLEightDecimalPlaces`：
   - 保留十进制解析断言；
   - 序列化断言由"输出不含 `e-`"改为"数值往返一致"（不再断言文本形式）。
3. **新增** `TestValidateModelPriceExcessivePrecision`：
   - 价格 `≥ 9e7`（×1e8 达 2^53）被拒绝；正常价格（如 `0.000000076234102728`）通过。

### 6.2 回归验证命令

```bash
cd ai-gateway-api
go test ./model/imodel_price/...
make test-model-cover-gate
```

### 6.3 端到端

配合 BFE 侧改造（`bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/`）：

- 用含科学计数法价格的 `model-list.yaml` 走 `/v1/model-prices/import` → InnerAPI 导出 → BFE 加载 → 发起请求，核对 Redis 扣减金额与手算 `round(用量 × 价格 × 1e8)` 一致。

## 7. 风险与缓解

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| 导出文本出现科学计数法，运维误判精度丢失 | issue-102 曾为此定制十进制输出 | `model-prices.md` 1.2 节明确说明两种表示法等价；BFE 计费按 float64 数值计算，与文本表示无关 |
| 下游对价格做文本级处理 | 如字符串比较、正则提取 | 全链路已确认按 JSON number 解析（BFE `encoding/json`、conf-agent 透传）；若第三方消费方存在文本处理，属其契约外用法，需在发布说明中告知 |
| 超 15 位有效数字的价格 | float64 无法无损表示 | 导入校验阶段发现即拒绝并提示（本次新增 ×1e8 上限校验）；未来若出现真实需求再评估 decimal 存储 |
| 删除自定义 MarshalJSON 引入回归 | 序列化路径变化 | 用例从"文本断言"改为"数值往返断言"，文本形式交由标准库保证合法性 |

## 8. 实施状态

- [x] `model/imodel_price/model_price.go` 删除 `PriceMap.MarshalJSON` / `TierPriceMap.MarshalJSON`；
- [x] `model/imodel_price/validate.go` 新增 ×1e8 上限校验（`checkPricePrecision`，默认价格与 tier 价格均覆盖）；
- [x] `import_test.go` 新增科学计数法用例、改造八位小数用例；
- [x] `validate_test.go` 新增超精度拒绝用例（含 12 位小数放行用例）；
- [x] `model-prices.md` 字段说明 / 1.2 节 / 2.2 节 / 校验规则同步；
- [x] `design-docs/sys-design/details/RMB配额分时段定价.md` 历史描述标注 v0.6 变更；
- [x] `design-docs/sys-design/模型层设计文档.md`（4.4 节校验要点、4.4.3 导入流程）与 `数据库设计文档.md`（model_prices 表说明）同步；
- [x] `test/integration/tests/model_price/import/import_test.go` 新增 MP-1-009（科学计数法导入）/ MP-1-010（超精度拒绝，422），`tests/model_price/design.md` 用例总览同步（8 → 10 例）；
- [x] `go test ./model/imodel_price/...` 与 `make test-model-cover-gate` 通过（覆盖率 78.4% ≥ 70%，环境无 make，手动执行等价命令）；
- [x] `test/integration` 全量 model_price 用例通过（需先重新构建项目根目录 `ai-gateway-api.exe`，集成测试以子进程方式启动该二进制）。

## 9. 参考文档

- `2026-08-27-issue-102-eight-decimal-price-precision/`（前序方案）
- `model/imodel_price/model_price.go`、`model/imodel_price/validate.go`、`model/imodel_price/import.go`、`model/imodel_price/import_test.go`
- `bfe/docs/zh_cn/modifications/2026-09-08-rmb-price-float-precision/design-changes.md`（BFE 侧配套改造）
- `design-docs/api-define/OpenAPI接口定义/model-prices.md`（接口定义，已同步）
