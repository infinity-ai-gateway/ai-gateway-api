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

package route_tables_test

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

func hasType(list []interface{}, typ string) bool {
	for _, item := range list {
		if m, ok := item.(map[string]interface{}); ok && m["type"] == typ {
			return true
		}
	}
	return false
}

func TestRouteTables_List(t *testing.T) {
	t.Run("RT-1-012 启动后默认 global 路由表存在", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"type": "global"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertListFieldLen(t, resp, "list", 1)

		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		item := data["list"].([]interface{})[0].(map[string]interface{})
		assert.Equal(t, "global", item["type"])
		assert.Equal(t, "global", item["owner"])
		assert.Equal(t, false, item["enabled"])
	})

	// 准备数据
	if _, err := testutil.GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
		"rules": []interface{}{
			map[string]interface{}{
				"name":      "global-default",
				"cond":      "default_t()",
				"targets":   []interface{}{map[string]interface{}{"cluster_name": "cluster_global", "weight": 100}},
				"fallbacks": []interface{}{},
			},
		},
	}); err != nil {
		t.Fatalf("setup global route failed: %v", err)
	}

	typeName := testutil.UniqueEntityTypeName()
	if _, err := testutil.CreateEntityType(typeName, 1); err != nil {
		t.Fatalf("setup entity type failed: %v", err)
	}
	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	if err != nil {
		t.Fatalf("setup entity failed: %v", err)
	}
	apiKeyID, err := testutil.CreateAPIKey(testutil.UniqueAPIKeyDesc(), "")
	if err != nil {
		t.Fatalf("setup api-key failed: %v", err)
	}

	t.Run("RT-1-001 无参数查询路由表列表", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}
		list := data["list"].([]interface{})
		assert.GreaterOrEqual(t, len(list), 3)
		assert.True(t, hasType(list, "global"))
		assert.True(t, hasType(list, "entity"))
		assert.True(t, hasType(list, "api_key"))
	})

	t.Run("RT-1-002 按 type=global 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"type": "global"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		for _, item := range data["list"].([]interface{}) {
			assert.Equal(t, "global", item.(map[string]interface{})["type"])
		}
	})

	t.Run("RT-1-003 按 type=entity 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"type": "entity"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		for _, item := range data["list"].([]interface{}) {
			assert.Equal(t, "entity", item.(map[string]interface{})["type"])
		}
	})

	t.Run("RT-1-004 按 type=api_key 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"type": "api_key"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		for _, item := range data["list"].([]interface{}) {
			assert.Equal(t, "api_key", item.(map[string]interface{})["type"])
		}
	})

	t.Run("RT-1-005 按 owner 精确匹配过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"owner": apiKeyID})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		list := data["list"].([]interface{})
		assert.GreaterOrEqual(t, len(list), 1, "按 apiKeyID 过滤应至少返回一条 api_key 路由表")
		for _, item := range list {
			assert.Equal(t, apiKeyID, item.(map[string]interface{})["owner"])
		}
	})

	t.Run("RT-1-011 按不存在的 owner 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"owner": "non-existent-owner"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		list := data["list"].([]interface{})
		assert.Empty(t, list, "按不存在的 owner 过滤应返回空列表")
	})

	t.Run("RT-1-006 按 enabled 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"enabled": "true"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		var data map[string]interface{}
		json.Unmarshal(resp.Data, &data)
		for _, item := range data["list"].([]interface{}) {
			assert.Equal(t, true, item.(map[string]interface{})["enabled"])
		}
	})

	t.Run("RT-1-007 分页参数边界", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{
			"page":      "1",
			"page_size": "1",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertListFieldLen(t, resp, "list", 1)
		testutil.AssertPagination(t, resp, 1, 1, 3)
	})

	t.Run("RT-1-008 page_size 超过最大值", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"page_size": "101"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 200 && resp.ErrNum != 422 {
			t.Errorf("expected ErrNum=200 or 422, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Run("RT-1-009 非法 type 值", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/route-tables", map[string]string{"type": "unknown"})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 200 && resp.ErrNum != 422 {
			t.Errorf("expected ErrNum=200 or 422, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Run("RT-1-010 空列表返回", func(t *testing.T) {
		// 需要全新数据库，跳过
		t.Skip("requires fresh database to verify empty list")
	})

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
		testutil.DeleteEntity(entityID)
		testutil.DeleteEntityType(typeName)
	})
}
