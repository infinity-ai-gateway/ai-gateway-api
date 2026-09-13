// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quota_reset

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/quota"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeBalanceSyncer struct {
	called bool
}

func (f *fakeBalanceSyncer) ResetExpiredBalances(ctx context.Context) error {
	f.called = true
	return nil
}

type fakeTxn struct{}

func (f *fakeTxn) AtomExecute(ctx context.Context, do func(context.Context) error) error {
	return do(ctx)
}

func TestTriggerResetAction(t *testing.T) {
	fakeSyncer := &fakeBalanceSyncer{}
	origScheduler := container.QuotaResetScheduler
	defer func() { container.QuotaResetScheduler = origScheduler }()

	container.QuotaResetScheduler = quota.NewQuotaResetScheduler(&fakeTxn{}, fakeSyncer, nil)

	req := httptest.NewRequest(http.MethodPost, "/inner-api/v1/quota/trigger-reset", nil)
	resp, err := TriggerResetAction(req)

	require.NoError(t, err)
	assert.True(t, fakeSyncer.called, "ResetExpiredBalances should be called")
	assert.Equal(t, map[string]string{"status": "ok"}, resp)

	// 连续触发应可重复执行
	fakeSyncer.called = false
	_, err = TriggerResetAction(req)
	require.NoError(t, err)
	assert.True(t, fakeSyncer.called)
}

func TestTriggerResetAction_NilScheduler(t *testing.T) {
	origScheduler := container.QuotaResetScheduler
	defer func() { container.QuotaResetScheduler = origScheduler }()

	container.QuotaResetScheduler = nil

	req := httptest.NewRequest(http.MethodPost, "/inner-api/v1/quota/trigger-reset", nil)
	resp, err := TriggerResetAction(req)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"status": "ok"}, resp)
}

func TestTriggerResetRoute(t *testing.T) {
	assert.Equal(t, "/quota/trigger-reset", TriggerResetRoute.Path)
	assert.Equal(t, http.MethodPost, TriggerResetRoute.Method)
	assert.NotNil(t, TriggerResetRoute.Handler)
}

// 以下接口保证 fakeBalanceSyncer 满足 BalanceSyncer 接口
var _ quota.BalanceSyncer = (*fakeBalanceSyncer)(nil)
