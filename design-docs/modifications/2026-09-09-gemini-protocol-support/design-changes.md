# Gemini 协议支持：ai-gateway-api 控制面设计变更说明

> 本文档只描述 `ai-gateway-api` 控制面的修改。BFE 数据面修改（`bfe/bfe_model_protocol/gemini/` 适配器、`detect.go` 识别、流式终止/最终 usage 判定下沉适配器等）见总体设计方案 `document-ai-gateway/迭代系统设计/v0.6/gemini协议支持/gemini协议支持-设计方案.md` §5。

## 1. 概述

### 1.1 变更背景

BFE 数据面即将支持 Google Gemini API 的原样转发。该协议与现有协议的关键差异：

| 项目 | OpenAI | Anthropic | Gemini |
|------|--------|-----------|--------|
| 认证头 | `Authorization: Bearer <key>` | `x-api-key: <key>` + `anthropic-version` | `x-goog-api-key: <key>`（无版本头） |
| 对话端点 | `POST /v1/chat/completions` 等 | `POST /v1/messages` | `POST /v1beta/models/{model}:generateContent`、`:streamGenerateContent` |
| 模型列表端点 | `GET /v1/models` | `GET /v1/models` | `GET /v1beta/models` |
| 非流式 usage | `usage.prompt_tokens` / `completion_tokens` | `usage.input_tokens` / `output_tokens` | `usageMetadata.promptTokenCount` / `candidatesTokenCount` / `cachedContentTokenCount` |
| 流式终止 | SSE `data: [DONE]` | SSE `message_stop` 事件 | 无终止事件，HTTP 流结束（EOF）即终止 |
| 流式 usage 位置 | 最后一个 chunk 的 `usage` | `message_delta` 事件 | 每个 chunk 均带累积 `usageMetadata` |

控制面需要：放开 `model_protocols` 枚举、按协议选认证头、discover 按协议选默认 URI 与解析器。

**协议能力保持 provider 级单一定义**：cluster 不引入协议声明字段，其导出的 `AIConf.ModelProtocols` 恒为 provider `model_protocols` 透传值。provider 收缩协议时所有透传型引用 cluster 自动跟随，不存在悬空引用，无需移除保护或一致性校验。

### 1.2 变更范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 涉及模块 | `model/iprovider`、接口定义文档、集成/单元测试 |
| 变更类型 | 枚举放开 + 认证头/discover 解析器扩展 |
| 变更规模 | 3 处代码改动 + 文档 + 测试，无 DDL 迁移 |

### 1.3 无需改动部分

- **`model/icluster_conf`**：cluster 不新增协议声明字段，`LLMConfig`、`validateClusterLLMConfigAgainstProvider`、`newAIConf` 均不变；`AIConf.ModelProtocols` 仍为 provider 透传值，导出链路零改动，BFE 零改动；
- 存储层（JSON 透传）、路由鉴权、限流、配额、审计——均无协议分支，天然兼容；
- conf-agent 不感知协议语义，无改动。

---

## 2. `model/iprovider` 改动

### 2.1 `ValidModelProtocols` 加 `gemini`（对应 G1）

`model/iprovider/provider.go:40-43`：

```go
var ValidModelProtocols = map[string]bool{
    "openai":    true,
    "anthropic": true,
    "gemini":    true, // 新增
}
```

这是协议白名单唯一入口，provider 创建/更新的值合法性校验（`provider.go:445-460`）自动生效。

### 2.2 `BuildAuthHeader` 加 gemini case（对应 G3）

`model/iprovider/provider.go:784-794`：

```go
switch protocol {
case "anthropic":
    header = http.Header{"x-api-key": []string{key}}
    header.Set("anthropic-version", "...")
case "gemini": // 新增
    header = http.Header{"x-goog-api-key": []string{key}}
default: // openai
    header = http.Header{"Authorization": []string{"Bearer " + key}}
}
```

> **注意**：现状 fallthrough-default 是 Bearer，gemini 必须显式 case，否则会被默认分支吞掉。

### 2.3 discover 默认 URI 按协议映射 + gemini 解析器（对应 G5）

`model/iprovider/discover.go` 两处：

1. **默认 URI**（`:90-93`）：现状固定 `GET /v1/models`，改为按协议映射（或在 parser 中携带端点信息）：

   | 协议 | 默认 URI |
   |------|----------|
   | openai / anthropic | `GET /v1/models`（不变） |
   | gemini | `GET /v1beta/models` |

2. **`modelProtocolParsers`**（`:158-173`）新增 gemini 解析器：`ListPath: "models"`、`IDField: "name"`，并在提取后**剥离 `models/` 前缀**（`models/gemini-2.5-pro` → `gemini-2.5-pro`）。

---

## 3. 文档与测试

| # | 位置 | 动作 |
|---|------|------|
| 1 | `design-docs/api-define/OpenAPI接口定义/providers.md` | 更新 `model_protocols` 枚举表、认证头说明、discover-models 协议说明 |
| 2 | `test/integration/tests/schema/openapi/provider.go:81` | schema 枚举加 `gemini` |
| 3 | 集成测试 | `provider/create`、`discover`、`list` 补 gemini 用例 |
| 4 | `model/iprovider/provider_test.go` | 白名单、BuildAuthHeader（gemini case）、discover 解析器（含 `models/` 前缀剥离）单测 |

---

## 4. 发布顺序与回滚

- **发布顺序**：bfe 先发布（识别 + 注册 + `ValidateProtocols` 校验通过），api 后放开枚举，web 最后加下拉选项；期间含 gemini 的 provider 配置不会产生，无中间态风险。上线 checklist 固化该顺序（本期不改代码机制化版本协商）。
- **回滚**：api 侧回滚枚举即恢复；bfe 适配器为纯新增代码，回滚无数据影响（但已下发的含 gemini 配置在回滚后的 bfe 上会加载失败，需先回滚配置）。

---

## 5. 风险与注意事项

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| bfe 未先发布 | api 先放开枚举时，导出含 gemini 的配置会被 bfe 配置加载拒绝（启动失败/热加载拒绝） | 上线 checklist 固化 bfe 先行；灰度期不产生含 gemini 的配置 |
| discover 默认 URI 映射影响现状 | 按协议映射默认 URI 可能误改 openai/anthropic 行为 | openai/anthropic 保持 `GET /v1/models` 不变，单测锁定 |
| 协议错误延迟到数据面 | provider 未声明 gemini 时，引用 cluster 创建不报错，gemini 请求在 BFE 转发阶段被 `PROVIDER_PROTOCOL_MISMATCH` 拒绝 | 与 anthropic 接入行为一致，本期接受；如后续需要控制面 fail-fast，再评审一致性校验 |
| 流式计费语义 | gemini 每 chunk 带累积 `usageMetadata`，取中间 chunk 会少计 | BFE 侧必须确保取最后一个含 usage 的 chunk（详见总体方案 §9） |
| 已知 gap | 网关自身错误恒为 OpenAI 风格 `AiErrorBody`，gemini 客户端收到的错误体非 gemini 原生格式 | 与 anthropic 客户端现状一致，文档标注；如需启用 `ErrorNormalizer` 另起小改动 |
