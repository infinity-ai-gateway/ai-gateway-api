# Model Price 测试用例设计文档

## 1. 模块概述

Model Price 模块负责模型定价数据的管理，支持：

- 通过 `model-list.yaml` 整表导入（`replace` / `merge` 两种模式）
- 单条记录的增删改查
- 按 `id` 或按 `(provider, model, mode)` 组合键查询/更新/删除
- 分时段定价：在 `tier_prices` 中为特定 tier（如 `peak`）设置差异化价格

本次新增该模块的集成测试，覆盖 OpenAPI `/model-prices` 下的全部接口，以及 `tier_prices` 字段在创建、更新、导入场景下的正例与异常校验。

## 2. 接口列表

| 编号 | 接口名称 | 方法 | 路径 | 说明 |
|------|----------|------|------|------|
| MP-1 | 整表导入 | POST | `/open-api/v1/model-prices/import` | 通过 YAML 文件批量导入，支持 replace/merge |
| MP-2 | 新增单条记录 | POST | `/open-api/v1/model-prices` | 创建单条模型定价 |
| MP-3 | 分页列表查询 | GET | `/open-api/v1/model-prices` | 支持 provider、mode 过滤 |
| MP-4 | 按 ID 查询单条 | GET | `/open-api/v1/model-prices/{id}` | - |
| MP-5 | 按组合键查询（列表过滤） | GET | `/open-api/v1/model-prices` | 需传 provider + model + mode，返回列表 |
| MP-6 | 按 ID 修改单条 | PUT | `/open-api/v1/model-prices/{id}` | 支持部分字段更新 |
| MP-7 | 按组合键修改单条 | PUT | `/open-api/v1/model-prices` | 需传 provider + model + mode |
| MP-8 | 按 ID 删除单条 | DELETE | `/open-api/v1/model-prices/{id}` | - |
| MP-9 | 按组合键删除单条 | DELETE | `/open-api/v1/model-prices` | 需传 provider + model + mode |
| MP-10 | 查询 Provider 名称列表 | GET | `/open-api/v1/model-prices/actions/get-providers` | 返回 model-prices 中所有 provider 去重列表 |

## 3. 测试用例统计

| 接口 | 测试用例数 |
|------|-----------|
| 整表导入 | 10 |
| 新增单条记录 | 8 |
| 分页列表查询 | 4 |
| 按 ID 查询单条 | 2 |
| 按组合键查询单条 | 3 |
| 按 ID 修改单条 | 4 |
| 按组合键修改单条 | 3 |
| 按 ID 删除单条 | 2 |
| 按组合键删除单条 | 2 |
| 查询 Provider 名称列表 | 2 |
| tier_prices 字段场景 | 6 |
| **合计** | **44** |

## 4. 认证方式

测试环境配置 `SkipTokenValidate=true`，所有请求无需携带认证头。

## 5. 目录结构

```
model_price/
├── design.md
├── import/
│   └── import_test.go
├── create/
│   └── create_test.go
├── list/
│   └── list_test.go
├── one/
│   └── one_test.go
├── update/
│   └── update_test.go
├── delete/
│   └── delete_test.go
├── get_providers/
│   └── get_providers_test.go
└── tier_prices/
    └── tier_prices_test.go
```

## 6. 测试数据约定

### 6.1 合法模型记录示例

```json
{
    "provider": "deepseek",
    "model": "deepseek-v3",
    "base_model": "deepseek-v3",
    "mode": "chat",
    "capabilities": ["chat", "reasoning", "tools"],
    "supported_parameters": ["temperature", "max_tokens"],
    "limits": {
        "context_window": 128000,
        "max_input_tokens": 128000,
        "max_output_tokens": 8192
    },
    "prices": {
        "input_cost_per_token": 0.000002,
        "output_cost_per_token": 0.000008
    },
    "tier_prices": {
        "peak": {
            "input_cost_per_token": 0.000004,
            "output_cost_per_token": 0.000016
        }
    },
    "metadata": {
        "source": "https://platform.deepseek.com/pricing",
        "notes": "DeepSeek V3"
    }
}
```

### 6.2 唯一性约束

`(provider, model, mode)` 三元组必须唯一。

### 6.3 分时段价格约束

- `tier_prices` 为可选字段；键名（tier name）初期仅支持 `peak`。
- 每个 tier 对应的价格对象键名须为 `prices` 的合法枚举键。
- 价格值必须为非负数。

### 6.4 测试用例编号规则

```
MP-{接口编号}-{场景编号}
```

例如：`MP-2-001` 表示 MP-2（新增单条记录）的第 1 个场景。

分时段定价专项测试使用独立编号：

```
MTP-1-{场景编号}
```

---

## 7. 整表导入（MP-1）

### 7.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 整表导入 |
| 方法 | POST |
| 路径 | `/open-api/v1/model-prices/import` |
| 说明 | 通过 `model-list.yaml` 批量导入，支持 replace / merge 模式 |

### 7.2 接口参数说明

#### 请求参数（Form-Data）

| 参数名 | 类型 | 必填 | 说明 | 合法性条件 |
|--------|------|------|------|------------|
| file | file | Y | YAML 文件 | 须为合法 YAML，版本 v1.0，default_currency=RMB |
| mode | string | N | 导入模式 | `replace`（默认）/ `merge` |

#### 返回数据字段

| 参数名 | 类型 | 说明 |
|--------|------|------|
| imported_count | int | 成功导入/更新的记录数 |
| skipped_count | int | 跳过的记录数 |
| errors | []string | 错误列表 |

### 7.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-1-001 | replace 模式全量替换 | 正常参数 | 导入后数据库中只有新数据 |
| MP-1-002 | merge 模式增量合并 | 正常参数 | 已存在记录更新，新记录插入 |
| MP-1-003 | 默认 replace 模式 | 正常参数 | 不传 mode 时按 replace 处理 |
| MP-1-004 | 非法 mode | 异常参数 | mode=invalid，返回 422 |
| MP-1-005 | 非法 YAML | 异常参数 | 文件内容非合法 YAML，返回 422 |
| MP-1-006 | 重复三元组 | 异常参数 | YAML 内 (provider,model,mode) 重复，返回 422 |
| MP-1-007 | limits 包含负数 | 合法性条件 | 返回 422 |
| MP-1-008 | 未知 provider 可导入 | 正常参数 | provider 不存在于 `/providers` 时仍可导入 |
| MP-1-009 | 科学计数法价格导入 | 正常参数 | 10~12 位小数价格以科学计数法书写，导入后按 float64 等价解析 |
| MP-1-010 | 超精度价格拒绝 | 合法性条件 | 价格 × 1e8 ≥ 2^53 时导入返回 422 |
| MP-1-011 | merge 模式整行覆盖：最小记录清空可选字段 | 契约钉死 | merge 命中已有记录时整行替换，未提供的可选字段被清空（model-prices.md §3.1 第 9 步） |

### 7.4 测试场景详细设计

#### MP-1-001：replace 模式全量替换（正常参数）

##### 设计思路

验证 replace 模式会清空已有数据并写入 YAML 中的全部记录。

##### 前提数据准备

已通过其他接口创建 1 条 model price 记录。

##### 执行步骤

1. 构造包含 2 条记录的 `model-list.yaml`。
2. 以 `mode=replace` 调用导入接口。
3. 验证返回 `imported_count=2`。
4. 调用列表接口，验证总条数为 2，且只有 YAML 中的 2 条记录。

##### 请求参数

```yaml
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4o
    base_model: gpt-4o
    mode: chat
    prices:
      input_cost_per_token: 0.0001
  - provider: deepseek
    model: deepseek-v3
    base_model: deepseek-v3
    mode: chat
    prices:
      input_cost_per_token: 0.000002
```

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| imported_count | 2 | Equals |
| skipped_count | 0 | Equals |
| errors | 空数组 | Len=0 |

---

#### MP-1-002：merge 模式增量合并（正常参数）

##### 设计思路

验证 merge 模式对已存在 `(provider, model, mode)` 记录进行更新，不存在的记录插入。

##### 前提数据准备

已存在 `(openai, gpt-4o, chat)` 记录。

##### 执行步骤

1. 构造 YAML：1 条与已存在记录同组合键但价格不同，1 条全新记录。
2. 以 `mode=merge` 调用导入接口。
3. 验证 `imported_count=2`。
4. 查询已存在记录，验证价格已更新；查询新记录，验证已插入。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| imported_count | 2 | Equals |

---

#### MP-1-004：非法 mode（异常参数）

##### 设计思路

验证 mode 仅支持 replace / merge。

##### 执行步骤

1. 上传任意合法 YAML，mode=invalid。
2. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 import mode 错误信息

---

#### MP-1-007：limits 包含负数（合法性条件）

##### 设计思路

验证 YAML 中 limits 字段值必须为非负整数。

##### 执行步骤

1. 构造 YAML，其中一条记录的 `limits.context_window=-1`。
2. 以 `mode=replace` 调用导入接口。
3. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 limit 错误信息

---

#### MP-1-008：未知 provider 可导入（正常参数）

##### 设计思路

验证 `/model-prices/import` 不再校验 provider 是否已存在于 `/providers`。

##### 执行步骤

1. 构造一个不存在于 `/providers` 的 provider 名称。
2. 构造包含该 provider 的 `model-list.yaml`。
3. 以 `mode=replace` 调用导入接口。
4. 验证返回 200，`imported_count=1`。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| imported_count | 1 | Equals |
| skipped_count | 0 | Equals |
| errors | 空数组 | Len=0 |

---

#### MP-1-009：科学计数法价格导入（正常参数）

##### 设计思路

验证 `model-list.yaml` 中的价格支持科学计数法表示（v0.6 放开，与十进制表示法等价合法）。目录数据中存在 10~12 位小数价格（如 `7.6234102728e-08`），十进制展开可读性差，科学计数法为权威书写方式。

##### 执行步骤

1. 构造 `model-list.yaml`，价格以科学计数法书写：`input_cost_per_token: 7.6234102728e-08`、`output_cost_per_token: 4.141631732e-06`。
2. 以 `mode=replace` 调用导入接口。
3. 验证返回 200，`imported_count=1`。
4. 按 `(provider, model, mode)` 查询记录，验证价格为等价 float64 值（`7.6234102728e-08` 与 `0.000000076234102728` 相等）。

##### 请求参数

```yaml
version: v1.0
default_currency: RMB
models:
  - provider: <unique-provider>
    model: sci-notation-model
    base_model: sci-notation-model
    mode: chat
    prices:
      input_cost_per_token: 7.6234102728e-08
      output_cost_per_token: 4.141631732e-06
```

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| imported_count | 1 | Equals |
| skipped_count | 0 | Equals |

记录查询校验：`prices.input_cost_per_token` = `7.6234102728e-08`（InDelta，1e-20）。

---

#### MP-1-010：超精度价格拒绝（合法性条件）

##### 设计思路

验证价格折算 `价格 × 1e8` 不得超过 2^53（约 9e15）。该上限保证下游 BFE 浮点成本计算 `用量 × (价格 × 1e8)` 在 float64 整数精确表示范围内；正常价格（< 1 元/token）不会触发，仅防御异常输入。

##### 执行步骤

1. 构造 `model-list.yaml`，其中 `input_cost_per_token: 9e7`（`9e7 × 1e8 = 9e15 ≥ 2^53`）。
2. 以 `mode=replace` 调用导入接口。
3. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 price exceeds the maximum representable precision 错误信息

---

#### MP-1-011：merge 模式整行覆盖：最小记录清空可选字段（契约钉死）

##### 设计思路

钉死 `mode=merge` 对已存在 `(provider, model, mode)` 记录的**整行覆盖**契约（api-define `model-prices.md` §3.1 第 9 步，issue #154 结论：保留整行覆盖、文档明示）：命中已有记录时以导入条目为权威定义，条目未提供的可选字段被清空，而非保留原值。该语义与 PUT 路径（`mergeModelPrice` 省略保留）刻意不同，防止未来重构无意中将契约改为字段合并。

##### 前提数据准备

已存在携带可选字段的记录：`capabilities`/`supported_parameters`/`limits`/`metadata` 均有值。

##### 执行步骤

1. POST 创建携带可选字段的记录。
2. 构造仅含必填五项（`provider`/`model`/`base_model`/`mode`/`prices`）的最小 YAML 记录，键与已有记录相同，价格不同。
3. 以 `mode=merge` 调用导入接口。
4. 验证 `imported_count=1`。
5. 按 id 查询记录，验证价格已更新为导入值，且 `capabilities`/`supported_parameters`/`limits`/`metadata`/`tier_prices` 均被清空（响应 `omitempty` → 字段缺失）。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| imported_count | 1 | Equals |
| prices.input_cost_per_token | 0.0002 | InDelta |
| capabilities / supported_parameters / limits / metadata / tier_prices | 不存在 | NotExists |

---

## 8. 新增单条记录（MP-2）

### 8.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 新增单条记录 |
| 方法 | POST |
| 路径 | `/open-api/v1/model-prices` |
| 说明 | 创建单条模型定价 |

### 8.2 接口参数说明

#### Body 参数

| 参数名 | 类型 | 必填 | 说明 | 合法性条件 |
|--------|------|------|------|------------|
| provider | string | Y | Provider / Cluster 标识 | 非空；不强制要求已存在于 `/providers` |
| model | string | Y | 模型名 | 非空 |
| base_model | string | Y | 归一化模型名 | 非空 |
| mode | string | Y | 请求模式 | 枚举值 |
| capabilities | []string | N | 能力列表 | 元素为枚举值 |
| supported_parameters | []string | N | 支持的请求参数 | 元素为枚举值 |
| limits | object | N | 限制对象 | 键名为枚举值；值为非负整数 |
| prices | object | Y | 价格对象 | 至少一个键；值为非负数；键名为枚举值 |
| tier_prices | object | N | 分时段价格 | 键名为 tier 名称（初期仅 `peak`）；值为价格对象，键名须在 `prices` 枚举内 |
| metadata | object | N | 元数据 | 键名为枚举值 |

#### 返回数据字段

返回完整记录，含系统生成的 `id`、`price_currency`（固定 RMB）、`created_at`、`updated_at`。

### 8.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-2-001 | 最小参数创建 | 正常参数 | 仅必填字段 |
| MP-2-002 | 完整参数创建 | 正常参数 | 含所有可选字段 |
| MP-2-003 | 缺少 provider | 必填校验 | 返回 422 |
| MP-2-004 | 非法 mode | 合法性条件 | mode=invalid，返回 422 |
| MP-2-005 | prices 包含负数 | 合法性条件 | 返回 422 |
| MP-2-006 | 重复三元组 | 异常参数 | 返回 422 |
| MP-2-007 | limits 包含负数 | 合法性条件 | 返回 422 |
| MP-2-008 | 未知 provider 可创建 | 正常参数 | provider 不存在于 `/providers` 时仍可创建 |

### 8.4 测试场景详细设计

#### MP-2-001：最小参数创建（正常参数）

##### 设计思路

验证仅传必填字段即可创建成功，且返回默认值正确。

##### 执行步骤

1. POST `/open-api/v1/model-prices`，Body 仅含必填字段。
2. 验证返回 200，返回记录中 `price_currency=RMB`。

##### 请求参数

```json
{
    "provider": "test-provider",
    "model": "test-model",
    "base_model": "test-model",
    "mode": "chat",
    "prices": {
        "input_cost_per_token": 0.001
    }
}
```

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| id | 大于 0 | Gt(0) |
| provider | "test-provider" | Equals |
| price_currency | "RMB" | Equals |
| created_at | 大于 0 | Gt(0) |

---

#### MP-2-005：prices 包含负数（合法性条件）

##### 设计思路

验证 prices 中所有价格必须为非负数。

##### 执行步骤

1. POST 请求，`prices.input_cost_per_token=-1`。
2. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 price 错误信息

---

#### MP-2-007：limits 包含负数（合法性条件）

##### 设计思路

验证创建时 limits 字段值必须为非负整数。

##### 执行步骤

1. POST 请求，`limits.context_window=-1`。
2. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 limit 错误信息

---

#### MP-2-008：未知 provider 可创建（正常参数）

##### 设计思路

验证 `/model-prices` 的 `provider` 字段不再强制引用 `/providers` 中已存在的 provider。

##### 执行步骤

1. 构造一个不存在于 `/providers` 的 provider 名称。
2. POST `/open-api/v1/model-prices`，Body 含必填字段。
3. 验证返回 200，记录创建成功。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| provider | 与请求一致 | Equals |
| price_currency | "RMB" | Equals |

---

## 9. 分页列表查询（MP-3）

### 9.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 分页列表查询 |
| 方法 | GET |
| 路径 | `/open-api/v1/model-prices` |
| 说明 | 支持 provider、mode 过滤 |

### 9.2 接口参数说明

#### Query 参数

| 参数名 | 类型 | 必填 | 说明 | 默认值/范围 |
|--------|------|------|------|------------|
| provider | string | N | 按 provider 过滤 | - |
| mode | string | N | 按 mode 过滤 | 枚举值 |
| page | int | N | 页码 | 1 |
| page_size | int | N | 每页条数 | 50，最大 1000 |

#### 返回数据字段

| 参数名 | 类型 | 说明 |
|--------|------|------|
| total | int64 | 总条数 |
| items | []ModelPrice | 模型定价记录列表 |

### 9.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-3-001 | 默认分页 | 正常参数 | page=1, page_size=50 |
| MP-3-002 | 自定义分页 | 边界值 | page=1, page_size=1 |
| MP-3-003 | 按 provider 过滤 | 正常参数 | 仅返回该 provider 记录 |
| MP-3-004 | 按 mode 过滤 | 正常参数 | 仅返回该 mode 记录 |

### 9.4 测试场景详细设计

#### MP-3-001：默认分页（正常参数）

##### 设计思路

验证无参数时返回默认分页数据。

##### 前提数据准备

已创建至少 2 条记录。

##### 执行步骤

1. GET `/open-api/v1/model-prices`。
2. 验证返回 200，`items` 非空，`total ≥ 2`。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| total | ≥ 2 | Gte(2) |
| items | 非空数组 | NotEmpty |

---

## 10. 按 ID 查询单条（MP-4）

### 10.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 按 ID 查询单条 |
| 方法 | GET |
| 路径 | `/open-api/v1/model-prices/{id}` |

### 10.2 接口参数说明

#### URI 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | Y | 记录 ID |

### 10.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-4-001 | 查询存在的记录 | 正常参数 | 返回完整记录 |
| MP-4-002 | 查询不存在的记录 | 异常参数 | 返回 404 |

### 10.4 测试场景详细设计

#### MP-4-001：查询存在的记录（正常参数）

##### 设计思路

验证按 id 可返回正确记录。

##### 前提数据准备

已创建 1 条记录，记录其 id。

##### 执行步骤

1. GET `/open-api/v1/model-prices/{id}`。
2. 验证返回 200，Data.id 与请求 id 一致。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| id | 与请求一致 | Equals |
| provider | 非空 | NotEmpty |

---

#### MP-4-002：查询不存在的记录（异常参数）

##### 设计思路

验证查询不存在的 id 返回 404。

##### 执行步骤

1. GET `/open-api/v1/model-prices/999999999`。
2. 验证返回 404。

##### 预期返回结果

**ErrNum**：404

---

## 11. 按组合键查询（MP-5）

### 11.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 按组合键查询 |
| 方法 | GET |
| 路径 | `/open-api/v1/model-prices` |
| 说明 | 通过 provider + model + mode 过滤查询；由于当前 GET `/model-prices` 为列表接口，返回符合过滤条件的列表（命中时 1 条，未命中时 0 条） |

### 11.2 接口参数说明

#### Query 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| provider | string | Y | Provider 标识 |
| model | string | Y | 模型名 |
| mode | string | Y | 请求模式 |

### 11.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-5-001 | 查询存在的组合键 | 正常参数 | 返回只包含 1 条记录的列表 |
| MP-5-002 | 缺少 query 参数 | 边界场景 | 按现有参数进行列表过滤 |
| MP-5-003 | 查询不存在的组合键 | 异常参数 | 返回空列表 |

### 11.4 测试场景详细设计

#### MP-5-001：查询存在的组合键（正常参数）

##### 设计思路

验证通过 provider/model/mode 可唯一定位记录；当前实现走列表接口，返回总条数为 1 的列表。

##### 前提数据准备

已存在 `(deepseek, deepseek-v3, chat)` 记录。

##### 执行步骤

1. GET `/open-api/v1/model-prices?provider=deepseek&model=deepseek-v3&mode=chat`。
2. 验证返回 200，`total=1`，`items[0]` 的字段匹配。

##### 预期返回结果

**ErrNum**：200

---

## 12. 按 ID 修改单条（MP-6）

### 12.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 按 ID 修改单条 |
| 方法 | PUT |
| 路径 | `/open-api/v1/model-prices/{id}` |
| 说明 | 支持部分字段更新 |

### 12.2 接口参数说明

#### URI 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| id | int64 | Y | 记录 ID |

#### Body 参数

同 MP-2，仅传需修改字段。

### 12.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-6-001 | 部分更新 prices | 正常参数 | 修改价格后查询一致 |
| MP-6-002 | 更新不存在的记录 | 异常参数 | 返回 404 |
| MP-6-003 | 更新为非法 mode | 合法性条件 | 返回 422 |
| MP-6-004 | 更新 limits 为负数 | 合法性条件 | 返回 422 |
| MP-6-005 | 只传单个价格键时其余键保留 | 正常参数 | PUT 仅传 `prices` 单键，未传入的价格键保留原值（issue #140） |

### 12.4 测试场景详细设计

#### MP-6-001：部分更新 prices（正常参数）

##### 设计思路

验证仅传 prices 可更新成功，未传字段保持原值。

##### 前提数据准备

已存在记录。

##### 执行步骤

1. PUT `/open-api/v1/model-prices/{id}`，Body 仅含新 prices。
2. 验证返回 200，Data.prices 为新值，provider/model 保持原值。
3. 再次 GET 该记录，验证一致。

##### 请求参数

```json
{
    "prices": {
        "input_cost_per_token": 0.00001,
        "output_cost_per_token": 0.00002
    }
}
```

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| prices.input_cost_per_token | 0.00001 | Equals |
| prices.output_cost_per_token | 0.00002 | Equals |

---

#### MP-6-004：按 id 更新 limits 为负数（合法性条件）

##### 设计思路

验证按 id 更新时 limits 字段值必须为非负整数。

##### 执行步骤

1. 先创建一条记录。
2. PUT `/open-api/v1/model-prices/{id}`，`limits.max_output_tokens=-1`。
3. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 limit 错误信息

---

## 13. 按组合键修改单条（MP-7）

### 13.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 按组合键修改单条 |
| 方法 | PUT |
| 路径 | `/open-api/v1/model-prices` |
| 说明 | 通过 provider + model + mode 定位并更新 |

### 13.2 接口参数说明

#### Query 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| provider | string | Y | Provider 标识 |
| model | string | Y | 模型名 |
| mode | string | Y | 请求模式 |

#### Body 参数

同 MP-6。

### 13.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-7-001 | 按组合键更新 prices | 正常参数 | 更新成功 |
| MP-7-002 | 缺少 query 参数 | 必填校验 | 返回 422 |
| MP-7-003 | 按组合键更新 limits 为负数 | 合法性条件 | 返回 422 |
| MP-7-004 | 按组合键只传单个价格键时其余键保留 | 正常参数 | PUT 仅传 `prices` 单键，未传入的价格键保留原值（issue #140） |

---

#### MP-7-003：按组合键更新 limits 为负数（合法性条件）

##### 设计思路

验证按组合键更新时 limits 字段值必须为非负整数。

##### 执行步骤

1. 先创建一条记录。
2. PUT `/open-api/v1/model-prices?provider=...&model=...&mode=chat`，`limits.context_window=-1`。
3. 验证返回 422。

##### 预期返回结果

**ErrNum**：422  
**ErrMsg**：包含 limit 错误信息

---

## 14. 按 ID 删除单条（MP-8）

### 14.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 按 ID 删除单条 |
| 方法 | DELETE |
| 路径 | `/open-api/v1/model-prices/{id}` |

### 14.2 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-8-001 | 删除存在的记录 | 正常参数 | 删除后再查询返回 404 |
| MP-8-002 | 删除不存在的记录 | 异常参数 | 返回 404 |

### 14.3 测试场景详细设计

#### MP-8-001：删除存在的记录（正常参数）

##### 设计思路

验证删除后记录不可再查询。

##### 前提数据准备

已创建 1 条记录。

##### 执行步骤

1. DELETE `/open-api/v1/model-prices/{id}`。
2. 验证返回 200，`Data.deleted=true`。
3. 再次 GET 该记录，验证返回 404。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| deleted | true | Equals |

---

## 15. 按组合键删除单条（MP-9）

### 15.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 按组合键删除单条 |
| 方法 | DELETE |
| 路径 | `/open-api/v1/model-prices` |
| 说明 | 通过 provider + model + mode 定位并删除 |

### 15.2 接口参数说明

#### Query 参数

| 参数名 | 类型 | 必填 | 说明 |
|--------|------|------|------|
| provider | string | Y | Provider 标识 |
| model | string | Y | 模型名 |
| mode | string | Y | 请求模式 |

### 15.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-9-001 | 删除存在的组合键记录 | 正常参数 | 删除成功 |
| MP-9-002 | 缺少 query 参数 | 必填校验 | 返回 422 |

---

## 16. 查询 Provider 名称列表（MP-10）

### 16.1 接口信息

| 项目 | 值 |
|------|-----|
| 模块 | Model Price |
| 接口名称 | 查询 Provider 名称列表 |
| 方法 | GET |
| 路径 | `/open-api/v1/model-prices/actions/get-providers` |
| 说明 | 返回 `model_prices` 表中所有 `provider` 名称的去重列表，按字典序排列 |

### 16.2 返回数据字段

| 参数名 | 类型 | 说明 |
|--------|------|------|
| providers | []string | provider 名称去重列表 |

### 16.3 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MP-10-001 | 空列表 | 正常参数 | 无 price 记录时返回空数组 |
| MP-10-002 | 返回去重 provider 列表 | 正常参数 | 创建多条记录后返回去重列表 |

### 16.4 测试场景详细设计

#### MP-10-001：空列表

##### 设计思路

验证无记录时返回空数组。

##### 执行步骤

1. 确保当前无 model price 记录（可先 replace 导入空列表）。
2. GET `/open-api/v1/model-prices/actions/get-providers`。
3. 验证返回 200，`providers` 为空数组。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| providers | [] | Len=0 |

---

#### MP-10-002：返回去重 provider 列表

##### 设计思路

验证返回的 provider 列表按字典序排列且已去重。

##### 前提数据准备

已创建 `(a-provider, m1, chat)`、`(b-provider, m2, chat)`、`(a-provider, m3, chat)` 三条记录。

##### 执行步骤

1. GET `/open-api/v1/model-prices/actions/get-providers`。
2. 验证返回 200，`providers` 为 `["a-provider", "b-provider"]`。

##### 预期返回结果

**ErrNum**：200  
**Data 字段校验**：

| 字段 | 预期值 | 校验方式 |
|------|--------|---------|
| providers | ["a-provider", "b-provider"] | Equals |
| providers 长度 | 2 | Len=2 |

---

## 17. 分时段价格字段测试（MTP-1）

### 17.1 测试目标

验证 `tier_prices` 在创建、查询、更新、`model-list.yaml` 导入等场景下能够正确写入、读取和校验。

### 17.2 测试场景总览

| 编号 | 场景 | 测试类型 | 简要说明 |
|------|------|---------|---------|
| MTP-1-001 | 创建含 tier_prices 的记录 | 正常参数 | 创建时携带 `tier_prices.peak`，查询返回一致 |
| MTP-1-002 | tier_prices 含非法 tier name | 异常参数 | `tier_prices` 中出现非 `peak` 键名，返回 422 |
| MTP-1-003 | tier_prices 含非法价格键 | 异常参数 | `tier_prices.peak` 中出现非 `prices` 枚举键，返回 422 |
| MTP-1-004 | tier_prices 含负数价格 | 合法性条件 | `tier_prices.peak.input_cost_per_token=-0.001`，返回 422 |
| MTP-1-005 | 更新 tier_prices | 正常参数 | PUT `/model-prices/{id}` 可单独更新 `tier_prices` |
| MTP-1-006 | model-list.yaml 导入含 tier_prices | 正常参数 | 导入的 YAML 携带 `tier_prices`，列表查询返回一致 |
| MTP-1-007 | 部分更新 tier_prices 同档未传键保留 | 正常参数 | PUT 只传 `tier_prices.peak` 单键时同档未传键保留原值（issue #140；未传 tier 整档保留、新 tier 加入由 `mergeTierPriceMap` 单测覆盖，当前校验仅允许 `peak` 档） |

### 17.3 请求参数示例

#### 创建含 tier_prices 的记录

```json
{
    "provider": "deepseek",
    "model": "deepseek-v3",
    "base_model": "deepseek-v3",
    "mode": "chat",
    "prices": {
        "input_cost_per_token": 0.0000045,
        "output_cost_per_token": 0.0000135
    },
    "tier_prices": {
        "peak": {
            "input_cost_per_token": 0.000009,
            "output_cost_per_token": 0.000027
        }
    }
}
```

#### model-list.yaml 导入示例

```yaml
version: v1.0
default_currency: RMB
models:
  - provider: deepseek-tier-test
    model: deepseek-v3
    base_model: deepseek-v3
    mode: chat
    prices:
      input_cost_per_token: 0.000002
    tier_prices:
      peak:
        input_cost_per_token: 0.000004
```

### 17.4 预期返回结果

- **MTP-1-001 / MTP-1-005 / MTP-1-006**：ErrNum=200，`tier_prices.peak` 与输入一致。
- **MTP-1-002 / MTP-1-003 / MTP-1-004**：ErrNum=422，ErrMsg 包含对应字段校验错误。

---

## 18. 附录

### 18.1 通用断言说明

- `Equals`：字段值与预期值相等
- `NotEmpty`：字段值非空
- `Gte(n)`：数值大于等于 n
- `Len=n`：数组/字符串长度等于 n
- `IsArray` / `IsObject`：类型校验

### 18.2 返回码约定

| ErrNum | 含义 |
|--------|------|
| 200 | 成功 |
| 404 | 记录不存在 |
| 422 | 参数校验失败 |
