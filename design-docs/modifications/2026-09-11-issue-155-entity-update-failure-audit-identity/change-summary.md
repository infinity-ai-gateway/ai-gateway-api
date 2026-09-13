# Issue #155：Entity 更新失败审计日志丢失资源身份修复方案

## 1. 问题来源

[rainway-ai-gateway/ai-gateway-api/issues/155](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/155)

> SC2101-TC046：URI 寻址的 Entity 更新在事务前置校验失败时，失败审计日志 status=2、error_msg 正确，但 resource_id/resource_name 为空串——失败记录无法关联到目标 Entity，按资源身份检索永远查不到。

复现（issue 已实证）：

- 极简 body `PUT /entities/entity-270`，body 仅 `{"parent_id": "entity-888"}`（parent 不存在触发前置校验失败）→ 失败日志 `resource_id=""`、`resource_name=""`（必现）；
- 全量 body 对照组（携带 id/name）→ 身份齐全。差异来源即根因。

影响：审计日志按资源身份检索是运维排查与合规审计的基础能力；更新失败的记录恰恰是最需要追溯的场景（谁在改哪个资源、为什么失败），身份缺失使失败日志无法归档到资源维度。`GET /operation-logs` 返回示例见 issue（id=19215，`resource_id`/`resource_name` 均为空串）。

## 2. 根因（本地 v0.0.9 代码核实，issue 基于 99d53df 的行号已漂移）

调用链：`PUT /entities/{id}`（`endpoints/openapi_v1/entity/update.go:35-64`）→ `EntityManager.UpdateEntity`（`model/entity/entity_manager.go:233`）。

关键点：endpoint 将 URI 的 `id` 绑定到 **`filter.EntityID`**（`update.go:35,45,64`），**不写入 `param`**；`param.EntityID` 仅来自请求体（json tag `"id"`）。极简 body 下 `param.EntityID` 为 nil。

`UpdateEntity` 在**事务之前**有 4 个前置校验失败分支（`entity_manager.go:234-260`），全部用 `entityParamIdentifiers(param)` 从请求体取身份（:236/:243/:249/:255）：

| 分支 | 位置 | 触发条件 |
|------|------|---------|
| 1 | :234-239 | body 带 `type`+`parent_id`，`checkEntityLevel` 失败 |
| 2 | :240-246 | 仅带 `parent_id`，`FetchEntityList` 查询出错 |
| 3 | :247-252 | 仅带 `parent_id`，记录不存在 |
| 4 | :253-258 | 仅带 `parent_id`，用库中 `type` 做 `checkEntityLevel` 失败 |

极简 body 下 `param.EntityID`/`param.Name` 均为 nil → 日志身份为空串。

对照（同函数内正确路径）：

- 事务内失败（:336-343）：`oldEntity != nil` 时改用 `entityParamIdentifiers(oldEntity)`（:337-340），身份正确；
- 成功路径（:349-363）：直接从 `oldEntity` 取身份，正确。

附带同因问题：4 个前置分支的 `change_summary.before` 传的是 `entityParamToMap(param)`（请求回显），非库中旧快照——失败日志的 before/after 对比无意义。

## 3. 同族排查结论（issue 建议项，已核实）

- **API-Key**：不受影响。`endpoints/openapi_v1/api_key/update.go:54` 将 URI id 写入 `param.ID`，且 `UpdateAPIKey`（`model/api_key/api_key.go:497`）无事务前置校验分支，失败路径用 `oldAPIKey` 身份；
- **Provider**：不受影响。按 name 寻址，失败路径（`model/iprovider/provider.go:264`）用 `oldProvider` 身份；
- **Cluster**：更新失败路径同样基于库中旧记录取身份（同模式）。

即该缺陷是 **Entity 特有**：其前置层级校验在 manager 层且先于事务内查询执行，叠加 endpoint 不把 URI id 回填进 param。

## 4. 目标

1. Entity 更新**所有失败路径**（4 个前置分支 + 事务内失败）的审计日志，`resource_id`/`resource_name` 可关联到目标 Entity——身份解析优先级：**库中记录 > URI 寻址（filter）> 请求体 > 空串**；
2. 前置分支在能拿到库中记录时，`change_summary.before` 使用库中旧快照而非请求回显；
3. 事务内失败与成功路径行为不变；
4. 回归验证：E2E SC2101-TC046 重跑 PASSED（断言 `resource_id == URI 中的 Entity ID`）。

## 5. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 主要文件 | `model/entity/entity_manager.go`（`UpdateEntity` 身份解析与 4 个前置分支） |
| 接口契约 | 无变化；仅审计日志字段填充语义修正 |
| 不在范围 | `resource_parent_id` 保留请求值（issue 确认合理：parent 是本次修改意图的一部分）；API-Key/Provider/Cluster 无同族缺陷无需改动；历史空身份日志不回溯修复 |
| 数据迁移 | 无 |

## 6. 最终方案

### 6.1 身份解析辅助函数

在 `entity_manager.go` 新增（或扩展 `entityParamIdentifiers`）：

```go
// resolveEntityIdentifiers 按 库中记录 > filter(URI 寻址) > 请求体 > 空串 解析身份
func resolveEntityIdentifiers(filter *EntityFilter, param, old *EntityParam) (entityID, entityName, parentID string) {
    if old != nil {
        return entityParamIdentifiers(old)
    }
    if filter != nil && filter.EntityID != nil && *filter.EntityID != "" {
        entityID = *filter.EntityID
    }
    if param != nil {
        pID, pName, pParent := entityParamIdentifiers(param)
        if entityID == "" {
            entityID = pID
        }
        if pName != "" {
            entityName = pName
        }
        if pParent != "" {
            parentID = pParent
        }
    }
    return
}
```

### 6.2 UpdateEntity 结构微调

在 `UpdateEntity` 入口处（前置校验之前）先做一次只读查询拿到 `oldEntity`（按唯一键单行，成本可忽略；事务内仍会重新查询以保证一致性，语义不变）：

```go
existing, fetchErr := m.storager.FetchEntityList(ctx, filter)
var oldEntity *EntityParam
if fetchErr == nil && len(existing) > 0 {
    oldEntity = existing[0]
}
```

随后 4 个前置分支统一改用：

```go
entityID, entityName, parentID := resolveEntityIdentifiers(filter, param, oldEntity)
before := entityParamToMap(oldEntity) // 有库中快照时用快照；nil 时保持请求回显
m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, before, entityParamToMap(param), err)
```

分支 2（查询出错）天然 `oldEntity == nil`，退化为 filter/请求体身份；分支 3（记录不存在）用 filter（URI id）；分支 1/4 正常命中记录时用库中身份（id+name 齐全）。

事务内失败（:336-343）与成功路径（:349-363）已是正确实现，不动。

### 6.3 语义说明

- 前置查询放在事务外：只读单行查询，与事务内重新查询不冲突，不改变并发语义；
- `parent_id` 仍取请求体值（当库中无记录时），与 issue 确认的一致。

## 7. 测试

- **单元测试**（`model/entity/entity_manager_test.go`，沿用现有 mock storager 模式）：
  1. 前置校验失败 + 极简 body（param 无 id/name）+ 库中存在记录 → 断言日志 `resource_id`=库中 ID、`resource_name`=库中 name、before=库中快照；
  2. 前置校验失败 + 记录不存在 → 断言 `resource_id`=filter.EntityID（URI id）；
  3. 事务内失败/成功路径行为不变（回归）。
- **集成测试**（`test/integration/tests/operation_log`）：PUT /entities/{id} 极简 body 触发前置校验失败 → `GET /operation-logs` 断言最新失败记录 `resource_id == URI id`；同步登记 `tests/entity/design.md` 与 `tests/operation_log/design.md`。
- **E2E**：重跑 SC2101-TC046。

## 8. 回归验证

1. 本地：`go build ./...`、`go vet ./...`、`go test ./...`、model 覆盖率门禁（≥70%）通过；
2. 部署后 E2E SC2101-TC046 PASSED（断言 `resource_id == URI 中的 Entity ID`）。
