# 操作日志（Operation Log）接口变更说明

## 1. 变更范围

本次变更新增一个只读 OpenAPI，用于查询配置操作日志。各既有写接口（如 `POST /entities`、`PUT /api-keys/{id}` 等）的内部实现会接入操作日志记录，但接口契约（请求/响应字段、路径、错误码）保持不变。

## 2. 新增接口

### 2.1 `GET /open-api/v1/operation-logs`

查询操作日志列表。

#### 请求参数（Query）

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| `operator_name` | string | 否 | 操作人名称，支持模糊匹配 |
| `action` | string | 否 | 操作动作，如 `create` / `update` / `delete` / `reset` |
| `resource_type` | string | 否 | 资源类型，如 `entity` / `api_key` / `provider` |
| `resource_id` | string | 否 | 资源业务 ID |
| `status` | int | 否 | `1` 成功，`2` 失败 |
| `start_time` | int64 | 否 | 起始时间戳（秒） |
| `end_time` | int64 | 否 | 结束时间戳（秒） |
| `page` | int | 否 | 页码，默认 1 |
| `page_size` | int | 否 | 每页条数，默认 20，最大 100 |

#### 响应示例

```json
{
  "ErrNum": 0,
  "ErrMsg": "success",
  "Data": {
    "total": 152,
    "page": 1,
    "page_size": 20,
    "list": [
      {
        "id": 1001,
        "log_id": "a1b2c3d4",
        "operator_type": 0,
        "operator_id": 1,
        "operator_name": "admin",
        "action": "update",
        "resource_type": "entity",
        "resource_id": "entity-5",
        "resource_name": "rd-team",
        "resource_parent_id": "entity-1",
        "status": 1,
        "error_msg": "",
        "change_summary": {
          "before": { "allow_models": ["gpt-4"] },
          "after": { "allow_models": ["gpt-4", "gpt-4o"] },
          "diff_keys": ["allow_models"]
        },
        "request_path": "/open-api/v1/entities/entity-5",
        "request_method": "PUT",
        "client_ip": "10.0.0.5",
        "user_agent": "Mozilla/5.0 ...",
        "created_at": 1725091200
      }
    ]
  }
}
```

#### 响应字段说明

| 字段名 | 类型 | 说明 |
|--------|------|------|
| `id` | int64 | 日志自增主键 |
| `log_id` | string | 请求唯一标识，与 access log 中的 LogID 一致 |
| `operator_type` | int8 | 操作者类型：`0` user，`1` token |
| `operator_id` | int64 | 操作者在对应表中的主键 ID |
| `operator_name` | string | 操作者名称 |
| `action` | string | 操作动作 |
| `resource_type` | string | 资源类型 |
| `resource_id` | string | 资源业务 ID |
| `resource_name` | string | 资源名称 |
| `resource_parent_id` | string | 资源父级业务 ID |
| `status` | int8 | `1` 成功，`2` 失败 |
| `error_msg` | string | 失败时的简要错误信息 |
| `change_summary` | object | 变更摘要，包含 `before` / `after` / `diff_keys` |
| `request_path` | string | 请求路径 |
| `request_method` | string | 请求方法 |
| `client_ip` | string | 客户端 IP |
| `user_agent` | string | User-Agent |
| `created_at` | int64 | 操作时间戳（秒） |

#### 权限控制

- 该接口仅允许 `system_admin` 或具有 `FeatureOperationLog` 读权限的角色访问。
- 写入操作日志由系统内部自动完成，不对外暴露写入接口。

## 3. 既有接口变更

以下既有 OpenAPI 在业务成功后（或失败后）会触发操作日志写入，但**接口契约不变**：

- `POST /entities`、`PUT /entities/{entity_id}`、`DELETE /entities/{entity_id}`
- `POST /entity-types`、`PUT /entity-types/{id}`、`DELETE /entity-types/{id}`
- `POST /api-keys`、`PUT /api-keys/{id}`、`DELETE /api-keys/{id}`
- `POST /providers`、`PUT /providers/{id}`、`DELETE /providers/{id}`
- `POST /clusters`、`PUT /clusters/{name}`、`DELETE /clusters/{name}`
- `POST /routes`、`PUT /routes/{id}`、`DELETE /routes/{id}`
- `POST /domains`、`PUT /domains/{name}`、`DELETE /domains/{name}`
- `POST /certificates`、`PUT /certificates/{id}`、`DELETE /certificates/{id}`
- `POST /quota-plans`、`PUT /quota-plans/{id}`、`DELETE /quota-plans/{id}`、`POST /quota-plans/{id}/reset`
- `POST /rate-limit-policies`、`PUT /rate-limit-policies/{id}`、`DELETE /rate-limit-policies/{id}`
- `POST /model-prices`、`PUT /model-prices/{id}`、`DELETE /model-prices/{id}`、`POST /model-prices/import`
- `POST /users`、`PUT /users/{id}`、`DELETE /users/{id}`
- `POST /tokens`、`DELETE /tokens/{id}`、用户-产品授权相关接口

## 4. 测试影响

- 新增 `GET /open-api/v1/operation-logs` 的单元测试与集成测试。
- 各配置域 Manager 的单元测试中，需验证写操作后是否正确调用 `OperationLogManager.Record()`。
- 需补充敏感字段脱敏测试，确保 api-key token、密码、私钥等不会被明文记录。
