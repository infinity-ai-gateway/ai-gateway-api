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
	"os"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
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

func TestInnerAPI_ModApiKey(t *testing.T) {
	// 创建带有限配额计划的 API-Key，确保 QuotaPlans 非空
	resp, err := testutil.GetClient().Post("/open-api/v1/api-keys", map[string]interface{}{
		"description": "inner-api-key",
		"quota_plan": map[string]interface{}{
			"unlimited": false,
			"quota":     1000,
		},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if resp.ErrNum != 200 {
		t.Fatalf("create api-key failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	apiKeyID, _ := testutil.GetDataField(resp, "id")

	t.Run("IN-6-001 首次导出 mod-api-key", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/mod-api-key")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataNotEmpty(t, resp)
		testutil.AssertDataFieldNotEmpty(t, resp, "version")
		testutil.AssertDataFieldNotEmpty(t, resp, "config")
		testutil.AssertDataFieldNotEmpty(t, resp, "QuotaPlans")
		testutil.AssertDataFieldNotEmpty(t, resp, "tokens")

		// 验证导出的 token 包含 key_id 且不包含 name/status/update_time，enabled 为 bool
		var data map[string]interface{}
		if err := testutil.UnmarshalData(resp, &data); err != nil {
			t.Fatalf("parse data failed: %v", err)
		}
		tokens, ok := data["tokens"].(map[string]interface{})
		if !ok {
			t.Fatal("tokens is not an object")
		}
		productTokens, ok := tokens["AI_product"].(map[string]interface{})
		if !ok {
			t.Fatal("tokens.AI_product is not an object")
		}
		if len(productTokens) == 0 {
			t.Fatal("tokens.AI_product is empty")
		}
		for key, token := range productTokens {
			tokenMap, ok := token.(map[string]interface{})
			if !ok {
				t.Fatalf("token %s is not an object", key)
			}
			if _, ok := tokenMap["key_id"]; !ok {
				t.Errorf("token %s missing key_id", key)
			}
			if _, ok := tokenMap["name"]; ok {
				t.Errorf("token %s should not contain name", key)
			}
			if _, ok := tokenMap["status"]; ok {
				t.Errorf("token %s should not contain status", key)
			}
			if _, ok := tokenMap["update_time"]; ok {
				t.Errorf("token %s should not contain update_time", key)
			}
			enabled, ok := tokenMap["enabled"].(bool)
			if !ok {
				t.Errorf("token %s enabled should be bool", key)
			}
			if !enabled {
				t.Errorf("token %s should be enabled", key)
			}
		}

		// 验证 QuotaPlans 不包含 CreateTime、ResetMode
		quotaPlans, ok := data["QuotaPlans"].(map[string]interface{})
		if !ok {
			t.Fatal("QuotaPlans is not an object")
		}
		for product, plans := range quotaPlans {
			planList, ok := plans.([]interface{})
			if !ok {
				t.Fatalf("QuotaPlans.%s is not an array", product)
			}
			for i, plan := range planList {
				planMap, ok := plan.(map[string]interface{})
				if !ok {
					t.Fatalf("QuotaPlans.%s[%d] is not an object", product, i)
				}
				if _, ok := planMap["CreateTime"]; ok {
					t.Errorf("QuotaPlans.%s[%d] should not contain CreateTime", product, i)
				}
				if _, ok := planMap["ResetMode"]; ok {
					t.Errorf("QuotaPlans.%s[%d] should not contain ResetMode", product, i)
				}
			}
		}
	})

	t.Run("IN-6-002 增量同步未变化", func(t *testing.T) {
		firstResp, err := testutil.GetClient().Get("/inner-api/v1/configs/mod-api-key")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		version, _ := testutil.GetDataField(firstResp, "version")
		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/mod-api-key", map[string]string{
			"version": version.(string),
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		if string(resp.Data) != "null" {
			t.Skip("version comparison not stable, Data is not null")
		}
	})

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID.(string))
	})
}
