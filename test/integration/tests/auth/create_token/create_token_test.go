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

package auth_test

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

func TestAuth_CreateToken(t *testing.T) {
	tokenDup := testutil.UniqueTokenName()
	if _, err := testutil.CreateToken(tokenDup, "System"); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	tests := []struct {
		name     string
		body     map[string]interface{}
		wantCode int
	}{
		{
			name:     "AUTH-9-001 创建 System Token",
			body:     map[string]interface{}{"name": testutil.UniqueTokenName(), "scope": "System"},
			wantCode: 200,
		},
		{
			name:     "AUTH-9-002 创建 Support Token",
			body:     map[string]interface{}{"name": testutil.UniqueTokenName(), "scope": "Support"},
			wantCode: 200,
		},
		{
			name:     "AUTH-9-003 创建 Token 缺少 name",
			body:     map[string]interface{}{"scope": "System"},
			wantCode: 422,
		},
		{
			name:     "AUTH-9-004 创建 Token 非法 scope",
			body:     map[string]interface{}{"name": testutil.UniqueTokenName(), "scope": "Product"},
			wantCode: 422,
		},
		{
			name:     "AUTH-9-005 重复创建 Token",
			body:     map[string]interface{}{"name": tokenDup, "scope": "System"},
			wantCode: 555,
		},
		{
			name:     "AUTH-9-006 非法 token name（以 . 开头）",
			body:     map[string]interface{}{"name": ".badtoken", "scope": "System"},
			wantCode: 422,
		},
		{
			name:     "AUTH-9-007 保留 token name（default）",
			body:     map[string]interface{}{"name": "default", "scope": "System"},
			wantCode: 422,
		},
		{
			name:     "AUTH-9-008 token name 包含空白",
			body:     map[string]interface{}{"name": "bad token", "scope": "System"},
			wantCode: 422,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := testutil.GetClient().Post("/open-api/v1/auth/tokens", tt.body)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.ErrNum != tt.wantCode {
				t.Errorf("expected ErrNum=%d, got ErrNum=%d, ErrMsg=%s", tt.wantCode, resp.ErrNum, resp.ErrMsg)
			}
		})
	}

	t.Cleanup(func() {
		testutil.DeleteToken(tokenDup)
	})
}
