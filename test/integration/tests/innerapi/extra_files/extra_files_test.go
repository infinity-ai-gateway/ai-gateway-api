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

package innerapi_test

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

func TestInnerAPI_ExtraFiles(t *testing.T) {
	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/extra_files/name_conf.data")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	// 该文件可能不存在，接受 200 或 404
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		t.Errorf("expected ErrNum=200 or 404, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
	}
}
