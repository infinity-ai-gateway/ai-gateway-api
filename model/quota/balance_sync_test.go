// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quota

import (
	"context"
	"testing"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/api_key"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBalanceSyncManager_shouldResetByPeriod(t *testing.T) {
	m := &BalanceSyncManager{}

	t.Run("nil lastResetAt returns true", func(t *testing.T) {
		assert.True(t, m.shouldResetByPeriod(nil, "weekly", time.Now()))
	})

	t.Run("unknown period returns false", func(t *testing.T) {
		last := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		assert.False(t, m.shouldResetByPeriod(&last, "daily", time.Now()))
	})

	t.Run("weekly same week returns false", func(t *testing.T) {
		// 2026-07-28 is Tuesday
		last := time.Date(2026, 7, 28, 10, 0, 0, 0, time.UTC)
		now := time.Date(2026, 7, 29, 10, 0, 0, 0, time.UTC)
		assert.False(t, m.shouldResetByPeriod(&last, "weekly", now))
	})

	t.Run("weekly different week returns true", func(t *testing.T) {
		last := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC) // Monday
		now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)   // next Monday
		assert.True(t, m.shouldResetByPeriod(&last, "weekly", now))
	})

	t.Run("monthly same month returns false", func(t *testing.T) {
		last := time.Date(2026, 7, 1, 10, 0, 0, 0, time.UTC)
		now := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
		assert.False(t, m.shouldResetByPeriod(&last, "monthly", now))
	})

	t.Run("monthly different month returns true", func(t *testing.T) {
		last := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)
		now := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
		assert.True(t, m.shouldResetByPeriod(&last, "monthly", now))
	})
}

func TestGetWeekStart(t *testing.T) {
	tests := []struct {
		name     string
		input    time.Time
		expected time.Time
	}{
		{
			name:     "Monday",
			input:    time.Date(2026, 7, 27, 15, 30, 0, 0, time.UTC),
			expected: time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Sunday",
			input:    time.Date(2026, 8, 2, 15, 30, 0, 0, time.UTC),
			expected: time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "Wednesday",
			input:    time.Date(2026, 7, 29, 15, 30, 0, 0, time.UTC),
			expected: time.Date(2026, 7, 27, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, getWeekStart(tt.input))
		})
	}
}

func TestGetMonthStart(t *testing.T) {
	input := time.Date(2026, 7, 28, 15, 30, 0, 0, time.UTC)
	expected := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, getMonthStart(input))
}

func TestBalanceSyncManager_ResetExpiredBalances(t *testing.T) {
	origClientSet := stateful.DefaultClientSet
	defer func() {
		stateful.DefaultClientSet = origClientSet
	}()
	mockRedis := stateful.NewMockRedisClient()
	stateful.DefaultClientSet = &stateful.ClientSet{RedisClient: mockRedis}

	ctx := context.Background()

	t.Run("resets balance and Redis when expired", func(t *testing.T) {
		mockRedis.Reset()
		planID := int64(1)
		quota := float64(1000)
		apiKey := "ak-1"
		entityID := "ent-1"
		lastReset := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)

		planStorager := &fakeQuotaPlanStorager{
			listFn: func(ctx context.Context, filter *QuotaPlanFilter) ([]*QuotaPlanParam, error) {
				return []*QuotaPlanParam{{
					ID:          &planID,
					Quota:       &quota,
					Unlimited:   lib.PBool(false),
					ResetPeriod: lib.PString("monthly"),
					LastResetAt: &lastReset,
				}}, nil
			},
			fetchFn: func(ctx context.Context, filter *QuotaPlanFilter) (*QuotaPlanParam, error) {
				return &QuotaPlanParam{ID: &planID, Quota: &quota}, nil
			},
		}
		apiKeyStorager := &fakeAPIKeyStorager{
			fetchListFn: func(ctx context.Context, filter *api_key.APIKeyFilter) ([]*api_key.APIKeyParam, error) {
				createdAt := time.Now()
				return []*api_key.APIKeyParam{{
					ID:          lib.PString("ak-id-1"),
					Key:         &apiKey,
					KeyCreateAt: &createdAt,
				}}, nil
			},
		}
		entityStorager := &fakeEntityStorager{
			listFn: func(ctx context.Context, filter *entity.EntityFilter) ([]*entity.EntityParam, error) {
				return []*entity.EntityParam{{EntityID: &entityID}}, nil
			},
		}

		// Redis 中 API-Key 剩余 500，Entity 剩余 500
		_, err := mockRedis.IncrBy(stateful.AIUsedQuotaKey(apiKey), 500)
		require.NoError(t, err)
		_, err = mockRedis.IncrBy(stateful.AIUsedQuotaKey(entityID), 500)
		require.NoError(t, err)

		m := NewBalanceSyncManager(&fakeTxn{}, apiKeyStorager, planStorager, entityStorager, quotacache.NewRedisQuotaCache(mockRedis), nil)
		require.NoError(t, m.ResetExpiredBalances(ctx))

		// 验证 quota_plans.last_reset_at 被更新
		require.Len(t, planStorager.updated, 1)
		assert.Equal(t, planID, *planStorager.updated[0].filter.ID)
		assert.NotNil(t, planStorager.updated[0].param.LastResetAt)

		// Redis 应该被重置为配额总量
		apiKeyRemaining, _ := mockRedis.GetInt64(stateful.AIUsedQuotaKey(apiKey))
		assert.Equal(t, int64(1000), apiKeyRemaining)
		entityRemaining, _ := mockRedis.GetInt64(stateful.AIUsedQuotaKey(entityID))
		assert.Equal(t, int64(1000), entityRemaining)
	})

	t.Run("skips when not expired", func(t *testing.T) {
		mockRedis.Reset()
		planID := int64(1)
		quota := float64(1000)
		lastReset := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)

		planStorager := &fakeQuotaPlanStorager{
			listFn: func(ctx context.Context, filter *QuotaPlanFilter) ([]*QuotaPlanParam, error) {
				return []*QuotaPlanParam{{
					ID:          &planID,
					Quota:       &quota,
					Unlimited:   lib.PBool(false),
					ResetPeriod: lib.PString("monthly"),
					LastResetAt: &lastReset,
				}}, nil
			},
		}
		m := NewBalanceSyncManager(&fakeTxn{}, &fakeAPIKeyStorager{}, planStorager, &fakeEntityStorager{}, quotacache.NewRedisQuotaCache(mockRedis), nil)

		require.NoError(t, m.ResetExpiredBalances(ctx))
		assert.Empty(t, planStorager.updated)
	})

	t.Run("nil last_reset_at triggers reset", func(t *testing.T) {
		mockRedis.Reset()
		planID := int64(1)
		quota := float64(1000)
		apiKey := "ak-nil"
		entityID := "ent-nil"

		planStorager := &fakeQuotaPlanStorager{
			listFn: func(ctx context.Context, filter *QuotaPlanFilter) ([]*QuotaPlanParam, error) {
				return []*QuotaPlanParam{{
					ID:          &planID,
					Quota:       &quota,
					Unlimited:   lib.PBool(false),
					ResetPeriod: lib.PString("weekly"),
					LastResetAt: nil,
				}}, nil
			},
			fetchFn: func(ctx context.Context, filter *QuotaPlanFilter) (*QuotaPlanParam, error) {
				return &QuotaPlanParam{ID: &planID, Quota: &quota}, nil
			},
		}
		apiKeyStorager := &fakeAPIKeyStorager{
			fetchListFn: func(ctx context.Context, filter *api_key.APIKeyFilter) ([]*api_key.APIKeyParam, error) {
				createdAt := time.Now()
				return []*api_key.APIKeyParam{{
					ID:          lib.PString("ak-id-nil"),
					Key:         &apiKey,
					KeyCreateAt: &createdAt,
				}}, nil
			},
		}
		entityStorager := &fakeEntityStorager{
			listFn: func(ctx context.Context, filter *entity.EntityFilter) ([]*entity.EntityParam, error) {
				return []*entity.EntityParam{{EntityID: &entityID}}, nil
			},
		}

		m := NewBalanceSyncManager(&fakeTxn{}, apiKeyStorager, planStorager, entityStorager, quotacache.NewRedisQuotaCache(mockRedis), nil)
		require.NoError(t, m.ResetExpiredBalances(ctx))

		require.Len(t, planStorager.updated, 1)
		assert.NotNil(t, planStorager.updated[0].param.LastResetAt)
	})
}

// fakeClock is a test Clock that returns a fixed time.
type fakeClock struct {
	t time.Time
}

func (f *fakeClock) Now() time.Time { return f.t }

// newResetTestManager creates a BalanceSyncManager configured for reset-period
// tests with a fake clock, mock Redis, and the supplied storagers.
func newResetTestManager(
	planStorager QuotaPlanStorager,
	apiKeyStorager api_key.APIKeyStorager,
	entityStorager entity.EntityStorager,
	clock Clock,
) *BalanceSyncManager {
	if clock == nil {
		clock = NewRealClock()
	}
	return NewBalanceSyncManager(&fakeTxn{}, apiKeyStorager, planStorager, entityStorager, quotacache.NewRedisQuotaCache(stateful.NewMockRedisClient()), clock)
}

func TestResetExpiredBalances_ConditionalUpdateUsesPeriodStart(t *testing.T) {
	ctx := context.Background()
	planID := int64(1)
	quota := float64(1000)
	apiKey := "ak-conditional"
	// last_reset_at is Monday 00:00:05, so the period start is Monday 00:00:00.
	lastReset := time.Date(2026, 7, 27, 0, 0, 5, 0, time.UTC)
	now := time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC) // next Monday

	planStorager := &fakeQuotaPlanStorager{
		listFn: func(ctx context.Context, filter *QuotaPlanFilter) ([]*QuotaPlanParam, error) {
			return []*QuotaPlanParam{{
				ID:          &planID,
				Quota:       &quota,
				Unlimited:   lib.PBool(false),
				ResetPeriod: lib.PString("weekly"),
				LastResetAt: &lastReset,
			}}, nil
		},
		fetchFn: func(ctx context.Context, filter *QuotaPlanFilter) (*QuotaPlanParam, error) {
			return &QuotaPlanParam{ID: &planID, Quota: &quota}, nil
		},
		updateFn: func(ctx context.Context, filter *QuotaPlanFilter, param *QuotaPlanParam) (int64, error) {
			require.NotNil(t, filter.LastResetAtBefore)
			expected := time.Date(2026, 8, 3, 0, 0, 0, 0, time.UTC)
			assert.Equal(t, expected, *filter.LastResetAtBefore)
			return 1, nil
		},
	}
	apiKeyStorager := &fakeAPIKeyStorager{
		fetchListFn: func(ctx context.Context, filter *api_key.APIKeyFilter) ([]*api_key.APIKeyParam, error) {
			createdAt := time.Now()
			return []*api_key.APIKeyParam{{
				ID:          lib.PString("ak-id-1"),
				Key:         &apiKey,
				KeyCreateAt: &createdAt,
			}}, nil
		},
	}

	m := NewBalanceSyncManager(&fakeTxn{}, apiKeyStorager, planStorager, &fakeEntityStorager{},
		quotacache.NewRedisQuotaCache(stateful.NewMockRedisClient()), &fakeClock{t: now})
	require.NoError(t, m.ResetExpiredBalances(ctx))
}

func TestResetExpiredBalances_WithFakeClock(t *testing.T) {
	ctx := context.Background()
	planID := int64(1)
	quota := float64(1000)
	apiKey := "ak-reset-1"
	entityID := "ent-reset-1"

	makePlanStorager := func(period string, lastResetAt *time.Time) *fakeQuotaPlanStorager {
		return &fakeQuotaPlanStorager{
			listFn: func(ctx context.Context, filter *QuotaPlanFilter) ([]*QuotaPlanParam, error) {
				return []*QuotaPlanParam{{
					ID:          &planID,
					Quota:       &quota,
					Unlimited:   lib.PBool(false),
					ResetPeriod: lib.PString(period),
					LastResetAt: lastResetAt,
				}}, nil
			},
			fetchFn: func(ctx context.Context, filter *QuotaPlanFilter) (*QuotaPlanParam, error) {
				return &QuotaPlanParam{ID: &planID, Quota: &quota}, nil
			},
		}
	}

	makeAPIKeyStorager := func() *fakeAPIKeyStorager {
		return &fakeAPIKeyStorager{
			fetchListFn: func(ctx context.Context, filter *api_key.APIKeyFilter) ([]*api_key.APIKeyParam, error) {
				createdAt := time.Now()
				return []*api_key.APIKeyParam{{
					ID:          lib.PString("ak-id-1"),
					Key:         &apiKey,
					KeyCreateAt: &createdAt,
				}}, nil
			},
		}
	}

	makeEntityStorager := func() *fakeEntityStorager {
		return &fakeEntityStorager{
			listFn: func(ctx context.Context, filter *entity.EntityFilter) ([]*entity.EntityParam, error) {
				return []*entity.EntityParam{{EntityID: &entityID}}, nil
			},
		}
	}

	t.Run("weekly resets on next Monday", func(t *testing.T) {
		lastReset := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)               // Monday
		clock := &fakeClock{t: time.Date(2026, 8, 3, 9, 0, 0, 0, time.UTC)}      // next Monday
		planStorager := makePlanStorager("weekly", &lastReset)
		m := newResetTestManager(planStorager, makeAPIKeyStorager(), makeEntityStorager(), clock)

		require.NoError(t, m.ResetExpiredBalances(ctx))

		require.Len(t, planStorager.updated, 1)
		assert.Equal(t, clock.Now(), *planStorager.updated[0].param.LastResetAt)
	})

	t.Run("weekly does not reset within same week", func(t *testing.T) {
		lastReset := time.Date(2026, 7, 27, 10, 0, 0, 0, time.UTC)               // Monday
		clock := &fakeClock{t: time.Date(2026, 7, 29, 9, 0, 0, 0, time.UTC)}     // Wednesday
		planStorager := makePlanStorager("weekly", &lastReset)
		m := newResetTestManager(planStorager, makeAPIKeyStorager(), makeEntityStorager(), clock)

		require.NoError(t, m.ResetExpiredBalances(ctx))
		assert.Empty(t, planStorager.updated)
	})

	t.Run("monthly resets on next month first day", func(t *testing.T) {
		lastReset := time.Date(2026, 7, 31, 10, 0, 0, 0, time.UTC)               // Jul 31
		clock := &fakeClock{t: time.Date(2026, 8, 1, 9, 0, 0, 0, time.UTC)}      // Aug 1
		planStorager := makePlanStorager("monthly", &lastReset)
		m := newResetTestManager(planStorager, makeAPIKeyStorager(), makeEntityStorager(), clock)

		require.NoError(t, m.ResetExpiredBalances(ctx))

		require.Len(t, planStorager.updated, 1)
		assert.Equal(t, clock.Now(), *planStorager.updated[0].param.LastResetAt)
	})

	t.Run("monthly does not reset within same month", func(t *testing.T) {
		lastReset := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)                // Aug 1
		clock := &fakeClock{t: time.Date(2026, 8, 15, 9, 0, 0, 0, time.UTC)}     // Aug 15
		planStorager := makePlanStorager("monthly", &lastReset)
		m := newResetTestManager(planStorager, makeAPIKeyStorager(), makeEntityStorager(), clock)

		require.NoError(t, m.ResetExpiredBalances(ctx))
		assert.Empty(t, planStorager.updated)
	})

	t.Run("monthly resets across year boundary", func(t *testing.T) {
		lastReset := time.Date(2025, 12, 31, 10, 0, 0, 0, time.UTC)              // Dec 31
		clock := &fakeClock{t: time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)}      // Jan 1
		planStorager := makePlanStorager("monthly", &lastReset)
		m := newResetTestManager(planStorager, makeAPIKeyStorager(), makeEntityStorager(), clock)

		require.NoError(t, m.ResetExpiredBalances(ctx))

		require.Len(t, planStorager.updated, 1)
		assert.Equal(t, clock.Now(), *planStorager.updated[0].param.LastResetAt)
	})
}
