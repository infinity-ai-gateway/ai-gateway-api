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

package global_route_rules_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestGlobalRouteRules_Get(t *testing.T) {
	t.Run("GRR-2-003 查询系统默认 Global 路由表", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/global-route-rules")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataNotEmpty(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "enabled", false)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		rules, ok := data["rules"].([]interface{})
		if !ok {
			t.Fatalf("rules is not an array")
		}
		assert.Empty(t, rules)
	})

	t.Run("GRR-2-001 查询已更新的 Global 路由表", func(t *testing.T) {
		_, err := testutil.GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
			"rules": []interface{}{
				map[string]interface{}{
					"name":  "global-default",
					"cond":  "default_t()",
					"targets": []interface{}{
						map[string]interface{}{"cluster_name": "cluster_global", "model": "", "weight": 100},
					},
					"fallbacks": []interface{}{},
				},
			},
		})
		if err != nil {
			t.Fatalf("put failed: %v", err)
		}
		resp, err := testutil.GetClient().Get("/open-api/v1/global-route-rules")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldNotEmpty(t, resp, "rules")
	})

	t.Run("GRR-2-002 查询未配置的 Global 路由表", func(t *testing.T) {
		// 该用例需要全新数据库，无法在同一进程内保证，故跳过
		t.Skip("requires fresh database to verify Data=null")
	})
}
