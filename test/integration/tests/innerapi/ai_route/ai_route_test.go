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

func TestInnerAPI_AiRoute(t *testing.T) {
	// 准备 global route
	_, err := testutil.GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{
				"name":      "global-default",
				"Cond":      "default_t()",
				"targets":   []interface{}{map[string]interface{}{"ClusterName": "cluster_global", "Weight": 100}},
				"fallbacks": []interface{}{},
			},
		},
	})
	if err != nil {
		t.Fatalf("setup global route failed: %v", err)
	}

	apiKeyID, err := testutil.CreateAPIKey("ai-route-key", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"route_rules": map[string]interface{}{
			"enabled": true,
			"rules": []interface{}{
				map[string]interface{}{
					"name":    "default",
					"Cond":    "default_t()",
					"targets": []interface{}{map[string]interface{}{"ClusterName": "cluster1", "Weight": 100}},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("IN-9-001 导出 ai-route", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/inner-api/v1/configs/ai-route")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataNotEmpty(t, resp)
		testutil.AssertDataFieldNotEmpty(t, resp, "Version")
		testutil.AssertDataFieldNotEmpty(t, resp, "RouteRules")
		testutil.AssertDataFieldNotEmpty(t, resp, "ApikeyRouteTableBindings")
	})

	t.Run("IN-9-002 ai-route 未启用路由表不导出", func(t *testing.T) {
		// 该用例需要禁用所有路由表，会影响其他测试，跳过
		t.Skip("requires disabling all route tables")
	})

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
	})
}
