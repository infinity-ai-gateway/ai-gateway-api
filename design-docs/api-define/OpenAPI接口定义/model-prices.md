# /model-prices

## 1. 数据模型

```json
{
  "id": 1,
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
    "output_cost_per_token": 0.000008,
    "cache_read_input_token_cost": 0.0000005
  },
  "tier_prices": {
    "peak": {
      "input_cost_per_token": 0.000004,
      "output_cost_per_token": 0.000016,
      "cache_read_input_token_cost": 0.000001
    }
  },
  "price_currency": "RMB",
  "metadata": {
    "source": "https://platform.deepseek.com/pricing",
    "notes": "DeepSeek V3"
  },
  "create_time": 1716883200,
  "update_time": 1716883200
}
```

**字段说明**

| 字段 | 类型 | 说明 | 合法性条件 |
|------|------|------|------------|
| `id` | int64 | 模型定价记录唯一标识 | 系统生成 |
| `provider` | string | Provider / Cluster 标识 | 必填；非空；长度 1-255；仅作为价格归集标识，不强制校验在 `/providers` 中存在 |
| `model` | string | 模型名 | 必填；非空；长度 1-255 |
| `base_model` | string | 归一化模型名 | 必填；非空；长度 1-255 |
| `mode` | string | 请求模式 | 必填；枚举值见下表 |
| `capabilities` | []string | 模型支持的能力列表 | 默认空数组；元素应为枚举值 |
| `supported_parameters` | []string | 支持的请求参数列表 | 默认空数组；元素应为枚举值 |
| `limits` | object | 限制对象 | 默认空对象；键名应为枚举值；所有限制字段必须为非负整数 |
| `prices` | object | 价格对象 | 必填；至少包含一个价格字段；所有价格字段必须为非负数；键名应为枚举值；未命中 tier 时作为 fallback 价格；支持科学计数法与十进制表示法（如 `1.5e-6` 与 `0.0000015` 等价），按 float64 解析 |
| `tier_prices` | object | 分时段价格对象 | 非必填；键为 tier name（**初期只支持 `peak`**），值为价格对象；内部键名应为 `prices` 枚举；与 provider 的 `tiers` 不做强制引用校验；同样支持科学计数法与十进制表示法 |
| `price_currency` | string | 价格货币 | 固定为 `RMB`，请求体中无需传入 |
| `metadata` | object | 元数据 | 默认空对象；键名应为枚举值 |
| `create_time` | int64 | 创建时间 | Unix 时间戳（秒） |
| `update_time` | int64 | 更新时间 | Unix 时间戳（秒） |

**枚举值定义**

`mode` 枚举值：

| 枚举值 | 说明 |
|--------|------|
| `chat` | 聊天对话 |
| `completion` | 文本补全 |
| `responses` | Responses API |
| `image_generation` | 图像生成 |
| `image_edit` | 图像编辑 |
| `embedding` | 文本嵌入 |
| `rerank` | 重排序 |
| `audio_speech` | 语音合成 |
| `audio_transcription` | 语音转录 |
| `video_generation` | 视频生成 |
| `ocr` | OCR |
| `search` | 搜索 |
| `realtime` | 实时交互 |

`capabilities` 枚举值：

| 枚举值 | 说明 |
|--------|------|
| `chat` | 聊天对话 |
| `vision` | 视觉理解 |
| `audio_input` | 音频输入 |
| `video_input` | 视频输入 |
| `reasoning` | 推理能力 |
| `tools` | 工具调用 |
| `structured_outputs` | 结构化输出 |
| `function_calling` | 函数调用 |
| `prompt_caching` | 提示缓存 |
| `computer_use` | 计算机使用 |
| `web_search` | 网页搜索 |
| `serverless` | Serverless |
| `image_generation` | 图像生成 |
| `embedding` | 文本嵌入 |
| `rerank` | 重排序 |
| `audio_speech` | 语音合成 |
| `audio_transcription` | 语音转录 |
| `video_generation` | 视频生成 |
| `ocr` | OCR |
| `search` | 搜索 |
| `realtime` | 实时交互 |

`supported_parameters` 枚举值：

| 枚举值 | 说明 |
|--------|------|
| `temperature` | 采样温度 |
| `top_p` | 核采样 |
| `max_tokens` | 最大生成 Token 数 |
| `tools` | 工具列表 |
| `tool_choice` | 工具选择 |
| `response_format` | 响应格式 |
| `reasoning` | 推理参数 |
| `image_input` | 图像输入 |
| `video_input` | 视频输入 |
| `audio_input` | 音频输入 |
| `voice` | 语音 |
| `speed` | 语速 |
| `size` | 尺寸 |
| `quality` | 质量 |
| `style` | 风格 |

`limits` 键名枚举：

| 键名 | 说明 |
|------|------|
| `context_window` | 上下文窗口大小 |
| `max_input_tokens` | 最大输入 Token 数 |
| `max_output_tokens` | 最大输出 Token 数 |
| `max_tokens` | 最大 Token 数 |

`prices` 键名枚举：

| 键名 | 说明 |
|------|------|
| `input_cost_per_token` | 每 Token 输入成本 |
| `output_cost_per_token` | 每 Token 输出成本 |
| `cache_read_input_token_cost` | 缓存读取输入 Token 成本 |
| `cache_creation_input_token_cost` | 缓存创建输入 Token 成本（5m TTL） |
| `cache_creation_input_token_cost_1h` | 缓存创建输入 Token 成本（1h TTL） |
| `input_cost_per_token_above_200k_tokens` | 超过 200k Token 的输入成本 |
| `output_cost_per_token_above_200k_tokens` | 超过 200k Token 的输出成本 |
| `input_cost_per_token_above_256k_tokens` | 超过 256k Token 的输入成本 |
| `output_cost_per_token_above_256k_tokens` | 超过 256k Token 的输出成本 |
| `input_cost_per_token_above_272k_tokens` | 超过 272k Token 的输入成本 |
| `output_cost_per_token_above_272k_tokens` | 超过 272k Token 的输出成本 |
| `input_cost_per_token_above_512k_tokens` | 超过 512k Token 的输入成本 |
| `output_cost_per_token_above_512k_tokens` | 超过 512k Token 的输出成本 |
| `output_cost_per_image` | 每张输出图像成本 |
| `input_cost_per_image_token` | 每图像输入 Token 成本 |
| `output_cost_per_pixel` | 每像素输出成本 |
| `output_cost_per_image_low_quality` | 低质量输出图像成本 |
| `output_cost_per_image_high_quality` | 高质量输出图像成本 |
| `input_cost_per_audio_per_second` | 每秒音频输入成本 |
| `input_cost_per_audio_token` | 每音频输入 Token 成本 |
| `output_cost_per_audio_token` | 每音频输出 Token 成本 |
| `input_cost_per_video_per_second` | 每秒视频输入成本 |
| `output_cost_per_second` | 每秒输出成本 |
| `input_cost_per_query` | 每次查询输入成本 |
| `search_context_cost_per_query` | 每次查询搜索上下文成本 |
| `ocr_cost_per_page` | 每页 OCR 成本 |
| `output_cost_per_character` | 每字符输出成本 |
| `output_cost_per_image_hd` | 高清输出图像成本 |
| `output_cost_per_video` | 每视频输出成本 |
| `output_cost_per_video_per_second` | 每秒视频输出成本 |

`metadata` 键名枚举：

| 键名 | 说明 |
|------|------|
| `source` | 价格来源 |
| `notes` | 备注 |

> **说明**：`capabilities`、`supported_parameters`、`limits`、`prices`、`metadata` 若传入非枚举值，系统可接收但建议告警或记录，便于后续收敛。

### 1.2 价格精度与 JSON 序列化

`prices` 与 `tier_prices.<tier>` 中的价格字段为浮点数（元），支持 8 位及以上小数精度（例如 `0.0000015`、`0.00000075`、`7.6234102728e-08`）。**科学计数法与十进制表示法等价合法**，输入（OpenAPI 请求体、`model-list.yaml`）按 `float64` 解析，两者解析结果完全一致。

序列化使用标准 JSON encoder：

- 请求体、响应体、InnerAPI 导出的 `cluster_conf.data` 中，价格可能以十进制（`0.0000015`）或科学计数法（`1.5e-6`）出现，均为同一数值的合法表示；
- 精度上限来自 `float64`（有效数字约 15 位）；单价格折算 `价格 × 1e8` 不得超过 2^53（约 9e15），超出将被校验拒绝；
- 该表示方式仅影响 JSON 文本，不改变 `float64` 数值语义与 BFE 定点整数扣减逻辑；
- YAML 导入（`model-list.yaml`）与 OpenAPI CRUD 均按原浮点数值解析，无需额外处理。

示例（两种表示法等价）：

```json
{
  "prices": {
    "input_cost_per_token": 1.5e-6,
    "output_cost_per_token": 0.0000045,
    "cache_read_input_token_cost": 5e-7
  },
  "tier_prices": {
    "peak": {
      "input_cost_per_token": 0.000003,
      "output_cost_per_token": 9e-6,
      "cache_read_input_token_cost": 0.000001
    }
  }
}
```

> 说明：早期版本（issue-102）曾强制价格以十进制表示法序列化；v0.6 起放开学
> 计数法，标准 encoder 对极小值（如 `1.5e-6`）可能输出科学计数法，属正常行为。

---

## 2. `model-list.yaml` 源格式说明

`/v1/model-prices/import` 接口通过 `model-list.yaml` 文件批量维护模型定价数据。该文件为 `model_prices` 表的权威数据源。

### 2.1 顶层结构

```yaml
version: v1.0                    # 格式版本，必填
default_currency: "RMB"          # 全局默认币种，当前仅支持 RMB
models:                          # 模型列表，必填
  - ...
```

### 2.2 模型记录结构

```yaml
models:
  - provider: "deepseek"
    model: "deepseek-v3"
    base_model: "deepseek-v3"
    mode: "chat"
    capabilities: ["chat", "reasoning", "tools"]
    supported_parameters: ["temperature", "max_tokens"]
    limits:
      context_window: 128000
      max_input_tokens: 128000
      max_output_tokens: 8192
    prices:
      input_cost_per_token: 0.000002
      output_cost_per_token: 0.000008
      cache_read_input_token_cost: 0.0000005
    tier_prices:
      peak:
        input_cost_per_token: 0.000004
        output_cost_per_token: 0.000016
        cache_read_input_token_cost: 0.000001
    metadata:
      source: "https://platform.deepseek.com/pricing"
      notes: "DeepSeek V3"
```

**字段说明**

| 字段 | 必填 | 说明 |
|------|------|------|
| `provider` | Y | Provider / Cluster 标识，对应 `model_prices.provider`；仅作为价格归集标识，不强制校验在 `/providers` 中存在 |
| `model` | Y | 模型名，对应 `model_prices.model` |
| `base_model` | Y | 归一化模型名，对应 `model_prices.base_model` |
| `mode` | Y | 模型模式，枚举值同第 1 节 `mode` 枚举 |
| `capabilities` | N | 能力列表，枚举值同第 1 节 `capabilities` 枚举 |
| `supported_parameters` | N | 支持的请求参数列表，枚举值同第 1 节 `supported_parameters` 枚举 |
| `limits` | N | 限制对象，键名枚举值同第 1 节 `limits` 枚举；所有限制字段必须为非负整数 |
| `prices` | Y | 价格对象，键名枚举值同第 1 节 `prices` 枚举；至少包含一个价格字段；未命中 tier 时作为 fallback 价格；支持科学计数法与十进制表示法 |
| `tier_prices` | N | 分时段价格对象，键为 tier name（**初期只支持 `peak`**），值为价格对象；内部键名枚举值同第 1 节 `prices` 枚举；支持科学计数法与十进制表示法 |
| `metadata` | N | 元数据，键名枚举值同第 1 节 `metadata` 枚举 |

> **唯一性约束**：`(provider, model, mode)` 三元组必须唯一。
> 
> **币种说明**：v0.4 仅支持 `RMB`，`default_currency` 与单条 `price_currency`（若填写）均须为 `RMB`。

### 2.3 完整示例

```yaml
version: v1.0
default_currency: "RMB"

models:
  - provider: "deepseek"
    model: "deepseek-v3"
    base_model: "deepseek-v3"
    mode: "chat"
    capabilities: ["chat", "reasoning", "tools", "structured_outputs", "prompt_caching"]
    supported_parameters: ["temperature", "top_p", "max_tokens", "tools", "tool_choice", "response_format", "reasoning"]
    limits:
      context_window: 128000
      max_input_tokens: 128000
      max_output_tokens: 8192
      max_tokens: 8192
    prices:
      input_cost_per_token: 0.000002
      output_cost_per_token: 0.000008
      cache_read_input_token_cost: 0.0000005
      cache_creation_input_token_cost: 0.000001
    tier_prices:
      peak:
        input_cost_per_token: 0.000004
        output_cost_per_token: 0.000016
        cache_read_input_token_cost: 0.000001
    metadata:
      source: "https://platform.deepseek.com/pricing"
      notes: "DeepSeek V3 官方 API"

  - provider: "openai"
    model: "gpt-4o"
    base_model: "gpt-4o"
    mode: "chat"
    capabilities: ["chat", "vision", "tools", "structured_outputs"]
    supported_parameters: ["temperature", "top_p", "max_tokens", "tools", "tool_choice", "response_format", "image_input"]
    limits:
      context_window: 128000
      max_input_tokens: 128000
      max_output_tokens: 4096
    prices:
      input_cost_per_token: 0.0000216
      output_cost_per_token: 0.000108
    metadata:
      source: "https://openai.com/pricing"
      notes: "OpenAI GPT-4o（美元报价已按汇率换算为 RMB）"
```

---

## 3. 接口清单

### 3.1 整表导入

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 整表导入 `model-list.yaml` | 支持 `replace` / `merge` 两种模式 |
| 端点 | /model-prices/import | - |
| 版本 | v1 | - |
| method | POST | - |

**输入参数（Form-Data）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| file | file | `model-list.yaml` 文件 | Y | - | 仅接受 YAML 文件 |
| mode | string | 导入模式 | N | `replace`（默认，全量替换）/ `merge`（增量合并） | 仅允许 `replace`、`merge` |

**处理逻辑**

1. 解析 YAML，校验 `version`、`default_currency`（必须为 `RMB`）；
2. 校验每条记录的 `(provider, model, mode)` 唯一性；
3. 校验必填字段：`provider`、`model`、`base_model`、`mode`、`prices`；
4. 校验 `prices` 中至少包含一个价格字段，且所有价格字段为非负数；
5. 若记录包含 `tier_prices`：
   - **初期 tier name 只支持 `peak`**；
   - 每个 tier 对应的价格对象中，键名须为 `prices` 枚举；
   - 所有 tier 价格字段必须为非负数。
6. 校验 `limits` 中所有限制字段值为非负整数；
7. `replace` 模式：先清空 `model_prices` 表，再写入新数据；
8. `merge` 模式：对已有 `(provider, model, mode)` 记录更新，新增记录插入。

**权限**

仅允许管理员调用。

**返回数据（Data内容）**

| 参数名 | 类型 | 参数含义 | 补充描述 |
| - | - | - | - |
| imported_count | int | 成功导入/更新的记录数 | - |
| skipped_count | int | 跳过的记录数 | - |
| errors | array | 错误列表 | 每条记录包含错误信息 |

**成功返回示例**

```json
{
    "ErrNum": 200,
    "ErrMsg": "success",
    "Data": {
        "imported_count": 7,
        "skipped_count": 0,
        "errors": []
    }
}
```

---

### 3.2 查询所有 Provider 名称列表

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 查询 `/model-prices` 数据中包含的所有 `provider` 名称的去重列表 | - |
| 端点 | /model-prices/actions/get-providers | - |
| 版本 | v1 | - |
| method | GET | - |

**处理逻辑**

1. 从 `model_prices` 表中按 `provider` 字段聚合去重；
2. 返回按字典序排列的 `provider` 名称列表。

**返回数据（Data内容）**

| 参数名 | 类型 | 参数含义 | 补充描述 |
| - | - | - | - |
| providers | []string | provider 名称列表 | 去重、按字典序排列 |

**成功返回示例**

```json
{
    "ErrNum": 200,
    "ErrMsg": "success",
    "Data": {
        "providers": [
            "deepseek",
            "openai",
            "qwen"
        ]
    }
}
```

---

### 3.3 新增单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 新增单条模型定价记录 | - |
| 端点 | /model-prices | - |
| 版本 | v1 | - |
| method | POST | - |

**输入参数（Body）**

字段同 [1. 数据模型](#1-数据模型)，但请求体中无需传入 `id`、`price_currency`、`create_time`、`update_time`。

**HTTP BODY参数示例**

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
        "output_cost_per_token": 0.000008,
        "cache_read_input_token_cost": 0.0000005
    },
    "tier_prices": {
        "peak": {
            "input_cost_per_token": 0.000004,
            "output_cost_per_token": 0.000016,
            "cache_read_input_token_cost": 0.000001
        }
    },
    "metadata": {
        "source": "https://platform.deepseek.com/pricing",
        "notes": "DeepSeek V3"
    }
}
```

**返回数据（Data内容）**

返回完整记录，含生成的 `id`、`price_currency`（固定 `RMB`）、`create_time`、`update_time`。

---

### 3.4 分页列表查询

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 分页查询模型定价记录 | 支持按 `provider`、`mode` 过滤 |
| 端点 | /model-prices | - |
| 版本 | v1 | - |
| method | GET | - |

**输入参数（Query）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| provider | string | 按 Provider 过滤 | N | - | - |
| mode | string | 按 Mode 过滤 | N | - | 须为 `mode` 枚举值 |
| page | int | 页码 | N | 默认1 | 必须 >0 |
| page_size | int | 每页条数 | N | 默认20，最大100 | 取值范围 1-100 |

**返回数据（Data内容）**

| 参数名 | 类型 | 参数含义 | 补充描述 |
| - | - | - | - |
| list | []ModelPrice | 模型定价记录列表 | 元素字段同 [1. 数据模型](#1-数据模型) |
| pagination | object | 分页信息 | 包含 `page`、`page_size`、`total` |

---

### 3.5 按 ID 查询单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 按 `id` 查询单条模型定价记录 | - |
| 端点 | /model-prices/{id} | - |
| 版本 | v1 | - |
| method | GET | - |

**输入参数（URI）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| id | int64 | 记录 ID | Y | - | 必填；须为存在的记录 ID |

**返回数据（Data内容）**

字段同 [1. 数据模型](#1-数据模型)。

---

### 3.6 按组合键查询单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 按 `(provider, model, mode)` 组合查询单条记录 | - |
| 端点 | /model-prices | - |
| 版本 | v1 | - |
| method | GET | - |

**输入参数（Query）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| provider | string | Provider 标识 | Y | - | 必填；非空 |
| model | string | 模型名 | Y | - | 必填；非空 |
| mode | string | 请求模式 | Y | - | 必填；须为 `mode` 枚举值 |

**返回数据（Data内容）**

字段同 [1. 数据模型](#1-数据模型)。

---

### 3.7 按 ID 修改单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 按 `id` 修改单条模型定价记录 | 支持部分字段更新 |
| 端点 | /model-prices/{id} | - |
| 版本 | v1 | - |
| method | PUT | - |

**输入参数（URI）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| id | int64 | 记录 ID | Y | - | 必填；须为存在的记录 ID |

**输入参数（Body）**

字段同 [1. 数据模型](#1-数据模型)，仅传需修改字段，未传入字段保持原值。请求体中无需传入 `id`、`price_currency`、`create_time`、`update_time`。

> map/切片字段更新粒度：
> - `prices`、`tier_prices`：**键级合并**。请求中传入的键覆盖对应键，未传入的键保留原值；`tier_prices` 未传入的 tier 整档保留（见 issue #140 修复）。
> - `limits`、`metadata`（map）、`capabilities`、`supported_parameters`（切片）：**整块替换**。传入即整体覆盖，未传入则保持原值不变。

**返回数据（Data内容）**

返回更新后的完整记录，字段同 [1. 数据模型](#1-数据模型)。

---

### 3.8 按组合键修改单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 按 `(provider, model, mode)` 组合修改单条记录 | 支持部分字段更新 |
| 端点 | /model-prices | - |
| 版本 | v1 | - |
| method | PUT | - |

**输入参数（Query）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| provider | string | Provider 标识 | Y | - | 必填；非空 |
| model | string | 模型名 | Y | - | 必填；非空 |
| mode | string | 请求模式 | Y | - | 必填；须为 `mode` 枚举值 |

**输入参数（Body）**

字段同 [1. 数据模型](#1-数据模型)，仅传需修改字段，未传入字段保持原值。请求体中无需传入 `price_currency`、`create_time`、`update_time`。

> map/切片字段更新粒度：
> - `prices`、`tier_prices`：**键级合并**。请求中传入的键覆盖对应键，未传入的键保留原值；`tier_prices` 未传入的 tier 整档保留（见 issue #140 修复）。
> - `limits`、`metadata`（map）、`capabilities`、`supported_parameters`（切片）：**整块替换**。传入即整体覆盖，未传入则保持原值不变。

**返回数据（Data内容）**

返回更新后的完整记录，字段同 [1. 数据模型](#1-数据模型)。

---

### 3.9 按 ID 删除单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 按 `id` 删除单条模型定价记录 | - |
| 端点 | /model-prices/{id} | - |
| 版本 | v1 | - |
| method | DELETE | - |

**输入参数（URI）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| id | int64 | 记录 ID | Y | - | 必填；须为存在的记录 ID |

**返回数据（Data内容）**

Data 为 null。

---

### 3.10 按组合键删除单条记录

**基本信息**

| 项目 | 值 | 说明 |
| - | - | - |
| 含义 | 按 `(provider, model, mode)` 组合删除单条记录 | - |
| 端点 | /model-prices | - |
| 版本 | v1 | - |
| method | DELETE | - |

**输入参数（Query）**

| 参数名 | 类型 | 参数含义 | 必填 | 补充描述 | 合法性条件 |
| - | - | - | - | - | - |
| provider | string | Provider 标识 | Y | - | 必填；非空 |
| model | string | 模型名 | Y | - | 必填；非空 |
| mode | string | 请求模式 | Y | - | 必填；须为 `mode` 枚举值 |

**返回数据（Data内容）**

Data 为 null。

---

## 4. 校验规则

1. `provider`、`model`、`base_model`、`mode` 必填；
2. `provider` 仅作为价格归集标识，不强制引用 `/providers` 中已存在的 provider；
3. `(provider, model, mode)` 组合不能重复；
4. `prices` 必填，至少包含一个价格字段；
5. 所有价格字段必须为非负数；支持科学计数法与十进制表示法（等价合法）；单价格折算 `价格 × 1e8` 不得超过 2^53（约 9e15），超出拒绝写入；
6. `tier_prices` 非必填；若传入：
   - **初期 tier name 只支持 `peak`**；
   - 每个 tier 对应的价格对象中，键名须为 `prices` 枚举；
   - 所有 tier 价格字段必须为非负数；
   - 与 provider 的 `tiers` 不做强制引用校验；若引用了 provider 未定义的 tier name，可记录告警，但不阻塞写入。
7. `price_currency` 当前只支持 `RMB`；
8. `mode` 必须是预定义枚举值；
9. `capabilities`、`supported_parameters` 若传入，其元素应取自对应枚举值（非枚举值可接收但建议告警或记录，便于后续收敛）；
10. `limits`、`prices`、`tier_prices.<tier>`、`metadata` 的键名应取自对应枚举值（非枚举键可接收但建议告警或记录，便于后续收敛）；
11. `limits` 中所有限制字段值必须为非负整数；
12. `/v1/model-prices/import` 仅接受 YAML 文件，且 `default_currency` 必须为 `RMB`；导入时不再校验 `provider` 是否已存在于 `/providers`。

---
