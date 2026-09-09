package innerapi

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

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

func TestInnerAPI_Schema(t *testing.T) {
	t.Run("server_data_conf", testServerDataConfSchema)
	t.Run("server_data_conf_tiered_pricing", testServerDataConfTieredPricingSchema)
	t.Run("gslb", testGSLBSchema)
	t.Run("cluster_table", testClusterTableSchema)
	t.Run("server_cert_conf", testServerCertConfSchema)
	t.Run("mod_api_key", testModAPIKeySchema)
	t.Run("mod_body_process", testModBodyProcessSchema)
	t.Run("rate_limit_policy", testRateLimitPolicySchema)
	t.Run("ai_route", testAIRouteSchema)
	t.Run("epp_data", testEppDataSchema)
	t.Run("server_data_conf_epp", testServerDataConfEppSchema)
}

// setupCluster 创建一个测试集群并返回名称
func setupCluster(t *testing.T) string {
	clusterName := testutil.UniqueClusterName()
	_, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": "deepseek",
		},
	})
	require.NoError(t, err)
	return clusterName
}

// setupAPIKeyWithRoute 创建一个启用 route_rules 的 API-Key
func setupAPIKeyWithRoute(t *testing.T, clusterName string) string {
	resp, err := testutil.GetClient().Post("/open-api/v1/api-keys", map[string]interface{}{
		"description": "inner-schema-route",
		"quota_plan": map[string]interface{}{
			"unlimited": false,
			"quota":     1000,
			"unit":      "total_token",
		},
		"route_rules": map[string]interface{}{
			"enabled": true,
			"rules": []interface{}{
				map[string]interface{}{
					"name":    "default",
					"cond":    "default_t()",
					"targets": []interface{}{map[string]interface{}{"cluster_name": clusterName, "model": "", "weight": 100}},
					"fallbacks": []interface{}{},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, resp.ErrNum, resp.ErrMsg)
	id, err := testutil.GetDataField(resp, "id")
	require.NoError(t, err)
	return id.(string)
}

func testServerDataConfSchema(t *testing.T) {
	clusterName := setupCluster(t)

	// Create an anthropic provider and cluster to verify AIConf.ModelProtocols export.
	anthropicProviderName := testutil.UniqueProviderName()
	_, err := testutil.CreateProvider(anthropicProviderName, map[string]interface{}{
		"model_protocols": []string{"anthropic"},
		"models":          []string{"claude-3-5-sonnet-20241022"},
	})
	require.NoError(t, err)

	anthropicClusterName := testutil.UniqueClusterName()
	_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": anthropicClusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"claude-3-5-sonnet-20241022"},
			"provider": anthropicProviderName,
		},
	})
	require.NoError(t, err)

	// Create a cluster with key_affinity to verify AIConf.KeyPolicy.SessionAffinity* export.
	affinityProviderName := testutil.UniqueProviderName()
	_, err = testutil.CreateProvider(affinityProviderName, map[string]interface{}{
		"models": []string{"deepseek-chat"},
	})
	require.NoError(t, err)

	affinityClusterName := testutil.UniqueClusterName()
	_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": affinityClusterName,
		"llm_config": map[string]interface{}{
			"models": []string{"deepseek-chat"},
			"key_affinity": map[string]interface{}{
				"enabled":        true,
				"ttl":            600,
				"redis_prefix":   "bfe:ai:key_affinity",
				"penalty_enable": true,
			},
			"provider": affinityProviderName,
		},
	})
	require.NoError(t, err)

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, ServerDataConfSchema)
		assertServerDataConfModelProtocols(t, resp.Data, anthropicClusterName)
		assertServerDataConfKeyAffinity(t, resp.Data, affinityClusterName)
	}

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
		testutil.DeleteCluster(anthropicClusterName)
		testutil.DeleteCluster(affinityClusterName)
		testutil.DeleteProvider(anthropicProviderName)
		testutil.DeleteProvider(affinityProviderName)
	})
}

// assertServerDataConfKeyAffinity 校验导出结果中指定 cluster 的 AIConf.KeyPolicy.SessionAffinity*。
func assertServerDataConfKeyAffinity(t *testing.T, data []byte, clusterName string) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal server_data_conf data: %v", err)
	}
	clusterConf, ok := payload["ClusterConf"].(map[string]interface{})
	if !ok {
		t.Fatal("ClusterConf is not an object")
	}
	config, ok := clusterConf["Config"].(map[string]interface{})
	if !ok {
		t.Fatal("ClusterConf.Config is not an object")
	}
	cluster, ok := config[clusterName].(map[string]interface{})
	if !ok {
		t.Fatalf("cluster %s not found in ClusterConf.Config", clusterName)
	}
	aiconf, ok := cluster["AIConf"].(map[string]interface{})
	if !ok {
		t.Fatalf("AIConf not found for cluster %s", clusterName)
	}
	keyPolicy, ok := aiconf["KeyPolicy"].(map[string]interface{})
	if !ok {
		t.Fatalf("AIConf.KeyPolicy is not an object for cluster %s", clusterName)
	}
	assert.Equal(t, true, keyPolicy["SessionAffinity"])
	assert.Equal(t, float64(600), keyPolicy["SessionAffinityTTL"])
	assert.Equal(t, "bfe:ai:key_affinity", keyPolicy["SessionAffinityRedisPrefix"])
	assert.Equal(t, true, keyPolicy["SessionAffinityPenaltyEnable"])
}

// assertServerDataConfModelProtocols 校验导出结果中指定 cluster 的 AIConf.ModelProtocols。
func assertServerDataConfModelProtocols(t *testing.T, data []byte, clusterName string) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal server_data_conf data: %v", err)
	}
	clusterConf, ok := payload["ClusterConf"].(map[string]interface{})
	if !ok {
		t.Fatal("ClusterConf is not an object")
	}
	config, ok := clusterConf["Config"].(map[string]interface{})
	if !ok {
		t.Fatal("ClusterConf.Config is not an object")
	}
	cluster, ok := config[clusterName].(map[string]interface{})
	if !ok {
		t.Fatalf("cluster %s not found in ClusterConf.Config", clusterName)
	}
	aiconf, ok := cluster["AIConf"].(map[string]interface{})
	if !ok {
		t.Fatalf("AIConf not found for cluster %s", clusterName)
	}
	modelProtocols, ok := aiconf["ModelProtocols"].([]interface{})
	if !ok || len(modelProtocols) != 1 {
		t.Fatalf("expected AIConf.ModelProtocols=[anthropic] for cluster %s, got %v", clusterName, aiconf["ModelProtocols"])
	}
	if modelProtocols[0] != "anthropic" {
		t.Fatalf("expected ModelProtocols[0]=anthropic for cluster %s, got %v", clusterName, modelProtocols[0])
	}
}

func testServerDataConfTieredPricingSchema(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	_, err := testutil.CreateProvider(providerName)
	require.NoError(t, err)

	tierResp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
		"time_zone": "Asia/Shanghai",
		"tiers": []interface{}{
			map[string]interface{}{
				"name": "peak",
				"time_ranges": []interface{}{
					map[string]interface{}{"weekdays": []int{1, 2, 3, 4, 5}, "start": "09:00", "end": "12:00"},
					map[string]interface{}{"weekdays": []int{1, 2, 3, 4, 5}, "start": "14:00", "end": "18:00"},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, tierResp.ErrNum, tierResp.ErrMsg)

	yamlContent := []byte(`version: v1.0
default_currency: RMB
models:
  - provider: ` + providerName + `
    model: deepseek-chat
    base_model: deepseek-chat
    mode: chat
    prices:
      input_cost_per_token: 0.000002
      output_cost_per_token: 0.000008
      cache_read_input_token_cost: 0.0000005
    tier_prices:
      peak:
        input_cost_per_token: 0.000004
        output_cost_per_token: 0.000016
        cache_read_input_token_cost: 0.000001
`)
	err = testutil.ImportModelPrices(yamlContent, "replace")
	require.NoError(t, err)

	clusterName := testutil.UniqueClusterName()
	_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	require.NoError(t, err)

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, ServerDataConfSchema)
		assertServerDataConfTieredPricing(t, resp.Data, clusterName)
	}

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
		testutil.DeleteModelPriceByQuery(providerName, "deepseek-chat", "chat")
		testutil.DeleteProvider(providerName)
	})
}

// assertServerDataConfTieredPricing 校验导出结果中指定 cluster 的 AIConf.ModelTable 携带分时段定价。
func assertServerDataConfTieredPricing(t *testing.T, data []byte, clusterName string) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal server_data_conf data: %v", err)
	}
	clusterConf, ok := payload["ClusterConf"].(map[string]interface{})
	if !ok {
		t.Fatal("ClusterConf is not an object")
	}
	config, ok := clusterConf["Config"].(map[string]interface{})
	if !ok {
		t.Fatal("ClusterConf.Config is not an object")
	}
	cluster, ok := config[clusterName].(map[string]interface{})
	if !ok {
		t.Fatalf("cluster %s not found in ClusterConf.Config", clusterName)
	}
	aiconf, ok := cluster["AIConf"].(map[string]interface{})
	if !ok {
		t.Fatalf("AIConf not found for cluster %s", clusterName)
	}
	modelTable, ok := aiconf["ModelTable"].(map[string]interface{})
	if !ok {
		t.Fatalf("AIConf.ModelTable is not an object for cluster %s", clusterName)
	}

	modelTableJSON, err := json.Marshal(modelTable)
	if err != nil {
		t.Fatalf("marshal model table: %v", err)
	}
	testutil.AssertSchema(t, &testutil.APIResponse{Data: modelTableJSON}, ModelTableSchema)

	assert.Equal(t, "RMB", modelTable["Currency"])
	assert.Equal(t, "Asia/Shanghai", modelTable["TimeZone"])

	tiers, ok := modelTable["Tiers"].([]interface{})
	require.True(t, ok, "ModelTable.Tiers should be an array")
	require.Len(t, tiers, 1)
	tier0, _ := tiers[0].(map[string]interface{})
	assert.Equal(t, "peak", tier0["Name"])

	models, ok := modelTable["Models"].([]interface{})
	require.True(t, ok, "ModelTable.Models should be an array")
	require.Len(t, models, 1)
	model0, _ := models[0].(map[string]interface{})
	assert.Equal(t, "deepseek-chat", model0["Model"])

	prices, ok := model0["Prices"].(map[string]interface{})
	require.True(t, ok, "ModelPrice.Prices should be an object")
	assert.Equal(t, 0.000002, prices["input_cost_per_token"])

	tierPrices, ok := model0["TierPrices"].(map[string]interface{})
	require.True(t, ok, "ModelPrice.TierPrices should be an object")
	peak, ok := tierPrices["peak"].(map[string]interface{})
	require.True(t, ok, "TierPrices.peak should be an object")
	assert.Equal(t, 0.000004, peak["input_cost_per_token"])
	assert.Equal(t, 0.000016, peak["output_cost_per_token"])
	assert.Equal(t, 0.000001, peak["cache_read_input_token_cost"])
}

func testGSLBSchema(t *testing.T) {
	clusterName := setupCluster(t)

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/gslb_data/gslb", map[string]string{
		"bfe_cluster": "BFE-AI_product.szyf",
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, GSLBSchema)
	}

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
	})
}

func testClusterTableSchema(t *testing.T) {
	clusterName := setupCluster(t)

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/gslb_data/cluster_table")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, ClusterTableSchema)
	}

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
	})
}

func testServerCertConfSchema(t *testing.T) {
	certName := testutil.UniqueCertName()
	certPEM, keyPEM, err := testutil.GenerateTestCert(certName)
	require.NoError(t, err)

	_, err = testutil.GetClient().Post("/open-api/v1/certificates", map[string]interface{}{
		"cert_name":         certName,
		"description":       "schema test",
		"is_default":        true,
		"cert_file_content": certPEM,
		"key_file_content":  keyPEM,
	})
	require.NoError(t, err)

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/protocol/server_cert_conf")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, ServerCertConfSchema)
	}

	t.Cleanup(func() {
		_ = testutil.DeleteCertificate(certName)
	})
}

func testModAPIKeySchema(t *testing.T) {
	resp, err := testutil.GetClient().Post("/open-api/v1/api-keys", map[string]interface{}{
		"description": "inner-schema-mod-api-key",
		"quota_plan": map[string]interface{}{
			"unlimited": false,
			"quota":     1000,
			"unit":      "total_token",
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, resp.ErrNum, resp.ErrMsg)
	id, err := testutil.GetDataField(resp, "id")
	require.NoError(t, err)
	apiKeyID := id.(string)

	innerResp, err := testutil.GetClient().Get("/inner-api/v1/configs/mod-api-key")
	require.NoError(t, err)
	testutil.AssertSuccess(t, innerResp)
	if innerResp.Data != nil && string(innerResp.Data) != "null" {
		testutil.AssertSchema(t, innerResp, ModAPIKeySchema)
		assertModAPIKeyFieldDetails(t, innerResp.Data)
	}

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
	})
}

// assertModAPIKeyFieldDetails 校验 /configs/mod-api-key 响应中 tokens 和 QuotaPlans 的字段细节：
// token 的 enabled 为 bool，不包含 status/update_time；QuotaPlan 不包含 CreateTime/ResetMode；Tags 包含 TagLevel。
func assertModAPIKeyFieldDetails(t *testing.T, data []byte) {
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal mod-api-key data: %v", err)
	}

	tokens, ok := payload["tokens"].(map[string]interface{})
	if !ok {
		t.Fatal("tokens is not an object")
	}
	for product, productTokens := range tokens {
		productTokenMap, ok := productTokens.(map[string]interface{})
		if !ok {
			t.Fatalf("tokens.%s is not an object", product)
		}
		for key, token := range productTokenMap {
			tokenMap, ok := token.(map[string]interface{})
			if !ok {
				t.Fatalf("tokens.%s.%s is not an object", product, key)
			}
			if _, ok := tokenMap["status"]; ok {
				t.Errorf("tokens.%s.%s should not contain status", product, key)
			}
			if _, ok := tokenMap["update_time"]; ok {
				t.Errorf("tokens.%s.%s should not contain update_time", product, key)
			}
			if enabled, ok := tokenMap["enabled"].(bool); !ok {
				t.Errorf("tokens.%s.%s.enabled should be bool", product, key)
			} else if !enabled {
				t.Errorf("tokens.%s.%s.enabled should be true", product, key)
			}
			if keyID, ok := tokenMap["key_id"].(string); !ok || keyID == "" {
				t.Errorf("tokens.%s.%s.key_id should be non-empty string", product, key)
			}
			if expiredTime, ok := tokenMap["expired_time"].(float64); !ok {
				t.Errorf("tokens.%s.%s.expired_time should be number", product, key)
			} else if expiredTime != -1 {
				// ok, can be -1 or a timestamp
			}
			if _, ok := tokenMap["unlimited_quota"].(bool); !ok {
				t.Errorf("tokens.%s.%s.unlimited_quota should be bool", product, key)
			}
			if models, ok := tokenMap["allow_models"].(string); ok && models == "*" {
				t.Errorf("tokens.%s.%s.allow_models should not be '*', got %q", product, key, models)
			}
			if quotaPlans, ok := tokenMap["quota_plans"].([]interface{}); ok {
				for i, qp := range quotaPlans {
					if _, ok := qp.(string); !ok {
						t.Errorf("tokens.%s.%s.quota_plans[%d] should be string", product, key, i)
					}
				}
			}
			if tags, ok := tokenMap["Tags"].([]interface{}); ok {
				for i, tag := range tags {
					tagMap, ok := tag.(map[string]interface{})
					if !ok {
						t.Fatalf("tokens.%s.%s.Tags[%d] is not an object", product, key, i)
					}
					if _, ok := tagMap["TagName"].(string); !ok {
						t.Errorf("tokens.%s.%s.Tags[%d].TagName should be string", product, key, i)
					}
					if _, ok := tagMap["TagValue"].(string); !ok {
						t.Errorf("tokens.%s.%s.Tags[%d].TagValue should be string", product, key, i)
					}
					if _, ok := tagMap["TagLevel"].(float64); !ok {
						t.Errorf("tokens.%s.%s.Tags[%d].TagLevel should be int", product, key, i)
					}
				}
			}
		}
	}

	quotaPlans, ok := payload["QuotaPlans"].(map[string]interface{})
	if !ok {
		t.Fatal("QuotaPlans is not an object")
	}
	for product, plans := range quotaPlans {
		planList, ok := plans.([]interface{})
		if !ok {
			t.Fatalf("QuotaPlans.%s is not an array", product)
		}
		for i, plan := range planList {
			planMap, ok := plan.(map[string]interface{})
			if !ok {
				t.Fatalf("QuotaPlans.%s[%d] is not an object", product, i)
			}
			if _, ok := planMap["CreateTime"]; ok {
				t.Errorf("QuotaPlans.%s[%d] should not contain CreateTime", product, i)
			}
			if _, ok := planMap["ResetMode"]; ok {
				t.Errorf("QuotaPlans.%s[%d] should not contain ResetMode", product, i)
			}
			if id, ok := planMap["Id"].(string); !ok || id == "" {
				t.Errorf("QuotaPlans.%s[%d].Id should be non-empty string", product, i)
			}
			if _, ok := planMap["Unlimited"].(bool); !ok {
				t.Errorf("QuotaPlans.%s[%d].Unlimited should be bool", product, i)
			}
			if _, ok := planMap["PassNoQuota"].(bool); !ok {
				t.Errorf("QuotaPlans.%s[%d].PassNoQuota should be bool", product, i)
			}
			if redisKey, ok := planMap["RedisKey"].(string); !ok || redisKey == "" {
				t.Errorf("QuotaPlans.%s[%d].RedisKey should be non-empty string", product, i)
			}
			if _, ok := planMap["ExpiredTime"].(float64); !ok {
				t.Errorf("QuotaPlans.%s[%d].ExpiredTime should be number", product, i)
			}
			if _, ok := planMap["Quota"].(float64); !ok {
				t.Errorf("QuotaPlans.%s[%d].Quota should be number", product, i)
			}
		}
	}
}

func testModBodyProcessSchema(t *testing.T) {
	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/mod-body-process")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, ModBodyProcessSchema)
	}
}

func testRateLimitPolicySchema(t *testing.T) {
	resp, err := testutil.GetClient().Post("/open-api/v1/api-keys", map[string]interface{}{
		"description": "inner-schema-rate-limit",
		"rate_limit_policy": map[string]interface{}{
			"enabled": true,
			"rules": map[string]interface{}{
				"tpm": []interface{}{
					map[string]interface{}{"name": "tpm-1m", "model": "*", "window_minutes": 1, "max_tokens": 10000, "step_minutes": 1},
				},
				"rpm": []interface{}{
					map[string]interface{}{"name": "rpm-1m", "model": "*", "window_minutes": 1, "max_requests": 10},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, resp.ErrNum, resp.ErrMsg)
	id, err := testutil.GetDataField(resp, "id")
	require.NoError(t, err)
	apiKeyID := id.(string)

	innerResp, err := testutil.GetClient().Get("/inner-api/v1/configs/rate-limit-policy")
	require.NoError(t, err)
	testutil.AssertSuccess(t, innerResp)
	if innerResp.Data != nil && string(innerResp.Data) != "null" {
		testutil.AssertSchema(t, innerResp, RateLimitPolicySchema)
		assertRateLimitPolicyFieldDetails(t, innerResp.Data)
	}

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
	})
}

// assertRateLimitPolicyFieldDetails 校验 /configs/rate-limit-policy 中每条 TPM/RPM 规则都包含非空的 redis_key。
func assertRateLimitPolicyFieldDetails(t *testing.T, data []byte) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal rate-limit-policy data: %v", err)
	}

	policies, ok := payload["RateLimitPolicies"].(map[string]interface{})
	if !ok {
		t.Fatal("RateLimitPolicies is not an object")
	}
	if len(policies) == 0 {
		t.Fatal("RateLimitPolicies is empty")
	}

	for policyKey, policy := range policies {
		policyMap, ok := policy.(map[string]interface{})
		if !ok {
			t.Fatalf("RateLimitPolicies.%s is not an object", policyKey)
		}
		if name, ok := policyMap["name"].(string); !ok || name == "" {
			t.Errorf("RateLimitPolicies.%s.name should be non-empty string", policyKey)
		}
		if enabled, ok := policyMap["enabled"].(bool); !ok {
			t.Errorf("RateLimitPolicies.%s.enabled should be bool", policyKey)
		} else if !enabled {
			t.Errorf("RateLimitPolicies.%s.enabled should be true", policyKey)
		}

		rules, ok := policyMap["rules"].(map[string]interface{})
		if !ok {
			t.Fatalf("RateLimitPolicies.%s.rules is not an object", policyKey)
		}

		assertRuleRedisKey(t, policyKey, "tpm", rules)
		assertRuleRedisKey(t, policyKey, "rpm", rules)
	}
}

func assertRuleRedisKey(t *testing.T, policyKey, ruleType string, rules map[string]interface{}) {
	ruleList, ok := rules[ruleType].([]interface{})
	if !ok {
		t.Fatalf("RateLimitPolicies.%s.rules.%s is not an array", policyKey, ruleType)
	}
	for i, item := range ruleList {
		rule, ok := item.(map[string]interface{})
		if !ok {
			t.Fatalf("RateLimitPolicies.%s.rules.%s[%d] is not an object", policyKey, ruleType, i)
		}
		redisKey, ok := rule["redis_key"].(string)
		if !ok || redisKey == "" {
			t.Errorf("RateLimitPolicies.%s.rules.%s[%d].redis_key should be non-empty string", policyKey, ruleType, i)
			continue
		}
		// redis_key 可能包含产品线/集群前缀（如 default_bfe_<policy>_RL_...），
		// 因此校验关键片段而非严格前缀。
		wantInfix := "RL_" + strings.ToUpper(ruleType) + "_" + policyKey + "_"
		if !strings.Contains(redisKey, wantInfix) {
			t.Errorf("RateLimitPolicies.%s.rules.%s[%d].redis_key format mismatch: got %s, want contain %s",
				policyKey, ruleType, i, redisKey, wantInfix)
		}
	}
}

func testAIRouteSchema(t *testing.T) {
	// 设置 global route
	_, err := testutil.GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
		"enabled": true,
		"rules": []interface{}{
			map[string]interface{}{
				"name":      "global-default",
				"cond":      "default_t()",
				"targets":   []interface{}{map[string]interface{}{"cluster_name": "cluster_global", "model": "", "weight": 100}},
				"fallbacks": []interface{}{},
			},
		},
	})
	require.NoError(t, err)

	clusterName := setupCluster(t)
	apiKeyID := setupAPIKeyWithRoute(t, clusterName)

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/ai-route")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	if resp.Data != nil && string(resp.Data) != "null" {
		testutil.AssertSchema(t, resp, AIRouteSchema)
	}

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
		testutil.DeleteCluster(clusterName)
	})
}

// ---------- epp_data ----------

// patchEppPoolForSchema 为 EPP schema 用例配置独立的实例池布局（EPP 池为单例）。
func patchEppPoolForSchema(t *testing.T) {
	resp, err := testutil.GetClient().Patch("/open-api/v1/epp-pool", map[string]interface{}{
		"groups": []interface{}{
			map[string]interface{}{
				"name": "g1",
				"instances": []interface{}{
					map[string]interface{}{"id": "epp-a", "host": "10.0.0.1", "port": 9002},
					map[string]interface{}{"id": "epp-b", "host": "10.0.0.2", "port": 9002},
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, resp.ErrNum, resp.ErrMsg)
}

// createEPPClusterForSchema 创建一个 EPP 模式集群（自带 provider），返回名称。
func createEPPClusterForSchema(t *testing.T, eppConfig map[string]interface{}) string {
	t.Helper()
	providerName := testutil.UniqueProviderName()
	_, err := testutil.CreateProvider(providerName)
	require.NoError(t, err)

	clusterName := testutil.UniqueClusterName()
	resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name":         clusterName,
		"balance_mode": "EPP",
		"epp_config":   eppConfig,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	require.NoError(t, err)
	require.Equal(t, 200, resp.ErrNum, resp.ErrMsg)
	return clusterName
}

func testEppDataSchema(t *testing.T) {
	patchEppPoolForSchema(t)

	clusterName := createEPPClusterForSchema(t, map[string]interface{}{
		"scheduling_profile": "throughput-first",
		"flow_control": map[string]interface{}{
			"max_requests": 100,
			"queue_ttl":    30,
		},
	})
	t.Cleanup(func() { testutil.DeleteCluster(clusterName) })

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/epp_data/config")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	testutil.AssertSchema(t, resp, EppDataSchema)

	// 定向断言（epp-data.md §3.2/§3.3）：
	// assignment 含该 cluster 条目且 primary 非空；epp_config 编译产物关键字段存在。
	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Data, &payload))
	config := payload["Config"].(map[string]interface{})

	assignment := config["assignment"].(map[string]interface{})
	entry, ok := assignment[clusterName].(map[string]interface{})
	require.True(t, ok, "assignment should contain cluster %s", clusterName)
	primary, ok := entry["primary"].(string)
	require.True(t, ok, "assignment.primary should be a string")
	require.NotEmpty(t, primary)

	eppConfig := config["epp_config"].(map[string]interface{})
	compiled, ok := eppConfig[clusterName].(map[string]interface{})
	require.True(t, ok, "epp_config should contain cluster %s", clusterName)
	plugins, ok := compiled["plugins"].([]interface{})
	require.True(t, ok, "compiled epp_config.plugins should be an array")
	require.NotEmpty(t, plugins)
	require.Contains(t, compiled, "featureGates")
	require.Contains(t, compiled, "schedulingProfiles")
	require.Contains(t, compiled, "dataLayer")
}

// ---------- server_data_conf EPP 导出 ----------

func testServerDataConfEppSchema(t *testing.T) {
	patchEppPoolForSchema(t)

	eppClusterName := createEPPClusterForSchema(t, map[string]interface{}{
		"scheduling_profile": "balanced",
	})
	wrrClusterName := testutil.UniqueClusterName()
	{
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName)
		require.NoError(t, err)
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": wrrClusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		})
		require.NoError(t, err)
		require.Equal(t, 200, resp.ErrNum, resp.ErrMsg)
		t.Cleanup(func() { testutil.DeleteProvider(providerName) })
	}
	t.Cleanup(func() {
		testutil.DeleteCluster(eppClusterName)
		testutil.DeleteCluster(wrrClusterName)
	})

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	testutil.AssertSchema(t, resp, ServerDataConfSchema)

	// EPP cluster：BalanceMode=EPP 且 EPPAddr 有序 [主, 备]（server-data-conf.md §3.3）。
	assignResp, err := testutil.GetClient().Get("/open-api/v1/epp-assignments", map[string]string{
		"cluster": eppClusterName,
	})
	require.NoError(t, err)
	require.Equal(t, 200, assignResp.ErrNum, assignResp.ErrMsg)
	var assignView map[string]interface{}
	require.NoError(t, json.Unmarshal(assignResp.Data, &assignView))
	entries := assignView["clusters"].([]interface{})
	entry := entries[0].(map[string]interface{})
	primary := entry["primary"].(map[string]interface{})
	standby := entry["standby"].(map[string]interface{})
	expectedAddrs := []interface{}{
		fmt.Sprintf("%s:%v", primary["host"], int(primary["port"].(float64))),
		fmt.Sprintf("%s:%v", standby["host"], int(standby["port"].(float64))),
	}
	assertServerDataConfEpp(t, resp.Data, eppClusterName, "EPP", expectedAddrs)

	// WRR cluster：BalanceMode=WRR 且无 EPPAddr。
	assertServerDataConfEpp(t, resp.Data, wrrClusterName, "WRR", nil)
}

// assertServerDataConfEpp 校验导出结果中指定 cluster 的 GslbBasic.BalanceMode / EPPAddr
// （server-data-conf.md §3.3：EPPAddr 有序 [主, 备]，WRR 时为 null）。
func assertServerDataConfEpp(t *testing.T, data []byte, clusterName, wantMode string, wantEPPAddr []interface{}) {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal server_data_conf data: %v", err)
	}
	clusterConf := payload["ClusterConf"].(map[string]interface{})
	config := clusterConf["Config"].(map[string]interface{})
	cluster, ok := config[clusterName].(map[string]interface{})
	if !ok {
		t.Fatalf("cluster %s not found in ClusterConf.Config", clusterName)
	}
	gslbBasic, ok := cluster["GslbBasic"].(map[string]interface{})
	if !ok {
		t.Fatalf("GslbBasic not found for cluster %s", clusterName)
	}
	assert.Equal(t, wantMode, gslbBasic["BalanceMode"])
	if wantEPPAddr == nil {
		assert.Nil(t, gslbBasic["EPPAddr"], "WRR cluster should not export EPPAddr")
		return
	}
	eppAddr, ok := gslbBasic["EPPAddr"].([]interface{})
	if !assert.True(t, ok, "EPPAddr should be an array for cluster %s", clusterName) {
		return
	}
	assert.Equal(t, wantEPPAddr, eppAddr, "EPPAddr should be ordered [primary, standby]")
}
