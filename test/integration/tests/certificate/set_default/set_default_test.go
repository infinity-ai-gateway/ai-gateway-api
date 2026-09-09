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

func TestCertificate_SetDefault(t *testing.T) {
	defaultCert := testutil.UniqueCertName()
	if _, err := testutil.CreateCertificate(defaultCert, true); err != nil {
		t.Fatalf("setup default cert failed: %v", err)
	}
	certA := testutil.UniqueCertName()
	certB := testutil.UniqueCertName()
	if _, err := testutil.CreateCertificate(certA, true); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if _, err := testutil.CreateCertificate(certB, false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	t.Run("CERT-5-001 设置默认证书", func(t *testing.T) {
		resp, err := testutil.GetClient().Patch("/open-api/v1/certificates/"+certB+"/default", map[string]interface{}{})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "cert_name", certB)
		testutil.AssertDataFieldEquals(t, resp, "is_default", true)

		oldResp, err := testutil.GetClient().Get("/open-api/v1/certificates/" + certA)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, oldResp)
		testutil.AssertDataFieldEquals(t, oldResp, "is_default", false)
	})

	t.Cleanup(func() {
		testutil.DeleteCertificate(certA)
		testutil.DeleteCertificate(certB)
		testutil.DeleteCertificate(defaultCert)
	})
}
