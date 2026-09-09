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

func TestProvider_ListNames(t *testing.T) {
	providerA := testutil.UniqueProviderName()
	providerB := testutil.UniqueProviderName()

	if _, err := testutil.CreateProvider(providerA); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := testutil.CreateProvider(providerB); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("PV-7-001 获取所有 Provider 名称列表", func(t *testing.T) {
		resp, err := testutil.GetClient().Get("/open-api/v1/providers/actions/get-provider-names")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldNotEmpty(t, resp, "names")
	})

	t.Cleanup(func() {
		testutil.DeleteProvider(providerA)
		testutil.DeleteProvider(providerB)
	})
}
