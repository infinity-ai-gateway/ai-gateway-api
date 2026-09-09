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
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
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

// startModelServer starts a local HTTP server that returns the given body for every request.
// It returns the host and port to be used as addr/port in a discover-models request.
func startModelServer(t *testing.T, body string) (string, int) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)

	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatalf("parse server url: %v", err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatalf("parse server port: %v", err)
	}
	return u.Hostname(), port
}

func TestProvider_DiscoverModels(t *testing.T) {
	t.Run("PV-6-001 OpenAI 协议模型发现", func(t *testing.T) {
		host, port := startModelServer(t, `{"data":[{"id":"m1"},{"id":"m2"}]}`)

		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "openai",
			"schema":         "http",
			"addr":           host,
			"port":           port,
			"uri":            "/v1/models",
			"apikey":         "sk-xxx",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "models", []interface{}{"m1", "m2"})
	})

	t.Run("PV-6-002 Anthropic 协议模型发现", func(t *testing.T) {
		host, port := startModelServer(t, `{"models":[{"model_id":"claude-3-opus-20240229","display_name":"Claude 3 Opus"}]}`)

		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "anthropic",
			"schema":         "http",
			"addr":           host,
			"port":           port,
			"uri":            "/v1/models",
			"apikey":         "sk-xxx",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "models", []interface{}{"claude-3-opus-20240229"})
	})

	t.Run("PV-6-002b Gemini 协议模型发现", func(t *testing.T) {
		host, port := startModelServer(t, `{"models":[{"name":"models/gemini-2.5-pro"},{"name":"models/gemini-2.5-flash"}]}`)

		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "gemini",
			"schema":         "http",
			"addr":           host,
			"port":           port,
			"apikey":         "AIzaSyxxx",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "models", []interface{}{"gemini-2.5-pro", "gemini-2.5-flash"})
	})

	t.Run("PV-6-003 URI 为空时默认使用 /v1/models", func(t *testing.T) {
		host, port := startModelServer(t, `{"data":[{"id":"m1"}]}`)

		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "openai",
			"schema":         "http",
			"addr":           host,
			"port":           port,
			"apikey":         "sk-xxx",
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "models", []interface{}{"m1"})
	})

	t.Run("PV-6-004 缺少必填参数", func(t *testing.T) {
		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "openai",
			"schema":         "http",
			"port":           8080,
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PV-6-005 非法 model_protocol", func(t *testing.T) {
		host, port := startModelServer(t, `{"data":[]}`)

		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "unknown",
			"schema":         "http",
			"addr":           host,
			"port":           port,
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertErrCode(t, resp, 422)
	})

	t.Run("PV-6-006 不带 apikey", func(t *testing.T) {
		host, port := startModelServer(t, `{"data":[{"id":"m1"}]}`)

		resp, err := testutil.GetClient().Post("/open-api/v1/providers/tools/discover-models", map[string]interface{}{
			"model_protocol": "openai",
			"schema":         "http",
			"addr":           host,
			"port":           port,
		})
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		testutil.AssertSuccess(t, resp)
		testutil.AssertDataFieldEquals(t, resp, "models", []interface{}{"m1"})
	})
}
