package openapi

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
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

func TestOpenAPI_Schema(t *testing.T) {
	t.Run("entity_types", testEntityTypeSchema)
	t.Run("entities", testEntitySchema)
	t.Run("api_keys", testAPIKeySchema)
	t.Run("providers", testProviderSchema)
	t.Run("clusters", testClusterSchema)
	t.Run("certificates", testCertificateSchema)
	t.Run("auth", testAuthSchema)
	t.Run("model_prices", testModelPriceSchema)
	t.Run("route_tables", testRouteTableSchema)
	t.Run("global_route_rules", testGlobalRouteRulesSchema)
	t.Run("epp_pool", testEppPoolSchema)
	t.Run("epp_assignments", testEppAssignmentsSchema)
}

// ---------- entity-types ----------

func testEntityTypeSchema(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()

	resp, err := testutil.GetClient().Post("/open-api/v1/entity-types", map[string]interface{}{
		"type_name":   typeName,
		"description": "schema test",
		"level":       1,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	testutil.AssertSchema(t, resp, EntityTypeSchema)

	listResp, err := testutil.GetClient().Get("/open-api/v1/entity-types")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertPagedListSchema(t, listResp, EntityTypeSchema)

	oneResp, err := testutil.GetClient().Get("/open-api/v1/entity-types/" + typeName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, EntityTypeSchema)

	patchResp, err := testutil.GetClient().Patch("/open-api/v1/entity-types/"+typeName, map[string]interface{}{
		"description": "updated schema test",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)
	testutil.AssertSchema(t, patchResp, EntityTypeSchema)

	t.Cleanup(func() {
		testutil.DeleteEntityType(typeName)
	})
}

// ---------- entities ----------

func testEntitySchema(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err)

	entityName := testutil.UniqueEntityName()
	createResp, err := testutil.GetClient().Post("/open-api/v1/entities", map[string]interface{}{
		"name": entityName,
		"type": typeName,
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        1000000,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)
	testutil.AssertSchema(t, createResp, EntitySchema)

	id, err := testutil.GetDataField(createResp, "id")
	require.NoError(t, err)
	entityID := id.(string)

	listResp, err := testutil.GetClient().Get("/open-api/v1/entities")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertPagedListSchema(t, listResp, EntityListItemSchema)

	oneResp, err := testutil.GetClient().Get("/open-api/v1/entities/" + entityID)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, EntitySchema)

	putResp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
		"name": entityName + "-updated",
		"type": typeName,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, putResp)
	testutil.AssertSchema(t, putResp, EntitySchema)

	patchResp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
		"allow_models": []string{"*"},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)
	testutil.AssertSchema(t, patchResp, EntitySchema)

	qpResp, err := testutil.GetClient().Get("/open-api/v1/entities/" + entityID + "/quota-plan")
	require.NoError(t, err)
	testutil.AssertSuccess(t, qpResp)
	testutil.AssertSchema(t, qpResp, QuotaPlanWithBalanceSchema)

	resetResp, err := testutil.GetClient().Post("/open-api/v1/entities/"+entityID+"/quota-plan/reset", map[string]interface{}{})
	require.NoError(t, err)
	testutil.AssertSuccess(t, resetResp)
	testutil.AssertSchema(t, resetResp, QuotaResetResultSchema)

	t.Cleanup(func() {
		testutil.DeleteEntity(entityID)
		testutil.DeleteEntityType(typeName)
	})
}

// ---------- api-keys ----------

func testAPIKeySchema(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()
	_, err := testutil.CreateEntityType(typeName, 1)
	require.NoError(t, err)

	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	require.NoError(t, err)

	clusterName := testutil.UniqueClusterName()
	_, err = testutil.CreateCluster(clusterName)
	require.NoError(t, err)

	createResp, err := testutil.GetClient().Post("/open-api/v1/api-keys", map[string]interface{}{
		"description": "schema test",
		"entity_id":   entityID,
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        1000000,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
		"rate_limit_policy": map[string]interface{}{
			"enabled": true,
			"rules": map[string]interface{}{
				"tpm": []interface{}{
					map[string]interface{}{"name": "tpm-1m", "model": "*", "window_minutes": 1, "max_tokens": 10000, "step_minutes": 1},
				},
				"rpm": []interface{}{
					map[string]interface{}{"name": "rpm-1m", "model": "*", "window_minutes": 1, "max_requests": 100},
				},
				"max_concurrency": 50,
			},
		},
		"route_rules": map[string]interface{}{
			"enabled": true,
			"rules": []interface{}{
				map[string]interface{}{
					"name":  "default",
					"cond":  "default_t()",
					"targets": []interface{}{
						map[string]interface{}{"cluster_name": clusterName, "model": "", "weight": 100},
					},
					"fallbacks": []interface{}{},
				},
			},
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)
	testutil.AssertSchema(t, createResp, APIKeySchema)

	id, err := testutil.GetDataField(createResp, "id")
	require.NoError(t, err)
	apiKeyID := id.(string)

	listResp, err := testutil.GetClient().Get("/open-api/v1/api-keys")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertPagedListSchema(t, listResp, APIKeyListItemSchema)

	oneResp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + apiKeyID)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, APIKeyListItemSchema)

	putResp, err := testutil.GetClient().Put("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"description": "schema test updated",
		"enabled":     true,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, putResp)
	testutil.AssertSchema(t, putResp, APIKeySchema)

	patchResp, err := testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"description": "schema test patched",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)
	testutil.AssertSchema(t, patchResp, APIKeySchema)

	qpResp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + apiKeyID + "/quota-plan")
	require.NoError(t, err)
	testutil.AssertSuccess(t, qpResp)
	testutil.AssertSchema(t, qpResp, QuotaPlanWithBalanceSchema)

	resetResp, err := testutil.GetClient().Post("/open-api/v1/api-keys/"+apiKeyID+"/quota-plan/reset", map[string]interface{}{})
	require.NoError(t, err)
	testutil.AssertSuccess(t, resetResp)
	testutil.AssertSchema(t, resetResp, QuotaResetResultSchema)

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
		testutil.DeleteEntity(entityID)
		testutil.DeleteEntityType(typeName)
		testutil.DeleteCluster(clusterName)
	})
}

// ---------- providers ----------

func testProviderSchema(t *testing.T) {
	providerName := testutil.UniqueProviderName()

	createResp, err := testutil.GetClient().Post("/open-api/v1/providers", map[string]interface{}{
		"name":        providerName,
		"description": "schema test",
		"instance_pool": []interface{}{
			map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080},
		},
		"model_protocols": []string{"openai"},
		"models":          []string{"deepseek-chat"},
		"time_zone":       "Asia/Shanghai",
		"tiers": []interface{}{
			map[string]interface{}{
				"name": "peak",
				"time_ranges": []interface{}{
					map[string]interface{}{
						"weekdays": []int{1, 2, 3, 4, 5},
						"start":    "09:00",
						"end":      "12:00",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)
	testutil.AssertSchema(t, createResp, ProviderSchema)
	testutil.AssertDataFieldEquals(t, createResp, "time_zone", "Asia/Shanghai")

	listResp, err := testutil.GetClient().Get("/open-api/v1/providers")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertPagedListSchema(t, listResp, ProviderSchema)

	oneResp, err := testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, ProviderSchema)

	patchResp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
		"description":     "schema test updated",
		"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
		"model_protocols": []string{"openai"},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)
	testutil.AssertSchema(t, patchResp, ProviderSchema)

	// Also validate anthropic protocol provider creation.
	anthropicProviderName := testutil.UniqueProviderName()
	anthropicResp, err := testutil.GetClient().Post("/open-api/v1/providers", map[string]interface{}{
		"name":            anthropicProviderName,
		"description":     "schema test anthropic",
		"model_protocols": []string{"anthropic"},
		"models":          []string{"claude-3-5-sonnet-20241022"},
		"instance_pool": []interface{}{
			map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080},
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, anthropicResp)
	testutil.AssertSchema(t, anthropicResp, ProviderSchema)
	assertProviderModelProtocols(t, anthropicResp.Data, []string{"anthropic"})

	t.Cleanup(func() {
		testutil.DeleteProvider(providerName)
		testutil.DeleteProvider(anthropicProviderName)
	})
}

// assertProviderModelProtocols 校验 Provider 响应中的 model_protocols 字段值。
func assertProviderModelProtocols(t *testing.T, data []byte, want []string) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal provider data: %v", err)
	}
	protocols, ok := payload["model_protocols"].([]interface{})
	if !ok || len(protocols) != len(want) {
		t.Fatalf("expected model_protocols=%v, got %v", want, payload["model_protocols"])
	}
	for i, v := range want {
		if protocols[i] != v {
			t.Fatalf("expected model_protocols[%d]=%q, got %v", i, v, protocols[i])
		}
	}
}

// ---------- clusters ----------

func testClusterSchema(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	_, err := testutil.CreateProvider(providerName)
	require.NoError(t, err)

	clusterName := testutil.UniqueClusterName()

	createResp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name":        clusterName,
		"description": "schema test",
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)
	testutil.AssertSchema(t, createResp, ClusterSchema)

	listResp, err := testutil.GetClient().Get("/open-api/v1/clusters")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertListSchema(t, listResp, ClusterSchema)

	oneResp, err := testutil.GetClient().Get("/open-api/v1/clusters/" + clusterName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, ClusterSchema)

	patchResp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
		"description": "schema test updated",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)
	testutil.AssertSchema(t, patchResp, ClusterSchema)

	// EPP 模式集群：balance_mode/epp_config 字段与嵌套结构校验。
	eppProviderName := testutil.UniqueProviderName()
	_, err = testutil.CreateProvider(eppProviderName)
	require.NoError(t, err)

	eppClusterName := testutil.UniqueClusterName()
	eppClusterBody := map[string]interface{}{
		"name":         eppClusterName,
		"description":  "schema test epp",
		"balance_mode": "EPP",
		"epp_config": map[string]interface{}{
			"scheduling_profile":       "latency-first",
			"cache_affinity":           "high",
			"prefix_cache_affinity":    true,
			"session_affinity_enabled": true,
			"session_affinity_header":  "x-session-id",
			"kv_cache_utilization_max": 0.85,
			"flow_control": map[string]interface{}{
				"max_requests":          200,
				"queue_ttl":             45,
				"no_endpoint_queue_ttl": 120,
				"enable_eviction":       true,
			},
		},
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": eppProviderName,
		},
	}
	eppCreateResp, err := testutil.GetClient().Post("/open-api/v1/clusters", eppClusterBody)
	require.NoError(t, err)
	testutil.AssertSuccess(t, eppCreateResp)
	testutil.AssertSchema(t, eppCreateResp, ClusterSchema)

	eppOneResp, err := testutil.GetClient().Get("/open-api/v1/clusters/" + eppClusterName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, eppOneResp)
	testutil.AssertSchema(t, eppOneResp, ClusterSchema)

	// epp_config 原样回读（存储保留用户原始 JSON，未携带字段不落盘）。
	assertEppConfigEcho(t, eppOneResp.Data, map[string]interface{}{
		"scheduling_profile":       "latency-first",
		"cache_affinity":           "high",
		"prefix_cache_affinity":    true,
		"session_affinity_enabled": true,
		"session_affinity_header":  "x-session-id",
		"kv_cache_utilization_max": 0.85,
		"flow_control": map[string]interface{}{
			"max_requests":          float64(200),
			"queue_ttl":             float64(45),
			"no_endpoint_queue_ttl": float64(120),
			"enable_eviction":       true,
		},
	})

	// WRR 集群携带 epp_config（休眠保留）：创建成功且 GET 原样返回。
	dormantClusterName := testutil.UniqueClusterName()
	dormantResp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name":         dormantClusterName,
		"balance_mode": "WRR",
		"epp_config":   map[string]interface{}{"scheduling_profile": "throughput-first"},
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": eppProviderName,
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, dormantResp)
	testutil.AssertSchema(t, dormantResp, ClusterSchema)

	dormantOneResp, err := testutil.GetClient().Get("/open-api/v1/clusters/" + dormantClusterName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, dormantOneResp)
	testutil.AssertSchema(t, dormantOneResp, ClusterSchema)
	assertEppConfigEcho(t, dormantOneResp.Data, map[string]interface{}{
		"scheduling_profile": "throughput-first",
	})

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
		testutil.DeleteCluster(eppClusterName)
		testutil.DeleteCluster(dormantClusterName)
		testutil.DeleteProvider(providerName)
		testutil.DeleteProvider(eppProviderName)
	})
}

// assertEppConfigEcho 校验 GET cluster 回读中 epp_config 与写入值一致（原样回读）。
func assertEppConfigEcho(t *testing.T, data []byte, want map[string]interface{}) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal cluster data: %v", err)
	}
	eppConfig, ok := payload["epp_config"].(map[string]interface{})
	if !ok {
		t.Fatalf("epp_config is not an object: %v", payload["epp_config"])
	}
	for key, wantVal := range want {
		require.Equal(t, wantVal, eppConfig[key], "epp_config.%s echo mismatch", key)
	}
}

// ---------- certificates ----------

func testCertificateSchema(t *testing.T) {
	certName := testutil.UniqueCertName()
	certPEM, keyPEM, err := testutil.GenerateTestCert(certName)
	require.NoError(t, err)

	createResp, err := testutil.GetClient().Post("/open-api/v1/certificates", map[string]interface{}{
		"cert_name":         certName,
		"description":       "schema test",
		"is_default":        true,
		"cert_file_content": certPEM,
		"key_file_content":  keyPEM,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)
	testutil.AssertSchema(t, createResp, CertificateSchema)

	listResp, err := testutil.GetClient().Get("/open-api/v1/certificates")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertListSchema(t, listResp, CertificateSchema)

	oneResp, err := testutil.GetClient().Get("/open-api/v1/certificates/" + certName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, CertificateSchema)

	defaultResp, err := testutil.GetClient().Patch("/open-api/v1/certificates/"+certName+"/default", map[string]interface{}{})
	require.NoError(t, err)
	testutil.AssertSuccess(t, defaultResp)
	testutil.AssertSchema(t, defaultResp, CertificateSchema)

	t.Cleanup(func() {
		// 默认证书不能直接删除，需先创建另一个默认证书再删
		// 这里简单处理：保留默认证书，测试中忽略删除错误
		_ = testutil.DeleteCertificate(certName)
	})
}

// ---------- auth ----------

func testAuthSchema(t *testing.T) {
	userName := testutil.UniqueUserName()
	password := "Password123!"

	createUserResp, err := testutil.GetClient().Post("/open-api/v1/auth/users", map[string]interface{}{
		"user_name": userName,
		"password":  password,
		"is_admin":  true,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createUserResp)

	listUsersResp, err := testutil.GetClient().Get("/open-api/v1/auth/users")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listUsersResp)
	testutil.AssertListSchema(t, listUsersResp, UserSchema)

	oneUserResp, err := testutil.GetClient().Get("/open-api/v1/auth/users/" + userName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneUserResp)
	testutil.AssertSchema(t, oneUserResp, UserSchema)

	sessionResp, err := testutil.GetClient().Post("/open-api/v1/auth/session-keys", map[string]interface{}{
		"user_name": userName,
		"password":  password,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, sessionResp)
	testutil.AssertSchema(t, sessionResp, SessionKeySchema)

	sessionKeyVal, err := testutil.GetDataField(sessionResp, "session_key")
	require.NoError(t, err)

	tokenName := testutil.UniqueTokenName()
	createTokenResp, err := testutil.GetClient().Post("/open-api/v1/auth/tokens", map[string]interface{}{
		"name":  tokenName,
		"scope": "System",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createTokenResp)
	testutil.AssertSchema(t, createTokenResp, CreateTokenResponseSchema)

	listTokensResp, err := testutil.GetClient().Get("/open-api/v1/auth/tokens")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listTokensResp)
	testutil.AssertListSchema(t, listTokensResp, TokenSchema)

	oneTokenResp, err := testutil.GetClient().Get("/open-api/v1/auth/tokens/" + tokenName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneTokenResp)
	testutil.AssertSchema(t, oneTokenResp, TokenSchema)

	metaResp, err := testutil.GetClient().Get("/open-api/v1/meta")
	require.NoError(t, err)
	testutil.AssertSuccess(t, metaResp)
	testutil.AssertSchema(t, metaResp, MetaSchema)

	t.Cleanup(func() {
		testutil.DeleteToken(tokenName)
		testutil.GetClient().Delete("/open-api/v1/auth/session-keys/" + sessionKeyVal.(string))
		testutil.DeleteUser(userName)
	})
}

// ---------- model-prices ----------

func testModelPriceSchema(t *testing.T) {
	schemaProvider := "schema-test-provider"
	schemaProvider2 := "schema-test-provider-2"
	_, err := testutil.CreateProvider(schemaProvider)
	require.NoError(t, err)
	_, err = testutil.CreateProvider(schemaProvider2)
	require.NoError(t, err)

	yamlContent := []byte(`version: v1.0
default_currency: RMB
models:
  - provider: schema-test-provider
    model: schema-test-model
    base_model: schema-test-model
    mode: chat
    capabilities: [chat]
    supported_parameters: [temperature]
    limits:
      context_window: 128000
    prices:
      input_cost_per_token: 0.000002
      output_cost_per_token: 0.000008
    tier_prices:
      peak:
        input_cost_per_token: 0.000004
        output_cost_per_token: 0.000016
    metadata:
      source: test
`)
	importResp, err := testutil.GetClient().PostMultipartFile("/open-api/v1/model-prices/import", "file", "model-list.yaml", yamlContent, map[string]string{
		"mode": "replace",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, importResp)
	testutil.AssertSchema(t, importResp, ModelPriceImportResultSchema)

	createResp, err := testutil.GetClient().Post("/open-api/v1/model-prices", map[string]interface{}{
		"provider":             schemaProvider2,
		"model":                "schema-test-model-2",
		"base_model":           "schema-test-model-2",
		"mode":                 "chat",
		"capabilities":         []string{"chat"},
		"supported_parameters": []string{"temperature"},
		"limits": map[string]interface{}{
			"context_window": 128000,
		},
		"prices": map[string]interface{}{
			"input_cost_per_token":  0.000002,
			"output_cost_per_token": 0.000008,
		},
		"tier_prices": map[string]interface{}{
			"peak": map[string]interface{}{
				"input_cost_per_token":  0.000004,
				"output_cost_per_token": 0.000016,
			},
		},
		"metadata": map[string]interface{}{
			"source": "test",
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)
	testutil.AssertSchema(t, createResp, ModelPriceSchema)

	id, err := testutil.GetDataField(createResp, "id")
	require.NoError(t, err)
	modelPriceID := int64(id.(float64))

	listResp, err := testutil.GetClient().Get("/open-api/v1/model-prices")
	require.NoError(t, err)
	testutil.AssertSuccess(t, listResp)
	testutil.AssertSchema(t, listResp, ModelPriceListResponseSchema)

	oneResp, err := testutil.GetClient().Get(fmt.Sprintf("/open-api/v1/model-prices/%d", modelPriceID))
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, ModelPriceSchema)

	updateResp, err := testutil.GetClient().Put(fmt.Sprintf("/open-api/v1/model-prices/%d", modelPriceID), map[string]interface{}{
		"provider":             schemaProvider2,
		"model":                "schema-test-model-2",
		"base_model":           "schema-test-model-2",
		"mode":                 "chat",
		"capabilities":         []string{"chat", "vision"},
		"supported_parameters": []string{"temperature"},
		"limits": map[string]interface{}{
			"context_window": 128000,
		},
		"prices": map[string]interface{}{
			"input_cost_per_token":  0.000003,
			"output_cost_per_token": 0.000009,
		},
		"tier_prices": map[string]interface{}{
			"peak": map[string]interface{}{
				"input_cost_per_token":  0.000006,
				"output_cost_per_token": 0.000018,
			},
		},
		"metadata": map[string]interface{}{
			"source": "test",
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, updateResp)
	testutil.AssertSchema(t, updateResp, ModelPriceSchema)

	getProvidersResp, err := testutil.GetClient().Get("/open-api/v1/model-prices/actions/get-providers")
	require.NoError(t, err)
	testutil.AssertSuccess(t, getProvidersResp)
	testutil.AssertSchema(t, getProvidersResp, ModelPriceGetProvidersResponseSchema)

	t.Cleanup(func() {
		testutil.DeleteModelPrice(modelPriceID)
		testutil.DeleteModelPriceByQuery(schemaProvider, "schema-test-model", "chat")
		testutil.DeleteProvider(schemaProvider)
		testutil.DeleteProvider(schemaProvider2)
	})
}

// ---------- route-tables ----------

func testRouteTableSchema(t *testing.T) {
	resp, err := testutil.GetClient().Get("/open-api/v1/route-tables")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	testutil.AssertPagedListSchema(t, resp, RouteTableSchema)
}

// ---------- global-route-rules ----------

func testGlobalRouteRulesSchema(t *testing.T) {
	resp, err := testutil.GetClient().Get("/open-api/v1/global-route-rules")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	testutil.AssertSchema(t, resp, GlobalRouteRulesSchema)

	putResp, err := testutil.GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
		"enabled": true,
		"rules":   []interface{}{},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, putResp)
	testutil.AssertSchema(t, putResp, GlobalRouteRulesSchema)
}



// ---------- epp-pool ----------

func testEppPoolSchema(t *testing.T) {
	// EPP 池为单例：本用例 PATCH 自己的组布局后回读。
	patchResp, err := testutil.GetClient().Patch("/open-api/v1/epp-pool", map[string]interface{}{
		"groups": []interface{}{
			map[string]interface{}{
				"name": "g1",
				"instances": []interface{}{
					map[string]interface{}{"id": "epp-a", "host": "10.0.0.1", "port": 9002},
					map[string]interface{}{"id": "epp-b", "host": "10.0.0.2", "port": 9002},
				},
			},
			map[string]interface{}{
				"name": "g2",
				"instances": []interface{}{
					map[string]interface{}{"id": "epp-c", "host": "10.0.0.3", "port": 9002},
				},
			},
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)
	testutil.AssertSchema(t, patchResp, EppPoolSchema)

	getResp, err := testutil.GetClient().Get("/open-api/v1/epp-pool")
	require.NoError(t, err)
	testutil.AssertSuccess(t, getResp)
	testutil.AssertSchema(t, getResp, EppPoolSchema)

	// 定向断言：name 为配置的单例池名，组与实例字段完整回读。
	nameVal, err := testutil.GetDataField(getResp, "name")
	require.NoError(t, err)
	require.Equal(t, "EPP.pool", nameVal)
	groupsVal, err := testutil.GetDataField(getResp, "groups")
	require.NoError(t, err)
	groups := groupsVal.([]interface{})
	require.Len(t, groups, 2)
	g1 := groups[0].(map[string]interface{})
	require.Equal(t, "g1", g1["name"])
	insts := g1["instances"].([]interface{})
	require.Len(t, insts, 2)
	inst0 := insts[0].(map[string]interface{})
	require.Equal(t, "epp-a", inst0["id"])
	require.Equal(t, "10.0.0.1", inst0["host"])
	require.Equal(t, float64(9002), inst0["port"])
}

// ---------- epp-assignments ----------

func testEppAssignmentsSchema(t *testing.T) {
	// PATCH 自己的组布局（EPP 池单例，不依赖其他用例的池状态）。
	patchResp, err := testutil.GetClient().Patch("/open-api/v1/epp-pool", map[string]interface{}{
		"groups": []interface{}{
			map[string]interface{}{
				"name": "g1",
				"instances": []interface{}{
					map[string]interface{}{"id": "epp-a", "host": "10.0.0.1", "port": 9002},
					map[string]interface{}{"id": "epp-b", "host": "10.0.0.2", "port": 9002},
				},
			},
			map[string]interface{}{
				"name": "g2",
				"instances": []interface{}{
					map[string]interface{}{"id": "epp-c", "host": "10.0.0.3", "port": 9002},
				},
			},
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, patchResp)

	providerName := testutil.UniqueProviderName()
	_, err = testutil.CreateProvider(providerName)
	require.NoError(t, err)

	clusterName := testutil.UniqueClusterName()
	createResp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name":         clusterName,
		"balance_mode": "EPP",
		"epp_config":   map[string]interface{}{"scheduling_profile": "balanced"},
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
		testutil.DeleteProvider(providerName)
	})

	// 全量视图：结构 + 定向断言（该 cluster 已自动分配、g2 空闲）。
	viewResp, err := testutil.GetClient().Get("/open-api/v1/epp-assignments")
	require.NoError(t, err)
	testutil.AssertSuccess(t, viewResp)
	testutil.AssertSchema(t, viewResp, EppAssignmentsViewSchema)

	assertAssignmentEntry(t, viewResp.Data, clusterName, false)

	idleVal, err := testutil.GetDataField(viewResp, "idle_groups")
	require.NoError(t, err)
	require.Contains(t, idleVal, "g2")
	unassignedVal, err := testutil.GetDataField(viewResp, "unassigned_clusters")
	require.NoError(t, err)
	require.NotContains(t, unassignedVal, clusterName)

	// 单条过滤查询：同结构 schema。
	oneResp, err := testutil.GetClient().Get("/open-api/v1/epp-assignments", map[string]string{
		"cluster": clusterName,
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, oneResp)
	testutil.AssertSchema(t, oneResp, EppAssignmentsViewSchema)
	assertAssignmentEntry(t, oneResp.Data, clusterName, false)

	// 手工覆写后单条响应结构（standby 换为另一实例）。
	entry := findAssignmentEntry(t, oneResp.Data, clusterName)
	standby := entry["standby"].(map[string]interface{})
	putResp, err := testutil.GetClient().Put("/open-api/v1/epp-assignments/"+clusterName, map[string]interface{}{
		"group_name":          "g1",
		"primary_instance_id": standby["id"],
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, putResp)
	testutil.AssertSchema(t, putResp, EppClusterAssignmentOverrideSchema)

	putPrimary, err := testutil.GetDataField(putResp, "primary")
	require.NoError(t, err)
	require.Equal(t, standby["id"], putPrimary.(map[string]interface{})["id"])
}

// findAssignmentEntry 从全量视图中取出指定 cluster 的分配条目。
func findAssignmentEntry(t *testing.T, data []byte, clusterName string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal assignments data: %v", err)
	}
	clusters := payload["clusters"].([]interface{})
	for _, item := range clusters {
		entry := item.(map[string]interface{})
		if entry["cluster"] == clusterName {
			return entry
		}
	}
	t.Fatalf("cluster %s not found in assignments view", clusterName)
	return nil
}

// assertAssignmentEntry 定向断言指定 cluster 已分配（group/primary/standby 非空）。
func assertAssignmentEntry(t *testing.T, data []byte, clusterName string, unassigned bool) {
	t.Helper()
	entry := findAssignmentEntry(t, data, clusterName)
	require.Equal(t, unassigned, entry["degraded"].(bool))
	if unassigned {
		require.Nil(t, entry["primary"])
		return
	}
	require.NotNil(t, entry["group"])
	primary, ok := entry["primary"].(map[string]interface{})
	require.True(t, ok, "primary should be an object")
	require.NotEmpty(t, primary["id"])
	require.NotEmpty(t, primary["host"])
	require.NotNil(t, entry["standby"])
}
