# 操作日志集成测试设计文档

## 1. 模块概述

操作日志模块用于记录 `ai-gateway-api` 控制面配置变更行为，并将日志持久化到数据库。集成测试通过调用各配置域的写接口，验证对应操作日志能够正确生成，并验证查询接口的过滤、分页能力。

## 2. 接口列表

| 编号 | 接口名称 | 方法 | 路径 | 说明 |
|------|----------|------|------|------|
| OL-1 | 查询操作日志 | GET | `/open-api/v1/operation-logs` | 支持多维度过滤与分页 |

## 3. 测试用例统计

| 场景 | 测试用例数 |
|------|-----------|
| 多域 API 操作日志生成 | 13 |
| 查询过滤与分页 | 7 |
| update diff_keys 验证 | 2 |
| 溯源字段记录 | 1 |
| 失败日志资源身份 | 1 |
| **合计** | **24** |

## 4. 认证方式

测试环境配置 `SkipTokenValidate=true`，所有请求无需携带认证头。

## 5. 目录结构

```
operation_log/
├── design.md
└── list/
    └── list_test.go
```

## 6. 多域 API 操作日志生成

### 6.1 设计思路

覆盖控制面主要配置域的写操作：创建、更新、删除。通过轮询查询操作日志接口，确认每条写操作都能在 10 秒内生成对应的操作日志记录。

### 6.2 覆盖域与操作

| 编号 | 资源类型 | 操作 | 触发 API |
|------|----------|------|----------|
| OL-GEN-001 | `entity_type` | `create` | `POST /open-api/v1/entity-types` |
| OL-GEN-002 | `entity_type` | `update` | `PATCH /open-api/v1/entity-types/{type_name}` |
| OL-GEN-003 | `entity` | `create` | `POST /open-api/v1/entities` |
| OL-GEN-004 | `api_key` | `create` | `POST /open-api/v1/api-keys` |
| OL-GEN-005 | `api_key` | `delete` | `DELETE /open-api/v1/api-keys/{id}` |
| OL-GEN-006 | `provider` | `create` | `POST /open-api/v1/providers` |
| OL-GEN-007 | `cluster` | `create` | `POST /open-api/v1/clusters` |
| OL-GEN-008 | `certificate` | `create` | `POST /open-api/v1/certificates`（默认证书） |
| OL-GEN-009 | `certificate` | `create` | `POST /open-api/v1/certificates`（非默认证书） |
| OL-GEN-010 | `user` | `create` | `POST /open-api/v1/auth/users` |
| OL-GEN-011 | `token` | `create` | `POST /open-api/v1/auth/tokens` |
| OL-GEN-012 | `route` | `update` | `PUT /open-api/v1/global-route-rules` |

### 6.3 校验点

每条操作日志记录应满足：

- `resource_type` 与预期一致。
- `action` 与预期一致（`create` / `update` / `delete`）。
- `status` 为 `1`（成功）。
- `request_path` 非空。
- `created_at` 非空。
- `resource_id` 或 `resource_name` 可被正确查询到。

> 说明：`cluster`、`user`、`token` 在数据库中使用内部数值 ID 作为 `resource_id`，因此测试中改为按 `resource_name` 查询。

### 6.4 依赖与数据准备

1. 创建 `entity` 前需先创建 `entity-type`。
2. 创建 `api-key` 前需先创建 `entity`。
3. 创建 `cluster` 前需先创建 `provider`。
4. 更新 Global 路由表前需先创建 `cluster`。
5. 创建非默认证书前需先创建默认证书。

### 6.5 清理策略

测试完成后按相反顺序删除创建的资源并重置 Global 路由表：ResetGlobalRouteRules、token、user、certificate、cluster、provider、api-key、entity、entity-type。

---

## 7. 查询操作日志

### 7.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Operation Log |
| 接口名称 | 查询操作日志列表 |
| 方法 | GET |
| 路径 | `/open-api/v1/operation-logs` |
| 说明 | 支持按操作人、动作、资源类型、资源 ID、状态、时间范围过滤与分页 |

### 7.2 接口参数说明

#### 7.2.1 请求参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `operator_name` | string | 否 | 操作人名称，模糊匹配 |
| `action` | string | 否 | 操作动作，如 `create` / `update` / `delete` |
| `resource_type` | string | 否 | 资源类型 |
| `resource_id` | string | 否 | 资源业务 ID |
| `resource_name` | string | 否 | 资源名称 |
| `status` | int | 否 | `1` 成功，`2` 失败 |
| `start_time` | int64 | 否 | 起始时间戳（秒） |
| `end_time` | int64 | 否 | 结束时间戳（秒） |
| `page` | int | 否 | 页码，默认 1 |
| `page_size` | int | 否 | 每页条数，默认 20，最大 100 |

#### 7.2.2 返回数据字段

| 参数名 | 类型 | 说明 |
|--------|------|------|
| `list` | []OperationLog | 操作日志列表 |
| `pagination.page` | int | 当前页码 |
| `pagination.page_size` | int | 每页条数 |
| `pagination.total` | int64 | 总条数 |

`OperationLog` 主要字段：

| 字段名 | 类型 | 说明 |
|--------|------|------|
| `id` | int64 | 日志自增主键 |
| `log_id` | string | 请求唯一标识 |
| `operator_type` | int8 | `0` user，`1` token |
| `operator_name` | string | 操作人名称 |
| `action` | string | 操作动作 |
| `resource_type` | string | 资源类型 |
| `resource_id` | string | 资源业务 ID |
| `resource_name` | string | 资源名称 |
| `status` | int8 | `1` 成功，`2` 失败 |
| `change_summary` | object | 变更前后摘要 |
| `request_path` | string | 请求路径 |
| `request_method` | string | 请求方法 |
| `created_at` | int64 | 操作时间戳 |

### 7.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| OL-2-001 | 无条件查询 | 正常参数 | 返回包含前置操作日志的分页结构 |
| OL-2-002 | 按 action 过滤 | 正常参数 | `action=create` 时所有记录动作均为 create |
| OL-2-003 | 按 resource_type 过滤 | 正常参数 | `resource_type=entity` 仅返回 entity 相关日志 |
| OL-2-004 | 按 resource_id 过滤 | 正常参数 | 精确匹配指定 entity ID |
| OL-2-005 | 按不存在 action 过滤 | 异常参数 | 返回空列表，total=0 |
| OL-2-006 | 分页查询 | 边界值 | `page=1&page_size=2` |
| OL-2-007 | 第二页仍有 total | 边界值 | `page=2&page_size=2`，验证 total 不为 0 |

### 7.4 测试场景详细设计

#### 7.4.1 OL-2-001：无条件查询

##### 设计思路

验证无条件查询返回分页结构，且列表中包含当前测试创建的 entity create 日志。

##### 前提数据准备

已创建 `entity-type`、`entity`、`api-key`。

##### 执行步骤

1. 发送 GET 请求到 `/open-api/v1/operation-logs`。
2. 验证返回结构。
3. 在列表中查找 `resource_type=entity`、`action=create`、`resource_id=<entity_id>` 的记录，校验字段完整。

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| `list` | 数组 | IsArray |
| `pagination.page` | 1 | Equals |
| `pagination.page_size` | 20 | Equals |
| `pagination.total` | ≥ 3 | Gte |

---

#### 7.4.2 OL-2-002：按 action 过滤

##### 设计思路

验证按 `action=create` 过滤后，返回记录的动作均为 `create`。

##### 请求参数

```
action=create
```

##### 预期返回结果

**ErrNum**：200  
**total** ≥ 3

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| `list[*].action` | 全部为 `create` | Equals |

---

#### 7.4.3 OL-2-003：按 resource_type 过滤

##### 设计思路

验证按 `resource_type=entity` 过滤后，仅返回 entity 相关日志。

##### 请求参数

```
resource_type=entity
```

##### 预期返回结果

**ErrNum**：200  
**total** ≥ 1

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| `list[*].resource_type` | 全部为 `entity` | Equals |

---

#### 7.4.4 OL-2-004：按 resource_id 过滤

##### 设计思路

验证按 `resource_id=<entity_id>` 可精确定位到指定资源的操作日志。

##### 请求参数

```
resource_id=<entity_id>
```

##### 预期返回结果

**ErrNum**：200  
**total** ≥ 1

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| `list[0].resource_id` | `<entity_id>` | Equals |
| `list[0].action` | `create` | Equals |

---

#### 7.4.5 OL-2-005：按不存在 action 过滤

##### 设计思路

验证按不存在的 `action` 过滤时返回空列表。

##### 请求参数

```
action=not_exist_action
```

##### 预期返回结果

**ErrNum**：200  
**total**：0  
**list**：空数组

---

#### 7.4.6 OL-2-006：分页查询

##### 设计思路

验证分页参数生效，返回指定条数记录且 total 正确。

##### 请求参数

```
page=1&page_size=2
```

##### 预期返回结果

**ErrNum**：200

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| `list` | 长度为 2 | Len=2 |
| `pagination.page` | 1 | Equals |
| `pagination.page_size` | 2 | Equals |
| `pagination.total` | ≥ 3 | Gte |

---

#### 7.4.7 OL-2-007：第二页仍有 total

##### 设计思路

验证分页到第二页时，`pagination.total` 仍然返回正确总数，而不是因 count SQL 带 LIMIT 导致 total 变为 0（issue #123）。

##### 前提数据准备

已创建至少 3 个 `entity-type`，产生 3 条 `entity_type/create` 日志。

##### 请求参数

```
resource_type=entity_type&action=create&page=2&page_size=2
```

##### 预期返回结果

**ErrNum**：200

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| `list` | 长度 ≥ 1 | Gte |
| `pagination.page` | 2 | Equals |
| `pagination.page_size` | 2 | Equals |
| `pagination.total` | ≥ 3 | Gte |

---

## 8. 失败日志资源身份（issue #155）

### 8.1 设计思路

Entity 更新在事务前置校验失败时（如极简 body 只带 `parent_id` 且指向不存在的父节点），失败审计日志必须保留资源身份（`resource_id`/`resource_name`），否则失败记录无法按资源维度检索（issue #155 / E2E SC2101-TC046 回归锚点）。

### 8.2 覆盖场景

| 编号 | 场景 | 触发 API | 校验重点 |
|------|------|----------|----------|
| OL-ID-001 | 前置校验失败的更新日志保留资源身份 | `PUT /open-api/v1/entities/{id}`（body 仅 `parent_id` 指向不存在实体） | `status=2`；`resource_id` == URI 中的 Entity ID；`resource_name` 为库中名称；`error_msg` 非空 |

### 8.3 校验点

- 失败日志 `resource_id`/`resource_name` 非空（修复前置分支从请求体取身份，极简 body 下为空串）。
- `resource_parent_id` 保留请求值（修改意图的一部分）。
- 通过 `WaitForOperationLog` 按 `resource_id` 过滤轮询，直接锚定目标记录。

---

## 9. update diff_keys 验证

### 9.1 设计思路

针对 `update` 动作的操作日志，除校验基本字段外，进一步验证 `change_summary` 中包含 `before`、`after` 以及 `diff_keys`，确保变更差异可被前端正确展示。

### 9.2 覆盖场景

| 编号 | 资源类型 | 操作 | 触发 API | 校验重点 |
|------|----------|------|----------|----------|
| OL-DIFF-001 | `entity_type` | `update` | `PATCH /open-api/v1/entity-types/{type_name}` | `diff_keys` 包含变更的字段 |
| OL-DIFF-002 | `route` | `update` | `PUT /open-api/v1/global-route-rules` | `diff_keys` 包含 `rules`；`before` 与 `after` 分别对应清空与写入后的 Global 路由表 |

### 9.3 数据准备

- `OL-DIFF-002` 需要先创建 `provider` 与 `cluster`，再使用 `ResetGlobalRouteRules` 将 Global 路由表置空作为 `before` 状态，最后调用 `SetGlobalRouteRules` 写入一条规则作为 `after` 状态。

### 9.4 校验点

- `change_summary` 非空且包含 `before`、`after`、`diff_keys`。
- `diff_keys` 数组中包含预期变更的字段名。
- 由于操作日志异步落库且可能存在历史日志，测试通过比对日志 `id` 确保拿到的是本次操作产生的新记录。

---

## 10. 溯源字段记录（issue #127）

### 10.1 设计思路

验证操作日志正确记录 `user_agent` 与 `client_ip` 溯源字段，满足审计合规要求（能回答"谁、从哪发起"）。

### 10.2 覆盖场景

| 编号 | 场景 | 触发 API | 校验重点 |
|------|------|----------|----------|
| OL-TRACE-001 | 记录 User-Agent | `POST /open-api/v1/entity-types` | `user_agent` 非空，与请求 `User-Agent` 头一致 |
| OL-TRACE-001 | 记录真实来源 IP | `POST /open-api/v1/entity-types` | `client_ip` 非空，回退自 `RemoteAddr`/`X-Forwarded-For` |

### 10.3 校验点

- `user_agent` 应记录请求的 `User-Agent` 头（修复前恒为空）。
- `client_ip` 优先取自定义 `ClientIp` 头（兼容既有调用方），否则回退 `X-Forwarded-For` 首段，最后回退 `RemoteAddr` 去端口。

---

## 11. 工具辅助函数

集成测试在 `testutil` 中新增/使用以下辅助函数：

- `QueryOperationLogs(query map[string]string) (*OperationLogListResult, *APIResponse, error)`：查询操作日志。
- `WaitForOperationLog(filter map[string]string, timeout time.Duration) (*OperationLogEntry, error)`：轮询等待匹配的操作日志出现，默认最长 10 秒，避免固定 sleep 导致的不稳定。
- `ResetGlobalRouteRules() error`：清空 Global 路由表。
- `SetGlobalRouteRules(rules []interface{}) error`：设置 Global 路由表。
- `SimpleRouteRule(name, clusterName string) map[string]interface{}`：构造一条最简单的 Global 路由规则。

## 12. 注意事项

1. 操作日志为异步批量落库，默认 5 秒 flush 一次；测试使用轮询而非固定 sleep 等待日志出现。
2. 不同测试用例共享同一 SQLite 数据库，但每个用例使用唯一资源 ID / 名称，避免相互干扰。
3. `cluster`、`user`、`token` 的 `resource_id` 为内部数值 ID，测试中通过 `resource_name` 查询。
4. `route` 类型的 Global 路由表操作日志使用 `resource_id=global`、`resource_name=global` 标识。
5. 测试环境 `SkipTokenValidate=true`，无需认证头。
