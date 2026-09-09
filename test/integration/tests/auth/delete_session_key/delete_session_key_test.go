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

func TestAuth_DeleteSessionKey(t *testing.T) {
	userName := testutil.UniqueUserName()
	password := "password@123"
	if err := testutil.CreateUser(userName, password); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	createResp, err := testutil.GetClient().Post("/open-api/v1/auth/session-keys", map[string]interface{}{
		"user_name": userName,
		"password":  password,
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	sessionKey, _ := testutil.GetDataField(createResp, "session_key")

	t.Run("AUTH-8-001 删除 Session Key", func(t *testing.T) {
		resp, err := testutil.GetClient().Delete("/open-api/v1/auth/session-keys/" + sessionKey.(string))
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
	})

	t.Run("AUTH-8-002 删除不存在 Session Key", func(t *testing.T) {
		resp, err := testutil.GetClient().Delete("/open-api/v1/auth/session-keys/non_existent_key")
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 404 && resp.ErrNum != 401 {
			t.Errorf("expected ErrNum=404 or 401, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
	})

	t.Cleanup(func() {
		testutil.DeleteUser(userName)
	})
}
