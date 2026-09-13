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

package tier_prices_test

import (
	"encoding/json"
	"fmt"
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

func TestModelPrice_TierPrices(t *testing.T) {
	t.Run("MTP-1-001 创建含 tier_prices 的记录", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := testutil.UniqueName("model")

		id, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token":  0.0000045,
				"output_cost_per_token": 0.0000135,
			},
			"tier_prices": map[string]interface{}{
				"peak": map[string]interface{}{
					"input_cost_per_token":  0.000009,
					"output_cost_per_token": 0.000027,
				},
			},
		})
		if err != nil {
			t.Fatalf("create model price failed: %v", err)
		}
		defer testutil.DeleteModelPrice(id)

		resp, err := testutil.GetClient().Get("/open-api/v1/model-prices/" + int64ToStr(id))
		if err != nil {
			t.Fatalf("get model price failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		tierPrices, ok := data["tier_prices"].(map[string]interface{})
		if !assert.True(t, ok, "tier_prices should be an object") {
			return
		}
		peak, ok := tierPrices["peak"].(map[string]interface{})
		if !assert.True(t, ok, "tier_prices.peak should be an object") {
			return
		}
		assert.Equal(t, 0.000009, peak["input_cost_per_token"])
		assert.Equal(t, 0.000027, peak["output_cost_per_token"])
	})

	t.Run("MTP-1-002 tier_prices 含非法 tier name", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := testutil.UniqueName("model")

		resp, err := testutil.GetClient().Post("/open-api/v1/model-prices", map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
			"tier_prices": map[string]interface{}{
				"off_peak": map[string]interface{}{
					"input_cost_per_token": 0.002,
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("MTP-1-003 tier_prices 含非法价格键", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := testutil.UniqueName("model")

		resp, err := testutil.GetClient().Post("/open-api/v1/model-prices", map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
			"tier_prices": map[string]interface{}{
				"peak": map[string]interface{}{
					"invalid_price_key": 0.002,
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("MTP-1-004 tier_prices 含负数价格", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := testutil.UniqueName("model")

		resp, err := testutil.GetClient().Post("/open-api/v1/model-prices", map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
			"tier_prices": map[string]interface{}{
				"peak": map[string]interface{}{
					"input_cost_per_token": -0.001,
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("MTP-1-005 更新 tier_prices", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := testutil.UniqueName("model")

		id, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
		})
		if err != nil {
			t.Fatalf("create model price failed: %v", err)
		}
		defer testutil.DeleteModelPrice(id)

		resp, err := testutil.GetClient().Put("/open-api/v1/model-prices/"+int64ToStr(id), map[string]interface{}{
			"tier_prices": map[string]interface{}{
				"peak": map[string]interface{}{
					"input_cost_per_token": 0.002,
				},
			},
		})
		if err != nil {
			t.Fatalf("update model price failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		tierPrices, ok := data["tier_prices"].(map[string]interface{})
		if !assert.True(t, ok, "tier_prices should be an object") {
			return
		}
		peak, ok := tierPrices["peak"].(map[string]interface{})
		if !assert.True(t, ok, "tier_prices.peak should be an object") {
			return
		}
		assert.Equal(t, 0.002, peak["input_cost_per_token"])
	})

	t.Run("MTP-1-006 model-list.yaml 导入含 tier_prices", func(t *testing.T) {
		yamlContent := []byte(`version: v1.0
default_currency: RMB
models:
  - provider: deepseek-tier-test
    model: deepseek-v3
    base_model: deepseek-v3
    mode: chat
    prices:
      input_cost_per_token: 0.000002
    tier_prices:
      peak:
        input_cost_per_token: 0.000004
`)
		result, _, err := testutil.ImportModelPricesWithResult(yamlContent, "replace")
		if err != nil {
			t.Fatalf("import model prices failed: %v", err)
		}
		assert.Equal(t, 1, result.ImportedCount)
		defer testutil.DeleteModelPriceByQuery("deepseek-tier-test", "deepseek-v3", "chat")

		resp, err := testutil.GetClient().Get("/open-api/v1/model-prices")
		if err != nil {
			t.Fatalf("list model prices failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		list, err := testutil.GetDataListField(resp, "list")
		if err != nil {
			t.Fatalf("get list field: %v", err)
		}
		found := false
		for _, item := range list {
			m, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			if m["provider"] != "deepseek-tier-test" || m["model"] != "deepseek-v3" {
				continue
			}
			found = true
			tierPrices, ok := m["tier_prices"].(map[string]interface{})
			if assert.True(t, ok, "tier_prices should be an object") {
				peak, ok := tierPrices["peak"].(map[string]interface{})
				if assert.True(t, ok, "tier_prices.peak should be an object") {
					assert.Equal(t, 0.000004, peak["input_cost_per_token"])
				}
			}
		}
		assert.True(t, found, "imported model price should be listed")
	})

	t.Run("MTP-1-007 部分更新 tier_prices 同档未传键保留", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := testutil.UniqueName("model")

		id, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.000001,
			},
			"tier_prices": map[string]interface{}{
				"peak": map[string]interface{}{
					"input_cost_per_token":        0.0000091,
					"output_cost_per_token":       0.0000102,
					"cache_read_input_token_cost": 0.0000031,
				},
			},
		})
		if err != nil {
			t.Fatalf("create model price failed: %v", err)
		}
		defer testutil.DeleteModelPrice(id)

		resp, err := testutil.GetClient().Put("/open-api/v1/model-prices/"+int64ToStr(id), map[string]interface{}{
			"tier_prices": map[string]interface{}{
				"peak": map[string]interface{}{
					"input_cost_per_token": 0.0000111,
				},
			},
		})
		if err != nil {
			t.Fatalf("update model price failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		assertTierPrices := func(tag string, data map[string]interface{}) {
			tierPrices, ok := data["tier_prices"].(map[string]interface{})
			if !assert.True(t, ok, "%s: tier_prices should be an object", tag) {
				return
			}
			peak, ok := tierPrices["peak"].(map[string]interface{})
			if !assert.True(t, ok, "%s: tier_prices.peak should be an object", tag) {
				return
			}
			assert.InDelta(t, 0.0000111, peak["input_cost_per_token"], 0.0000001, "%s: submitted key overridden", tag)
			assert.InDelta(t, 0.0000102, peak["output_cost_per_token"], 0.0000001, "%s: same-tier unsubmitted key kept (issue #140)", tag)
			assert.InDelta(t, 0.0000031, peak["cache_read_input_token_cost"], 0.0000001, "%s: same-tier unsubmitted key kept (issue #140)", tag)
		}

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		assertTierPrices("put response", data)

		// verify persisted state via GET
		getResp, err := testutil.GetClient().Get("/open-api/v1/model-prices/" + int64ToStr(id))
		if err != nil {
			t.Fatalf("get model price failed: %v", err)
		}
		testutil.AssertSuccess(t, getResp)
		var got map[string]interface{}
		if err := json.Unmarshal(getResp.Data, &got); err != nil {
			t.Fatalf("unmarshal get data: %v", err)
		}
		assertTierPrices("get response", got)
	})
}

func int64ToStr(v int64) string {
	return fmt.Sprintf("%d", v)
}
