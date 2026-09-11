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
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fetchProviderModels returns the provider's current model list via GET.
func fetchProviderModels(t *testing.T, providerName string) []interface{} {
	t.Helper()
	resp, err := testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
	require.NoError(t, err)
	testutil.AssertSuccess(t, resp)
	models, err := testutil.GetDataField(resp, "models")
	require.NoError(t, err)
	list, ok := models.([]interface{})
	require.True(t, ok, "models should be a list")
	return list
}

// PV-SYNC-1-004: PATCH provider to remove a model that is referenced by a
// cluster returns 409, and the provider record must remain unchanged
// (the update is rolled back with the transaction). This pins the
// issue #156 contract: a rejected update must not persist.
func TestProvider_ReferencedModelUpdateRollsBack(t *testing.T) {
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

	models := fetchProviderModels(t, providerName)
	assert.ElementsMatch(t, []interface{}{"m1", "m2"}, models,
		"409 path must roll back the provider update (issue #156)")
}

// PV-SYNC-1-005 (control arm): PATCH provider to remove a model that is NOT
// referenced by any cluster succeeds, and the change persists.
func TestProvider_UnreferencedModelUpdatePersists(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName, map[string]interface{}{
		"models": []string{"m1", "m2"},
	}); err != nil {
		t.Fatalf("setup provider failed: %v", err)
	}
	defer testutil.DeleteProvider(providerName)

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
	testutil.AssertSuccess(t, resp)

	models := fetchProviderModels(t, providerName)
	assert.ElementsMatch(t, []interface{}{"m1"}, models,
		"successful update must persist")
}
