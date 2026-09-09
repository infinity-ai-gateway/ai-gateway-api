# sys-design 文档索引

本目录存放 `ai-gateway-api` 系统级设计文档，覆盖总体架构、接口层、模型层、存储层、数据库设计以及关键模块的细节设计。

---

## 一、总体与分层设计

| 文档名称 | 相对路径 | 摘要说明 |
|---------|---------|---------|
| 总体设计文档 | [总体设计文档.md](./总体设计文档.md) | 描述 `ai-gateway-api` 在 AI 网关中的定位、功能范围、三层架构（接口层/模型层/存储层）、组件交互关系、核心数据流以及 OpenAPI 与 InnerAPI 的职责划分。 |
| 接口层设计文档 | [接口层设计文档.md](./接口层设计文档.md) | 描述管理面 OpenAPI（`/open-api/v1`）与数据面 InnerAPI（`/inner-api/v1`）的路由组织、统一的 `xreq.Endpoint` 抽象、全局与业务中间件链、子包划分以及典型接口实现模式。 |
| 模型层设计文档 | [模型层设计文档.md](./模型层设计文档.md) | 描述 `model/` 层的子包职责、Manager + Storager 接口的分层模式、Param/Filter 设计、事务管理、典型业务流程以及各业务模型（API-Key、Entity、Provider、Cluster、Quota、RateLimit、Route 等）的交互方式。 |
| 存储层设计文档 | [存储层设计文档.md](./存储层设计文档.md) | 描述 `storage/rdb` 层的 DAO + Storage 两层结构、通用 DAO 设计模式、事务与连接管理、31 张表的 DAO 映射关系以及 Storage 实现如何向上暴露接口供模型层调用。 |
| 数据库设计文档 | [数据库设计文档.md](./数据库设计文档.md) | 描述 `ai-gateway-api` 当前实现中全部 31 张持久化表的字段、约束、索引、JSON 字段结构以及表间逻辑关系，覆盖基础配置、集群、路由、证书、API-Key、Entity、配额、限流、模型定价、操作日志等模块。 |

---

## 二、细节设计（details/）

| 文档名称 | 相对路径 | 摘要说明 |
|---------|---------|---------|
| 操作日志模块 | [details/操作日志模块.md](./details/操作日志模块.md) | 描述 `model/ioperlog` 操作日志设计：覆盖 entity / api-key / provider / cluster 等域的变更记录、成功与失败双路径日志、异步批量写入、敏感字段脱敏与 `GET /operation-logs` 查询接口。 |
| 认证授权机制 | [details/认证授权机制.md](./details/认证授权机制.md) | 描述 `model/iauth` 的认证授权设计，包括用户与 Token 共用 `users` 表、`Visitor` 统一抽象、Scope 作用域、Feature-Action 权限模型、四种认证方式以及中间件集成与数据库表设计。 |
| InnerAPI 配置导出与版本控制 | [details/InnerAPI配置导出与版本控制.md](./details/InnerAPI配置导出与版本控制.md) | 描述面向 BFE/Conf Agent 的 InnerAPI 配置导出机制，包括 `VersionControlManager` 的 MD5 签名比对、版本号生成与同 Topic 严格单调递增保证（`uk_name_version` 唯一约束 + 版本 +1s 抬升 + 冲突重试，issue #142）、`config_versions` 表持久化、9 类配置导出主题、增量同步流程，以及 `mod-api-key` 的批量预加载 + 内存回溯性能优化。 |
| API-Key 与 Entity 关联及模型继承 | [details/API-Key与Entity关联及模型继承.md](./details/API-Key与Entity关联及模型继承.md) | 描述 API-Key 与 Entity 的挂载关系、Entity 层级树约束、模型白名单交集与黑名单继承、配额计划层级合并、限流策略与路由规则的层级收集，以及导出到 BFE 时的最终生效规则。 |
| 限流策略与导出 | [details/限流策略与导出.md](./details/限流策略与导出.md) | 描述限流策略的数据模型（TPM/RPM/并发数）、JSON 配置结构、CRUD 校验、API-Key/Entity 引用关系、按 Entity 层级向上合并导出到 BFE 的流程，以及 BFE 侧预期行为与边界情况。 |
| 路由规则管理 | [details/路由规则管理.md](./details/路由规则管理.md) | 区分产品级 BFE 路由规则与 AI 路由规则，描述 `route_rules` 表的三级（Global/Entity/API-Key）管理、校验规则、与 API-Key/Entity 生命周期的一致性、导出到 BFE 的绑定顺序与文件格式。 |
| 配额余额同步机制 | [details/配额余额同步机制.md](./details/配额余额同步机制.md) | 描述 Redis 作为余额唯一真实来源的架构，包括 OpenAPI 查询余额直接读 Redis、`QuotaCache` 批量读取接口（`BatchGetRemaining` / `GetInt64Batch`）、`QuotaResetScheduler` 调度器（含 Redis 分布式锁 + 看门狗续期）、`BalanceSyncManager` 的过期重置（原子 `SET` Redis + `last_reset_at` 条件更新）、`QuotaPlanManager.ApplyQuotaPlanChange` 对 quota_plan 变更的差异化处理、自然周/月重置逻辑、Redis Key 生命周期以及 `last_reset_at` 上移至 `quota_plans` 表的设计；包含 RMB 配额定点精度设计与 `Clock` 时钟注入。 |
| Provider 与 Cluster 概念分离 | [details/provider与cluster概念分离.md](./details/provider与cluster概念分离.md) | 描述 Provider 与 Cluster 解耦后的数据模型、接口变化、BFE 配置生成转换、引用关系、迁移策略与风险。 |
| Claude 协议转发支持 | [details/Claude协议转发支持.md](./details/Claude协议转发支持.md) | 描述 ai-gateway-api 控制面为支持 BFE 转发 Claude Messages API 所需配合的修改，包括 `AIConf.ModelProtocols` 透传、模型发现解析器下沉与废弃配置清理。 |
| Redis Key 清理机制 | [details/Redis Key 清理机制.md](./details/Redis%20Key%20清理机制.md) | 描述 API-Key / Entity 删除及 rate-limit 规则变更时的 Redis Key 清理逻辑，包括 Quota Key 与 Rate-Limit Key 格式、触发场景、对 BFE 的影响以及控制面立即清理实现方案。 |
| 模型定价管理 | [api-define/OpenAPI接口定义/model-prices.md](../api-define/OpenAPI接口定义/model-prices.md) | 描述 `/model-prices` 管理接口、`model-list.yaml` 导入格式、`ModelPrice` 数据模型与校验规则，是 InnerAPI `AIConf.ModelTable` 的数据源。 |
| RMB 配额分时段定价 | [details/RMB配额分时段定价.md](./details/RMB配额分时段定价.md) | 描述 RMB 配额按 provider 时段模板与 model-prices 分时段价格进行计费的设计，包括 `time_zone` / `tiers` / `tier_prices` 数据模型、控制面导出逻辑、BFE 数据面时段匹配与成本计算。 |
| EPP 调度对接 | [details/EPP调度对接.md](./details/EPP调度对接.md) | 描述 EPP（llm-d 调度器）接入后控制面的四块能力：cluster `balance_mode`/`epp_config`（简化用户形态 + 确定性编译）、`/epp-pool` 静态实例池、分配器（贪心确定性算法、悬空修复、周期对账）、双向下发（server_data_conf 的 `BalanceMode`/`EPPAddr` 含降级语义、epp_data 单 topic 两段合并）。 |

---

## 三、阅读建议

1. **快速建立全局认知**：先阅读《总体设计文档.md》。
2. **理解代码分层**：依次阅读《接口层设计文档.md》《模型层设计文档.md》《存储层设计文档.md》。
3. **查看具体表结构**：参考《数据库设计文档.md》。
4. **深入关键模块**：根据关注领域选择 `details/` 下的细节文档。
