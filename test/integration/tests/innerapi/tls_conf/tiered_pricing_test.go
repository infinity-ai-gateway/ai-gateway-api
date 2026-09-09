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
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
	"github.com/stretchr/testify/assert"
)

func TestInnerAPI_TlsConf_TieredPricing(t *testing.T) {
	t.Run("IN-TIER-1-001 导出 ClusterConf 含分时段定价", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		providerResp, err := testutil.GetClient().Post("/open-api/v1/providers", map[string]interface{}{
			"name": providerName,
			"instance_pool": []interface{}{
				map[string]interface{}{
					
					"addr":   "10.0.0.1",
					"weight": 100,
					"port":   8080,
				},
			},
			"model_protocols": []string{"openai"},
			"models":          []string{"deepseek-chat"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		if providerResp.ErrNum != 200 {
			t.Fatalf("create provider failed: ErrNum=%d, ErrMsg=%s", providerResp.ErrNum, providerResp.ErrMsg)
		}
		defer testutil.DeleteProvider(providerName)

		_, err = testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"time_zone": "Asia/Shanghai",
			"tiers": []interface{}{
				map[string]interface{}{
					"name": "peak",
					"time_ranges": []interface{}{
						map[string]interface{}{
							"weekdays": []int{1, 2, 3, 4, 5},
							"start":    "09:00",
							"end":      "12:00",
						},
						map[string]interface{}{
							"weekdays": []int{1, 2, 3, 4, 5},
							"start":    "14:00",
							"end":      "18:00",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("setup pricing tiers failed: %v", err)
		}

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
		if err := testutil.ImportModelPrices(yamlContent, "replace"); err != nil {
			t.Fatalf("import model prices failed: %v", err)
		}
		defer testutil.DeleteModelPriceByQuery(providerName, "deepseek-chat", "chat")

		clusterName := testutil.UniqueClusterName()
		createResp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup cluster failed: %v", err)
		}
		if createResp.ErrNum != 200 {
			t.Fatalf("create cluster failed: ErrNum=%d, ErrMsg=%s", createResp.ErrNum, createResp.ErrMsg)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		modelTable, ok := aiconf["ModelTable"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf.ModelTable should be an object") {
			return
		}
		assert.Equal(t, "RMB", modelTable["Currency"])
		assert.Equal(t, "Asia/Shanghai", modelTable["TimeZone"])

		tiers, ok := modelTable["Tiers"].([]interface{})
		if !assert.True(t, ok, "ModelTable.Tiers should be an array") {
			return
		}
		assert.Len(t, tiers, 1)
		tier0, _ := tiers[0].(map[string]interface{})
		assert.Equal(t, "peak", tier0["Name"])

		models, ok := modelTable["Models"].([]interface{})
		if !assert.True(t, ok, "ModelTable.Models should be an array") {
			return
		}
		assert.Len(t, models, 1)
		model0, _ := models[0].(map[string]interface{})
		assert.Equal(t, "deepseek-chat", model0["Model"])

		prices, ok := model0["Prices"].(map[string]interface{})
		if !assert.True(t, ok, "ModelPrice.Prices should be an object") {
			return
		}
		assert.Equal(t, 0.000002, prices["input_cost_per_token"])

		tierPrices, ok := model0["TierPrices"].(map[string]interface{})
		if !assert.True(t, ok, "ModelPrice.TierPrices should be an object") {
			return
		}
		peak, ok := tierPrices["peak"].(map[string]interface{})
		if !assert.True(t, ok, "TierPrices.peak should be an object") {
			return
		}
		assert.Equal(t, 0.000004, peak["input_cost_per_token"])
		assert.Equal(t, 0.000016, peak["output_cost_per_token"])
		assert.Equal(t, 0.000001, peak["cache_read_input_token_cost"])
	})

	t.Run("IN-TIER-1-002 未配置时段规则时导出固定价格", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		providerResp, err := testutil.GetClient().Post("/open-api/v1/providers", map[string]interface{}{
			"name": providerName,
			"instance_pool": []interface{}{
				map[string]interface{}{
					
					"addr":   "10.0.0.1",
					"weight": 100,
					"port":   8080,
				},
			},
			"model_protocols": []string{"openai"},
			"models":          []string{"deepseek-chat"},
		})
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		if providerResp.ErrNum != 200 {
			t.Fatalf("create provider failed: ErrNum=%d, ErrMsg=%s", providerResp.ErrNum, providerResp.ErrMsg)
		}
		defer testutil.DeleteProvider(providerName)

		yamlContent := []byte(`version: v1.0
default_currency: RMB
models:
  - provider: ` + providerName + `
    model: deepseek-chat
    base_model: deepseek-chat
    mode: chat
    prices:
      input_cost_per_token: 0.0001
`)
		if err := testutil.ImportModelPrices(yamlContent, "replace"); err != nil {
			t.Fatalf("import model prices failed: %v", err)
		}
		defer testutil.DeleteModelPriceByQuery(providerName, "deepseek-chat", "chat")

		clusterName := testutil.UniqueClusterName()
		createResp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup cluster failed: %v", err)
		}
		if createResp.ErrNum != 200 {
			t.Fatalf("create cluster failed: ErrNum=%d, ErrMsg=%s", createResp.ErrNum, createResp.ErrMsg)
		}
		defer testutil.DeleteCluster(clusterName)

		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/tls_conf/server_data_conf")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		aiconf := extractAIConf(t, resp, clusterName)
		modelTable, ok := aiconf["ModelTable"].(map[string]interface{})
		if !assert.True(t, ok, "AIConf.ModelTable should be an object") {
			return
		}
		assert.Equal(t, "RMB", modelTable["Currency"])

		// Tiers 为空或不存在
		tiers, ok := modelTable["Tiers"].([]interface{})
		if ok {
			assert.Empty(t, tiers)
		}

		models, ok := modelTable["Models"].([]interface{})
		if !assert.True(t, ok, "ModelTable.Models should be an array") {
			return
		}
		assert.Len(t, models, 1)
		model0, _ := models[0].(map[string]interface{})
		prices, ok := model0["Prices"].(map[string]interface{})
		if !assert.True(t, ok, "ModelPrice.Prices should be an object") {
			return
		}
		assert.Equal(t, 0.0001, prices["input_cost_per_token"])

		// TierPrices 为空或不存在
		tierPrices, ok := model0["TierPrices"].(map[string]interface{})
		if ok {
			assert.Empty(t, tierPrices)
		}
	})
}

