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

package api_key_test

import (
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

func TestAPIKey_QuotaQuery(t *testing.T) {
	apiKeyID, err := testutil.CreateAPIKey("quota-query-key", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        100,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
	})
	if err != nil {
		t.Fatalf("setup quota failed: %v", err)
	}

	t.Run("AK-7-001 查询配额计划含 balance", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + apiKeyID + "/quota-plan")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldNotEmpty(t, resp, "balance")
	})

	t.Run("AK-7-002 查询无限配额 API-Key 的 quota-plan", func(t *testing.T) {
		unlimitedID, err := testutil.CreateAPIKey("unlimited-key", "")
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+unlimitedID, map[string]interface{}{
			"quota_plan": map[string]interface{}{"unlimited": true},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		resp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + unlimitedID + "/quota-plan")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 200 && resp.ErrNum != 404 {
			t.Errorf("expected ErrNum=200 or 404, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
		t.Cleanup(func() {
			testutil.DeleteAPIKey(unlimitedID)
		})
	})

	t.Run("AK-7-003 配额计划余额从 Redis 实时读取", func(t *testing.T) {
		limitedID, err := testutil.CreateAPIKey("limited-key", "")
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+limitedID, map[string]interface{}{
			"quota_plan": map[string]interface{}{
				"unlimited": false,
				"quota":     100,
				"unit":      "total_token",
			},
		})
		if err != nil {
			t.Fatalf("setup quota failed: %v", err)
		}

		resp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + limitedID + "/quota-plan")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "quota", float64(100))
		testutil.AssertDataFieldNotEmpty(t, resp, "balance")

		balance, err := testutil.GetDataField(resp, "balance")
		if err != nil {
			t.Fatalf("get balance failed: %v", err)
		}
		balanceMap, ok := balance.(map[string]interface{})
		if !ok {
			t.Fatalf("balance is not an object")
		}
		if used, ok := balanceMap["used"].(float64); !ok || used != 0 {
			t.Errorf("expected balance.used=0, got %v", balanceMap["used"])
		}
		if remaining, ok := balanceMap["remaining"].(float64); !ok || remaining != 100 {
			t.Errorf("expected balance.remaining=100, got %v", balanceMap["remaining"])
		}

		t.Cleanup(func() {
			testutil.DeleteAPIKey(limitedID)
		})
	})

	t.Run("AK-7-004 查询 RMB 配额余额精度", func(t *testing.T) {
		rmbID, err := testutil.CreateAPIKey("rmb-balance-key", "")
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+rmbID, map[string]interface{}{
			"quota_plan": map[string]interface{}{
				"unlimited":    false,
				"quota":        1000.12345678,
				"unit":         "RMB",
				"reset_period": "monthly",
			},
		})
		if err != nil {
			t.Fatalf("setup quota failed: %v", err)
		}

		resp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + rmbID + "/quota-plan")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		balance, err := testutil.GetDataField(resp, "balance")
		if err != nil {
			t.Fatalf("get balance failed: %v", err)
		}
		balanceMap, ok := balance.(map[string]interface{})
		if !ok {
			t.Fatalf("balance is not an object")
		}
		assert.InDelta(t, float64(1000.12345678), balanceMap["remaining"], 0.00000001)
		assert.InDelta(t, float64(0), balanceMap["used"], 0.00000001)

		t.Cleanup(func() {
			testutil.DeleteAPIKey(rmbID)
		})
	})

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
	})
}
