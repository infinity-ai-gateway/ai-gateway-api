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

package pricing_tiers_test

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

func TestProvider_PricingTiers(t *testing.T) {
	t.Run("PT-1-001 JSON 设置高峰模板", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		resp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
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
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		testutil.AssertDataFieldEquals(t, resp, "time_zone", "Asia/Shanghai")
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		tiers, ok := data["tiers"].([]interface{})
		if !assert.True(t, ok, "tiers should be an array") {
			return
		}
		assert.Len(t, tiers, 1)
		tier0, _ := tiers[0].(map[string]interface{})
		assert.Equal(t, "peak", tier0["name"])
	})

	t.Run("PT-1-002 text/yaml 设置高峰模板", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		yamlContent := []byte(`time_zone: "Asia/Shanghai"
tiers:
  - name: "peak"
    time_ranges:
      - weekdays: [1, 2, 3, 4, 5]
        start: "09:00"
        end: "12:00"
      - weekdays: [1, 2, 3, 4, 5]
        start: "14:00"
        end: "18:00"
`)
		resp, err := testutil.UpdatePricingTiersYAML(providerName, yamlContent)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "time_zone", "Asia/Shanghai")
	})

	t.Run("PT-1-003 multipart/form-data YAML 设置高峰模板", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		yamlContent := []byte(`time_zone: "Asia/Shanghai"
tiers:
  - name: "peak"
    time_ranges:
      - weekdays: [1, 2, 3, 4, 5]
        start: "09:00"
        end: "12:00"
`)
		resp, err := testutil.UpdatePricingTiersMultipartYAML(providerName, yamlContent)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "time_zone", "Asia/Shanghai")
	})

	t.Run("PT-1-004 provider 不存在", func(t *testing.T) {
		resp, err := testutil.UpdatePricingTiers(testutil.UniqueProviderName(), map[string]interface{}{
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
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("PT-1-005 非法 time_zone", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		resp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"time_zone": "Mars/Phobos",
			"tiers": []interface{}{
				map[string]interface{}{
					"name": "peak",
					"time_ranges": []interface{}{
						map[string]interface{}{
							"start": "09:00",
							"end":   "12:00",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PT-1-006 非法 tier name", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		resp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"tiers": []interface{}{
				map[string]interface{}{
					"name": "off_peak",
					"time_ranges": []interface{}{
						map[string]interface{}{
							"start": "09:00",
							"end":   "12:00",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PT-1-007 同一 tier 时间重叠", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		resp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"tiers": []interface{}{
				map[string]interface{}{
					"name": "peak",
					"time_ranges": []interface{}{
						map[string]interface{}{
							"start": "09:00",
							"end":   "12:00",
						},
						map[string]interface{}{
							"start": "10:00",
							"end":   "11:00",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PT-1-008 end <= start", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		resp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"tiers": []interface{}{
				map[string]interface{}{
					"name": "peak",
					"time_ranges": []interface{}{
						map[string]interface{}{
							"start": "12:00",
							"end":   "09:00",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PT-1-009 weekdays 越界", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
		}
		defer testutil.DeleteProvider(providerName)

		resp, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"tiers": []interface{}{
				map[string]interface{}{
					"name": "peak",
					"time_ranges": []interface{}{
						map[string]interface{}{
							"weekdays": []int{7},
							"start":    "09:00",
							"end":      "12:00",
						},
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PT-1-010 GET provider 返回 tiers", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		_, err := testutil.CreateProvider(providerName, nil)
		if err != nil {
			t.Fatalf("setup provider failed: %v", err)
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
					},
				},
			},
		})
		if err != nil {
			t.Fatalf("setup pricing tiers failed: %v", err)
		}

		resp, err := testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("get provider failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "time_zone", "Asia/Shanghai")
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		tiers, ok := data["tiers"].([]interface{})
		if !assert.True(t, ok, "tiers should be an array") {
			return
		}
		assert.Len(t, tiers, 1)
	})
}
