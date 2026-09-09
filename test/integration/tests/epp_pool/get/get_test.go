package epp_pool_test

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

func patchEppPool(t *testing.T, body map[string]interface{}) *testutil.APIResponse {
	resp, err := testutil.GetClient().Patch("/open-api/v1/epp-pool", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func TestEppPool_Get(t *testing.T) {
	t.Run("EP-1-001 池不存在返回 404 语义错误", func(t *testing.T) {
		// 契约 epp-pool.md §2.1：池由首次 PATCH 创建，从未配置过时返回错误。
		resp, err := testutil.GetClient().Get("/open-api/v1/epp-pool")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("EP-1-002 配置后回读组与实例字段", func(t *testing.T) {
		resp := patchEppPool(t, map[string]interface{}{
			"groups": []interface{}{
				map[string]interface{}{
					"name": "g1",
					"instances": []interface{}{
						map[string]interface{}{"id": "epp-b", "host": "10.0.0.2", "port": 9002},
						map[string]interface{}{"id": "epp-a", "host": "10.0.0.1", "port": 9002},
					},
				},
				map[string]interface{}{
					"name": "g2",
					"instances": []interface{}{
						map[string]interface{}{"id": "epp-c", "host": "10.0.0.3", "port": 9002},
					},
				},
			},
		})
		testutil.AssertSuccess(t, resp)

		// 响应体即池详情：组按名称排序、组内实例按 id 排序
		var data map[string]interface{}
		if err := json.Unmarshal(resp.Data, &data); err != nil {
			t.Fatalf("unmarshal data failed: %v", err)
		}
		assert.Equal(t, "EPP.pool", data["name"])
		groups := data["groups"].([]interface{})
		require.Len(t, groups, 2)

		g1 := groups[0].(map[string]interface{})
		assert.Equal(t, "g1", g1["name"])
		insts := g1["instances"].([]interface{})
		require.Len(t, insts, 2)
		inst0 := insts[0].(map[string]interface{})
		assert.Equal(t, "epp-a", inst0["id"])
		assert.Equal(t, "10.0.0.1", inst0["host"])
		assert.Equal(t, float64(9002), inst0["port"])
		inst1 := insts[1].(map[string]interface{})
		assert.Equal(t, "epp-b", inst1["id"])
		assert.Equal(t, "10.0.0.2", inst1["host"])

		g2 := groups[1].(map[string]interface{})
		assert.Equal(t, "g2", g2["name"])
		insts2 := g2["instances"].([]interface{})
		require.Len(t, insts2, 1)

		// GET 回读与 PATCH 响应一致
		getResp, err := testutil.GetClient().Get("/open-api/v1/epp-pool")
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		testutil.AssertSuccess(t, getResp)
		assert.Equal(t, string(resp.Data), string(getResp.Data), "GET should match PATCH response")
	})
}
