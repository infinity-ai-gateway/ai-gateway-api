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

package provider_test

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

type providerListResponse struct {
	List       []map[string]interface{} `json:"list"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}

func TestProvider_List(t *testing.T) {
	providerA := testutil.UniqueProviderName()
	providerB := testutil.UniqueProviderName()
	providerC := testutil.UniqueProviderName()

	_, err := testutil.CreateProvider(providerA)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_, err = testutil.CreateProvider(providerB, map[string]interface{}{
		"model_protocols": []string{"anthropic"},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_, err = testutil.CreateProvider(providerC, map[string]interface{}{
		"model_protocols": []string{"gemini"},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("PV-2-001 无分页参数返回全部", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var list providerListResponse
		if err := json.Unmarshal(resp.Data, &list); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		assert.GreaterOrEqual(t, list.Pagination.Total, int64(2))
		assert.Len(t, list.List, int(list.Pagination.Total))
	})

	t.Run("PV-2-002 自定义分页", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers", map[string]string{
			"page":      "1",
			"page_size": "1",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var list providerListResponse
		if err := json.Unmarshal(resp.Data, &list); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		assert.GreaterOrEqual(t, list.Pagination.Total, int64(2))
		assert.Len(t, list.List, 1)
	})

	t.Run("PV-2-003 按 model_protocol 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers", map[string]string{
			"model_protocol": "openai",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var list providerListResponse
		if err := json.Unmarshal(resp.Data, &list); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		assert.GreaterOrEqual(t, list.Pagination.Total, int64(1))
		for _, item := range list.List {
			protocols, ok := item["model_protocols"].([]interface{})
			assert.True(t, ok, "model_protocols should be an array")
			found := false
			for _, p := range protocols {
				if p == "openai" {
					found = true
					break
				}
			}
			assert.True(t, found, "each returned provider must contain openai in model_protocols")
		}
	})

	t.Run("PV-2-004 按 model_protocol=gemini 过滤", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers", map[string]string{
			"model_protocol": "gemini",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		var list providerListResponse
		if err := json.Unmarshal(resp.Data, &list); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		assert.GreaterOrEqual(t, list.Pagination.Total, int64(1))
		for _, item := range list.List {
			protocols, ok := item["model_protocols"].([]interface{})
			assert.True(t, ok, "model_protocols should be an array")
			found := false
			for _, p := range protocols {
				if p == "gemini" {
					found = true
					break
				}
			}
			assert.True(t, found, "each returned provider must contain gemini in model_protocols")
		}
	})

	t.Cleanup(func() {
		testutil.DeleteProvider(providerA)
		testutil.DeleteProvider(providerB)
		testutil.DeleteProvider(providerC)
	})
}
