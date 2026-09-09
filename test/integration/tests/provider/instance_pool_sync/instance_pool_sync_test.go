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

package instance_pool_sync_test

import (
	"encoding/json"
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

// TestProvider_InstancePoolSyncToInnerAPI verifies that modifying a provider's
// instance_pool via Open API is reflected in the Inner API cluster_table export.
func TestProvider_InstancePoolSyncToInnerAPI(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName, map[string]interface{}{
		"instance_pool": []interface{}{
			map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080},
		},
	}); err != nil {
		t.Fatalf("setup provider failed: %v", err)
	}
	defer testutil.DeleteProvider(providerName)

	clusterName := testutil.UniqueClusterName()
	_, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	if err != nil {
		t.Fatalf("setup cluster failed: %v", err)
	}
	defer testutil.DeleteCluster(clusterName)

	// Initial export should contain the old backend.
	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/gslb_data/cluster_table")
	if err != nil {
		t.Fatalf("initial cluster_table export failed: %v", err)
	}
	testutil.AssertSuccess(t, resp)
	assertBackendExists(t, resp, clusterName, "10.0.0.1_8080", "10.0.0.1", 8080)
	assertBackendNotExists(t, resp, clusterName, "10.0.0.2_8081")

	// Update provider instance_pool to the new backend.
	resp, err = testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
		"instance_pool": []interface{}{
			map[string]interface{}{"addr": "10.0.0.2", "weight": 100, "port": 8081},
		},
		"model_protocols": []string{"openai"},
	})
	if err != nil {
		t.Fatalf("update provider failed: %v", err)
	}
	testutil.AssertSuccess(t, resp)

	// Re-export cluster_table; the new backend should appear and the old one disappear.
	resp, err = testutil.GetClient().Get("/inner-api/v1/configs/gslb_data/cluster_table")
	if err != nil {
		t.Fatalf("cluster_table export after update failed: %v", err)
	}
	testutil.AssertSuccess(t, resp)
	assertBackendExists(t, resp, clusterName, "10.0.0.2_8081", "10.0.0.2", 8081)
	assertBackendNotExists(t, resp, clusterName, "10.0.0.1_8080")
}

func clusterTableConfig(t *testing.T, resp *testutil.APIResponse) map[string]interface{} {
	t.Helper()
	var data map[string]interface{}
	require.NoError(t, json.Unmarshal(resp.Data, &data))
	config, ok := data["Config"].(map[string]interface{})
	require.True(t, ok, "cluster_table Config should be an object")
	return config
}

func subClusterBackends(t *testing.T, config map[string]interface{}, clusterName string) []map[string]interface{} {
	t.Helper()
	cluster, ok := config[clusterName].(map[string]interface{})
	require.True(t, ok, "cluster %s should exist in cluster_table", clusterName)

	// The cluster has a single sub-cluster created from the provider.
	var backends []map[string]interface{}
	for _, v := range cluster {
		subClusterBackends, ok := v.([]interface{})
		if !ok {
			continue
		}
		for _, b := range subClusterBackends {
			backend, ok := b.(map[string]interface{})
			require.True(t, ok, "backend should be an object")
			backends = append(backends, backend)
		}
	}
	return backends
}

func assertBackendExists(t *testing.T, resp *testutil.APIResponse, clusterName, name, addr string, port int) {
	t.Helper()
	config := clusterTableConfig(t, resp)
	backends := subClusterBackends(t, config, clusterName)
	for _, b := range backends {
		if b["Name"] == name && b["Addr"] == addr && int(b["Port"].(float64)) == port {
			return
		}
	}
	t.Fatalf("backend %s (%s:%d) not found in cluster %s", name, addr, port, clusterName)
}

// PV-SYNC-1-002: deleting a provider key that is referenced by a cluster
// should be rejected with 409 Conflict.
func TestProvider_DeleteReferencedKeyReturnsConflict(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName, map[string]interface{}{
		"keys": []interface{}{
			map[string]interface{}{"name": "k1", "key": "sk-111111111111"},
			map[string]interface{}{"name": "k2", "key": "sk-222222222222"},
		},
	}); err != nil {
		t.Fatalf("setup provider failed: %v", err)
	}
	defer testutil.DeleteProvider(providerName)

	clusterName := testutil.UniqueClusterName()
	_, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
			"keys": []interface{}{
				map[string]interface{}{"name": "k1", "weight": 100},
			},
		},
	})
	if err != nil {
		t.Fatalf("setup cluster failed: %v", err)
	}
	defer testutil.DeleteCluster(clusterName)

	resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
		"instance_pool": []interface{}{
			map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080},
		},
		"keys": []interface{}{
			map[string]interface{}{"name": "k2", "key": "sk-222222222222"},
		},
		"model_protocols": []string{"openai"},
	})
	if err != nil {
		t.Fatalf("update provider failed: %v", err)
	}
	testutil.AssertErrCode(t, resp, 409)
}

// PV-SYNC-1-003: deleting a provider model that is referenced by a cluster
// should be rejected with 409 Conflict.
func TestProvider_DeleteReferencedModelReturnsConflict(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName, map[string]interface{}{
		"models": []string{"m1", "m2"},
	}); err != nil {
		t.Fatalf("setup provider failed: %v", err)
	}
	defer testutil.DeleteProvider(providerName)

	clusterName := testutil.UniqueClusterName()
	_, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"m2"},
			"provider": providerName,
		},
	})
	if err != nil {
		t.Fatalf("setup cluster failed: %v", err)
	}
	defer testutil.DeleteCluster(clusterName)

	resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
		"instance_pool": []interface{}{
			map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080},
		},
		"models":          []string{"m1"},
		"model_protocols": []string{"openai"},
	})
	if err != nil {
		t.Fatalf("update provider failed: %v", err)
	}
	testutil.AssertErrCode(t, resp, 409)
}

func assertBackendNotExists(t *testing.T, resp *testutil.APIResponse, clusterName, name string) {
	t.Helper()
	config := clusterTableConfig(t, resp)
	backends := subClusterBackends(t, config, clusterName)
	for _, b := range backends {
		if b["Name"] == name {
			t.Fatalf("unexpected backend %s found in cluster %s", name, clusterName)
		}
	}
}
