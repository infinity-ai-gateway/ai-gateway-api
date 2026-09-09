# Issue #147：PATCH /providers/{provider_name} 省略字段被静默覆盖修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/147](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/147)

> 调用 PATCH /providers/{provider_name} 只修改部分字段、请求体省略其他非必填字段时，被省略的字段不会保留原值，而是被静默覆盖。

受影响字段与现象：

| 省略字段 | 实际行为 | 影响 |
|------|------|------|
| `time_zone` | 被重置为 `Asia/Shanghai` | 此前自定义时区（如 UTC）丢失 |
| `tiers` | 被清空（DB 写入 `"null"`） | peak 时段定价丢失 |
| `models` | 被清空（写入 `"[]"`） | 模型列表丢失 |
| `keys` | 被清空（写入 `"[]"`） | API Key 列表丢失 |
| `model_endpoint` | 被重置为默认 `{https, /v1/models}` | 自定义端点丢失 |

`description`（`*string`）不受影响——nil 指针被 DAO 跳过，原值保留。`instance_pool`、`model_protocols` 因 `ValidateProviderParam` 强制非空（`model/iprovider/provider.go:441-460`），PATCH 必须提供，不在本次范围。

## 2. 根因

调用链：`UpdateAction`（`endpoints/openapi_v1/provider/update.go:41`）→ `ProviderManager.UpdateProvider`（`model/iprovider/provider.go:195`）→ `RDBProviderStorager.UpdateProvider`（`storage/rdb/provider/provider.go:54`）→ `toDAOParam`（同文件 :156）→ `dao.TProviderUpdate` → `struct2map`（`storage/rdb/internal/dao/internal/builder.go:43`）。

DAO 层 `struct2map` 本就支持部分更新：nil 指针字段跳过（`builder.go:58-63`）、零值切片跳过（:65-68），不进 SET 子句即保留列原值。

问题出在上游 `toDAOParam` 在写库前把"未提供（nil）"抹平成"显式提供默认值"：

1. **`FillDefaults` 补默认值**（`storage/rdb/provider/provider.go:161` → `model/iprovider/provider.go:725-750`）：`ModelEndpoint == nil` → `DefaultModelEndpoint()`；`Models == nil` → `[]string{}`；`Keys == nil` → `[]ProviderKey{}`；`TimeZone == nil` → `&"Asia/Shanghai"`。指针/切片由 nil 变为非 nil。
2. **`marshalJSON` 把 nil 切片固化为非 nil 指针**（`storage/rdb/provider/provider.go:244-251`）：`if v == nil` 只判 nil interface；`[]T` 类型的 nil 切片作为 `interface{}` 传入时动态类型非 nil，于是走 `json.Marshal`——nil 切片 → `"null"`、`FillDefaults` 产出的空切片 → `"[]"`，两种结果都是非 nil `*string`。
3. 最终 `TProviderParam` 中 `TimeZone`、`ModelEndpoint`、`Models`、`Keys`、`Tiers` 全为非 nil 指针，`struct2map` 不再跳过，五个字段全部进 SET 子句，原值被覆盖。

对照：`4106af9` 给 `instance_pool`/`keys`/`models` 加的 origX 快照 + 还原 nil 逻辑（`model/iprovider/provider.go:218-251`）只服务于 sync hook 的触发判定与快照构建，不影响 DB 写入语义——storager 拿到的仍是被抹平后的 param。

历史：`models`/`keys` 的"省略即清空"自 provider/cluster 分离（`c52dbab`）起存在；`time_zone`/`tiers` 在 `faf9556`（RMB 分时段定价能力）落地后加入同一覆盖路径。属初始缺陷，非近期回归。

## 3. 目标

1. PATCH 请求体未提供的字段（`model_endpoint` / `models` / `keys` / `time_zone` / `tiers`），保持 provider 原配置不变——部分更新语义，与 `description` 现有行为一致；
2. 请求体显式提供的字段仍正常更新（含显式传空数组/空 tiers 的"全量替换/清空"语义）；
3. Create 路径默认值语义不变：不传 `time_zone` 仍默认 `Asia/Shanghai`，不传 `models`/`keys` 仍为空数组；
4. 合同文档（`providers.md` §2.4）明确 PATCH 部分更新语义。

## 4. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `storage/rdb/provider/provider.go`（`toDAOParam` / `marshalJSON` / `RDBProviderStorager.UpdateProvider`）；`model/iprovider/provider.go`（`UpdateProvider` hook 快照段，可选简化） |
| 接口契约 | PATCH 请求/响应结构不变；仅"省略字段"语义从"被覆盖为默认/空"修正为"保留原值" |
| 不在范围 | `instance_pool` / `model_protocols` 仍保持必填校验；Create 路径行为不变 |
| 数据迁移 | 无（已被静默覆盖的历史数据不回溯修复） |

## 5. 最终方案

核心思路：让 Update 路径产出 nil 指针，交给 DAO 的 nil-skip 保留原值。为 Update 提供独立的 param→DAO 转换路径，不调用 `FillDefaults`。

### 5.1 拆分 create/update 转换路径

`storage/rdb/provider/provider.go`：

- `RDBProviderStorager.CreateProvider` 继续走原 `toDAOParam`（含 `FillDefaults`），创建默认值语义不变；
- 新增 `toDAOParamForUpdate`，`RDBProviderStorager.UpdateProvider` 改用它，**不调用 `FillDefaults`**；
- 指针字段直接透传：`TimeZone: param.TimeZone`——nil 即保留 nil，DAO 跳过；非 nil 即更新。

### 5.2 nil 切片 → nil 指针

新增 `marshalJSONPtr`（或等价短路逻辑），替代 Update 路径的 `marshalJSON`：

```go
// marshalJSONPtr returns nil for a nil interface or nil slice, so the DAO
// layer's nil-skip keeps the existing column value on partial updates.
func marshalJSONPtr(v interface{}) (*string, error) {
    if v == nil {
        return nil, nil
    }
    rv := reflect.ValueOf(v)
    if rv.Kind() == reflect.Slice && rv.IsNil() {
        return nil, nil
    }
    data, err := json.Marshal(v)
    if err != nil {
        return nil, err
    }
    return lib.PString(string(data)), nil
}
```

`toDAOParamForUpdate` 中 `Models` / `Keys` / `Tiers` / `InstancePool` / `ModelProtocols` 均改用 `marshalJSONPtr`：省略（nil 切片）→ nil 指针 → DAO 跳过；显式提供（含空数组）→ 正常 marshal 进 SET 子句。

> 语义对照：`models: []` 显式传入仍写 `"[]"`（清空），`models` 省略则保留原值——"提供即更新、省略即保留"双向正确。

### 5.3 ModelEndpoint 子字段默认值

`ValidateProviderParam` 在非 nil 时已对 `ModelEndpoint` 补 `Schema`/`URI` 默认值（`model/iprovider/provider.go:462-471`），因此 Update 路径去掉 `FillDefaults` 后，显式提供 endpoint 的兜底行为不受影响；省略时 nil 指针被 DAO 跳过。

### 5.4 manager 层 hook 快照段（伴随清理，非必需）

修复后 storager 不再调用 `FillDefaults`，param 不会再被原地 mutation，`model/iprovider/provider.go:241-251` 的"还原 nil"块（`if origKeys == nil { param.Keys = nil }` 等）退化为 no-op。可保留作防御，也可删除；origX 快照 + `clusterConfigChanged` 判定逻辑保留不动。`applyProviderUpdate` 本就按 nil 保留 existing 值（`:866-872`），无需修改。

### 5.5 顺带修正：PATCH 的 time_zone 合法性校验

`ValidateProviderParam` 当前不校验 `TimeZone`（`validateTimeZone` 仅被 `ValidatePricingTiersParam` 调用）。修复后 PATCH 可显式传 `time_zone`，建议在 `ValidateProviderParam` 内补：`if param.TimeZone != nil { validateTimeZone(*param.TimeZone) }`——`validateTimeZone` 对空串放行，对非法时区报错，不影响省略场景。

### 5.6 修复点一览

| 字段 | 当前 nil 时的抹平点 | 修复后 nil 处理 |
|------|------|------|
| `model_endpoint` | `FillDefaults` → `DefaultModelEndpoint()`（provider.go:726-734） | Update 路径不补默认，nil → DAO skip |
| `models` | `FillDefaults` → `[]string{}` → `"[]"` | Update 路径 nil → nil 指针 → DAO skip |
| `keys` | `FillDefaults` → `[]ProviderKey{}` → `"[]"` | 同上 |
| `time_zone` | `FillDefaults` → `&"Asia/Shanghai"`（provider.go:745-748） | Update 路径不补默认，nil → DAO skip |
| `tiers` | `marshalJSON`（nil 切片）→ `"null"` | Update 路径 nil → nil 指针 → DAO skip |

## 6. 合同文档补充（providers.md §2.4）

`design-docs/api-define/OpenAPI接口定义/providers.md` §2.4"注意"处调整为：

- PATCH 为**部分更新**语义：未提供的字段保留原值不变；
- `keys`、`models`、`tiers` 作为数组，**显式提供时按全量替换处理**；省略时保留原值（本次修复前省略会被静默清空，属缺陷）；
- `time_zone`、`model_endpoint` 同理：提供即更新，省略保留原值。

## 7. 回归防护

1. **storager 单测**（`storage/rdb/provider/provider_test.go`，新文件）：
   - `toDAOParamForUpdate`：五个字段分别为 nil 时，`TProviderParam` 对应字段为 nil 指针；
   - 显式空切片（`[]string{}`、`[]PricingTier{}`）产出非 nil 指针（`"[]"`），与 nil 区分；
   - `toDAOParam`（Create 路径）行为不变：nil 输入仍补默认值；
   - `marshalJSONPtr`：nil interface、nil 切片、空切片、非空切片的四种分支。
2. **manager 单测**（`model/iprovider/provider_test.go` 补充）：
   - PATCH 只传 `description`，断言 storager 收到的 param 五个字段仍为 nil（配合 fake storager 捕获入参）；
   - PATCH 显式传 `keys: []`，断言触发 `ProviderKeyRefChecker` 且写库参数为非 nil 空数组。
3. **E2E / 集成测试**：
   - 设 `time_zone="UTC"` + peak tier + 自定义 models/keys/model_endpoint，PATCH 只改 description，断言五项原值不变；
   - PATCH 只改 `instance_pool`，断言其余字段不变；
   - PATCH 只改 `time_zone`（提供值），断言 tiers 等其他字段不变；
   - Create 回归：不传 `time_zone` 仍默认 `Asia/Shanghai`，不传 models/keys 仍为空数组。
   - 修复部署前 grep 集成测试与上游脚本，确认无调用方依赖"PATCH 省略 keys/models 即清空"的旧（缺陷）行为。

## 8. 风险与兼容性

| 项目 | 说明 |
|------|------|
| 兼容性 | PATCH 请求/响应结构不变；仅"省略字段"语义修正为保留原值，属缺陷修复而非行为变更 |
| 主要风险 | 若调用方依赖"PATCH 不传 keys/models 即清空"的旧行为会被打破——该行为与部分更新语义相悖，应视为修复；需要清空的调用方应显式传 `[]` |
| Create 路径 | `FillDefaults` 仍被 `CreateProvider` 使用，拆分路径后创建默认值语义不受影响 |
| 校验层 | `validatePricingTiers(nil)` 对 nil 放行，`instance_pool`/`model_protocols` 必填校验不变，无需放宽 |
| sync hook | origX 快照 + `clusterConfigChanged` 判定与 `applyProviderUpdate` 均按"显式提供才触发、nil 保留 existing"工作，与修复后语义一致，无需改动 |

---

## 9. 实施记录（2026-09-09）

已按本方案完成实施：

| 变更 | 文件 | 说明 |
|------|------|------|
| 拆分 Update 转换路径 | `storage/rdb/provider/provider.go` | 新增 `toDAOParamForUpdate`（不调用 `FillDefaults`），`RDBProviderStorager.UpdateProvider` 改用它；`marshalJSONPtr` 对 nil interface/nil 指针/nil 切片返回 nil 指针，空切片正常 marshal |
| time_zone 校验 | `model/iprovider/provider.go` | `ValidateProviderParam` 新增 `TimeZone != nil` 时的 `validateTimeZone` 校验，省略仍放行 |
| storager 单测 | `storage/rdb/provider/provider_test.go`（新文件） | 转换函数分支 + 基于内存 SQLite 的端到端部分更新/显式清空/Create 默认值测试 |
| manager 单测 | `model/iprovider/provider_test.go` | 省略字段以 nil 到达 storager、显式空 keys 触发 ref checker hook、time_zone 校验三分支 |
| 合同文档 | `design-docs/api-define/OpenAPI接口定义/providers.md` §2.4 | 明确 PATCH 部分更新语义 |

验证：`go test ./...` 全量通过；`model/iprovider` 覆盖率 72.2%（门槛 70%）。集成测试侧确认：`UpdateProvider` 客户端封装（`integration-test/test-cases/implementation/common/api_client.go:723`）注释本就声明"fields left unset are preserved by the server"，唯一调用点 SC28 只覆盖 `instance_pool`，无"省略即清空"依赖。

*文档生成日期：2026-09-09*
