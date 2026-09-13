# /operation-logs

## 1. 数据模型

```json
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
```

**字段说明**

| 字段 | 类型 | 说明 | 可能取值 |
|------|------|------|----------|
| `id` | int64 | 日志自增主键 | - |
| `log_id` | string | 请求唯一标识，与 access log 中的 LogID 一致 | - |
| `operator_type` | int8 | 操作者类型 | `0` user，`1` token |
| `operator_id` | int64 | 操作者在对应表中的主键 ID | - |
| `operator_name` | string | 操作者名称 | - |
| `action` | string | 操作动作 | `create` / `update` / `delete` / `reset` / `import` / `bind` / `unbind` |
| `resource_type` | string | 资源类型 | `entity` / `entity_type` / `api_key` / `provider` / `cluster` / `route` / `certificate` / `quota_plan` / `model_price` / `user` / `token` |
| `resource_id` | string | 被操作资源业务 ID | - |
| `resource_name` | string | 被操作资源名称 | - |
| `resource_parent_id` | string | 资源父级业务 ID | 如 entity 层级中的父节点 |
| `status` | int8 | 操作结果 | `1` success，`2` failed |
| `error_msg` | string | 失败时的简要错误信息 | 成功时为空 |
| `change_summary` | object | 变更摘要 | 包含 `before` / `after` / `diff_keys`，敏感字段已脱敏 |
| `request_path` | string | 请求路径 | - |
| `request_method` | string | 请求方法 | - |
| `client_ip` | string | 客户端 IP | - |
| `user_agent` | string | User-Agent | - |
| `created_at` | int64 | 操作时间 | Unix 时间戳（秒） |

> **说明**：`change_summary` 中已对 api-key token、密码、证书私钥等敏感字段进行脱敏，不会记录原始敏感信息。

---

## 2. 接口清单

### 2.1 查询操作日志列表

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 查询操作日志列表 | - |
| 端点 | /operation-logs | - |
| 版本 | v1 | - |
| method | GET | - |
| 权限 | system_admin / FeatureOperationLog 读权限 | 暂仅系统管理员可查询 |

**输入参数（Query）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| `operator_name` | string | 操作人名称 | N | 支持模糊匹配 | - |
| `action` | string | 操作动作 | N | 如 `create` / `update` / `delete` | - |
| `resource_type` | string | 资源类型 | N | 如 `entity` / `api_key` | - |
| `resource_id` | string | 资源业务 ID | N | - | - |
| `resource_name` | string | 资源名称 | N | 支持模糊匹配 | - |
| `status` | int | 操作结果 | N | `1` 成功，`2` 失败 | - |
| `start_time` | int64 | 起始时间戳（秒） | N | - | - |
| `end_time` | int64 | 结束时间戳（秒） | N | - | - |
| `page` | int | 页码 | N | 默认 1 | 参见 [00-common.md](./00-common.md) |
| `page_size` | int | 每页条数 | N | 默认 20，最大 100 | 参见 [00-common.md](./00-common.md) |

**约束**

- 该接口为只读接口，不修改任何配置。
- `start_time` 与 `end_time` 同时传入时，查询该时间闭区间内的日志。
- 分页参数遵循全局通用 Query 参数约定。

**返回数据（Data 内容）**

```json
{
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
  ],
  "pagination": {
    "page": 1,
    "page_size": 20,
    "total": 152
  }
}
```

**返回字段说明**

| 字段 | 类型 | 说明 |
|------|------|------|
| `list` | array | 操作日志列表，元素类型见第 1 节数据模型 |
| `pagination.page` | int | 当前页码 |
| `pagination.page_size` | int | 当前每页条数 |
| `pagination.total` | int64 | 总记录数 |

**成功返回示例**

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
        "operator_name": "admin",
        "action": "update",
        "resource_type": "entity",
        "resource_id": "entity-5",
        "resource_name": "rd-team",
        "resource_parent_id": "entity-1",
        "status": 1,
        "change_summary": {
          "before": { "allow_models": ["gpt-4"] },
          "after": { "allow_models": ["gpt-4", "gpt-4o"] },
          "diff_keys": ["allow_models"]
        },
        "request_path": "/open-api/v1/entities/entity-5",
        "request_method": "PUT",
        "client_ip": "10.0.0.5",
        "created_at": 1725091200
      }
    ]
  }
}
```

**失败返回示例**

```json
{
  "ErrNum": 402,
  "ErrMsg": "Feature Access Deny",
  "Data": null
}
```

---

## 3. 写入行为说明

操作日志的写入由系统在内部自动完成，**不对外暴露写入接口**。

以下 OpenAPI 写操作成功后（或失败后）会触发操作日志记录，但接口契约（请求/响应字段、路径、错误码）保持不变：

- `/entities` 的 POST / PUT / DELETE
- `/api-keys` 的 POST / PUT / DELETE
- `/providers` 的 POST / PUT / DELETE
- `/clusters` 的 POST / PUT / DELETE
- `/routes` 的 POST / PUT / DELETE
- `/global-route-rules` 的 PUT
- `/certificates` 的 POST / PUT / DELETE
- `/quota-plans` 的 POST / PUT / DELETE / reset
- `/model-prices` 的 POST / PUT / DELETE / import
- `/auth/users` 的 POST / PUT / DELETE
- `/auth/tokens` 的 POST / DELETE
