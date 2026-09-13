# Certificate 测试用例设计文档

## 1. 模块概述

Certificate 模块负责 TLS 证书的管理，包括创建、列表、详情、设置默认、删除。v0.3.0 新增 `GET /certificates/one` 接口，支持通过 ID 或名称查询单条证书。

## 2. 接口列表

| 编号 | 接口名称 | 方法 | 路径 | 说明 |
|------|----------|------|------|------|
| CERT-1 | 创建证书 | POST | `/open-api/v1/certificates` | 返回不包含 cert/key 内容 |
| CERT-2 | 证书列表 | GET | `/open-api/v1/certificates` | 数组 |
| CERT-3 | 证书详情（按名称） | GET | `/open-api/v1/certificates/{cert_name}` | 按证书名称查询 |
| CERT-4 | 证书详情（按 ID 或名称） | GET | `/open-api/v1/certificates/one` | v0.3.0 新增 |
| CERT-5 | 设为默认证书 | PATCH | `/open-api/v1/certificates/{cert_name}/default` | 旧默认自动降级 |
| CERT-6 | 删除证书 | DELETE | `/open-api/v1/certificates/{cert_name}` | 默认证书不可删除 |

## 3. 测试用例统计

| 接口 | 测试用例数 |
|------|-----------|
| 创建证书 | 13 |
| 证书列表 | 1 |
| 证书详情（按名称） | 2 |
| 证书详情（按 ID 或名称） | 2 |
| 设为默认证书 | 1 |
| 删除证书 | 2 |
| **合计** | **21** |

## 4. 认证方式

测试环境配置 `SkipTokenValidate=true`，所有请求无需携带认证头。

## 5. 目录结构

```
certificate/
├── design.md
├── create/
│   └── create_test.go
├── list/
│   └── list_test.go
├── detail/
│   └── detail_test.go
├── one/
│   └── one_test.go
├── set_default/
│   └── set_default_test.go
└── delete/
    └── delete_test.go
```

## 6. 创建证书

### 6.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Certificate |
| 接口名称 | 创建证书 |
| 方法 | POST |
| 路径 | `/open-api/v1/certificates` |
| 说明 | 创建 TLS 证书，返回证书元数据（不含内容字段） |

### 6.2 接口参数说明

#### 6.2.1 请求参数

##### Body 参数

| 参数名 | 类型 | 必填 | 说明 | 合法性条件 |
|--------|------|------|------|------------|
| cert_name | string | Y | 证书名，必须唯一 | 长度 2-64；仅允许字母、数字、`_`、`-`、`.`；不能以 `.`、`-`、`_` 开头或结尾；全局唯一；不能包含空白字符 |
| description | string | Y | 证书描述 | 必填；长度 2-256；不能包含控制字符 |
| is_default | bool | Y | 是否是默认证书 | 必填 |
| cert_file_name | string | Y | 主证书文件名 | - |
| cert_file_content | string | Y | 主证书文件内容 | 必填；须为合法 PEM X.509 证书 |
| key_file_name | string | Y | 主证书密钥名 | - |
| key_file_content | string | Y | 主证书密钥文件内容 | 必填；须为与证书匹配的 PEM 私钥 |
| expired_date | string | Y | 主证书过期时间，如 `2021-08-23 16:02:31` | - |

#### 6.2.2 返回数据字段

| 参数名 | 类型 | 说明 |
|--------|------|------|
| cert_name | string | 证书名 |
| description | string | 证书描述 |
| is_default | bool | 是否是默认证书 |
| cert_file_name | string | 主证书文件名 |
| key_file_name | string | 主证书密钥名 |
| expired_date | string | 主证书过期时间 |

> `cert_file_content`、`key_file_content` 不返回。

### 6.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| CERT-1-001 | 创建非默认证书 | 正常参数 | 返回元数据不含内容字段 |
| CERT-1-002 | 创建默认证书 | 正常参数 | 旧默认自动变为非默认 |
| CERT-1-003 | 证书与密钥不匹配 | 异常参数 | 验证 ErrNum=422 或 500 |
| CERT-1-004 | cert_name 含 `/` | 异常参数 | 验证 ErrNum=422（Issue #133） |
| CERT-1-005 | cert_name 含 `?` | 异常参数 | 验证 ErrNum=422（Issue #133） |
| CERT-1-006 | cert_name 含 `#` | 异常参数 | 验证 ErrNum=422（Issue #133） |
| CERT-1-007 | cert_name 含空格 | 异常参数 | 验证 ErrNum=422（Issue #133） |
| CERT-1-008 | cert_name 含 `%` | 异常参数 | 验证 ErrNum=422（Issue #133） |
| CERT-1-009 | cert_name 长度为 1 | 异常参数 | 验证 ErrNum=422 |
| CERT-1-010 | cert_name 长度为 65 | 异常参数 | 验证 ErrNum=422 |
| CERT-1-011 | cert_name 以 `-` 开头 | 异常参数 | 验证 ErrNum=422 |
| CERT-1-012 | cert_name 以 `_` 结尾 | 异常参数 | 验证 ErrNum=422 |
| CERT-1-013 | cert_name 含 `.`（合法） | 正常参数 | 验证含点证书名创建成功（Issue #133） |

### 6.4 测试场景详细设计

#### 6.4.1 CERT-1-001：创建非默认证书（正常参数）

##### 设计思路

验证创建非默认证书的基本功能，返回字段不包含证书内容。

##### 前提数据准备

生成有效的自签名证书与密钥对。

##### 执行步骤

1. 发送 POST 请求到 `/open-api/v1/certificates`，传入完整参数，`is_default=false`。
2. 验证响应状态码和返回结构。
3. 验证返回对象不包含 `cert_file_content` 和 `key_file_content`。

##### 请求参数

```json
{
    "cert_name": "cert_001",
    "description": "测试证书",
    "is_default": false,
    "cert_file_name": "cert.pem",
    "cert_file_content": "-----BEGIN CERTIFICATE-----...-----END CERTIFICATE-----",
    "key_file_name": "key.pem",
    "key_file_content": "-----BEGIN RSA PRIVATE KEY-----...-----END RSA PRIVATE KEY-----",
    "expired_date": "2026-08-23 16:02:31"
}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | "cert_001" | Equals |
| description | "测试证书" | Equals |
| is_default | false | Equals |
| cert_file_name | "cert.pem" | Equals |
| key_file_name | "key.pem" | Equals |
| expired_date | "2026-08-23 16:02:31" | Equals |
| cert_file_content | 不存在 | NotExists |
| key_file_content | 不存在 | NotExists |

---

#### 6.4.2 CERT-1-002：创建默认证书（正常参数）

##### 设计思路

验证创建默认证书时，若系统中已存在默认证书，旧默认自动变为非默认。

##### 前提数据准备

系统中已存在旧默认证书 A。

##### 执行步骤

1. 发送 POST 请求创建新证书 B，`is_default=true`。
2. 验证 B 的 `is_default=true`。
3. 查询旧证书 A，验证其 `is_default=false`。

##### 请求参数

```json
{
    "cert_name": "cert_default_new",
    "description": "新默认证书",
    "is_default": true,
    "cert_file_name": "cert.pem",
    "cert_file_content": "-----BEGIN CERTIFICATE-----...-----END CERTIFICATE-----",
    "key_file_name": "key.pem",
    "key_file_content": "-----BEGIN RSA PRIVATE KEY-----...-----END RSA PRIVATE KEY-----",
    "expired_date": "2026-08-23 16:02:31"
}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | "cert_default_new" | Equals |
| is_default | true | Equals |

---

#### 6.4.3 CERT-1-003：证书与密钥不匹配（异常参数）

##### 设计思路

验证证书文件与密钥文件不匹配时返回错误。

##### 前提数据准备

准备不匹配的证书内容与密钥内容。

##### 执行步骤

1. 发送 POST 请求，`cert_file_content` 与 `key_file_content` 不匹配。
2. 验证返回错误码。

##### 请求参数

```json
{
    "cert_name": "cert_mismatch",
    "description": "不匹配证书",
    "is_default": false,
    "cert_file_name": "cert.pem",
    "cert_file_content": "-----BEGIN CERTIFICATE-----INVALID-----END CERTIFICATE-----",
    "key_file_name": "key.pem",
    "key_file_content": "-----BEGIN RSA PRIVATE KEY-----INVALID-----END RSA PRIVATE KEY-----",
    "expired_date": "2026-08-23 16:02:31"
}
```

##### 预期返回结果

**ErrNum**：422 或 500  
**ErrMsg**：证书与密钥不匹配或解析失败的错误信息  
**Data**：null

---

#### 6.4.4 CERT-1-004 ~ CERT-1-012：cert_name 命名规范校验（异常参数）

##### 设计思路

验证 `cert_name` 不符合命名规范（[CertName](../../../design-docs/api-define/OpenAPI接口定义/00-common.md)）时创建请求返回 422。覆盖 Issue #133 报告的 URL 语义字符（`/`、`?`、`#`、空格、`%`）与长度/首尾约束边界。除 `cert_name` 外的参数均合法（使用 `GenerateTestCert` 生成匹配的证书与密钥），确保唯一失败原因是名称非法。

> 背景：创建侧原本无字符集限制，含 `/` 的证书创建成功后 DELETE 路由（单路径段绑定）无法寻址，形成"存在但不可删除"的残留。本组用例防止该缺陷回归。

##### 前提数据准备

无（依赖测试前置创建的默认证书，非法名请求不产生任何数据）。

##### 执行步骤

1. 发送 POST 请求到 `/open-api/v1/certificates`，`cert_name` 分别取下列非法值，其余参数合法。
2. 验证返回 ErrNum=422。

##### 请求参数

| 用例 | cert_name | 非法原因 |
|------|-----------|---------|
| CERT-1-004 | `demo/child` | 含 `/` |
| CERT-1-005 | `demo?x=1` | 含 `?` |
| CERT-1-006 | `demo#1` | 含 `#` |
| CERT-1-007 | `demo cert` | 含空格 |
| CERT-1-008 | `demo%2F` | 含 `%` |
| CERT-1-009 | `a` | 长度 1（小于下限 2） |
| CERT-1-010 | 65 个 `a` | 长度 65（超过上限 64） |
| CERT-1-011 | `-demo` | 以 `-` 开头 |
| CERT-1-012 | `demo_` | 以 `_` 结尾 |

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：cert_name 包含非法字符或长度不合法的参数错误信息  
**Data**：null

---

#### 6.4.13 CERT-1-013：cert_name 含 `.`（正常参数）

##### 设计思路

验证含 `.` 的证书名（如 `<唯一名>.qa`）属于合法字符集，创建成功并可正常删除，作为 CERT-1-004~012 的对照组。

##### 前提数据准备

无。

##### 执行步骤

1. 发送 POST 请求到 `/open-api/v1/certificates`，`cert_name` 为含 `.` 的唯一名，其余参数合法。
2. 验证返回成功且元数据正确（不含内容字段）。
3. 测试清理阶段删除该证书。

##### 请求参数

```json
{
    "cert_name": "<unique>.qa",
    "description": "命名规范校验",
    "is_default": false,
    "cert_file_content": "-----BEGIN CERTIFICATE-----...-----END CERTIFICATE-----",
    "key_file_content": "-----BEGIN RSA PRIVATE KEY-----...-----END RSA PRIVATE KEY-----"
}
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | 与请求一致 | Equals |
| cert_file_content | 不存在 | NotExists |
| key_file_content | 不存在 | NotExists |

---

## 7. 证书列表

### 7.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Certificate |
| 接口名称 | 证书列表 |
| 方法 | GET |
| 路径 | `/open-api/v1/certificates` |
| 说明 | 获取全体证书信息列表 |

### 7.2 接口参数说明

#### 7.2.1 请求参数

无

#### 7.2.2 返回数据字段

Data 为数组，元素字段同 6.2.2。

### 7.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| CERT-2-001 | 证书列表 | 正常参数 | 返回数组元素不含内容字段 |

### 7.4 测试场景详细设计

#### 7.4.1 CERT-2-001：证书列表（正常参数）

##### 设计思路

验证列表接口返回所有证书元数据，且不包含内容字段。

##### 前提数据准备

已创建至少一个证书。

##### 执行步骤

1. 发送 GET 请求到 `/open-api/v1/certificates`。
2. 验证返回数组元素字段完整且不含 `cert_file_content`/`key_file_content`。

##### 请求参数

无

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| Data | 数组 | IsArray |
| Data[*].cert_name | 非空字符串 | NotEmpty |
| Data[*].cert_file_content | 不存在 | NotExists |
| Data[*].key_file_content | 不存在 | NotExists |

---

## 8. 证书详情（按名称）

### 8.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Certificate |
| 接口名称 | 证书详情（按名称） |
| 方法 | GET |
| 路径 | `/open-api/v1/certificates/{cert_name}` |
| 说明 | 按证书名称获取单个证书信息 |

### 8.2 接口参数说明

#### 8.2.1 请求参数

##### URI 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| cert_name | string | Y | 证书名称 |

#### 8.2.2 返回数据字段

同 6.2.2。

### 8.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| CERT-3-001 | 查询已存在证书 | 正常参数 | 返回证书元数据 |
| CERT-3-002 | 查询不存在的证书 | 异常参数 | 验证 ErrNum=404 |

### 8.4 测试场景详细设计

#### 8.4.1 CERT-3-001：查询已存在证书（正常参数）

##### 设计思路

验证按名称查询证书详情的基本功能。

##### 前提数据准备

已创建证书 `cert_001`。

##### 执行步骤

1. 发送 GET 请求到 `/open-api/v1/certificates/cert_001`。
2. 验证返回字段完整且不含内容字段。

##### 请求参数

URI：`cert_001`

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | "cert_001" | Equals |
| description | 与创建时一致 | Equals |
| is_default | 与创建时一致 | Equals |
| cert_file_content | 不存在 | NotExists |
| key_file_content | 不存在 | NotExists |

---

#### 8.4.2 CERT-3-002：查询不存在的证书（异常参数）

##### 设计思路

验证查询不存在的证书时返回 404。

##### 前提数据准备

无

##### 执行步骤

1. 发送 GET 请求到 `/open-api/v1/certificates/non_existent_cert`。
2. 验证返回错误码。

##### 请求参数

URI：`non_existent_cert`

##### 预期返回结果

**ErrNum**：404  
**ErrMsg**：证书不存在的错误信息  
**Data**：null

---

## 9. 证书详情（按 ID 或名称）

### 9.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Certificate |
| 接口名称 | 证书详情（按 ID 或名称） |
| 方法 | GET |
| 路径 | `/open-api/v1/certificates/one` |
| 说明 | v0.3.0 新增，通过 ID 或名称查询单条证书 |

### 9.2 接口参数说明

#### 9.2.1 请求参数

##### Query 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | string | N | 证书内部 ID |
| name | string | N | 证书名称 |

> `id` 与 `name` 至少传一个。

#### 9.2.2 返回数据字段

同 6.2.2。

### 9.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| CERT-4-001 | 按名称查询单条证书 | 正常参数 | 通过 name 查询 |
| CERT-4-002 | 按 ID 查询单条证书 | 正常参数 | 通过 id 查询 |

### 9.4 测试场景详细设计

#### 9.4.1 CERT-4-001：按名称查询单条证书（正常参数）

##### 设计思路

验证通过 `name` 参数查询单条证书的功能。

##### 前提数据准备

已创建证书 `cert_001`。

##### 执行步骤

1. 发送 GET 请求到 `/open-api/v1/certificates/one?name=cert_001`。
2. 验证返回字段与目标证书一致。

##### 请求参数

```
name=cert_001
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | "cert_001" | Equals |
| cert_file_content | 不存在 | NotExists |
| key_file_content | 不存在 | NotExists |

---

#### 9.4.2 CERT-4-002：按 ID 查询单条证书（正常参数）

##### 设计思路

验证通过 `id` 参数查询单条证书的功能。

##### 前提数据准备

已创建证书，并记录其内部 ID。

##### 执行步骤

1. 发送 GET 请求到 `/open-api/v1/certificates/one?id=<cert_id>`。
2. 验证返回字段与目标证书一致。

##### 请求参数

```
id=<cert_id>
```

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | 与目标证书一致 | Equals |
| cert_file_content | 不存在 | NotExists |
| key_file_content | 不存在 | NotExists |

---

## 10. 设为默认证书

### 10.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Certificate |
| 接口名称 | 设为默认证书 |
| 方法 | PATCH |
| 路径 | `/open-api/v1/certificates/{cert_name}/default` |
| 说明 | 将指定证书设为默认，旧默认自动降级 |

### 10.2 接口参数说明

#### 10.2.1 请求参数

##### URI 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| cert_name | string | Y | 证书名称 |

#### 10.2.2 返回数据字段

同 6.2.2。

### 10.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| CERT-5-001 | 设置默认证书 | 正常参数 | 旧默认变为非默认 |

### 10.4 测试场景详细设计

#### 10.4.1 CERT-5-001：设置默认证书（正常参数）

##### 设计思路

验证将非默认证书设为默认时，旧默认证书自动变为非默认。

##### 前提数据准备

已存在默认证书 A，已创建非默认证书 B。

##### 执行步骤

1. 发送 PATCH 请求到 `/open-api/v1/certificates/B/default`。
2. 验证返回 B 的 `is_default=true`。
3. 查询证书 A，验证其 `is_default=false`。

##### 请求参数

URI：`B`

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| cert_name | "B" | Equals |
| is_default | true | Equals |

---

## 11. 删除证书

### 11.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Certificate |
| 接口名称 | 删除证书 |
| 方法 | DELETE |
| 路径 | `/open-api/v1/certificates/{cert_name}` |
| 说明 | 删除证书记录及文件内容，默认证书不可删除 |

### 11.2 接口参数说明

#### 11.2.1 请求参数

##### URI 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| cert_name | string | Y | 证书名称 |

#### 11.2.2 返回数据字段

Data 为 null。

### 11.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| CERT-6-001 | 删除非默认证书 | 正常参数 | 删除成功，再次查询返回 404 |
| CERT-6-002 | 删除默认证书 | 业务规则 | 默认证书不可删除 |

### 11.4 测试场景详细设计

#### 11.4.1 CERT-6-001：删除非默认证书（正常参数）

##### 设计思路

验证删除非默认证书成功，并级联清理文件内容。

##### 前提数据准备

已创建非默认证书 `cert_del`。

##### 执行步骤

1. 发送 DELETE 请求到 `/open-api/v1/certificates/cert_del`。
2. 验证返回成功。
3. 再次查询证书，验证返回 404。

##### 请求参数

URI：`cert_del`

##### 预期返回结果

**ErrNum**：200  
**ErrMsg**：success

**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| Data | null | IsNull |

---

#### 11.4.2 CERT-6-002：删除默认证书（业务规则）

##### 设计思路

验证默认证书不可删除。

##### 前提数据准备

已创建默认证书。

##### 执行步骤

1. 发送 DELETE 请求到默认证书路径。
2. 验证返回错误码。

##### 请求参数

URI：默认证书名

##### 预期返回结果

**ErrNum**：422 或 409  
**ErrMsg**：默认证书不可删除的错误信息  
**Data**：null

---

## 12. 依赖与数据准备

1. 测试用例需要生成自签名证书对，可在 `testutil` 中提供 `GenerateCert()` 辅助函数。
2. 数据库需初始化默认证书，或测试用例首先创建默认证书以验证删除约束。

## 13. 注意事项

1. v0.3.0 新增 `/certificates/one` 详情接口，支持通过 ID 或名称查询单条证书。
2. 所有查询接口返回的证书元数据均不包含 `cert_file_content` 与 `key_file_content`。
3. 全局必须保留且仅保留一个默认证书。
4. 测试环境 `SkipTokenValidate=true`，无需认证头。
