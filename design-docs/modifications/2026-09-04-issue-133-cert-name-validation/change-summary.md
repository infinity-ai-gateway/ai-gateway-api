# Issue #133：证书名称 cert_name 字符集校验收紧与存量残留清理

> 对应上游 Issue：[rainway-ai-gateway/ai-gateway-api#133](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/133)

## 1. 概述

### 1.1 问题现象

通过 `POST /open-api/v1/certificates` 可以成功创建 `cert_name` 中包含 `/` 的证书（如 `sc3201-tc009-qa-20260904-1b8dfc35/child`），GET 列表也能正常返回；但 `DELETE /certificates/{cert_name}` 对已确认存在的该证书恒返回 404，证书永远无法删除，形成环境残留：

1. 资源泄漏：被接受的证书名成为不可回收残留，污染证书列表与管理面数据；
2. 测试/运维 cleanup 无法闭环：E2E 残留检查持续报警，只能手工进库/重启清理；
3. 契约不对称：创建/查询接受的名字集合 ⊃ 删除可寻址的名字集合，违反 CRUD 对称性。

### 1.2 根因

- **创建侧**（`endpoints/openapi_v1/certificate/create.go:38`）：`Name *string validate:"required,min=2"`——只校验非空与最小长度，对字符集无任何限制，`/`、`?`、`#`、空格等 URL 语义字符均可通过。
- **寻址侧**（`endpoints/openapi_v1/certificate/delete.go:36`、`one.go:29`、`update.go:36`）：路由均为单路径段绑定 `/certificates/{cert_name}`，handler 内用 `CertName` 精确过滤 `FetchCertificates`。含 `/` 的名字在路由层即无法匹配（路径被切成两段），请求根本到不了 handler；即使用 `%2F` 编码形式到达，网关/框架对 `%2F` 的解码处理也不可靠。
- 后果：名字一旦含 `/`，该证书进入"存在但不可寻址"状态，DELETE 路由集合与 create 接受集合的差集即残留集合。

### 1.3 变更目标

1. 创建/更新证书时，`cert_name` 仅允许 URL path 安全字符集（字母、数字、`.`、`_`、`-`），非法名返回 422 并给出明确错误消息；
2. 存量库中已存在的非法名可被一次性清理（运维 SQL 或改名）；
3. 删除/查询路由保持不变，无需改动；
4. 与项目其他资源（cluster、api-key 等）的命名规则对齐。

### 1.4 变更范围

| 范围 | 说明 |
|------|------|
| 涉及仓库 | `ai-gateway-api` |
| 涉及模块 | `lib/validate`、`endpoints/openapi_v1/certificate` |
| 接口契约 | `POST /certificates` 请求参数 `cert_name` 的合法性条件变更 |
| 数据迁移 | 无表结构变更；存量非法名需一次性清洗（见第 6 节） |
| 数据兼容 | 历史非法名数据不做自动改写，保持只读 |

---

## 2. 方案选型

**采用方案 A（双向收紧，Issue 推荐方案）：**

1. 在 create 入参校验中增加 `cert_name` 字符集白名单，非法名返回 422；
2. 对存量库中已存在的非法名，提供一次性迁移/清洗 SQL；
3. 删除路由保持不变。

> **备选方案 B（仅修复寻址）**：将 delete 改为接收 query/body 参数寻址（如 `DELETE /certificates?cert_name=...`），不改 create 校验。**未采用**：虽然能解存量残留，但留下"允许 URL 不友好名字"的口子，且与项目其他资源的命名规则不一致，破坏 CRUD 对称性的根因未消除。

---

## 3. 现状代码分析

### 3.1 当前 `cert_name` 校验（缺失）

`endpoints/openapi_v1/certificate/create.go:37-45`：

```go
type CreateParam struct {
	Name *string `json:"cert_name" validate:"required,min=2"`
	// ...
}
```

仅 `required,min=2`，无字符集限制。删除/查询/更新路由上的 `OneParam.CertName`（`delete.go:30`）同样只校验 `required,min=2`（这部分保留不变）。

### 3.2 可复用的既有校验基建

`lib/validate/validate.go` 已有成熟的命名校验模式（Issue #81 实体名校验同一套基建）：

- `nameToken = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)`（`validate.go:38`）——ClusterName/UserName/TokenName 使用的字符集，**允许 `.`、`_`、`-`，不含 `/`、`?`、`#`、空格**；
- `validateName(s, minLen, maxLen, name)` + `validateNamePattern(s, re, name)` + `validateNameEdges(s, name)`（`validate.go:165-186`）——长度、字符集、首尾 `-`/`_` 限制的组合校验原语；
- `ClusterName`（`validate.go:247`）即按上述模式实现，可作为 `CertName` 的直接参照。

### 3.3 校验入口

`CreateAction`（`create.go:79`）在绑定参数后、调用 `CertificateManager.CreateCertificate` 前插入 `cert_name` 合法性检查。项目惯例是在 endpoint 层做该校验（参照 `endpoints/openapi_v1/bfe_cluster/create.go:36` 对 `validate.ClusterName` 的调用）。

---

## 4. 详细设计

### 4.1 新的 `cert_name` 校验规则

| 校验项 | 规则 |
|--------|------|
| 长度 | 2-64 字符（保留现有 `min=2` 约束，上限对齐 ClusterName 的 64） |
| 允许字符 | 字母 `a-zA-Z`、数字 `0-9`、`.`、`_`、`-` |
| 首尾限制 | 不能以 `-` 或 `_` 开头或结尾（`validateNameEdges`） |
| 空白/控制字符 | 不允许（已由字符集规则覆盖） |
| 全局唯一 | 由 model 层查重保证，不在校验函数内 |

错误处理：使用 `xerror.WrapParamErrorWithMsg`，返回 HTTP 422，消息明确指出的非法字符。

### 4.2 `lib/validate/validate.go` 修改

新增常量与校验函数（与 `ClusterName` 同构）：

```go
const (
	// ... 现有常量 ...
	MaxCertNameLength = 64
)

// CertName validates a certificate name.
func CertName(s string) error {
	if err := validateName(s, 2, MaxCertNameLength, "cert_name"); err != nil {
		return err
	}
	if err := validateNamePattern(s, nameToken, "cert_name"); err != nil {
		return err
	}
	if err := validateNameEdges(s, "cert_name"); err != nil {
		return err
	}
	return nil
}
```

> 复用 `nameToken`，与 ClusterName/UserName/TokenName 字符集完全一致，符合 Issue 中"对齐 cluster/api-key 命名惯例"的要求；长度下限取 2 以兼容现有 `min=2` 约束。

### 4.3 Endpoint 层修改

`endpoints/openapi_v1/certificate/create.go`：在 `CreateAction` 中绑定参数后增加：

```go
if err := validate.CertName(*param.Name); err != nil {
	return nil, err
}
```

> 删除（`delete.go`）、查询（`one.go`）、设默认（`update.go`）路由参数 `OneParam.CertName`/`UpdateParam.CertName` **不做字符集校验**（URI 绑定环节已天然限制为单路径段），保持现状。

### 4.4 API 文档更新

更新 `design-docs/api-define/OpenAPI接口定义/` 中证书接口定义文档的 `cert_name` 字段描述：

> 必填；长度 2-64 字符；仅允许字母、数字、`.`、`_`、`-`；不能以 `-`、`_` 开头或结尾

---

## 5. 涉及文件清单

| 文件 | 修改内容 |
|------|----------|
| `lib/validate/validate.go` | 新增 `MaxCertNameLength` 常量与 `CertName` 校验函数，复用 `nameToken`。 |
| `lib/validate/validate_test.go` | 新增 `TestCertName` 单元测试用例。 |
| `endpoints/openapi_v1/certificate/create.go` | `CreateAction` 中增加 `validate.CertName` 调用。 |
| `design-docs/api-define/OpenAPI接口定义/00-common.md` | 新增 `## 19. 证书名称（CertName）` 公共类型定义。 |
| `design-docs/api-define/OpenAPI接口定义/certificates.md` | 5 处 `cert_name` 字段描述改为引用 `CertName` 公共类型。 |

---

## 6. 存量非法数据清理（上线前一次性执行）

Issue 报告测试环境存在 2 个残留证书：`../sc3201-tc009-qa-20260904-1b8dfc35`、`sc3201-tc009-qa-20260904-1b8dfc35/child`。由于这些名字含 `/` 或 `.` 开头，无法通过任何 API 路径删除，需直接操作数据库：

```sql
-- 先查询确认（certificates 为证书表，列名以 db_ddl.sql 实际定义为准）
SELECT * FROM certificates WHERE cert_name NOT REGEXP '^[a-zA-Z0-9][a-zA-Z0-9._-]{1,63}$';

-- 删除残留（逐条确认后执行）
DELETE FROM certificates WHERE cert_name = 'sc3201-tc009-qa-20260904-1b8dfc35/child';
DELETE FROM certificates WHERE cert_name = '../sc3201-tc009-qa-20260904-1b8dfc35';
```

> 说明：
> - 生产环境执行删除前需确认无下游（BFE 导出配置、conf-agent）引用相关证书；
> - 若更倾向保留数据，可改为改名（UPDATE）为合法名后再走正常 API 管理。

---

## 7. 测试计划

### 7.1 单元测试（`lib/validate`）

新增 `TestCertName`：

| 用例 | 输入 | 期望 |
|------|------|------|
| 合法-常规 | `"demo-cert"` | 通过 |
| 合法-含数字/点 | `"tc009.qa-20260904"` | 通过 |
| 合法-含下划线 | `"my_cert_01"` | 通过 |
| 合法-最小长度 | `"ab"` | 通过 |
| 合法-最大长度 | 64 字符合法字符串 | 通过 |
| 非法-含 `/` | `"demo/child"` | 失败 |
| 非法-含 `?` | `"demo?x=1"` | 失败 |
| 非法-含 `#` | `"demo#1"` | 失败 |
| 非法-含空格 | `"demo cert"` | 失败 |
| 非法-含 `%` | `"demo%2F"` | 失败 |
| 非法-长度 1 | `"a"` | 失败 |
| 非法-超长 | 65 字符 | 失败 |
| 非法-以 `-` 开头 | `"-demo"` | 失败 |
| 非法-以 `_` 结尾 | `"demo_"` | 失败 |

### 7.2 集成测试（证书创建接口）

| 场景 | 请求体 | 期望 |
|------|--------|------|
| 创建含 `/` 的证书 | `{"cert_name":"demo/parent", ...}` | 422 |
| 创建含 `?`/`#`/空格的证书 | 同上各类非法字符 | 422 |
| 创建合法证书 | 合法 `cert_name` | 200，且可正常 GET/DELETE |

### 7.3 回归测试

- `go test ./lib/validate/...`
- `go test ./endpoints/openapi_v1/certificate/...`
- `make test-model-cover-gate`

---

## 8. 风险与注意事项

| 风险 | 说明 | 缓解措施 |
|------|------|----------|
| 历史非法名更新失败 | 若对已存在的非法名证书调用 PATCH `/certificates/{cert_name}/default`，URI 绑定仍无法匹配（路由层问题，非本次校验引入），行为与现状一致 | 本次不改动该路由；存量非法名按第 6 节清理 |
| 字符集与 ClusterName 一致但含 `.` | `.` 在 URL path 单段中无语义问题，但连续点（`..`）理论上可能被网关做路径规范化 | `validateNameEdges` 限制首尾字符；`..` 段在单段路由中仍为合法段，不影响寻址，保持与 ClusterName 一致 |
| 前端/调用方已有依赖宽松命名的脚本 | 收紧后含特殊字符的创建请求返回 422 | 属预期行为变更，在 API 文档与 Release Note 中显式说明；422 错误消息明确指出非法字符 |

---

## 9. 实施状态

### 9.1 已完成的修改

- [x] `lib/validate/validate.go`：新增 `MaxCertNameLength = 64` 常量与 `CertName` 校验函数，复用 `nameToken` 与 `validateNameEdges`（与 `ClusterName` 同构，长度下限取 2 兼容现有 `min=2`）；
- [x] `lib/validate/validate_test.go`：新增 `TestCertName`，覆盖合法边界（长度 2/64）、非法字符（`/`、`?`、`#`、空格、`%`）、长度越界与首尾 `-`/`_`；
- [x] `endpoints/openapi_v1/certificate/create.go`：`CreateAction` 绑定参数后增加 `validate.CertName` 调用，非法名返回 422；
- [x] 删除/查询/设默认路由（`delete.go` / `one.go` / `update.go`）保持不变。

### 9.2 验证结果

| 测试命令 | 结果 |
|----------|------|
| `go build ./...` | PASS |
| `go test ./lib/validate/...` | PASS |
| `go test ./endpoints/openapi_v1/certificate/...` | PASS |
| `go test ./endpoints/... ./lib/...` | PASS |
| `go test ./tests/certificate/...`（在 `test/integration` 模块执行，需先 `go build -o ai-gateway-api.exe .` 重新编译二进制） | PASS |

> API 定义文档已同步：`design-docs/api-define/OpenAPI接口定义/00-common.md` 新增 `## 19. 证书名称（CertName）` 公共类型；`certificates.md` 中 5 处 `cert_name` 字段描述改为引用该公共类型（内容与新实现一致，仅按 Issue #81 确立的惯例收敛为公共类型引用）。
>
> 集成测试设计文档已同步：`test/integration/tests/certificate/design.md` 统计表与创建证书用例（CERT-1-004~013）已更新。

### 9.3 上线前检查清单

- [x] 代码修改完成；
- [x] 单元测试通过；
- [x] 集成测试用例补充完成（`test/integration/tests/certificate/create/create_test.go` 新增 CERT-1-004~013，已通过）；
- [x] 设计文档与 API 文档已同步；
- [ ] 存量 2 个残留证书已按第 6 节 SQL 清理（需运维执行）；
- [ ] E2E SC3201-TC009 的 cleanup 不再出现 404；
- [ ] 提交 PR 并关联 Issue #133。

---

## 10. 参考文档

- [Issue #133](https://github.com/rainway-ai-gateway/ai-gateway-api/issues/133)
- `design-docs/modifications/2026-08-24-issue-81-entity-name-validation/`（同一套 `lib/validate` 命名校验基建的先例）
- `ai-gateway-api/endpoints/openapi_v1/certificate/create.go`
- `ai-gateway-api/endpoints/openapi_v1/certificate/delete.go`
- `ai-gateway-api/lib/validate/validate.go`（`ClusterName` / `nameToken` / `validateNameEdges`）
