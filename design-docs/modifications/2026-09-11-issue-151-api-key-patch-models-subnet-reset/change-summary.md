# Issue #151：PATCH /api-keys/{id} 省略 models/subnet 被静默重置为 ["*"] 修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/151](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/151)

> SC1101-TC019：API-Key PATCH 省略 models/subnet 字段时被静默重置为默认值 `["*"]`，违反"仅传需修改字段"契约。

接口文档 `design-docs/api-define/OpenAPI接口定义/api-keys.md` §2.5（部分更新 API-Key）约定 Body 参数"同 2.2.1 创建 API-Key 的 Body 参数，仅传需修改字段"。实际行为是 PATCH 请求省略 `models`/`subnet` 时，两字段不保留原值，而是被静默重置为 `["*"]`。

复现路径（E2E SC1101-TC019，2026-09-09，verified_commit 176d6027）：

1. POST /api-keys 创建 Key，显式指定 `models: ["controlled-model-a"]`、`subnet: [<自定义网段>]`；
2. PATCH /api-keys/{id}，Body 仅含需修改字段（省略 models/subnet）→ 返回 200；
3. GET /api-keys/{id} → models 已变为 `["*"]`，原白名单丢失（subnet 同路径同病）。

影响：任何客户端做局部更新（如只改 description 或 expired_time）都会意外解除模型白名单与网段白名单——既是数据丢失，也是静默权限放大（受限 Key 被放开到全部模型、全部来源网段），且 200 响应无任何提示。

## 2. 根因

调用链：`APIKeyUpdateAction`（`endpoints/openapi_v1/api_key/update.go:41`）→ `APIKeyManager.UpdateAPIKey`（`model/api_key/api_key.go:497`）→ `APIKeyStorager.UpdateAPIKey`（`storage/rdb/api_key/api_key.go:180`）→ `dao.TAPIKeyUpdate` → `internal.Update` → `Struct2Assign` → `struct2map(raw, true)`（`storage/rdb/internal/dao/internal/builder.go:43`）。

DAO 层 `struct2map` 支持部分更新：nil 指针字段跳过、不进 SET 子句即保留列原值。

逐层核实结论：

1. **Endpoint 无问题**（`endpoints/openapi_v1/api_key/update.go:50`）：`xreq.BindJSON` 绑定 Body，省略字段时 `param.Models`/`param.Subnet` 保持 nil，且构造传给 manager 的 `APIKeyParam` 时原样透传（`update.go:95-96`）。
2. **Manager 无守卫**（`model/api_key/api_key.go:497-587`）：对 `QuotaPlan`/`RateLimitPolicy`/`RouteRules` 都有 `if param.X != nil` 守卫（:518/:534/:556，省略即不动），唯独 models/subnet 直穿 storager。
3. **Storager 是病灶**（`storage/rdb/api_key/api_key.go:188-200`）：`UpdateAPIKey` 复用创建路径的默认值回填——
   ```go
   models := []string{"*"}
   if len(param.Models) > 0 {
       models = param.Models
   }
   modelsValue, _ := json.Marshal(models)
   data.AllowedModels = lib.PString(string(modelsValue))   // 省略时也写入 ["*"]
   ```
   `param.Models` 为 nil 时被解释为"取默认值"，产出非 nil 的 `["*"]` 并无条件赋给 `data.AllowedModels`，永远走不到 DAO 的 nil-skip 分支。subnet（:195-200）完全同构。
4. **DAO 语义可用**：`struct2map(raw, true)` 的 ignoreOpt=true 会跳过 nil 指针字段——只要 storager 不提前抹平，省略字段即可保留原值。

对照：`CreateAPIKey` 中保留 `["*"]` 默认值是文档约定行为（创建省略=默认全量），问题仅在 Update 路径照抄创建逻辑。同模块 `QuotaPlan`/`RateLimitPolicy`/`RouteRules` 已有"省略即不动"的守卫先例；Provider PATCH 同族缺陷已在 faf9556 / issue-147 修复（`design-docs/modifications/2026-09-09-issue-147-provider-patch-partial-update/`）。

## 3. 同族缺陷（同批修复）

`storage/rdb/entity/entity.go:158-171`：`entityDataToParam` 被 Create（:44）与 Update（:98）共用，其中：

```go
if len(param.AllowModels) > 0 {
    allowModelsJSON, _ := json.Marshal(param.AllowModels)
    data.AllowModels = lib.PString(string(allowModelsJSON))
} else {
    data.AllowModels = lib.PString("[]")   // 省略/显式空数组都写成 "[]"
}
```

`BlockModels`（:166-171）同构。Entity PATCH 省略 `allow_models`/`block_models` 时，原值被静默覆盖为 `[]`（E2E SC1203-TC028 已实证）。修复模式与 api_key 完全复用：拆分 create/update 转换路径。

## 4. 目标

1. PATCH 请求体未提供的 `models`/`subnet`（api_key）与 `allow_models`/`block_models`（entity），保持原配置不变——部分更新语义，与 `QuotaPlan`/`RateLimitPolicy`/`RouteRules` 现有行为一致；
2. 请求体显式提供的字段仍正常更新（含显式传 `["*"]` 恢复全量的语义）；
3. Create 路径默认值语义不变：api_key 不传 models/subnet 仍为 `["*"]`，entity 不传 allow_models/block_models 仍为 `[]`；
4. 回归验证：重跑 E2E SC1101-TC019 / SC1203-TC028 预期 PASSED；本地 `go test ./...` 与 model 覆盖率门禁（≥70%）通过。

## 5. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `storage/rdb/api_key/api_key.go`（`UpdateAPIKey`）；`storage/rdb/entity/entity.go`（`entityDataToParam` 拆分 create/update 路径） |
| 接口契约 | PATCH 请求/响应结构不变；仅"省略字段"语义从"重置为默认"修正为"保留原值" |
| 不在范围 | Create 路径行为不变；api-keys.md §2.5 文档措辞已有"仅传需修改字段"约定，无需改动；已被静默覆盖的历史数据不回溯修复 |
| 数据迁移 | 无 |

已知限制（与 issue 修复建议 §2 一致，取"仅 nil 语义"）：请求体显式传空数组 `[]` 与省略 nil 无法区分，二者均视为"不修改该字段"。如需"显式清空"语义需另立契约（nil=保留 vs []=清空），不在本次范围。

## 6. 最终方案

核心思路：让 Update 路径在字段未提供时产出 nil 指针，交给 DAO 的 nil-skip 保留原值，镜像 faf9556 / issue-147 的修法。

### 6.1 api_key：`UpdateAPIKey` 去掉默认值回填（`storage/rdb/api_key/api_key.go:180-203`）

```go
func (rpps *APIKeyStorager) UpdateAPIKey(ctx context.Context, filter *api_key.APIKeyFilter,
    param *api_key.APIKeyParam) (int64, error) {
    dbCtx, err := rpps.dbCtxFactory(ctx)
    if err != nil {
        return 0, err
    }

    data := newAPIKeyDataToParam(param)

    // 省略 models/subnet 时保持 nil，交由 DAO nil-skip 跳过该列，保留原值
    if len(param.Models) > 0 {
        modelsValue, _ := json.Marshal(param.Models)
        data.AllowedModels = lib.PString(string(modelsValue))
    }
    if len(param.Subnet) > 0 {
        subnetValue, _ := json.Marshal(param.Subnet)
        data.Subnet = lib.PString(string(subnetValue))
    }

    return dao.TAPIKeyUpdate(dbCtx, data, newAPIKeyFilterToParam(filter))
}
```

`CreateAPIKey` 路径不动（保留 `["*"]` 默认）。

### 6.2 entity：拆分 create/update 转换路径（`storage/rdb/entity/entity.go`）

- 保留 `entityDataToParam` 供 Create 使用（`[]` 默认不变）；
- 新增 update 专用转换（或给 `entityDataToParam` 加 `forUpdate bool` 参数，Create 传 false、Update 传 true）：`len(param.AllowModels) > 0` 才 marshal 赋值，否则保持 nil；`BlockModels` 同理。

### 6.3 单元测试

- `storage/rdb/api_key/api_key_test.go`：覆盖 UpdateAPIKey 省略 models/subnet 时 `TAPIKeyParam.AllowedModels`/`Subnet` 为 nil（DAO 跳过），提供时正确 marshal；Create 默认值行为不变；
- `storage/rdb/entity/entity_test.go`：同构覆盖 UpdateEntity 省略 allow_models/block_models 为 nil，Create 仍写 `[]`。

## 7. 回归验证

1. 本地：`go build ./...`、`go vet ./...`、`go test ./...`、`go test -cover ./model/...`（≥70% 门禁）全部通过；
2. 集成测试（`test/integration`，真实二进制子进程 + SQLite/miniredis）：新增用例
   - `tests/api_key/partial_update`：AK-5-006（PATCH 省略 models/subnet 保持白名单原值，issue #151 回归）、AK-5-007（显式修改 models/subnet 生效）；
   - `tests/entity/partial_update`：E-5-006（PATCH 省略 allow_models 保持原值）、E-5-007（PATCH 仅改 name 时 allow_models 保持原值）；
3. 部署后重跑 E2E：SC1101-TC019（api_key PATCH 省略 models/subnet 保持原值）、SC1203-TC028（entity PATCH 省略 allow_models/block_models 保持原值），预期 PASSED。
