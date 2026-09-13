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
	"encoding/json"
	"fmt"
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

func TestInnerAPI_RateLimitPolicy(t *testing.T) {
	apiKeyID, err := testutil.CreateAPIKey("rate-limit-key", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"rate_limit_policy": map[string]interface{}{
			"enabled": true,
			"rules": map[string]interface{}{
				"tpm": []interface{}{
					map[string]interface{}{
						"name": "1m", "model": "*", "window_minutes": 1, "max_tokens": 10000, "step_minutes": 1,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/rate-limit-policy")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	testutil.AssertSuccess(t, resp)
	testutil.AssertDataNotEmpty(t, resp)
	testutil.AssertDataFieldNotEmpty(t, resp, "Config")
	testutil.AssertDataFieldNotEmpty(t, resp, "RateLimitPolicies")
	testutil.AssertDataFieldNotEmpty(t, resp, "ApikeyRateLimitPolicyBindings")
	testutil.AssertDataFieldNotEmpty(t, resp, "Version")

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
	})
}

// TestInnerAPI_RateLimitPolicy_RedisKeyStable 验证 RPM/TPM 规则导出包含稳定的 redis_key，
// 且在规则名不变仅其他参数变化时 redis_key 保持不变（Issue #84）。
// 注意：根据 api-keys.md 约定，规则以 name 为标识；修改 name 等价于删除旧规则并创建新规则，
// redis_key 会随之变化，因此本测试不验证改名场景。
func TestInnerAPI_RateLimitPolicy_RedisKeyStable(t *testing.T) {
	apiKeyID, err := testutil.CreateAPIKey("rate-limit-redis-key-key", "")
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	// 绑定包含 TPM 与 RPM 的限流策略
	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"rate_limit_policy": map[string]interface{}{
			"enabled": true,
			"rules": map[string]interface{}{
				"tpm": []interface{}{
					map[string]interface{}{
						"name": "tpm-1m", "model": "*", "window_minutes": 1, "max_tokens": 10000, "step_minutes": 1,
					},
				},
				"rpm": []interface{}{
					map[string]interface{}{
						"name": "rpm-1m", "model": "*", "window_minutes": 1, "max_requests": 10,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	apiKeyValue, err := fetchAPIKeyValue(apiKeyID)
	if err != nil {
		t.Fatalf("fetch api key value failed: %v", err)
	}

	// 第一次导出并记录 redis_key
	before, err := exportRateLimitPolicy(apiKeyValue)
	if err != nil {
		t.Fatalf("first export failed: %v", err)
	}

	// 修改规则的非 name 参数（如 max_tokens / max_requests），规则名保持不变
	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyID, map[string]interface{}{
		"rate_limit_policy": map[string]interface{}{
			"enabled": true,
			"rules": map[string]interface{}{
				"tpm": []interface{}{
					map[string]interface{}{
						"name": "tpm-1m", "model": "*", "window_minutes": 1, "max_tokens": 20000, "step_minutes": 1,
					},
				},
				"rpm": []interface{}{
					map[string]interface{}{
						"name": "rpm-1m", "model": "*", "window_minutes": 1, "max_requests": 20,
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("update rule params failed: %v", err)
	}

	// 第二次导出并对比 redis_key
	after, err := exportRateLimitPolicy(apiKeyValue)
	if err != nil {
		t.Fatalf("second export failed: %v", err)
	}

	if before.policyKey != after.policyKey {
		t.Fatalf("policy key changed after update: %s -> %s", before.policyKey, after.policyKey)
	}
	if len(before.tpmRedisKeys) != len(after.tpmRedisKeys) {
		t.Fatalf("tpm rule count changed: %d -> %d", len(before.tpmRedisKeys), len(after.tpmRedisKeys))
	}
	if len(before.rpmRedisKeys) != len(after.rpmRedisKeys) {
		t.Fatalf("rpm rule count changed: %d -> %d", len(before.rpmRedisKeys), len(after.rpmRedisKeys))
	}
	for i := range before.tpmRedisKeys {
		if before.tpmRedisKeys[i] != after.tpmRedisKeys[i] {
			t.Errorf("tpm[%d] redis_key changed after update: %s -> %s", i, before.tpmRedisKeys[i], after.tpmRedisKeys[i])
		}
	}
	for i := range before.rpmRedisKeys {
		if before.rpmRedisKeys[i] != after.rpmRedisKeys[i] {
			t.Errorf("rpm[%d] redis_key changed after update: %s -> %s", i, before.rpmRedisKeys[i], after.rpmRedisKeys[i])
		}
	}

	t.Cleanup(func() {
		testutil.DeleteAPIKey(apiKeyID)
	})
}

type rateLimitExportSnapshot struct {
	policyKey    string
	tpmRedisKeys []string
	rpmRedisKeys []string
}

func fetchAPIKeyValue(apiKeyID string) (string, error) {
	resp, err := testutil.GetClient().Get("/open-api/v1/api-keys/" + apiKeyID)
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("get api key failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	val, err := testutil.GetDataField(resp, "key")
	if err != nil {
		return "", err
	}
	key, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("api key value is not string")
	}
	return key, nil
}

func exportRateLimitPolicy(apiKeyValue string) (*rateLimitExportSnapshot, error) {
	resp, err := testutil.GetClient().Get("/inner-api/v1/configs/rate-limit-policy")
	if err != nil {
		return nil, err
	}
	if resp.ErrNum != 200 {
		return nil, fmt.Errorf("export failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}

	var data map[string]json.RawMessage
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal data: %w", err)
	}

	var bindings map[string][]string
	if err := json.Unmarshal(data["ApikeyRateLimitPolicyBindings"], &bindings); err != nil {
		return nil, fmt.Errorf("unmarshal bindings: %w", err)
	}
	policies, ok := bindings[apiKeyValue]
	if !ok || len(policies) == 0 {
		return nil, fmt.Errorf("api key %s has no bound policies", apiKeyValue)
	}
	policyKey := policies[0]

	var policiesMap map[string]json.RawMessage
	if err := json.Unmarshal(data["RateLimitPolicies"], &policiesMap); err != nil {
		return nil, fmt.Errorf("unmarshal policies: %w", err)
	}
	policyRaw, ok := policiesMap[policyKey]
	if !ok {
		return nil, fmt.Errorf("policy %s not found in RateLimitPolicies", policyKey)
	}

	var policy struct {
		Rules struct {
			TPM []struct {
				RedisKey string `json:"redis_key"`
			} `json:"tpm"`
			RPM []struct {
				RedisKey string `json:"redis_key"`
			} `json:"rpm"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(policyRaw, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}

	snap := &rateLimitExportSnapshot{policyKey: policyKey}
	for _, rule := range policy.Rules.TPM {
		snap.tpmRedisKeys = append(snap.tpmRedisKeys, rule.RedisKey)
	}
	for _, rule := range policy.Rules.RPM {
		snap.rpmRedisKeys = append(snap.rpmRedisKeys, rule.RedisKey)
	}
	return snap, nil
}
