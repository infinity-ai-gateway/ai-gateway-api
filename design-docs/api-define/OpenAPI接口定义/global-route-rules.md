# /global-route-rules

## 1. 数据模型

```json
{
  "enabled": true,
  "rules": [
    {
      "name": "global-default",
      "cond": "default_t()",
      "targets": [
        {
          "cluster_name": "cluster_global",
          "model": "",
          "weight": 100
        }
      ],
      "fallbacks": [
        {
          "cluster_name": "cluster_global_fallback",
          "model": ""
        }
      ]
    }
  ]
}
```

**字段说明**

| 字段 | 类型 | 说明 | 可能取值 | 合法性条件 |
|------|------|------|----------|----------|
| `enabled` | bool | 是否启用该Global路由表 | `true`（启用）、`false`（禁用），**默认值为`true`** | 非必填；未传时默认 `true` |
| `rules` | array | 规则列表，按顺序匹配 | - | 必填；每个元素类型为 [RouteRule](./00-common.md#公共参数类型) |

**公共类型引用**

| 字段 | 公共类型 | 说明 |
|------|----------|------|
| `rules` | [RouteRule](./00-common.md#公共参数类型) | 路由规则元素，详细字段及合法性条件见 `00-common.md` |

**约束**

- `rules` 每个元素类型为 [RouteRule](./00-common.md#公共参数类型)。

---

## 2. 接口清单

### 2.1 更新Global路由表

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 全量更新Global路由表 | - |
| 端点 | /global-route-rules | - |
| 版本 | v1 | - |
| method | PUT | - |

**输入参数（Body）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| enabled | bool | 是否启用该Global路由表 | N | **默认值为`true`** | 非必填；未传时默认 `true` |
| rules | array | 规则列表 | Y | 同第1节数据模型中rules结构 | 必填；每个元素类型为 [RouteRule](./00-common.md#公共参数类型) |

**HTTP BODY参数示例**

```json
{
    "enabled": true,
    "rules": [
        {
            "name": "global-default",
            "cond": "default_t()",
            "targets": [
                {
                    "cluster_name": "cluster_global",
                    "model": "",
                    "weight": 100
                }
            ],
            "fallbacks": [
                {
                    "cluster_name": "cluster_global_fallback",
                    "model": ""
                }
            ]
        }
    ]
}
```

**执行逻辑**

1. 校验参数合法性
2. 若未传入 `enabled`，使用默认值 `true`
3. 校验 `rules` 中规则名称是否重复
4. 校验每个规则的 `targets` 权重总和是否为100
5. 校验每个规则的 `Cond` 表达式是否合法
6. 在持久化存储中，将 `type` 固定设置为 `global`，`owner` 固定设置为 `global`
7. 写入或替换 `global_default` 路由表
8. 记录操作日志（`resource_type=route`，`resource_id=global`）
9. 返回结果

**返回数据（Data内容）**

字段同第1节数据模型。

**成功返回示例**

```json
{
    "ErrNum": 200,
    "ErrMsg": "success",
    "Data": {
        "enabled": true,
        "rules": [
            {
                "name": "global-default",
                "cond": "default_t()",
                "targets": [
                    {
                        "cluster_name": "cluster_global",
                        "model": "",
                        "weight": 100
                    }
                ],
                "fallbacks": [
                    {
                        "cluster_name": "cluster_global_fallback",
                        "model": ""
                    }
                ]
            }
        ]
    }
}
```

---

### 2.2 查询Global路由表

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 查询Global路由表 | - |
| 端点 | /global-route-rules | - |
| 版本 | v1 | - |
| method | GET | - |

**返回数据（Data内容）**

字段同第1节数据模型。系统初始化时会自动创建一条默认Global路由表（`enabled=false`、`rules=[]`），因此正常情况下直接返回该记录；仅在记录被异常删除时，Data才可能为null。

---

