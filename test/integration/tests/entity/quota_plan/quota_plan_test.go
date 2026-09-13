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

func TestEntity_QuotaPlan(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()
	if _, err := testutil.CreateEntityType(typeName, 1); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// 更新为非无限配额
	_, err = testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        1000000,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
	})
	if err != nil {
		t.Fatalf("setup quota failed: %v", err)
	}

	t.Run("E-7-001 查询 Entity 配额计划", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/entities/" + entityID + "/quota-plan")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "unlimited", false)
		testutil.AssertDataFieldNotEmpty(t, resp, "balance")
	})

	t.Run("E-7-002 查询 RMB 配额余额精度", func(t *testing.T) {
		rmbEntityName := testutil.UniqueEntityName()
		rmbEntityID, err := testutil.CreateEntity(rmbEntityName, typeName, "")
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		defer testutil.DeleteEntity(rmbEntityID)

		patchResp, err := testutil.GetClient().Patch("/open-api/v1/entities/"+rmbEntityID, map[string]interface{}{
			"quota_plan": map[string]interface{}{
				"unlimited":    false,
				"quota":        2000.12345678,
				"unit":         "RMB",
				"reset_period": "monthly",
			},
		})
		if err != nil {
			t.Fatalf("setup quota failed: %v", err)
		}
		t.Logf("patch resp: ErrNum=%d ErrMsg=%s Data=%s", patchResp.ErrNum, patchResp.ErrMsg, string(patchResp.Data))

		resp, err := testutil.GetClient().Get("/open-api/v1/entities/" + rmbEntityID + "/quota-plan")
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
		assert.InDelta(t, float64(2000.12345678), balanceMap["remaining"], 0.00000001)
		assert.InDelta(t, float64(0), balanceMap["used"], 0.00000001)
	})

	t.Cleanup(func() {
		testutil.DeleteEntity(entityID)
		testutil.DeleteEntityType(typeName)
	})
}
