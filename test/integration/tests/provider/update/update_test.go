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

package provider_test

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

func TestProvider_Update(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("PV-4-001 更新 description", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
			"description":     "更新后的 Provider 描述",
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "description", "更新后的 Provider 描述")
	})

	t.Run("PV-4-002 更新 models", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
			"models":          []string{"deepseek-chat", "deepseek-coder", "deepseek-reasoner"},
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		models, _ := data["models"].([]interface{})
		assert.Len(t, models, 3)
	})

	t.Run("PV-4-003 更新 keys（全量替换）", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
			"keys": []interface{}{
				map[string]interface{}{
					"name": "key-primary",
					"key":  "sk-new-primary",
				},
				map[string]interface{}{
					"name": "key-tertiary",
					"key":  "sk-cccccccccccc",
				},
			},
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		keys, _ := data["keys"].([]interface{})
		assert.Len(t, keys, 2)
	})

	t.Run("PV-4-004 更新不存在的 Provider", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/non_existent_provider", map[string]interface{}{
			"description":     "x",
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("PV-4-005 请求体不包含 name", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
			"description":     "请求体未传 name",
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", providerName)
	})

	t.Run("PV-4-005a 更新时 instance_pool 不再包含 name 字段", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.99", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		require.NoError(t, json.Unmarshal(resp.Data, &data))
		insts, _ := data["instance_pool"].([]interface{})
		require.Len(t, insts, 1)
		inst, _ := insts[0].(map[string]interface{})
		_, ok := inst["name"]
		assert.False(t, ok, "instance should not contain name field")
		assert.Equal(t, "10.0.0.99", inst["addr"])
	})

	t.Run("PV-4-006 请求体包含 name", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/providers/"+providerName, map[string]interface{}{
			"name":            providerName,
			"description":     "请求体传了 name",
			"instance_pool":   []interface{}{map[string]interface{}{"addr": "10.0.0.1", "weight": 100, "port": 8080}},
			"model_protocols": []string{"openai"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Cleanup(func() {
		testutil.DeleteProvider(providerName)
	})
}
