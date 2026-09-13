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

package quota_period_reset_test

import (
	"bytes"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestQuotaPeriodReset_AutoResetAndIdempotency(t *testing.T) {
	typeName := testutil.UniqueEntityTypeName()
	if _, err := testutil.CreateEntityType(typeName, 1); err != nil {
		t.Fatalf("setup entity-type failed: %v", err)
	}
	defer testutil.DeleteEntityType(typeName)

	entityName := testutil.UniqueEntityName()
	entityID, err := testutil.CreateEntity(entityName, typeName, "")
	require.NoError(t, err, "setup entity failed")
	defer testutil.DeleteEntity(entityID)

	apiKeyAID, apiKeyAValue, err := testutil.CreateAPIKeyWithKey("quota-period-reset-ak-a", "")
	require.NoError(t, err, "setup api-key A failed")
	defer testutil.DeleteAPIKey(apiKeyAID)

	apiKeyCID, apiKeyCValue, err := testutil.CreateAPIKeyWithKey("quota-period-reset-ak-c", "")
	require.NoError(t, err, "setup api-key C failed")
	defer testutil.DeleteAPIKey(apiKeyCID)

	// 给 API-Key A 配置月度 total_token 配额
	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyAID, map[string]interface{}{
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        1000000,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
	})
	require.NoError(t, err, "patch api-key A quota plan failed")

	// 给 Entity B 配置月度 RMB 配额
	_, err = testutil.GetClient().Patch("/open-api/v1/entities/"+entityID, map[string]interface{}{
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        100.5,
			"unit":         "RMB",
			"reset_period": "monthly",
		},
	})
	require.NoError(t, err, "patch entity B quota plan failed")

	// 给 API-Key C 配置月度 total_token 配额（同周期不应被重复重置）
	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyCID, map[string]interface{}{
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        500000,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
	})
	require.NoError(t, err, "patch api-key C quota plan failed")

	// 模拟已消耗：将 Redis 剩余量调低
	sm.SetQuotaRemaining(apiKeyAValue, 100000, "total_token")
	sm.SetQuotaRemaining(entityID, 10.5, "RMB")
	sm.SetQuotaRemaining(apiKeyCValue, 100000, "total_token")

	// 将 A/B 的 last_reset_at 拨到上个月，C 保持在当前月
	previousMonthStart := time.Now().AddDate(0, -1, 0).AddDate(0, 0, -5)
	previousMonthStart = time.Date(previousMonthStart.Year(), previousMonthStart.Month(), 1, 0, 0, 0, 0, time.Local)
	require.NoError(t, sm.UpdateQuotaPlanLastResetAt(apiKeyAID, "api_key", previousMonthStart))
	require.NoError(t, sm.UpdateQuotaPlanLastResetAt(entityID, "entity", previousMonthStart))

	currentMonth := time.Date(time.Now().Year(), time.Now().Month(), 15, 12, 0, 0, 0, time.Local)
	require.NoError(t, sm.UpdateQuotaPlanLastResetAt(apiKeyCID, "api_key", currentMonth))

	// 触发周期重置
	resp, err := testutil.GetClient().Post("/inner-api/v1/quota/trigger-reset", map[string]interface{}{})
	require.NoError(t, err, "trigger reset failed")
	testutil.AssertSuccess(t, resp)

	t.Run("QR-1-001 跨周期 API-Key 月度配额自动重置", func(t *testing.T) {
		remaining := sm.GetQuotaRemaining(apiKeyAValue, "total_token")
		assert.InDelta(t, float64(1000000), remaining, 0.1, "API-Key A 剩余量应被重置为 quota")

		lastResetAt, err := sm.GetQuotaPlanLastResetAt(apiKeyAID, "api_key")
		require.NoError(t, err)
		currentMonthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
		assert.True(t, lastResetAt.After(currentMonthStart.Add(-1*time.Second)), "API-Key A last_reset_at 应更新到当前周期")
	})

	t.Run("QR-1-002 跨周期 Entity 月度 RMB 配额自动重置精度", func(t *testing.T) {
		remaining := sm.GetQuotaRemaining(entityID, "RMB")
		assert.InDelta(t, float64(100.5), remaining, 0.00001, "Entity B 剩余量应被重置为 quota")

		lastResetAt, err := sm.GetQuotaPlanLastResetAt(entityID, "entity")
		require.NoError(t, err)
		currentMonthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
		assert.True(t, lastResetAt.After(currentMonthStart.Add(-1*time.Second)), "Entity B last_reset_at 应更新到当前周期")
	})

	t.Run("QR-1-003 同周期内不重复重置", func(t *testing.T) {
		remaining := sm.GetQuotaRemaining(apiKeyCValue, "total_token")
		assert.InDelta(t, float64(100000), remaining, 0.1, "API-Key C 剩余量不应被重置")

		lastResetAt, err := sm.GetQuotaPlanLastResetAt(apiKeyCID, "api_key")
		require.NoError(t, err)
		assert.WithinDuration(t, currentMonth, *lastResetAt, time.Second, "API-Key C last_reset_at 不应变化")
	})

	t.Run("QR-1-004 已重置后再次触发保持幂等", func(t *testing.T) {
		// 第一次触发后，A 已被重置；再次调低并触发，不应再次重置
		sm.SetQuotaRemaining(apiKeyAValue, 200000, "total_token")

		resp, err := testutil.GetClient().Post("/inner-api/v1/quota/trigger-reset", map[string]interface{}{})
		require.NoError(t, err, "second trigger reset failed")
		testutil.AssertSuccess(t, resp)

		remaining := sm.GetQuotaRemaining(apiKeyAValue, "total_token")
		assert.InDelta(t, float64(200000), remaining, 0.1, "API-Key A 在同一周期内不应被二次重置")
	})
}

func TestQuotaPeriodReset_MultiInstanceLock(t *testing.T) {
	// 启动实例 A，并创建其 DB 与 Redis
	smA, err := testutil.StartServerWithSharedInfra(nil, "")
	require.NoError(t, err, "start server A failed")
	defer smA.Shutdown()

	// 启动实例 B，复用 A 的 Redis 与 DB
	smB, err := testutil.StartServerWithSharedInfra(smA.Redis, smA.DBPath)
	require.NoError(t, err, "start server B failed")
	defer smB.Shutdown()

	// 后续请求发到实例 A
	origURL := testutil.GetClient().BaseURL
	testutil.SetServerURL(smA.ServerURL)
	defer testutil.SetServerURL(origURL)

	apiKeyAID, apiKeyAValue, err := testutil.CreateAPIKeyWithKey("quota-period-reset-multi-ak", "")
	require.NoError(t, err, "setup api-key failed")
	defer testutil.DeleteAPIKey(apiKeyAID)

	_, err = testutil.GetClient().Patch("/open-api/v1/api-keys/"+apiKeyAID, map[string]interface{}{
		"quota_plan": map[string]interface{}{
			"unlimited":    false,
			"quota":        800000,
			"unit":         "total_token",
			"reset_period": "monthly",
		},
	})
	require.NoError(t, err, "patch api-key quota plan failed")

	// 模拟已消耗：Redis 剩余量调低
	smA.SetQuotaRemaining(apiKeyAValue, 50000, "total_token")

	// 将 last_reset_at 拨到上个月
	previousMonthStart := time.Now().AddDate(0, -1, 0).AddDate(0, 0, -5)
	previousMonthStart = time.Date(previousMonthStart.Year(), previousMonthStart.Month(), 1, 0, 0, 0, 0, time.Local)
	require.NoError(t, smA.UpdateQuotaPlanLastResetAt(apiKeyAID, "api_key", previousMonthStart))

	// 并发触发两个实例的重置接口
	var wg sync.WaitGroup
	var respA, respB *http.Response
	var errA, errB error

	wg.Add(2)
	go func() {
		defer wg.Done()
		respA, errA = http.Post(smA.ServerURL+"/inner-api/v1/quota/trigger-reset", "application/json", bytes.NewReader([]byte("{}")))
	}()
	go func() {
		defer wg.Done()
		respB, errB = http.Post(smB.ServerURL+"/inner-api/v1/quota/trigger-reset", "application/json", bytes.NewReader([]byte("{}")))
	}()
	wg.Wait()

	require.NoError(t, errA, "trigger server A failed")
	require.NoError(t, errB, "trigger server B failed")
	assert.Equal(t, http.StatusOK, respA.StatusCode, "server A trigger should return 200")
	assert.Equal(t, http.StatusOK, respB.StatusCode, "server B trigger should return 200")

	t.Run("QR-2-001 多实例共享 Redis 时只有一个实例实际完成重置", func(t *testing.T) {
		// 由于两个实例共享 Redis 与 DB，无论谁拿到锁，结果应一致：Redis 被重置、last_reset_at 更新
		remaining := smA.GetQuotaRemaining(apiKeyAValue, "total_token")
		assert.InDelta(t, float64(800000), remaining, 0.1, "API-Key 剩余量应被重置为 quota")

		lastResetAt, err := smA.GetQuotaPlanLastResetAt(apiKeyAID, "api_key")
		require.NoError(t, err)
		currentMonthStart := time.Date(time.Now().Year(), time.Now().Month(), 1, 0, 0, 0, 0, time.Local)
		assert.True(t, lastResetAt.After(currentMonthStart.Add(-1*time.Second)), "last_reset_at 应更新到当前周期")
	})

	t.Run("QR-2-002 锁释放后另一实例可再次获取锁并执行（幂等）", func(t *testing.T) {
		// 首次重置后 last_reset_at 已更新；再次调低 Redis 并从 B 触发，不应二次重置
		smA.SetQuotaRemaining(apiKeyAValue, 100000, "total_token")

		resp, err := http.Post(smB.ServerURL+"/inner-api/v1/quota/trigger-reset", "application/json", bytes.NewReader([]byte("{}")))
		require.NoError(t, err, "trigger server B failed")
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		remaining := smA.GetQuotaRemaining(apiKeyAValue, "total_token")
		assert.InDelta(t, float64(100000), remaining, 0.1, "同一周期内不应被二次重置")
	})
}
