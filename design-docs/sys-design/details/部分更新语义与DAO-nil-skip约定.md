# 部分更新语义与 DAO 的 nil-skip 约定

> 关联：issue [#151](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/151)（API-Key）、[#147](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/147)（Provider）；
> 变更记录：[modifications/2026-09-11-issue-151-api-key-patch-models-subnet-reset](../../modifications/2026-09-11-issue-151-api-key-patch-models-subnet-reset/change-summary.md)、[modifications/2026-09-09-issue-147-provider-patch-partial-update](../../modifications/2026-09-09-issue-147-provider-patch-partial-update/change-summary.md)。

---

## 1. 契约

OpenAPI 的 PATCH 接口（`/api-keys/{id}`、`/entities/{id}`、`/providers/{provider_name}` 等）约定 **"仅传需修改字段"**：请求体中省略的字段必须保留原值，不得被重置为默认值或空值。

该语义依赖两个前提：

1. **Endpoint 层**：Body 绑定（`xreq.BindJSON`）后，省略字段在 Param 结构中保持 Go 零值（指针为 nil、切片为 nil）；
2. **DAO 层 nil-skip**：`struct2map(raw, ignoreOpt=true)`（`storage/rdb/internal/dao/internal/builder.go:43`）在构建 UPDATE 的 SET 子句时跳过 nil 指针字段，被跳过的列不进 SET 子句，数据库保留原值。

因此**约束在 storager 层**：param → `T<Table>Param` 的转换绝不允许把"未提供（nil）"抹平成"显式提供默认值"。一旦转换产出非 nil 指针，nil-skip 失效，原值必然被覆盖。

## 2. 各资源 PATCH 省略字段行为

| 资源 | 省略即保留原值的字段 | 实现位置 |
|------|--------------------|---------|
| API-Key | `models`、`subnet` | `storage/rdb/api_key/api_key.go` `UpdateAPIKey`（仅 `len > 0` 时 marshal 赋值，否则保持 nil；issue #151） |
| Entity | `allow_models`、`block_models` | `storage/rdb/entity/entity.go` `entityDataToParamForUpdate`（Update 专用转换，省略保持 nil；Create 仍走 `entityDataToParam` 默认 `"[]"`；issue #151 同批修复） |
| Provider | `model_endpoint`、`models`、`keys`、`time_zone`、`tiers` | `storage/rdb/provider/provider.go` `toDAOParamForUpdate`（Update 独立路径不调用 `FillDefaults`；issue #147） |
| API-Key / Entity | `quota_plan`、`rate_limit_policy`、`route_rules` | Manager 层 `if param.X != nil` 守卫（`model/api_key/api_key.go:518/534/556`），省略即不下发子资源更新 |

## 3. Create 与 Update 的默认值语义不同

| 资源 | 字段 | Create 省略（文档行为） | PATCH 省略（修复后行为） |
|------|------|----------------------|------------------------|
| API-Key | `models` / `subnet` | 默认 `["*"]`（不限制） | 保留原值 |
| Entity | `allow_models` / `block_models` | 默认 `[]` | 保留原值 |
| Provider | `time_zone` | 默认 `Asia/Shanghai` | 保留原值 |
| Provider | `models` / `keys` | 默认 `[]` | 保留原值 |

因此 storager 必须为 Update 提供**独立的 param→DAO 转换路径**，不得复用创建路径的默认值回填逻辑。

## 4. 已知限制

- **显式空数组无法与省略区分**（API-Key / Entity）：`models`/`subnet`/`allow_models`/`block_models` 的修复取"仅 nil 语义"——请求体显式传 `[]` 与省略一样被视为"不修改该字段"。若产品需要"显式清空"语义（nil=保留 vs `[]`=清空/置默认），需另立契约并在接口文档写明。
- **Provider 例外**：issue-147 修复时 `models`/`keys`/`tiers` 区分了 nil（跳过）与非 nil 空切片（写入 `"[]"`，可显式清空）。
- **历史数据不回溯**：修复前已被静默覆盖的数据不在本次范围。

## 5. 回归验证

- 单元测试：`storage/rdb/api_key/api_key_test.go`、`storage/rdb/entity/entity_test.go`（sqlite 内存库，覆盖省略保留 / 显式写入 / Create 默认三种路径）；
- 集成测试：`test/integration/tests/api_key/partial_update`（AK-5-006/007）、`test/integration/tests/entity/partial_update`（E-5-006/007）；
- E2E：SC1101-TC019（API-Key）、SC1203-TC028（Entity）。
