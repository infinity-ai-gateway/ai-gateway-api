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

func minProviderBody(name string) map[string]interface{} {
	return map[string]interface{}{
		"name": name,
		"instance_pool": []interface{}{
			map[string]interface{}{
				
				"addr":   "10.0.0.1",
				"weight": 100,
				"port":   8080,
			},
		},
		"model_protocols": []string{"openai"},
	}
}

func TestProvider_Create(t *testing.T) {
	providerMin := testutil.UniqueProviderName()
	providerFull := testutil.UniqueProviderName()
	providerAnthropic := testutil.UniqueProviderName()
	providerNoInstName := testutil.UniqueProviderName()
	providerDup := testutil.UniqueProviderName()

	tests := []struct {
		name     string
		body     map[string]interface{}
		wantCode int
		check    func(t *testing.T, resp *testutil.APIResponse)
	}{
		{
			name:     "PV-1-001 最小参数创建 Provider",
			body:     minProviderBody(providerMin),
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				testutil.AssertDataFieldEquals(t, resp, "name", providerMin)
				testutil.AssertDataFieldEquals(t, resp, "description", "")
				var data map[string]interface{}
				json.Unmarshal(resp.Data, &data)
				models, _ := data["models"].([]interface{})
				assert.Empty(t, models)
				keys, _ := data["keys"].([]interface{})
				assert.Empty(t, keys)
			},
		},
		{
			name: "PV-1-002 完整参数创建 Provider",
			body: map[string]interface{}{
				"name":        providerFull,
				"description": "完整 Provider",
				"model_endpoint": map[string]interface{}{
					"schema": "https",
					"uri":    "/v1/models",
				},
				"models": []string{"deepseek-chat", "deepseek-coder"},
				"keys": []interface{}{
					map[string]interface{}{
						"name": "key-primary",
						"key":  "sk-aaaaaaaaaaaa",
					},
					map[string]interface{}{
						"name": "key-secondary",
						"key":  "sk-bbbbbbbbbbbb",
					},
				},
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "api.deepseek.com",
						"weight": 100,
						"port":   443,
					},
				},
				"model_protocols": []string{"openai"},
			},
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				testutil.AssertDataFieldEquals(t, resp, "name", providerFull)
				testutil.AssertDataFieldEquals(t, resp, "description", "完整 Provider")
				var data map[string]interface{}
				json.Unmarshal(resp.Data, &data)
				models, _ := data["models"].([]interface{})
				assert.Len(t, models, 2)
				keys, _ := data["keys"].([]interface{})
				assert.Len(t, keys, 2)
				insts, _ := data["instance_pool"].([]interface{})
				assert.Len(t, insts, 1)
			},
		},
		{
			name: "PV-1-002a 创建 anthropic 协议 Provider",
			body: map[string]interface{}{
				"name": providerAnthropic,
				"keys": []interface{}{
					map[string]interface{}{
						"name": "key-primary",
						"key":  "sk-ant-aaaaaaaaaaaa",
					},
				},
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "api.anthropic.com",
						"weight": 100,
						"port":   443,
					},
				},
				"model_protocols": []string{"anthropic"},
			},
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				var data map[string]interface{}
				json.Unmarshal(resp.Data, &data)
				protocols, _ := data["model_protocols"].([]interface{})
				assert.Equal(t, []interface{}{"anthropic"}, protocols)
			},
		},
		{
			name: "PV-1-002b instance_pool 不再包含 name 字段",
			body: map[string]interface{}{
				"name": providerNoInstName,
				"instance_pool": []interface{}{
					map[string]interface{}{
						"addr":   "10.0.0.99",
						"weight": 100,
						"port":   8080,
					},
				},
				"model_protocols": []string{"openai"},
			},
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				var data map[string]interface{}
				json.Unmarshal(resp.Data, &data)
				insts, _ := data["instance_pool"].([]interface{})
				assert.Len(t, insts, 1)
				inst, _ := insts[0].(map[string]interface{})
				_, ok := inst["name"]
				assert.False(t, ok, "instance should not contain name field")
				assert.Equal(t, "10.0.0.99", inst["addr"])
			},
		},
		{
			name:     "PV-1-003 重复 Provider 名称",
			body:     minProviderBody(providerDup),
			wantCode: 555,
		},
		{
			name: "PV-1-004 缺少 instance_pool",
			body: map[string]interface{}{
				"name":            testutil.UniqueProviderName(),
				"model_protocols": []string{"openai"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-005 缺少 model_protocols",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 100,
						"port":   8080,
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-006 非法 model_protocols",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 100,
						"port":   8080,
					},
				},
				"model_protocols": []string{"invalid"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-007 instance_pool 为空数组",
			body: map[string]interface{}{
				"name":            testutil.UniqueProviderName(),
				"instance_pool":   []interface{}{},
				"model_protocols": []string{"openai"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-008 实例 port 非法",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 100,
						"port":   0,
					},
				},
				"model_protocols": []string{"openai"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-009 非法 name",
			body: map[string]interface{}{
				"name": "-bad-name-",
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 100,
						"port":   8080,
					},
				},
				"model_protocols": []string{"openai"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-010 weight 超过 100",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 101,
						"port":   8080,
					},
				},
				"model_protocols": []string{"openai"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-011 重复实例 (addr+port)",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 50,
						"port":   8080,
					},
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 50,
						"port":   8080,
					},
				},
				"model_protocols": []string{"openai"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-012 模型重复",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 100,
						"port":   8080,
					},
				},
				"model_protocols": []string{"openai"},
				"models":          []string{"m", "m"},
			},
			wantCode: 422,
		},
		{
			name: "PV-1-013 keys 中存在重复 name",
			body: map[string]interface{}{
				"name": testutil.UniqueProviderName(),
				"instance_pool": []interface{}{
					map[string]interface{}{
						
						"addr":   "10.0.0.1",
						"weight": 100,
						"port":   8080,
					},
				},
				"model_protocols": []string{"openai"},
				"keys": []interface{}{
					map[string]interface{}{"name": "same", "key": "sk-1"},
					map[string]interface{}{"name": "same", "key": "sk-2"},
				},
			},
			wantCode: 422,
		},
	}

	// 预先创建重复 Provider
	if _, err := testutil.GetClient().Post("/open-api/v1/providers", minProviderBody(providerDup)); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := testutil.GetClient().Post("/open-api/v1/providers", tt.body)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			if resp.ErrNum != tt.wantCode {
				t.Errorf("expected ErrNum=%d, got ErrNum=%d, ErrMsg=%s", tt.wantCode, resp.ErrNum, resp.ErrMsg)
			}
			if tt.check != nil && resp.ErrNum == 200 {
				tt.check(t, resp)
			}
		})
	}

	t.Cleanup(func() {
		testutil.DeleteProvider(providerMin)
		testutil.DeleteProvider(providerFull)
		testutil.DeleteProvider(providerAnthropic)
		testutil.DeleteProvider(providerNoInstName)
		testutil.DeleteProvider(providerDup)
	})
}
