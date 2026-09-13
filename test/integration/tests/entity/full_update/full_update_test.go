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

func TestEntity_FullUpdate(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()
	if _, err := testutil.CreateEntityType(typeName, 1); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	otherName := testutil.UniqueEntityName()
	_, err = testutil.CreateEntity(otherName, typeName, "")
	if err != nil {
		t.Fatalf("setup other failed: %v", err)
	}

	t.Run("E-4-001 全量更新 Entity name", func(t *testing.T) {
		newName := testutil.UniqueEntityName()
		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         newName,
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", newName)
		testutil.AssertDataFieldEquals(t, resp, "type", typeName)
	})

	t.Run("E-4-002 全量更新后查询一致性", func(t *testing.T) {
		newName := testutil.UniqueEntityName()
		_, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         newName,
			"type":         typeName,
			"allow_models": []string{"gpt-4"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		resp, err := testutil.GetClient().Get("/open-api/v1/entities/" + entityID)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "name", newName)
		testutil.AssertDataFieldEquals(t, resp, "allow_models", []interface{}{"gpt-4"})
	})

	t.Run("E-4-003 全量更新冲突 name", func(t *testing.T) {
		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         otherName,
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 555 && resp.ErrNum != 556 && resp.ErrNum != 500 {
			t.Errorf("expected conflict error, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Run("E-4-004 全量更新修改 type", func(t *testing.T) {
		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         testutil.UniqueEntityName(),
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "type", typeName)
	})

	t.Run("E-4-005 全量更新非法 name（含首尾空白）", func(t *testing.T) {
		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         " badname ",
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("E-4-006 全量更新 quota_plan 切换为 RMB", func(t *testing.T) {
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

		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+id, map[string]interface{}{
			"name":         testutil.UniqueEntityName(),
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan": map[string]interface{}{
				"unlimited":    false,
				"quota":        1234.56,
				"unit":         "RMB",
				"reset_period": "monthly",
			},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
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
		assert.InDelta(t, float64(1234.56), qp["quota"], 0.00001)

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
		assert.InDelta(t, float64(1234.56), balance["remaining"], 0.00001)
		assert.InDelta(t, float64(0), balance["used"], 0.00001)
	})

	t.Run("E-4-003 全量更新 name 为含大写字母", func(t *testing.T) {
		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         "BadName",
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 422 {
			t.Errorf("expected ErrNum=422, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Run("E-4-004 全量更新 name 以 - 结尾", func(t *testing.T) {
		resp, err := testutil.GetClient().Put("/open-api/v1/entities/"+entityID, map[string]interface{}{
			"name":         "badname-",
			"type":         typeName,
			"allow_models": []string{"*"},
			"block_models": []string{},
			"quota_plan":   map[string]interface{}{"unlimited": true},
			"rate_limit_policy": map[string]interface{}{
				"enabled": false,
			},
			"route_rules": map[string]interface{}{
				"enabled": false,
				"rules":   []interface{}{},
			},
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 422 {
			t.Errorf("expected ErrNum=422, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Cleanup(func() {
		testutil.DeleteEntity(entityID)
		testutil.DeleteEntityType(typeName)
	})
}
