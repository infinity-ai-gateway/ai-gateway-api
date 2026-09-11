# Issue #152：PATCH /entities/{id} 省略 allow_models/block_models 被静默重置为 [] 修复方案

> 本缺陷与 issue #151（API-Key models/subnet 省略重置为 `["*"]`）同属"PATCH 省略覆盖"家族，
> 已在同一批次修复。本文档按 issue #152 单独记录根因与方案，实现与 issue #151 共用同一模式。

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/152](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/152)

> SC1203-TC028：Entity PATCH 省略 allow_models/block_models 字段时被静默重置为 `[]`，违反"仅传需修改字段"契约。

接口文档 `design-docs/api-define/OpenAPI接口定义/entities.md` §2.5（部分更新 Entity）约定 Body 参数"同 4.2.1 创建 Entity 的 Body 参数，仅传需修改字段"。实际行为是 PATCH 请求省略 `allow_models`/`block_models` 时，两字段不保留原值，而是被静默重置为 `[]`。

复现路径（E2E SC1203-TC028，2026-09-09，verified_commit 54cda19）：

1. POST /entities 创建 Entity，显式指定 `allow_models: ["controlled-model-a"]`、`block_models: [<自定义值>]`；
2. PATCH /entities/{id}，Body 仅含需修改字段（省略 allow_models/block_models）→ 返回成功；
3. GET /entities/{id} → `allow_models` 已变为 `[]`（断言输出：`FAILED_PRODUCT: field allow_models changed from ["controlled-model-a"] to []`）；`block_models` 经人工复验同病被重置为 `[]`。

影响：任何局部更新（如只改 name、挂载 quota_plan）都会静默清空该 Entity 的模型允许/阻止列表。Entity 处于配额与权限继承树的节点位置，两字段被清空会改变该节点及其子树的有效模型集——既是数据丢失，也是权限语义的静默变更，且成功响应无任何提示。

## 2. 根因

`storage/rdb/entity/entity.go` 中 Create 与 Update 共用同一个参数转换器 `entityDataToParam`（`CreateEntity` :44、`UpdateEntity` :98 调用，函数定义 :146），该转换器把"省略"烘焙成"默认空数组"：

```go
// storage/rdb/entity/entity.go（entityDataToParam 内，修复前）
// 转换 AllowModels 为 JSON 字符串
if len(param.AllowModels) > 0 {
    allowModelsJSON, _ := json.Marshal(param.AllowModels)
    data.AllowModels = lib.PString(string(allowModelsJSON))
} else {
    data.AllowModels = lib.PString("[]")   // 省略 → 写死 "[]"，非 nil，绕过 DAO nil-skip
}
// BlockModels 完全同构
```

- 请求省略字段 → `param.AllowModels`/`param.BlockModels` 为 nil → else 分支产出非 nil 的 `"[]"` 并无条件落库，覆盖原值；
- DAO 层 `struct2map(raw, true)` 本有 nil-skip 语义（`storage/rdb/internal/dao/internal/builder.go:43`），但 nil 在转换器里被抹平成非 nil，Update 场景永远走不到跳过分支；
- 创建路径此默认合理（创建省略=空列表是文档行为），问题在 Update 路径复用了带默认值的转换器。

对照：同目录 `entity_type.go` 的 Update 不填默认值，无此缺陷（SC1203-TC027 通过）；同族 Provider PATCH 已在 faf9556 修复（Update 独立路径省略产出 nil 交 DAO 跳过）；API-Key 同族缺陷见 issue #151。

## 3. 目标

1. PATCH 请求体未提供的 `allow_models`/`block_models` 保持原值——部分更新语义，与同模块 `quota_plan`/`rate_limit_policy`/`route_rules` 的 Manager 层守卫行为一致；
2. 请求体显式提供的字段仍正常更新；
3. Create 路径默认值语义不变：不传 `allow_models`/`block_models` 仍写 `"[]"`；
4. 回归验证：重跑 E2E SC1203-TC028 预期 PASSED；本地 `go test ./...` 与 model 覆盖率门禁（≥70%）通过。

## 4. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `storage/rdb/entity/entity.go`（拆分 create/update 转换路径） |
| 接口契约 | PATCH 请求/响应结构不变；仅"省略字段"语义从"重置为 `[]`"修正为"保留原值" |
| 不在范围 | Create 路径行为不变；entities.md §2.5 文档措辞已有"仅传需修改字段"约定，无需改动；已被静默覆盖的历史数据不回溯修复 |
| 数据迁移 | 无 |

已知限制（与 issue #151 取同一"仅 nil 语义"）：请求体显式传空数组 `[]` 与省略 nil 无法区分，二者均视为"不修改该字段"。若产品需要"显式清空"语义（nil=保留 vs `[]`=清空），需另立契约并在 §2.5 写明。

## 5. 最终方案

核心思路：为 Update 提供独立的 param→DAO 转换路径，省略字段产出 nil 指针，交给 DAO 的 nil-skip 保留原值，镜像 Provider faf9556 / issue-147 的修法。

`storage/rdb/entity/entity.go`：

1. 抽出公共基础转换 `entityBaseDataToParam`（EntityID/Name/Type/ParentID/QuotaPlanID/RateLimitPolicyID/RouteRulesID）；
2. 原 `entityDataToParam` 保留供 Create 使用（`AllowModels`/`BlockModels` 省略时默认 `"[]"`，语义不变）；
3. 新增 `entityDataToParamForUpdate` 供 `UpdateEntity` 使用：仅当 `len(param.AllowModels) > 0` / `len(param.BlockModels) > 0` 时才 marshal 赋值，否则保持 nil，交 DAO 跳过该列。

## 6. 单元测试

`storage/rdb/entity/entity_test.go`（sqlite 内存库，沿用 `storage/rdb/provider/provider_test.go` 模式）：

- `TestUpdateEntity_OmittedModelsPreserveValues`：创建时显式指定 allow/block_models，PATCH 仅改 name → 两字段原值保留；
- `TestUpdateEntity_ProvidedModelsAreWritten`：显式更新 → 正确写入；
- `TestCreateEntity_OmittedModelsDefaultToEmpty`：Create 省略 → 默认空列表（读回为空），语义不变。

## 7. 回归验证

1. 本地：`go build ./...`、`go vet ./...`、`go test ./...`、`go test -cover ./model/...`（≥70% 门禁，实测 80.3%）全部通过；
2. 集成测试（`test/integration`，真实二进制子进程）：`tests/entity/partial_update` 新增 E-5-006（PATCH 省略 allow_models 保持原值）、E-5-007（PATCH 仅改 name 时 allow_models 保持原值），全部 PASS；
3. 部署后重跑 E2E SC1203-TC028（断言为 fail-fast，若仍失败报告只呈现第一个失配字段，人工核对时请两字段都看），预期 PASSED。

## 8. 实施记录

- 与 issue #151 同批实现，提交：`9fa8c62 fix(api-key): PATCH 省略 models/subnet 不再重置为 ["*"]（issue #151）`（分支 `v0.0.9`，已推送 origin）；
- 设计文档：[issue-151 变更记录](../2026-09-11-issue-151-api-key-patch-models-subnet-reset/change-summary.md)、[sys-design 细节文档《部分更新语义与DAO的nil-skip约定》](../../sys-design/details/部分更新语义与DAO-nil-skip约定.md)。
