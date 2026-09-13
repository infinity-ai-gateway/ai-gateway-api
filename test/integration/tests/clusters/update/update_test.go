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

package clusters_test

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

func TestClusters_Update(t *testing.T) {
	clusterName := testutil.UniqueClusterName()
	if _, err := testutil.CreateCluster(clusterName); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	providerUpdate := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerUpdate, map[string]interface{}{
		"models": []string{"qwen-turbo"},
	}); err != nil {
		t.Fatalf("setup providerUpdate failed: %v", err)
	}

	providerKeys := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerKeys, map[string]interface{}{
		"keys": []interface{}{
			map[string]interface{}{"name": "new-primary", "key": "sk-new-primary"},
			map[string]interface{}{"name": "new-secondary", "key": "sk-new-secondary"},
		},
	}); err != nil {
		t.Fatalf("setup providerKeys failed: %v", err)
	}

	providerPrefix := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerPrefix, map[string]interface{}{
		"models": []string{"openrouter/anthropic/claude-sonnet-4", "deepseek-chat"},
	}); err != nil {
		t.Fatalf("setup providerPrefix failed: %v", err)
	}

	t.Run("CL-4-001 更新 description", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"description": "更新后的集群描述",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", clusterName)
	})

	t.Run("CL-4-002 更新 llm_config", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models":   []string{"qwen-turbo"},
				"provider": providerUpdate,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", clusterName)
	})

	t.Run("CL-4-003 更新后查询一致性", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/clusters/" + clusterName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "description", "更新后的集群描述")
	})

	t.Run("CL-4-004 更新不存在的集群", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/non_existent_cluster", map[string]interface{}{
			"description": "x",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("CL-4-006 更新 keys（全量替换）", func(t *testing.T) {
		clusterUpdateKeys := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterUpdateKeys,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerKeys,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if resp.ErrNum != 200 {
			t.Fatalf("setup create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
		}
		defer testutil.DeleteCluster(clusterUpdateKeys)

		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterUpdateKeys, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"keys": []interface{}{
					map[string]interface{}{
						"name":   "new-primary",
						"weight": 60,
					},
					map[string]interface{}{
						"name":   "new-secondary",
						"weight": 40,
					},
				},
				"key_policy": map[string]interface{}{
					"strategy":    "weighted_random",
					"max_retries": 5,
				},
				"provider": providerKeys,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Get("/open-api/v1/clusters/" + clusterUpdateKeys)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", clusterUpdateKeys)
	})

	t.Run("CL-4-007 更新 key_policy", func(t *testing.T) {
		clusterUpdatePolicy := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterUpdatePolicy,
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"keys": []interface{}{
					map[string]interface{}{"name": "new-primary", "weight": 60},
					map[string]interface{}{"name": "new-secondary", "weight": 40},
				},
				"provider": providerKeys,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if resp.ErrNum != 200 {
			t.Fatalf("setup create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
		}
		defer testutil.DeleteCluster(clusterUpdatePolicy)

		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterUpdatePolicy, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"key_policy": map[string]interface{}{
					"strategy":              "weighted_random",
					"retry_backoff_initial": 200,
					"retry_backoff_max":     2000,
				},
				"provider": providerKeys,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Get("/open-api/v1/clusters/" + clusterUpdatePolicy)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", clusterUpdatePolicy)
	})

	t.Run("CL-4-008 更新 match_prefix / strip_prefix", func(t *testing.T) {
		clusterUpdatePrefix := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterUpdatePrefix,
			"llm_config": map[string]interface{}{
				"models":       []string{"openrouter/anthropic/claude-sonnet-4"},
				"match_prefix": "openrouter/",
				"strip_prefix": true,
				"provider":     providerPrefix,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if resp.ErrNum != 200 {
			t.Fatalf("setup create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
		}
		defer testutil.DeleteCluster(clusterUpdatePrefix)

		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterUpdatePrefix, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models":       []string{"deepseek-chat"},
				"match_prefix": "deepseek/",
				"strip_prefix": false,
				"provider":     providerPrefix,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Get("/open-api/v1/clusters/" + clusterUpdatePrefix)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		llm, _ := data["llm_config"].(map[string]interface{})
		assert.Equal(t, "deepseek/", llm["match_prefix"])
		assert.Equal(t, false, llm["strip_prefix"])

		resp, err = testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("inner api request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var innerData map[string]interface{}
		if err := json.Unmarshal(resp.Data, &innerData); err != nil {
			t.Fatalf("unmarshal inner data failed: %v", err)
		}
		clusterConf, _ := innerData["ClusterConf"].(map[string]interface{})
		config, _ := clusterConf["Config"].(map[string]interface{})
		cluster, _ := config[clusterUpdatePrefix].(map[string]interface{})
		aiconf, _ := cluster["AIConf"].(map[string]interface{})
		assert.Equal(t, "deepseek/", aiconf["MatchPrefix"])
		assert.Equal(t, false, aiconf["StripPrefix"])
	})

	t.Run("CL-4-011 更新 key_affinity", func(t *testing.T) {
		clusterUpdateAffinity := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterUpdateAffinity,
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"key_affinity": map[string]interface{}{
					"enabled":        false,
					"ttl":            600,
					"redis_prefix":   "bfe:ai:key_affinity",
					"penalty_enable": true,
				},
				"provider": providerPrefix,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if resp.ErrNum != 200 {
			t.Fatalf("setup create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
		}
		defer testutil.DeleteCluster(clusterUpdateAffinity)

		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterUpdateAffinity, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models": []string{"deepseek-chat"},
				"key_affinity": map[string]interface{}{
					"enabled":        true,
					"ttl":            1200,
					"redis_prefix":   "bfe:ai:key_affinity:v2",
					"penalty_enable": false,
				},
				"provider": providerPrefix,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Get("/open-api/v1/clusters/" + clusterUpdateAffinity)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		llm, _ := data["llm_config"].(map[string]interface{})
		affinity, _ := llm["key_affinity"].(map[string]interface{})
		assert.Equal(t, true, affinity["enabled"])
		assert.Equal(t, float64(1200), affinity["ttl"])
		assert.Equal(t, "bfe:ai:key_affinity:v2", affinity["redis_prefix"])
		assert.Equal(t, false, affinity["penalty_enable"])

		resp, err = testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("inner api request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var innerData map[string]interface{}
		if err := json.Unmarshal(resp.Data, &innerData); err != nil {
			t.Fatalf("unmarshal inner data failed: %v", err)
		}
		clusterConf, _ := innerData["ClusterConf"].(map[string]interface{})
		config, _ := clusterConf["Config"].(map[string]interface{})
		cluster, _ := config[clusterUpdateAffinity].(map[string]interface{})
		aiconf, _ := cluster["AIConf"].(map[string]interface{})
		keyPolicy, _ := aiconf["KeyPolicy"].(map[string]interface{})
		assert.Equal(t, true, keyPolicy["SessionAffinity"])
		assert.Equal(t, float64(1200), keyPolicy["SessionAffinityTTL"])
		assert.Equal(t, "bfe:ai:key_affinity:v2", keyPolicy["SessionAffinityRedisPrefix"])
		assert.Equal(t, false, keyPolicy["SessionAffinityPenaltyEnable"])
	})

	t.Run("CL-4-012 请求体不包含 name", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"description": "请求体未传 name",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", clusterName)
	})

	t.Run("CL-4-013 请求体包含 name", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"name":        clusterName,
			"description": "请求体传了 name",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
		testutil.DeleteProvider(providerUpdate)
		testutil.DeleteProvider(providerKeys)
		testutil.DeleteProvider(providerPrefix)
	})
}

func TestClusters_Update_PreventDeleteReferencedModel(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName, map[string]interface{}{
		"models": []string{"test-model", "other-model"},
	}); err != nil {
		t.Fatalf("setup provider failed: %v", err)
	}

	clusterName := testutil.UniqueClusterName()
	resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"test-model"},
			"provider": providerName,
		},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if resp.ErrNum != 200 {
		t.Fatalf("setup create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}

	// Create API-Key with route rule referencing the cluster model.
	apiKeyResp, err := testutil.GetClient().Post("/open-api/v1/api-keys", map[string]interface{}{
		"description": "model-ref-key",
		"route_rules": map[string]interface{}{
			"enabled": true,
			"rules": []interface{}{
				map[string]interface{}{
					"name": "rule-ref-model",
					"cond": "default_t()",
					"targets": []interface{}{
						map[string]interface{}{
							"cluster_name": clusterName,
							"model":        "test-model",
							"weight":       100,
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("create api-key failed: %v", err)
	}
	if apiKeyResp.ErrNum != 200 {
		t.Fatalf("create api-key failed: %d %s", apiKeyResp.ErrNum, apiKeyResp.ErrMsg)
	}
	apiKeyID, _ := testutil.GetDataField(apiKeyResp, "id")

	t.Run("CL-4-009 删除被路由引用的模型应被拦截", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models":   []string{"other-model"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 409)
		assert.Contains(t, resp.ErrMsg, "Rule rule-ref-model Refer To Model test-model In Cluster")
	})

	t.Run("CL-4-010 清理路由引用后可删除模型", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID.(string), map[string]interface{}{
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("update api-key failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"llm_config": map[string]interface{}{
				"models":   []string{"other-model"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
	})

	t.Cleanup(func() {
		testutil.GetClient().Delete("/open-api/v1/api-keys/" + apiKeyID.(string))
		testutil.DeleteCluster(clusterName)
		testutil.DeleteProvider(providerName)
	})
}
