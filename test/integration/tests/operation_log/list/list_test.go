// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.

package operation_log_test

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sm *testutil.ServerManager

func TestMain(m *testing.M) {
	var err error
	sm, err = testutil.StartServer()
	if err != nil {
		panic("failed to start server: " + err.Error())
	}
	code := m.Run()
	sm.Shutdown()
	os.Exit(code)
}

// TestOperationLog_GenerationForVariousAPIs 验证各类配置域的写操作都会生成对应的操作日志。
func TestOperationLog_GenerationForVariousAPIs(t *testing.T) {
	client := testutil.GetClient()

	// 1. 创建 entity-type
	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err, "create entity type failed")

	// 2. 创建 entity
	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	require.NoError(t, err, "create entity failed")

	// 3. 创建 api-key
	apiKeyID, err := testutil.CreateAPIKey("test-key-for-operation-log", entityID)
	require.NoError(t, err, "create api key failed")

	// 4. 创建 provider
	providerName := testutil.UniqueProviderName()
	_, err = testutil.CreateProvider(providerName)
	require.NoError(t, err, "create provider failed")

	// 5. 创建 cluster（依赖 provider）
	clusterName := testutil.UniqueClusterName()
	_, err = testutil.CreateCluster(clusterName)
	require.NoError(t, err, "create cluster failed")

	// 5.1 更新 Global 路由表（产生 route/update 日志）
	require.NoError(t, testutil.ResetGlobalRouteRules(), "reset global route rules failed")
	require.NoError(t, testutil.SetGlobalRouteRules([]interface{}{testutil.SimpleRouteRule("global-route-for-operation-log", clusterName)}), "set global route rules failed")

	// 6. 创建 certificate：先创建默认证书，再创建非默认证书。
	defaultCertName := testutil.UniqueCertName()
	_, err = testutil.CreateCertificate(defaultCertName, true)
	require.NoError(t, err, "create default certificate failed")

	certName := testutil.UniqueCertName()
	_, err = testutil.CreateCertificate(certName, false)
	require.NoError(t, err, "create certificate failed")

	// 7. 创建 user
	userName := testutil.UniqueUserName()
	password := "TestPass123!"
	require.NoError(t, testutil.CreateUser(userName, password), "create user failed")

	// 8. 创建 token
	tokenName := testutil.UniqueTokenName()
	_, err = testutil.CreateToken(tokenName, "System")
	require.NoError(t, err, "create token failed")

	// 9. 更新 entity-type（产生 update 日志）
	resp, err := client.Patch("/open-api/v1/entity-types/"+typeName, map[string]interface{}{
		"description": "updated description",
	})
	require.NoError(t, err, "update entity-type request failed")
	testutil.AssertSuccess(t, resp)

	// 10. 删除 api-key（产生 delete 日志）
	require.NoError(t, testutil.DeleteAPIKey(apiKeyID), "delete api key failed")

	// 轮询等待各域操作日志落库。
	// 大部分域使用业务 ID（resource_id）查询；cluster/user/token 使用资源名称（resource_name）查询。
	expectedLogs := []struct {
		resourceType string
		action       string
		resourceID   string
		resourceName string
	}{
		{"entity_type", "create", typeName, ""},
		{"entity_type", "update", typeName, ""},
		{"entity", "create", entityID, ""},
		{"api_key", "create", apiKeyID, ""},
		{"api_key", "delete", apiKeyID, ""},
		{"provider", "create", providerName, ""},
		{"cluster", "create", "", clusterName},
		{"route", "update", "global", ""},
		{"certificate", "create", defaultCertName, ""},
		{"certificate", "create", certName, ""},
		{"user", "create", "", userName},
		{"token", "create", "", tokenName},
	}

	for _, expected := range expectedLogs {
		t.Run(fmt.Sprintf("%s/%s", expected.resourceType, expected.action), func(t *testing.T) {
			filter := map[string]string{
				"resource_type": expected.resourceType,
				"action":        expected.action,
			}
			if expected.resourceID != "" {
				filter["resource_id"] = expected.resourceID
			}
			if expected.resourceName != "" {
				filter["resource_name"] = expected.resourceName
			}

			entry, err := testutil.WaitForOperationLog(filter, 0)
			require.NoError(t, err, "expected operation log not found")
			assert.Equal(t, expected.action, entry.Action)
			assert.Equal(t, expected.resourceType, entry.ResourceType)
			assert.Equal(t, float64(1), entry.Status, "operation should be success")
			assert.NotEmpty(t, entry.RequestPath)
			assert.NotEmpty(t, entry.CreatedAt)
		})
	}

	// 清理资源
	_ = testutil.ResetGlobalRouteRules()
	_ = testutil.DeleteToken(tokenName)
	_ = testutil.DeleteUser(userName)
	_ = testutil.DeleteCertificate(certName)
	_ = testutil.DeleteCertificate(defaultCertName)
	_ = testutil.DeleteCluster(clusterName)
	_ = testutil.DeleteProvider(providerName)
	_ = testutil.DeleteEntity(entityID)
	_ = testutil.DeleteEntityType(typeName)
}

// TestOperationLog_ListFilters 验证操作日志查询接口的过滤与分页能力。
func TestOperationLog_ListFilters(t *testing.T) {
	// 前置：创建 entity-type、entity 和 api-key，产生操作日志。
	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err, "setup entity type failed")

	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	require.NoError(t, err, "setup entity failed")

	apiKeyID, err := testutil.CreateAPIKey("test-key-for-operation-log", entityID)
	require.NoError(t, err, "setup api key failed")

	// 等待异步操作日志落库。
	_, err = testutil.WaitForOperationLog(map[string]string{
		"resource_type": "api_key",
		"action":        "create",
		"resource_id":   apiKeyID,
	}, 0)
	require.NoError(t, err, "api_key create log not found")

	tests := []struct {
		name       string
		query      map[string]string
		minTotal   int
		wantAction string
	}{
		{
			name:     "OL-2-001 无条件查询",
			query:    map[string]string{},
			minTotal: 3,
		},
		{
			name:       "OL-2-002 按 action 过滤",
			query:      map[string]string{"action": "create"},
			minTotal:   3,
			wantAction: "create",
		},
		{
			name:     "OL-2-003 按 resource_type 过滤",
			query:    map[string]string{"resource_type": "entity"},
			minTotal: 1,
		},
		{
			name:     "OL-2-004 按 resource_id 过滤",
			query:    map[string]string{"resource_id": entityID},
			minTotal: 1,
		},
		{
			name:     "OL-2-005 按不存在 action 过滤",
			query:    map[string]string{"action": "not_exist_action"},
			minTotal: 0,
		},
		{
			name:     "OL-2-006 分页查询",
			query:    map[string]string{"page": "1", "page_size": "2"},
			minTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, resp, err := testutil.QueryOperationLogs(tt.query)
			require.NoError(t, err)
			testutil.AssertSuccess(t, resp)

			assert.GreaterOrEqual(t, result.Pagination.Total, int64(tt.minTotal), "total should >= %d", tt.minTotal)
			if tt.minTotal > 0 {
				testutil.AssertDataNotEmpty(t, resp)
				assert.NotEmpty(t, result.List)
			}

			if tt.wantAction != "" {
				for _, item := range result.List {
					assert.Equal(t, tt.wantAction, item.Action)
				}
			}

			// 校验列表中至少包含一条当前测试创建的 entity 日志，且 change_summary 已被记录。
			if tt.name == "OL-2-001 无条件查询" {
				foundEntityLog := false
				for _, item := range result.List {
					if item.ResourceType == "entity" && item.ResourceID == entityID && item.Action == "create" {
						foundEntityLog = true
						assert.Equal(t, entityName, item.ResourceName)
						assert.Equal(t, float64(1), item.Status)
						assert.NotEmpty(t, item.CreatedAt)
						assert.NotNil(t, item.ChangeSummary)
					}
				}
				assert.True(t, foundEntityLog, "should find create entity operation log")
			}
		})
	}

	// 清理
	_ = testutil.DeleteAPIKey(apiKeyID)
	_ = testutil.DeleteEntity(entityID)
	_ = testutil.DeleteEntityType(typeName)
}

// TestOperationLog_FailedOperationRecordsError 验证写操作失败时同样会生成 status=2 的操作日志。
// 该用例复现 GitHub issue #117：删除仍被路由规则引用的集群时接口返回 500，但此前操作日志为空。
func TestOperationLog_FailedOperationRecordsError(t *testing.T) {
	client := testutil.GetClient()

	// 1. 清理全局路由规则，避免受其他测试影响。
	require.NoError(t, testutil.ResetGlobalRouteRules(), "reset global route rules failed")

	// 2. 创建 cluster。
	clusterName := testutil.UniqueClusterName()
	_, err := testutil.CreateCluster(clusterName)
	require.NoError(t, err, "create cluster failed")

	// 3. 设置 global route rules 引用该 cluster，使删除失败。
	require.NoError(t, testutil.SetGlobalRouteRules([]interface{}{testutil.SimpleRouteRule("failed-op-ref", clusterName)}), "set global route rules failed")

	// 4. 调用删除接口，预期失败。
	resp, err := client.Delete("/open-api/v1/clusters/" + clusterName)
	require.NoError(t, err, "delete cluster request failed")
	assert.NotEqual(t, 200, resp.ErrNum, "expected delete cluster to fail")

	// 5. 轮询等待失败的操作日志落库。
	entry, err := testutil.WaitForOperationLog(map[string]string{
		"resource_type": "cluster",
		"action":        "delete",
		"resource_name": clusterName,
		"status":        "2",
	}, 0)
	require.NoError(t, err, "expected failed operation log not found")

	assert.Equal(t, "delete", entry.Action)
	assert.Equal(t, "cluster", entry.ResourceType)
	assert.Equal(t, clusterName, entry.ResourceName)
	assert.Equal(t, float64(2), entry.Status, "operation should be failed")
	assert.NotEmpty(t, entry.ErrorMsg, "failed operation log should contain error message")
	assert.NotEmpty(t, entry.RequestPath)
	assert.NotEmpty(t, entry.CreatedAt)

	// 6. 清理。
	require.NoError(t, testutil.ResetGlobalRouteRules(), "cleanup global route rules failed")
	_ = testutil.DeleteCluster(clusterName)
}

// TestOperationLog_FailedEntityUpdateKeepsResourceIdentity 验证 Entity 前置校验失败时
// 失败审计日志仍保留资源身份（issue #155 / SC2101-TC046 回归锚点）。
func TestOperationLog_FailedEntityUpdateKeepsResourceIdentity(t *testing.T) {
	client := testutil.GetClient()

	// 1. 创建 entity-type（level 1）与目标 entity。
	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err, "create entity type failed")
	defer testutil.DeleteEntityType(typeName)

	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	require.NoError(t, err, "create entity failed")
	defer testutil.DeleteEntity(entityID)

	// 2. PUT 极简 body（仅 parent_id 且指向不存在的 entity），触发事务前置层级校验失败。
	resp, err := client.Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
		"parent_id": "entity-not-exist-888",
	})
	require.NoError(t, err, "update entity request failed")
	assert.NotEqual(t, 200, resp.ErrNum, "expected update entity to fail")

	// 3. 轮询等待失败日志，断言资源身份齐全（resource_id == URI 中的 Entity ID）。
	entry, err := testutil.WaitForOperationLog(map[string]string{
		"resource_type": "entity",
		"action":        "update",
		"resource_id":   entityID,
		"status":        "2",
	}, 0)
	require.NoError(t, err, "expected failed operation log with resource identity not found")

	assert.Equal(t, "entity", entry.ResourceType)
	assert.Equal(t, "update", entry.Action)
	assert.Equal(t, entityID, entry.ResourceID)
	assert.Equal(t, entityName, entry.ResourceName)
	assert.Equal(t, float64(2), entry.Status, "operation should be failed")
	assert.NotEmpty(t, entry.ErrorMsg, "failed operation log should contain error message")
}

// TestOperationLog_UpdateEntityTypeHasDiffKeys 验证 update 动作的操作日志包含 diff_keys。
func TestOperationLog_UpdateEntityTypeHasDiffKeys(t *testing.T) {
	client := testutil.GetClient()

	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err, "setup entity type failed")

	// 更新 entity-type 的 description，产生 update 日志。
	resp, err := client.Patch("/open-api/v1/entity-types/"+typeName, map[string]interface{}{
		"description": "updated description for diff_keys test",
	})
	require.NoError(t, err, "update entity-type request failed")
	testutil.AssertSuccess(t, resp)

	entry, err := testutil.WaitForOperationLog(map[string]string{
		"resource_type": "entity_type",
		"action":        "update",
		"resource_id":   typeName,
	}, 0)
	require.NoError(t, err, "expected update operation log not found")

	assert.Equal(t, "update", entry.Action)
	assert.Equal(t, "entity_type", entry.ResourceType)
	assert.Equal(t, typeName, entry.ResourceID)

	require.NotNil(t, entry.ChangeSummary, "change_summary should be present")
	assert.Contains(t, entry.ChangeSummary, "before")
	assert.Contains(t, entry.ChangeSummary, "after")
	require.Contains(t, entry.ChangeSummary, "diff_keys")

	diffKeys, ok := entry.ChangeSummary["diff_keys"].([]interface{})
	require.True(t, ok, "diff_keys should be an array")
	assert.Contains(t, diffKeys, "description")

	// 清理
	_ = testutil.DeleteEntityType(typeName)
}

// TestOperationLog_UpdateGlobalRouteRulesHasLog 验证更新 Global 路由表会生成操作日志（issue #124）。
func TestOperationLog_UpdateGlobalRouteRulesHasLog(t *testing.T) {
	// 0. 记录当前 route/global 的最大日志 ID，避免被旧日志干扰。
	var maxIDBefore float64
	result, _, err := testutil.QueryOperationLogs(map[string]string{
		"resource_type": "route",
		"action":        "update",
		"resource_id":   "global",
	})
	require.NoError(t, err)
	for _, item := range result.List {
		if item.ID > maxIDBefore {
			maxIDBefore = item.ID
		}
	}

	// 1. 准备依赖：provider + cluster。
	providerName := testutil.UniqueProviderName()
	_, err = testutil.CreateProvider(providerName)
	require.NoError(t, err, "setup provider failed")

	clusterName := testutil.UniqueClusterName()
	_, err = testutil.CreateCluster(clusterName)
	require.NoError(t, err, "setup cluster failed")

	// 2. 将 Global 路由表重置为空，作为 before 状态。
	require.NoError(t, testutil.ResetGlobalRouteRules(), "reset global route rules failed")

	// 3. 更新 Global 路由表，产生 update 日志。
	ruleName := "global-route-log-diff"
	require.NoError(t, testutil.SetGlobalRouteRules([]interface{}{testutil.SimpleRouteRule(ruleName, clusterName)}), "set global route rules failed")

	// 4. 等待本次操作产生的新日志落库。
	deadline := time.Now().Add(10 * time.Second)
	var entry *testutil.OperationLogEntry
	for time.Now().Before(deadline) {
		result, _, err = testutil.QueryOperationLogs(map[string]string{
			"resource_type": "route",
			"action":        "update",
			"resource_id":   "global",
			"status":        "1",
		})
		require.NoError(t, err)
		for i := range result.List {
			if result.List[i].ID > maxIDBefore {
				entry = &result.List[i]
				break
			}
		}
		if entry != nil {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	require.NotNil(t, entry, "expected new global route rules operation log not found")

	// 5. 校验操作日志字段。
	assert.Equal(t, "update", entry.Action)
	assert.Equal(t, "route", entry.ResourceType)
	assert.Equal(t, "global", entry.ResourceID)
	assert.Equal(t, "global", entry.ResourceName)
	assert.Equal(t, float64(1), entry.Status)
	assert.Equal(t, "/open-api/v1/global-route-rules", entry.RequestPath)
	assert.Equal(t, "PUT", entry.RequestMethod)
	assert.NotEmpty(t, entry.CreatedAt)

	// 6. 校验变更摘要包含 before / after / diff_keys。
	require.NotNil(t, entry.ChangeSummary, "change_summary should be present")
	assert.Contains(t, entry.ChangeSummary, "before")
	assert.Contains(t, entry.ChangeSummary, "after")
	require.Contains(t, entry.ChangeSummary, "diff_keys")

	diffKeys, ok := entry.ChangeSummary["diff_keys"].([]interface{})
	require.True(t, ok, "diff_keys should be an array")
	assert.Contains(t, diffKeys, "rules")

	// 7. 清理。
	require.NoError(t, testutil.ResetGlobalRouteRules(), "cleanup global route rules failed")
	_ = testutil.DeleteCluster(clusterName)
	_ = testutil.DeleteProvider(providerName)
}

// TestOperationLog_PaginationPageTwoHasTotal 验证操作日志分页到第二页时 total 不为 0（issue #123）。
func TestOperationLog_PaginationPageTwoHasTotal(t *testing.T) {
	// 创建 3 个 entity-type，产生至少 3 条 create 日志，以便分页。
	names := make([]string, 3)
	for i := range names {
		names[i] = testutil.UniqueEntityTypeName()
		_, err := testutil.CreateEntityType(names[i], 1)
		require.NoError(t, err, "create entity type failed")
	}

	// 等待日志落库。
	_, err := testutil.WaitForOperationLog(map[string]string{
		"resource_type": "entity_type",
		"action":        "create",
		"resource_id":   names[2],
	}, 0)
	require.NoError(t, err, "expected entity_type create log not found")

	// 查询第一页。
	page1, resp, err := testutil.QueryOperationLogs(map[string]string{
		"resource_type": "entity_type",
		"action":        "create",
		"page":          "1",
		"page_size":     "2",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	assert.Len(t, page1.List, 2, "page 1 should contain 2 records")
	assert.GreaterOrEqual(t, page1.Pagination.Total, int64(3), "total should be at least 3")

	// 查询第二页，重点校验 total 不应变为 0。
	page2, resp, err := testutil.QueryOperationLogs(map[string]string{
		"resource_type": "entity_type",
		"action":        "create",
		"page":          "2",
		"page_size":     "2",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	assert.GreaterOrEqual(t, page2.Pagination.Total, int64(3), "page 2 total should still be at least 3")
	assert.GreaterOrEqual(t, len(page2.List), 1, "page 2 should contain at least 1 record")

	// 清理。
	for _, name := range names {
		_ = testutil.DeleteEntityType(name)
	}
}

// TestOperationLog_RecordsUserAgentAndClientIP 验证操作日志记录 user_agent 与真实 client_ip（issue #127）。
func TestOperationLog_RecordsUserAgentAndClientIP(t *testing.T) {
	// 创建 entity-type 产生一条 create 日志；Go HTTP 客户端会自动携带 User-Agent。
	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err, "create entity type failed")

	entry, err := testutil.WaitForOperationLog(map[string]string{
		"resource_type": "entity_type",
		"action":        "create",
		"resource_id":   typeName,
	}, 0)
	require.NoError(t, err, "expected entity_type create log not found")

	// user_agent 应记录请求 User-Agent 头。
	assert.NotEmpty(t, entry.UserAgent, "user_agent should not be empty")

	// client_ip 应回退到真实来源 IP（测试进程经 loopback 访问，应为 127.0.0.1）。
	assert.NotEmpty(t, entry.ClientIP, "client_ip should not be empty")

	// 清理。
	_ = testutil.DeleteEntityType(typeName)
}
