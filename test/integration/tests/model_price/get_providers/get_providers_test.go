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

package model_price_test

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

type getProvidersResponse struct {
	Providers []string `json:"providers"`
}

func TestModelPrice_GetProviders(t *testing.T) {
	t.Run("MP-10-001 空列表", func(t *testing.T) {
		_, _, _ = testutil.ImportModelPricesWithResult([]byte("version: v1.0\ndefault_currency: RMB\nmodels: []\n"), "replace")

		resp, err := testutil.GetClient().Get("/open-api/v1/model-prices/actions/get-providers")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data getProvidersResponse
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		assert.Empty(t, data.Providers)
	})

	t.Run("MP-10-002 返回去重 provider 列表", func(t *testing.T) {
		providerA := testutil.UniqueName("a-provider")
		providerB := testutil.UniqueName("b-provider")

		id1, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   providerA,
			"model":      "model-1",
			"base_model": "model-1",
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
		})
		if err != nil {
			t.Fatalf("create model price 1 failed: %v", err)
		}
		defer testutil.DeleteModelPrice(id1)

		id2, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   providerB,
			"model":      "model-2",
			"base_model": "model-2",
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.002,
			},
		})
		if err != nil {
			t.Fatalf("create model price 2 failed: %v", err)
		}
		defer testutil.DeleteModelPrice(id2)

		id3, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   providerA,
			"model":      "model-3",
			"base_model": "model-3",
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.003,
			},
		})
		if err != nil {
			t.Fatalf("create model price 3 failed: %v", err)
		}
		defer testutil.DeleteModelPrice(id3)

		resp, err := testutil.GetClient().Get("/open-api/v1/model-prices/actions/get-providers")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data getProvidersResponse
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		assert.Len(t, data.Providers, 2)
		assert.Contains(t, data.Providers, providerA)
		assert.Contains(t, data.Providers, providerB)
		assert.Less(t, data.Providers[0], data.Providers[1])
	})
}
