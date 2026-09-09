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

func TestProvider_Delete(t *testing.T) {
	t.Run("PV-5-001 删除 Provider", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		if _, err := testutil.CreateProvider(providerName); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		resp, err := testutil.GetClient().Delete("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("PV-5-002 删除不存在的 Provider", func(t *testing.T) {
		resp, err := testutil.GetClient().Delete("/open-api/v1/providers/non_existent_provider")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("PV-5-003 删除被 Cluster 引用的 Provider", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		if _, err := testutil.CreateProvider(providerName); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		clusterName := testutil.UniqueClusterName()
		resp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
			"name": clusterName,
			"llm_config": map[string]interface{}{
				"models":   []string{"deepseek-chat"},
				"provider": providerName,
			},
		})
		if err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		if resp.ErrNum != 200 {
			t.Fatalf("create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
		}

		resp, err = testutil.GetClient().Delete("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 409 {
			t.Errorf("expected ErrNum=409, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}

		t.Cleanup(func() {
			testutil.DeleteCluster(clusterName)
			testutil.DeleteProvider(providerName)
		})
	})

	t.Run("PV-5-004 删除被 ModelPrice 引用的 Provider", func(t *testing.T) {
		providerName := testutil.UniqueProviderName()
		if _, err := testutil.CreateProvider(providerName); err != nil {
			t.Fatalf("setup failed: %v", err)
		}

		id, err := testutil.CreateModelPrice(map[string]interface{}{
			"provider":   providerName,
			"model":      "test-model",
			"base_model": "test-model",
			"mode":       "chat",
			"prices": map[string]interface{}{
				"input_cost_per_token": 0.001,
			},
		})
		if err != nil {
			t.Fatalf("setup model price failed: %v", err)
		}

		resp, err := testutil.GetClient().Delete("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)

		resp, err = testutil.GetClient().Get("/open-api/v1/providers/" + providerName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 404)

		t.Cleanup(func() {
			testutil.DeleteModelPrice(id)
		})
	})
}
