package model_price_test

import (
	"fmt"
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

func TestModelPrice_Delete(t *testing.T) {
	t.Run("MP-8-001 按 id 删除存在的记录", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := "delete-model"
		id, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		resp, err := testutil.GetClient().Delete("/open-api/v1/model-prices/" + fmt.Sprintf("%d", id))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataNull(t, resp)

		getResp, err := testutil.GetClient().Get("/open-api/v1/model-prices/" + fmt.Sprintf("%d", id))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		testutil.AssertErrCode(t, getResp, 404)
	})

	t.Run("MP-8-002 按 id 删除不存在的记录", func(t *testing.T) {
		resp, err := testutil.GetClient().Delete("/open-api/v1/model-prices/999999999")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("MP-9-001 按组合键删除存在的记录", func(t *testing.T) {
		provider := testutil.UniqueName("provider")
		model := "delete-query-model"
		id, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   provider,
			"model":      model,
			"base_model": model,
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		resp, err := testutil.GetClient().DeleteWithQuery("/open-api/v1/model-prices", map[string]string{
			"provider": provider,
			"model":    model,
			"mode":     "chat",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataNull(t, resp)

		getResp, err := testutil.GetClient().Get("/open-api/v1/model-prices/" + fmt.Sprintf("%d", id))
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		testutil.AssertErrCode(t, getResp, 404)
	})

	t.Run("MP-9-002 按组合键删除缺少 query 参数", func(t *testing.T) {
		resp, err := testutil.GetClient().DeleteWithQuery("/open-api/v1/model-prices", map[string]string{
			"provider": testutil.UniqueName("provider"),
			"model":    "m",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})
}
