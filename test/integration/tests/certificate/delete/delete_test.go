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

package certificate_test

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

func TestCertificate_Delete(t *testing.T) {
	defaultCert := testutil.UniqueCertName()
	if _, err := testutil.CreateCertificate(defaultCert, true); err != nil {
		t.Fatalf("setup default cert failed: %v", err)
	}

	t.Run("CERT-6-001 删除非默认证书", func(t *testing.T) {
		certName := testutil.UniqueCertName()
		if _, err := testutil.CreateCertificate(certName, false); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		resp, err := testutil.GetClient().Delete("/open-api/v1/certificates/" + certName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		resp, _ = testutil.GetClient().Get("/open-api/v1/certificates/" + certName)
		testutil.AssertErrCode(t, resp, 404)
	})

	t.Run("CERT-6-002 删除默认证书", func(t *testing.T) {
		certName := testutil.UniqueCertName()
		if _, err := testutil.CreateCertificate(certName, true); err != nil {
			t.Fatalf("setup failed: %v", err)
		}
		resp, err := testutil.GetClient().Delete("/open-api/v1/certificates/" + certName)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		if resp.ErrNum != 409 && resp.ErrNum != 422 && resp.ErrNum != 500 {
			t.Errorf("expected ErrNum=409/422/500, got ErrNum=%d, ErrMsg=%s", resp.ErrNum, resp.ErrMsg)
		}
		t.Cleanup(func() {
			testutil.DeleteCertificate(certName)
		})
	})

	t.Cleanup(func() {
		testutil.DeleteCertificate(defaultCert)
	})
}
