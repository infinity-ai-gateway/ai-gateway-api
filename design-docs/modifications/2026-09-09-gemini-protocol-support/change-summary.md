# Gemini 协议支持（ai-gateway-api 控制面）变更摘要

## 1. 背景

AI 网关目前已支持 `openai` 与 `anthropic` 两种 model-protocol。2026-08-23 的 Claude 协议支持（`design-docs/modifications/2026-08-23-claude-protocol-support/`）确立了"协议适配层 + 控制面枚举放开"的新增协议标准路径；BFE 协议适配层设计文档（`bfe/docs/zh_cn/sys_design/model_protocol_adapter.md`）第 6 节"新增协议接入指南"已**以 gemini 为例**预留了扩展路径。本期按该路径落地 Gemini 协议支持。

Gemini 协议与现有协议的差异点：

- 认证头为 `x-goog-api-key: {key}`（无版本头）；
- 请求路径形态不同：`/v1beta/models/{model}:generateContent`、`:streamGenerateContent`；
- 模型列表端点为 `GET /v1beta/models`，响应 `models[].name`（形如 `models/gemini-2.5-pro`）；
- usage 字段为 camelCase 的 `usageMetadata.promptTokenCount` / `candidatesTokenCount` / `cachedContentTokenCount`；
- 流式无 SSE 终止事件（HTTP 流结束即终止），且每个 chunk 均带累积 `usageMetadata`。

本期延续透传原则：不做 gemini ↔ openai/anthropic 的协议翻译。

## 2. 目标

| # | 目标 | 验证标准 |
|---|------|----------|
| G1 | provider 的 `model_protocols` 支持声明 `gemini` | 创建/更新 provider 传 `["gemini"]` 通过校验；schema 测试更新 |
| G2 | gemini 客户端请求被 BFE 正确识别并按 gemini 协议转发 | 命中 `ModelProtocols: ["gemini"]` 的 cluster；协议不匹配时 BFE 返回 `PROVIDER_PROTOCOL_MISMATCH`（BFE 侧目标，见关联文档） |
| G3 | 认证头正确注入 | 转发上游时注入 `x-goog-api-key: {key}` |
| G4 | usage（token 计费）正确提取 | 非流式与流式 `usageMetadata` 均提取到 `UsageFields`，计费金额正确（BFE 侧目标） |
| G5 | 模型发现（discover-models）支持 gemini provider | 对 gemini provider 执行 discover 返回模型列表（默认 URI `GET /v1beta/models`） |

## 3. 范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api`（本说明）；BFE 数据面改动见 `bfe/` 侧设计文档 |
| 主要文件 | `model/iprovider/provider.go`、`model/iprovider/discover.go`、接口定义文档、集成/单元测试 |
| 接口契约 | `/providers` 的 `model_protocols` 枚举新增 `gemini`；`/clusters` 接口契约不变 |
| 数据迁移 | 无 DDL 变更；`providers.model_protocols` 与 cluster `llm_config` 均为 JSON 列，无枚举约束 |
| BFE 影响 | BFE 需先于（或同步于）本改动发布（BFE 配置加载期 `modelprotocol.ValidateProtocols` 会拒绝未知协议名） |

## 4. 关键决策

| 决策 | 说明 |
|------|------|
| 不做协议翻译 | 透传原则，与 Claude 支持一致；gemini 客户端只能路由到 gemini 协议上游 |
| `BuildAuthHeader` 必须显式加 gemini case | 现状 fallthrough-default 是 `Bearer`，不显式 case 会被默认分支吞掉 |
| discover 默认 URI 按协议映射 | gemini 默认 `GET /v1beta/models`（非 `/v1/models`），并保持 openai/anthropic 默认不变 |
| 协议能力仅在 provider 级声明 | cluster 不新增 `model_protocols` 字段：cluster 协议能力恒透传 provider，单一事实来源；provider 收缩协议时透传 cluster 自动跟随，不存在悬空引用，无需移除保护与一致性校验。per-cluster 协议限制如需支持，通过拆分为独立 provider 实现 |
| 网关自身错误保持 OpenAI 风格 | gemini 客户端收到的网关错误体非 gemini 原生格式，与 anthropic 客户端现状一致，标注为已知 gap |

## 5. 关联文档

- 详细设计：`design-changes.md`
- 接口变更：`api-changes.md`
- 总体设计方案：`document-ai-gateway/迭代系统设计/v0.6/gemini协议支持/gemini协议支持-设计方案.md`
- 先例记录：`design-docs/modifications/2026-08-23-claude-protocol-support/`（Claude 协议支持）
- 相关接口定义：`design-docs/api-define/OpenAPI接口定义/providers.md`、`design-docs/api-define/OpenAPI接口定义/clusters.md`
