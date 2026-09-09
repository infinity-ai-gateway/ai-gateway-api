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

package innerapi_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
	"github.com/stretchr/testify/assert"
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

func TestInnerAPI_TlsConf(t *testing.T) {
	// 创建默认证书以确保证书配置非空
	certName := testutil.UniqueCertName()
	if _, err := testutil.CreateCertificate(certName, true); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("IN-1-001 首次导出 TLS/Server 配置", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataNotEmpty(t, resp)
		testutil.AssertDataFieldNotEmpty(t, resp, "Version")
		testutil.AssertDataFieldNotEmpty(t, resp, "HostTable")
		testutil.AssertDataFieldNotEmpty(t, resp, "RouteTable")
		testutil.AssertDataFieldNotEmpty(t, resp, "ClusterConf")
	})

	t.Run("IN-1-002 导出 ClusterConf 含多 Key AIConf", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, map[string]interface{}{
			"keys": []interface{}{
				map[string]interface{}{"name": "primary", "key": "sk-aaaaaaaaaaaa"},
				map[string]interface{}{"name": "secondary", "key": "sk-bbbbbbbbbbbb"},
			},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		clusterName := testutil.UniqueClusterName()
		_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"keys": []interface{}{
					map[string]interface{}{
						"name":   "primary",
						"weight": 70,
					},
					map[string]interface{}{
						"name":   "secondary",
						"weight": 30,
					},
				},
				"key_policy": map[string]interface{}{
					"strategy":              "weighted_random",
					"max_retries":           3,
					"retry_backoff_initial": 100,
					"retry_backoff_max":     5000,
				},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		clusterConf, ok := data["ClusterConf"].(map[string]interface{})
		if !assert.True(t, ok, "ClusterConf should be an object") {
			return
		}
		config, ok := clusterConf["Config"].(map[string]interface{})
		if !assert.True(t, ok, "ClusterConf.Config should be an object") {
			return
		}
		cluster, ok := config[clusterName].(map[string]interface{})
		if !assert.True(t, ok, "target cluster should exist in ClusterConf.Config") {
			return
		}
		aiconf, ok := cluster["AIConf"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf should be an object") {
			return
		}
		keys, ok := aiconf["Keys"].([]interface{})
		if !assert.True(t, ok, "AIConf.Keys should be an array") {
			return
		}
		assert.Len(t, keys, 2)
		key0, _ := keys[0].(map[string]interface{})
		assert.Equal(t, "primary", key0["Name"])
		assert.Equal(t, "sk-aaaaaaaaaaaa", key0["Key"])
		assert.Equal(t, float64(70), key0["Weight"])

		policy, ok := aiconf["KeyPolicy"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf.KeyPolicy should be an object") {
			return
		}
		assert.Equal(t, "weighted_random", policy["Strategy"])
		assert.Equal(t, float64(3), policy["MaxRetries"])
		assert.Equal(t, float64(100), policy["RetryBackoffInitial"])
		assert.Equal(t, float64(5000), policy["RetryBackoffMax"])
		assert.Equal(t, true, policy["SessionAffinity"])
		assert.Equal(t, float64(600), policy["SessionAffinityTTL"])
		assert.Equal(t, "bfe:ai:key_affinity", policy["SessionAffinityRedisPrefix"])
		assert.Equal(t, true, policy["SessionAffinityPenaltyEnable"])
	})

	t.Run("IN-1-003 导出 ClusterConf 含模型定价表", func(t *testing.T) {
		providerName := "openai"
		_, err := testutil.CreateProvider(providerName, map[string]interface{}{
			"models": []string{"gpt-4o", "gpt-4o-mini"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		yamlContent := []byte(`version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4o
    base_model: gpt-4o
    mode: chat
    prices:
      input_cost_per_token: 0.0001
  - provider: openai
    model: gpt-4o-mini
    base_model: gpt-4o-mini
    mode: chat
    prices:
      input_cost_per_token: 0.00001
`)
		if err := testutil.ImportModelPrices(yamlContent, "replace"); err != nil {
			t.Fatalf("import model prices failed: %v", err)
		}

		clusterName := testutil.UniqueClusterName()
		_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"gpt-4o"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		clusterConf, ok := data["ClusterConf"].(map[string]interface{})
		if !assert.True(t, ok, "ClusterConf should be an object") {
			return
		}
		config, ok := clusterConf["Config"].(map[string]interface{})
		if !assert.True(t, ok, "ClusterConf.Config should be an object") {
			return
		}
		cluster, ok := config[clusterName].(map[string]interface{})
		if !assert.True(t, ok, "target cluster should exist in ClusterConf.Config") {
			return
		}
		aiconf, ok := cluster["AIConf"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf should be an object") {
			return
		}
		modelTable, ok := aiconf["ModelTable"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf.ModelTable should be an object") {
			return
		}
		assert.Equal(t, "RMB", modelTable["Currency"])
		models, ok := modelTable["Models"].([]interface{})
		if !assert.True(t, ok, "ModelTable.Models should be an array") {
			return
		}
		assert.Len(t, models, 2)

		modelNames := make([]string, 0, len(models))
		for _, m := range models {
			mm, ok := m.(map[string]interface{})
			if assert.True(t, ok, "model entry should be an object") {
				modelNames = append(modelNames, mm["Model"].(string))
				assert.Equal(t, "openai", mm["Provider"])
			}
		}
		assert.Contains(t, modelNames, "gpt-4o")
		assert.Contains(t, modelNames, "gpt-4o-mini")
	})

	t.Run("IN-TLS-1-004 AIConf 包含 MatchPrefix / StripPrefix", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, map[string]interface{}{
			"models": []string{"openrouter/anthropic/claude-sonnet-4"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		clusterName := testutil.UniqueClusterName()
		_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":       []string{"openrouter/anthropic/claude-sonnet-4"},
				"match_prefix": "openrouter/",
				"strip_prefix": true,
				"provider":     providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		assert.Equal(t, "openrouter/", aiconf["MatchPrefix"])
		assert.Equal(t, true, aiconf["StripPrefix"])
	})

	t.Run("IN-TLS-1-005 未配置前缀时 AIConf 为默认值", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		_, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": "deepseek",
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		v, ok := aiconf["MatchPrefix"]
		if ok && v != nil {
			assert.Equal(t, "", v, "MatchPrefix should be empty if present")
		}
		assert.Equal(t, false, aiconf["StripPrefix"])
	})

	t.Run("IN-TLS-1-006 仅 match_prefix、strip_prefix=false 时的导出", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, map[string]interface{}{
			"models": []string{"openrouter/anthropic/claude-sonnet-4"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		clusterName := testutil.UniqueClusterName()
		_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":       []string{"openrouter/anthropic/claude-sonnet-4"},
				"match_prefix": "openrouter/",
				"strip_prefix": false,
				"provider":     providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		assert.Equal(t, "openrouter/", aiconf["MatchPrefix"])
		assert.Equal(t, false, aiconf["StripPrefix"])
	})

	t.Run("IN-TLS-1-007 AIConf 包含 ModelProtocols（anthropic）", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, map[string]interface{}{
			"models":          []string{"claude-3-opus-20240229"},
			"model_protocols": []string{"anthropic"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		clusterName := testutil.UniqueClusterName()
		_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"claude-3-opus-20240229"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		protocols, ok := aiconf["ModelProtocols"].([]interface{})
		if !assert.True(t, ok, "AIConf.ModelProtocols should be an array") {
			return
		}
		assert.Equal(t, []interface{}{"anthropic"}, protocols)
	})

	t.Run("IN-TLS-1-008 AIConf.KeyPolicy 包含 SessionAffinity*", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, map[string]interface{}{
			"models": []string{"deepseek-chat"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		clusterName := testutil.UniqueClusterName()
		_, err = testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"key_affinity": map[string]interface{}{
					"enabled":        true,
					"ttl":            600,
					"redis_prefix":   "bfe:ai:key_affinity",
					"penalty_enable": true,
				},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		keyPolicy, ok := aiconf["KeyPolicy"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf.KeyPolicy should be an object") {
			return
		}
		assert.Equal(t, true, keyPolicy["SessionAffinity"])
		assert.Equal(t, float64(600), keyPolicy["SessionAffinityTTL"])
		assert.Equal(t, "bfe:ai:key_affinity", keyPolicy["SessionAffinityRedisPrefix"])
		assert.Equal(t, true, keyPolicy["SessionAffinityPenaltyEnable"])
	})

	t.Run("IN-TLS-1-009 未配置 key_affinity 时 AIConf.KeyPolicy 为默认值", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		_, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": "deepseek",
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		keyPolicy, ok := aiconf["KeyPolicy"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf.KeyPolicy should be an object") {
			return
		}
		assert.Equal(t, true, keyPolicy["SessionAffinity"])
		assert.Equal(t, float64(600), keyPolicy["SessionAffinityTTL"])
		assert.Equal(t, "bfe:ai:key_affinity", keyPolicy["SessionAffinityRedisPrefix"])
		assert.Equal(t, true, keyPolicy["SessionAffinityPenaltyEnable"])
	})

	t.Cleanup(func() {
		testutil.DeleteCertificate(certName)
	})
}

func extractAIConf(t *testing.T, resp *testutil.APIResponse, clusterName string) map[string]interface{} {
	t.Helper()
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	clusterConf, ok := data["ClusterConf"].(map[string]interface{})
	if !assert.True(t, ok, "ClusterConf should be an object") {
		return nil
	}
	config, ok := clusterConf["Config"].(map[string]interface{})
	if !assert.True(t, ok, "ClusterConf.Config should be an object") {
		return nil
	}
	cluster, ok := config[clusterName].(map[string]interface{})
	if !assert.True(t, ok, "target cluster should exist in ClusterConf.Config") {
		return nil
	}
	aiconf, ok := cluster["AIConf"].(map[string]interface{})
	if !assert.True(t, ok, "AIConf should be an object") {
		return nil
	}
	return aiconf
}
