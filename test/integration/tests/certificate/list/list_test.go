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
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestCertificate_List(t *testing.T) {
	defaultCert := testutil.UniqueCertName()
	if _, err := testutil.CreateCertificate(defaultCert, true); err != nil {
		t.Fatalf("setup default cert failed: %v", err)
	}
	certName := testutil.UniqueCertName()
	if _, err := testutil.CreateCertificate(certName, false); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	resp, err := testutil.GetClient().Get("/open-api/v1/certificates")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	testutil.AssertSuccess(t, resp)

	var list []interface{}
	if err := json.Unmarshal(resp.Data, &list); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	assert.GreaterOrEqual(t, len(list), 1)
	for _, item := range list {
		cert := item.(map[string]interface{})
		assert.NotEmpty(t, cert["cert_name"])
		assert.NotContains(t, cert, "cert_file_content")
		assert.NotContains(t, cert, "key_file_content")
	}

	t.Cleanup(func() {
		testutil.DeleteCertificate(certName)
		testutil.DeleteCertificate(defaultCert)
	})
}
