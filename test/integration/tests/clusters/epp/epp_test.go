package clusters_test

import (
	"encoding/json"
	"os"
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

func minEPPClusterBody(name, provider string, eppConfig interface{}) map[string]interface{} {
	body := map[string]interface{}{
		"name": name,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": provider,
		},
	}
	if eppConfig != nil {
		body["balance_mode"] = "EPP"
		body["epp_config"] = eppConfig
	}
	return body
}

func getCluster(t *testing.T, name string) map[string]interface{} {
	t.Helper()
	resp, err := testutil.GetClient().Get("/open-api/v1/clusters/" + name)
	if err != nil {
		t.Fatalf("get cluster failed: %v", err)
	}
	testutil.AssertSuccess(t, resp)
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal cluster failed: %v", err)
	}
	return data
}

func TestClusters_EPP(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName); err != nil {
		t.Fatalf("setup provider failed: %v", err)
	}
	defer testutil.DeleteProvider(providerName)

	validConfig := map[string]interface{}{
		"scheduling_profile":        "balanced",
		"prefix_cache_affinity":     true,
		"kv_cache_utilization_max":  0.9,
		"session_affinity_enabled":  true,
		"session_affinity_header":   "x-session-id",
		"flow_control": map[string]interface{}{
			"max_requests":         100,
			"queue_ttl":            30,
			"no_endpoint_queue_ttl": 600,
			"enable_eviction":      false,
		},
	}

	t.Run("CL-EPP-001 EPP 模式缺少 epp_config 返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name":         testutil.UniqueClusterName(),
			"balance_mode": "EPP",
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-002 epp_config 空对象视为未配置返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-003 scheduling_profile 枚举越界返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{
				"scheduling_profile": "super-fast",
			}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-004 cache_affinity 枚举越界返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{
				"cache_affinity": "extreme",
			}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-005 kv_cache_utilization_max 越界返回 422", func(t *testing.T) {
		for _, v := range []interface{}{0, -0.1, 1.1, 2} {
			resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
				testutil.UniqueClusterName(), providerName, map[string]interface{}{
					"kv_cache_utilization_max": v,
				}))
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			testutil.AssertErrCode(t, resp, 422)
		}
	})

	t.Run("CL-EPP-006 queue_ttl 负数返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{
				"flow_control": map[string]interface{}{"queue_ttl": -1},
			}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-007 max_requests 非法值返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{
				"flow_control": map[string]interface{}{"max_requests": 0},
			}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-008 session_affinity_enabled=true 缺 header 返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{
				"session_affinity_enabled": true,
			}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-009 epp_config 携带未知字段返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(
			testutil.UniqueClusterName(), providerName, map[string]interface{}{
				"scheduling_profile": "balanced",
				"unknown_field":      1,
			}))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-010 balance_mode 非法枚举返回 422", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name":         testutil.UniqueClusterName(),
			"balance_mode": "LC",
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("CL-EPP-011 EPP 完整配置创建成功并原样回读", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(clusterName, providerName, validConfig))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		data := getCluster(t, clusterName)
		assert.Equal(t, "EPP", data["balance_mode"])
		eppConfig, ok := data["epp_config"].(map[string]interface{})
		require.True(t, ok, "epp_config should be echoed back")
		assert.Equal(t, "balanced", eppConfig["scheduling_profile"])
		assert.Equal(t, true, eppConfig["prefix_cache_affinity"])
		assert.Equal(t, 0.9, eppConfig["kv_cache_utilization_max"])
		assert.Equal(t, true, eppConfig["session_affinity_enabled"])
		assert.Equal(t, "x-session-id", eppConfig["session_affinity_header"])
		fc := eppConfig["flow_control"].(map[string]interface{})
		assert.Equal(t, float64(100), fc["max_requests"])
		assert.Equal(t, float64(30), fc["queue_ttl"])

		t.Cleanup(func() { testutil.DeleteCluster(clusterName) })
	})

	t.Run("CL-EPP-012 WRR 携带合法 epp_config 创建成功（休眠保留）", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		body := map[string]interface{}{
			"name":         clusterName,
			"balance_mode": "WRR",
			"epp_config":   map[string]interface{}{"scheduling_profile": "throughput-first"},
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		}
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", body)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		data := getCluster(t, clusterName)
		assert.Equal(t, "WRR", data["balance_mode"])
		eppConfig, ok := data["epp_config"].(map[string]interface{})
		require.True(t, ok, "epp_config should be retained in WRR mode")
		assert.Equal(t, "throughput-first", eppConfig["scheduling_profile"])

		t.Cleanup(func() { testutil.DeleteCluster(clusterName) })
	})

	t.Run("CL-EPP-013 balance_mode 缺省回读为 WRR 且 epp_config 为 null", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		if _, err := testutil.CreateCluster(clusterName); err != nil {
			t.Fatalf("setup cluster failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		data := getCluster(t, clusterName)
		assert.Equal(t, "WRR", data["balance_mode"])
		eppConfig, exists := data["epp_config"]
		assert.True(t, exists, "epp_config field should be present")
		assert.Nil(t, eppConfig, "epp_config should be null when never configured")
	})

	t.Run("CL-EPP-014 EPP 到 WRR 再回 EPP 配置与分配保留", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", minEPPClusterBody(clusterName, providerName, map[string]interface{}{
			"scheduling_profile": "latency-first",
		}))
		if err != nil {
			t.Fatalf("create failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		defer testutil.DeleteCluster(clusterName)

		// EPP -> WRR：仅改 balance_mode，epp_config 休眠保留
		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"balance_mode": "WRR",
		})
		if err != nil {
			t.Fatalf("patch to WRR failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		data := getCluster(t, clusterName)
		assert.Equal(t, "WRR", data["balance_mode"])
		eppConfig, ok := data["epp_config"].(map[string]interface{})
		require.True(t, ok, "epp_config should be retained after EPP -> WRR")
		assert.Equal(t, "latency-first", eppConfig["scheduling_profile"])

		// WRR -> EPP：无需重新携带 epp_config，休眠配置继续生效
		resp, err = testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"balance_mode": "EPP",
		})
		if err != nil {
			t.Fatalf("patch to EPP failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		data = getCluster(t, clusterName)
		assert.Equal(t, "EPP", data["balance_mode"])
		eppConfig, ok = data["epp_config"].(map[string]interface{})
		require.True(t, ok, "epp_config should survive WRR -> EPP")
		assert.Equal(t, "latency-first", eppConfig["scheduling_profile"])
	})

	t.Run("CL-EPP-015 WRR 休眠状态下更新携带非法 epp_config 返回 422", func(t *testing.T) {
		clusterName := testutil.UniqueClusterName()
		if _, err := testutil.CreateCluster(clusterName); err != nil {
			t.Fatalf("setup cluster failed: %v", err)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Patch("/open-api/v1/clusters/"+clusterName, map[string]interface{}{
			"epp_config": map[string]interface{}{"kv_cache_utilization_max": 1.5},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})
}
