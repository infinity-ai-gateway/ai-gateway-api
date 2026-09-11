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

package entity_test

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

func TestEntity_PartialUpdate(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()
	if _, err := testutil.CreateEntityType(typeName, 1); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("E-5-001 部分更新 allow_models", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"allow_models": []string{"gpt-4"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "type", typeName)
	})

	t.Run("E-5-002 部分更新后查询一致性", func(t *testing.T) {
		_, err := testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"block_models": []string{"gpt-4-32k"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp, err := testutil.GetClient().Get("/open-api/v1/entities/" + entityID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "block_models", []interface{}{"gpt-4-32k"})
	})

	t.Run("E-5-003 部分更新非法 route_rules（规则名重复）", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"route_rules": map[string]interface{}{
				"enabled": true,
				"rules": []interface{}{
					map[string]interface{}{
						"name": "dup",
						"cond": "default_t()",
						"targets": []interface{}{
							map[string]interface{}{"cluster_name": "c1", "weight": 100},
						},
					},
					map[string]interface{}{
						"name": "dup",
						"cond": "default_t()",
						"targets": []interface{}{
							map[string]interface{}{"cluster_name": "c2", "weight": 100},
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

	t.Run("E-5-004 部分更新 quota_plan 切换为 RMB", func(t *testing.T) {
		id, err := testutil.CreateEntity(testutil.UniqueEntityName(), typeName, "")
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteEntity(id)

		_, err = testutil.GetClient().Patch("/open-api/v1/entities/"+id, map[string]interface{}{
			"quota_plan": map[string]interface{}{
				"unlimited":    false,
				"quota":        100000,
				"unit":         "total_token",
				"reset_period": "monthly",
			},
		})
		if err != nil {
			t.Fatalf("setup quota failed: %v", err)
		}

		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+id, map[string]interface{}{
			"quota_plan": map[string]interface{}{
				"unlimited":    false,
				"quota":        777.7777,
				"unit":         "RMB",
				"reset_period": "weekly",
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data: %v", err)
		}
		qp := data["quota_plan"].(map[string]interface{})
		assert.Equal(t, "RMB", qp["unit"])
		assert.InDelta(t, float64(777.7777), qp["quota"], 0.00001)

		qpResp, err := testutil.GetClient().Get("/open-api/v1/entities/" + id + "/quota-plan")
		if err != nil {
			t.Fatalf("query quota-plan failed: %v", err)
		}
		testutil.AssertSuccess(t, qpResp)
		var qpData map[string]interface{}
		if err := json.Unmarshal(qpResp.Data, &qpData); err != nil {
			t.Fatalf("unmarshal quota-plan data: %v", err)
		}
		balance := qpData["balance"].(map[string]interface{})
		assert.InDelta(t, float64(777.7777), balance["remaining"], 0.00001)
		assert.InDelta(t, float64(0), balance["used"], 0.00001)
	})

	t.Run("E-5-004 部分更新 name 为含空格字符串", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name": "bad name",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 422 {
			t.Errorf("expected ErrNum=422, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Run("E-5-005 部分更新 name 为以 _ 开头", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name": "_badname",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 422 {
			t.Errorf("expected ErrNum=422, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Run("E-5-006 部分更新省略 allow_models/block_models 保持原值（issue #151 回归）", func(t *testing.T) {
		// 创建时显式指定 allow_models/block_models
		createResp, err := testutil.GetClient().Post("/open-api/v1/entities", map[string]interface{}{
			"name":         testutil.UniqueEntityName(),
			"type":         typeName,
			"allow_models": []string{"model-a"},
			"block_models": []string{"model-b"},
		})
		if err != nil {
			t.Fatalf("create entity failed: %v", err)
		}
		testutil.AssertSuccess(t, createResp)
		id, err := testutil.GetDataField(createResp, "id")
		if err != nil {
			t.Fatalf("get id: %v", err)
		}
		patchID := id.(string)
		defer testutil.DeleteEntity(patchID)

		// PATCH 仅修改 block_models，省略 allow_models
		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+patchID, map[string]interface{}{
			"block_models": []string{"model-c"},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		// allow_models 必须保持原值，不得被重置为 []
		detail, err := testutil.GetClient().Get("/open-api/v1/entities/" + patchID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, detail)
		testutil.AssertDataFieldEquals(t, detail, "allow_models", []interface{}{"model-a"})
		testutil.AssertDataFieldEquals(t, detail, "block_models", []interface{}{"model-c"})
	})

	t.Run("E-5-007 部分更新省略 models 时 name 修改生效且模型保持", func(t *testing.T) {
		entityName := testutil.UniqueEntityName()
		createResp, err := testutil.GetClient().Post("/open-api/v1/entities", map[string]interface{}{
			"name":         entityName,
			"type":         typeName,
			"allow_models": []string{"model-a", "model-b"},
		})
		if err != nil {
			t.Fatalf("create entity failed: %v", err)
		}
		testutil.AssertSuccess(t, createResp)
		id, err := testutil.GetDataField(createResp, "id")
		if err != nil {
			t.Fatalf("get id: %v", err)
		}
		patchID := id.(string)
		defer testutil.DeleteEntity(patchID)

		newName := testutil.UniqueEntityName()
		resp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+patchID, map[string]interface{}{
			"name": newName,
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		detail, err := testutil.GetClient().Get("/open-api/v1/entities/" + patchID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, detail)
		testutil.AssertDataFieldEquals(t, detail, "name", newName)
		testutil.AssertDataFieldEquals(t, detail, "allow_models", []interface{}{"model-a", "model-b"})
	})

	t.Cleanup(func() {
		testutil.DeleteEntity(entityID)
		testutil.DeleteEntityType(typeName)
	})
}
