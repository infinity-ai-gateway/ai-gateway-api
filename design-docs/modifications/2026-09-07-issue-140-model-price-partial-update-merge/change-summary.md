# Issue #140：Model Price PUT 部分更新按键合并修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/140](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/140)

> 按合同 OpenAPI 接口定义 model-prices.md §3.7/§3.8，PUT 部分更新语义为"仅传需修改字段，未传入字段保持原值"。实际行为：请求里只要带了 `prices` 或 `tier_prices`（哪怕只想改其中一个键），整个 map 被请求体替换，未传入的价格键被静默清除，HTTP 200 无任何告警。

危害：任何"只调一个价"的运维操作都会静默丢失同 map 内其余价格（`output_cost_per_token`、`cache_read_input_token_cost` 等），下游 BFE 计费按缺失键回退，属静默计费配置数据丢失。E2E SC1301-TC014（指纹 dbf76872）保持 REAL_VERIFICATION_BLOCKED/FAILED_PRODUCT 终态等待本修复。

## 2. 根因

`endpoints/openapi_v1/model_price/update.go` 的 `mergeModelPrice`（被 `UpdateByIDAction` 与 `UpdateByQueryAction` 两条 PUT 路径共用）只对标量字段做了"非空才覆盖"，所有 map 字段是整块指针赋值：

```go
// update.go:125-129（修复前）
if len(src.Prices) > 0 {
    merged.Prices = src.Prices // 整 map 替换，无按键合并
}
if len(src.TierPrices) > 0 {
    merged.TierPrices = src.TierPrices // 两层嵌套（tier→价格键）同样整块替换
}
```

即"部分更新"的粒度实现到了字段级（map 有没有传），没有下钻到键级（map 里哪些键传了）。DB 写入侧（`UpdateModelPrice` 收到 merged 记录）没有问题，缺陷收敛在这一个 merge 函数。

## 3. 目标

1. `prices`：PUT 传入的键覆盖对应键，未传入的键保留存量值；
2. `tier_prices`：两层合并——请求中出现的 tier 名按键级合并，未传入的 tier 整档保留；
3. 合并不得污染调用方持有的 `existing` 记录（`merged := *dst` 是浅拷贝，必须新建 map 写入，不能原地改 `dst` 的 map）；
4. 补齐 handler 层单测回归防护；
5. 在 `model-prices.md` §3.7/§3.8 明确 map/切片字段的合并语义契约。

## 4. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `endpoints/openapi_v1/model_price/update.go`（`mergeModelPrice`） |
| 接口契约 | PUT 响应字段不变；仅 `prices` / `tier_prices` 的合并粒度从"整块替换"修正为"键级合并" |
| 同族字段 | `limits`、`metadata`（map）、`capabilities`、`supported_parameters`（切片）**保持现状整块替换**，本次仅在合同文档中显式写明语义（见 §6） |
| 数据迁移 | 无 |

## 5. 最终方案

### 5.1 键级深合并实现

`mergeModelPrice` 中两个价格 map 改为调用合并辅助函数：

```go
if len(src.Prices) > 0 {
    merged.Prices = mergePriceMap(dst.Prices, src.Prices)
}
if len(src.TierPrices) > 0 {
    merged.TierPrices = mergeTierPriceMap(dst.TierPrices, src.TierPrices)
}
```

- `mergePriceMap(dst, src)`：新建 map，先拷贝 `dst` 全键，再用 `src` 键覆盖——`src` 未传的键保留 `dst` 原值；
- `mergeTierPriceMap(dst, src)`：新建外层 map；`src` 中已存在的 tier 按键级合并（新建内层 map），`src` 中新增的 tier 整档加入，`dst` 有而 `src` 未传的 tier 整档保留；
- 全程不原地修改 `dst`/`src` 持有的任何 map，避免污染 `existing` 记录。

### 5.2 修复后行为对照（issue 复现路径）

PUT `{"prices":{"input_cost_per_token":3.1e-06},"tier_prices":{"peak":{"input_cost_per_token":1.11e-05}}}`：

| 字段 | 修复前 | 修复后 |
|------|--------|--------|
| `prices` | 只剩 `input`，`output` 被静默清除 | `input` 更新为 3.1e-06，`output` 保留 6.2e-06 |
| `tier_prices.peak` | 只剩 `input`，`output`/`cache_read` 被静默清除 | `input` 更新，同档未传键保留；其他 tier 整档保留 |

## 6. 合同文档补充（model-prices.md §3.7/§3.8）

在"仅传需修改字段，未传入字段保持原值"处补充 map/切片字段的粒度说明：

- `prices`、`tier_prices`：**键级合并**。传入的键覆盖对应键，未传入的键/档保留原值；
- `limits`、`metadata`（map）、`capabilities`、`supported_parameters`（切片）：**整块替换**。传入即整体覆盖，未传入保持原值不变。

## 7. 回归防护

1. **handler 单测**（`endpoints/openapi_v1/model_price/update_test.go`，直接测纯函数 `mergeModelPrice`）：
   - PUT 只传单键 → 同 map 其余键保留；
   - PUT 传同档部分键 → 同档未传键保留、其他 tier 整档保留；PUT 传新 tier 名 → 旧 tier 全档保留；
   - 不传 map → 存量 map 完全不变；
   - 合并不修改 `dst` 持有的 map（别名/污染断言）；
   - 标量字段合并不受影响。
2. **E2E**：SC1301-TC014 修复部署后走 requeue-real-verification 重跑，全链验证并解除 FAILED_PRODUCT 终态。

## 8. 风险与兼容性

| 项目 | 说明 |
|------|------|
| 兼容性 | PUT 请求/响应结构不变；仅 `prices`/`tier_prices` 语义从"整块替换"修正为合同约定的"键级合并"，属于缺陷修复而非行为变更 |
| 主要风险 | 若下游已依赖"整块替换"副作用（先 PUT 清空再重建），修复后空键不再被清除；该用法本身不符合合同语义，如有需要应改用显式空值键覆盖 |
| 同族字段 | `limits`/`metadata`/切片字段维持整块替换并在合同写明，避免本次扩大变更面 |

---

*文档生成日期：2026-09-07*
