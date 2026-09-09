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

func TestProvider_One(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	if _, err := testutil.CreateProvider(providerName); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("PV-3-001 查询存在的 Provider", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", providerName)

		var data map[string]interface{}
		require.NoError(t, json.Unmarshal(resp.Data, &data))
		assert.Equal(t, "Asia/Shanghai", data["time_zone"])
		assert.Equal(t, []interface{}{}, data["tiers"])
	})

	t.Run("PV-3-002 查询不存在的 Provider", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers/non_existent_provider")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("PV-3-003 查询已设置 pricing-tiers 的 Provider", func(t *testing.T) {
		_, err := testutil.UpdatePricingTiers(providerName, map[string]interface{}{
			"time_zone": "America/New_York",
			"tiers": []map[string]interface{}{
				{
					"name": "peak",
					"time_ranges": []map[string]interface{}{
						{"weekdays": []int{1, 2, 3, 4, 5}, "start": "09:00", "end": "12:00"},
					},
				},
			},
		})
		require.NoError(t, err)

		resp, err := testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data map[string]interface{}
		require.NoError(t, json.Unmarshal(resp.Data, &data))
		assert.Equal(t, "America/New_York", data["time_zone"])

		tiers, ok := data["tiers"].([]interface{})
		require.True(t, ok, "tiers should be an array")
		require.Len(t, tiers, 1)
		tier0, ok := tiers[0].(map[string]interface{})
		require.True(t, ok)
		assert.Equal(t, "peak", tier0["name"])
	})

	t.Cleanup(func() {
		testutil.DeleteProvider(providerName)
	})
}
