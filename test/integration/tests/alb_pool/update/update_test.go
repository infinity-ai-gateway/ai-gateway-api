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

package alb_pool_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func patchAlbPool(t *testing.T, body map[string]interface{}) *testutil.APIResponse {
	resp, err := testutil.GetClient().Patch("/open-api/v1/alb-pool", body)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	return resp
}

func restoreDefaultPool(t *testing.T) {
	patchAlbPool(t, map[string]interface{}{
		"instances": []interface{}{
			map[string]interface{}{
				"hostname": "127.0.0.1",
				"ip":       "127.0.0.1",
				"weight":   1,
				"ports": map[string]interface{}{
					"Default": 8080,
				},
			},
		},
	})
}

func TestAlbPool_Update(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]interface{}
		wantCode int
		skip     string
		check    func(t *testing.T, resp *testutil.APIResponse)
	}{
		{
			name: "BP-2-001 更新实例列表",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "127.0.0.1",
						"ip":       "127.0.0.1",
						"weight":   1,
						"ports": map[string]interface{}{
							"Default": 8080,
						},
					},
				},
			},
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				testutil.AssertDataFieldNotEmpty(t, resp, "name")
				var data map[string]interface{}
				json.Unmarshal(resp.Data, &data)
				insts := data["instances"].([]interface{})
				assert.Len(t, insts, 1)
				inst := insts[0].(map[string]interface{})
				assert.Equal(t, "127.0.0.1", inst["hostname"])
				assert.Equal(t, "127.0.0.1", inst["ip"])
				assert.NotContains(t, inst, "tags")
			},
		},
		{
			name: "BP-2-002 更新后查询一致性",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "host-a",
						"ip":       "10.0.0.1",
						"weight":   50,
						"ports": map[string]interface{}{
							"Default": 8090,
						},
					},
					map[string]interface{}{
						"hostname": "host-b",
						"ip":       "10.0.0.2",
						"weight":   50,
						"ports": map[string]interface{}{
							"Default": 8091,
						},
					},
				},
			},
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				getResp, err := testutil.GetClient().Get("/open-api/v1/alb-pool")
				if err != nil {
					t.Fatalf("get failed: %v", err)
				}
				testutil.AssertSuccess(t, getResp)
				assert.Equal(t, string(resp.Data), string(getResp.Data), "GET should match PATCH response")
			},
		},
		{
			name:     "BP-2-003 更新为空列表",
			body:     map[string]interface{}{"instances": []interface{}{}},
			wantCode: 422,
		},
		{
			name:     "BP-2-004 缺少 instances",
			body:     map[string]interface{}{},
			wantCode: 422,
		},
		{
			name: "BP-2-005 缺少实例必填字段",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "127.0.0.1",
						"weight":   1,
						"ports": map[string]interface{}{
							"Default": 8080,
						},
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "BP-2-006 ports 不含 Default",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "127.0.0.1",
						"ip":       "127.0.0.1",
						"weight":   1,
						"ports": map[string]interface{}{
							"Other": 8080,
						},
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "BP-2-007 实例权重超出范围",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "127.0.0.1",
						"ip":       "127.0.0.1",
						"weight":   101,
						"ports": map[string]interface{}{
							"Default": 8080,
						},
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "BP-2-008 非法 hostname",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "-bad",
						"ip":       "127.0.0.1",
						"weight":   1,
						"ports": map[string]interface{}{
							"Default": 8080,
						},
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "BP-2-009 非法 IP",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "host-a",
						"ip":       "not-an-ip",
						"weight":   1,
						"ports": map[string]interface{}{
							"Default": 8080,
						},
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "BP-2-010 重复端口值",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "host-a",
						"ip":       "10.0.0.1",
						"weight":   1,
						"ports": map[string]interface{}{
							"Default": 8080,
							"Admin":   8080,
						},
					},
				},
			},
			wantCode: 422,
		},
		{
			name: "BP-2-011 weight 为 0 时按默认值 1 处理",
			body: map[string]interface{}{
				"instances": []interface{}{
					map[string]interface{}{
						"hostname": "host-a",
						"ip":       "10.0.0.1",
						"weight":   0,
						"ports": map[string]interface{}{
							"Default": 8080,
						},
					},
				},
			},
			wantCode: 200,
			check: func(t *testing.T, resp *testutil.APIResponse) {
				var data map[string]interface{}
				json.Unmarshal(resp.Data, &data)
				insts := data["instances"].([]interface{})
				require.Len(t, insts, 1)
				inst := insts[0].(map[string]interface{})
				assert.Equal(t, float64(1), inst["weight"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			resp := patchAlbPool(t, tt.body)
			if resp.ErrNum != tt.wantCode {
				t.Errorf("expected ErrNum=%d, got ErrNum=%d, ErrMsg=%s", tt.wantCode, resp.ErrNum, resp.ErrMsg)
			}
			if tt.check != nil {
				tt.check(t, resp)
			}
			restoreDefaultPool(t)
		})
	}
}
