# Gemini 协议支持：API 接口变更说明

## 1. 变更范围

| 接口类型 | 变更内容 |
|----------|----------|
| OpenAPI `/providers` | `model_protocols` 枚举新增 `gemini`；discover-models 对 gemini provider 默认调用 `GET /v1beta/models` |
| OpenAPI `/clusters` | 无变更。`llm_config` 契约不变，不新增 cluster 级协议声明字段（协议能力保持 provider 级单一定义） |
| InnerAPI `/configs/tls_conf/server_data_conf` | 无变更。`AIConf.ModelProtocols` 仍为 provider `model_protocols` 的透传值 |

---

## 2. OpenAPI 变更：`/providers`

### 2.1 `model_protocols` 枚举新增 `gemini`

**变更原因**：支持创建/更新声明 gemini 协议的 provider，使 gemini 客户端请求可路由到 gemini 协议上游。

**变更后**：枚举取值扩展为 `openai` / `anthropic` / `gemini`，可任意组合，例如：

```json
{
  "model_protocols": ["gemini"]
}
```

**校验规则**（现状已有，行为不变）：

- 每个值必须 ∈ `ValidModelProtocols`（`model/iprovider/provider.go:40-43` 新增 `"gemini": true`）；
- 为空或未设置时，BFE 侧按兜底 `["openai"]` 对待。

### 2.2 discover-models 支持 gemini

**变更原因**：gemini 模型列表端点与响应结构不同。

| 协议 | 默认 URI | 列表路径 | ID 字段 | 备注 |
|------|----------|----------|---------|------|
| openai | `GET /v1/models`（现状不变） | `data` | `id` | 保持现状 |
| anthropic | `GET /v1/models`（现状不变） | `models` | `model_id` | 保持现状 |
| gemini（新增） | `GET /v1beta/models` | `models` | `name` | 需剥离 `models/` 前缀，如 `models/gemini-2.5-pro` → `gemini-2.5-pro` |

**变更后行为**（`model/iprovider/discover.go`）：

1. `modelProtocolParsers` 新增 gemini 解析器；
2. 默认 URI 由现状固定 `GET /v1/models`（`discover.go:90-93`）改为**按协议映射**（或在 parser 中携带端点信息）：gemini → `GET /v1beta/models`，openai/anthropic 保持 `GET /v1/models` 不变。

---

## 3. OpenAPI 无变更说明：`/clusters`

`/clusters` 的 `llm_config` 结构不变，仍通过 `llm_config.provider` 引用 provider，协议能力由被引用 provider 的 `model_protocols` 表达并经导出链路透传。

**不引入 cluster 级 `model_protocols` 声明的决策**：

- cluster 协议能力恒为 provider 透传值，保持单一事实来源，避免 provider/cluster 双源语义；
- provider 收缩 `model_protocols` 时，所有透传型引用 cluster 自动跟随，不存在悬空引用，因此无需 provider 移除保护（反查校验）；
- 若未来需要 per-cluster 协议限制（如 provider 同时声明 openai+gemini，但某 cluster 仅暴露其一），通过为该 cluster 拆分独立 provider 实现；如该需求高频出现，再评审 cluster 级声明。

**已知取舍**：provider 未声明 gemini 时，创建引用它的 cluster 不会在控制面报错，gemini 请求会在 BFE 转发阶段被 `PROVIDER_PROTOCOL_MISMATCH` 拒绝（错误延迟到数据面）。本期接受该行为，与 anthropic 接入时一致。

---

## 4. InnerAPI 无变更说明

`/configs/tls_conf/server_data_conf` 中 `AIConf.ModelProtocols` 取值规则不变：恒等于 cluster 所引用 provider 的 `model_protocols` 透传值。BFE 零改动。

---

## 5. 依赖的 BFE 侧变更

BFE 数据面改动（适配器目录 `bfe/bfe_model_protocol/gemini/`、`detect.go` 识别、流式终止/最终 usage 判定下沉适配器、SSE 终止语义等）见总体设计方案 `document-ai-gateway/迭代系统设计/v0.6/gemini协议支持/gemini协议支持-设计方案.md` §5。

**发布顺序约束**：BFE 在配置加载期调用 `modelprotocol.ValidateProtocols`（`bfe/bfe_server/bfe_confdata_load.go:54-56`、`:133-136`），未知协议名会导致启动失败/热加载拒绝。因此 **bfe 必须先于（或同步于）ai-gateway-api 发布**；若 api 先放开枚举而 bfe 未升级，导出含 gemini 的配置会被 bfe 拒绝加载。
