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

package quotacache

import (
	"context"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisQuotaCache_ResetToQuotaAtomic(t *testing.T) {
	client := stateful.NewMockRedisClient()
	cache := NewRedisQuotaCache(client)
	ctx := context.Background()
	key := "test-key"
	quota := float64(1000)

	err := cache.ResetToQuotaAtomic(ctx, key, &quota, nil)
	require.NoError(t, err)

	remaining, err := cache.GetRemaining(ctx, key, nil)
	require.NoError(t, err)
	assert.Equal(t, quota, remaining)
}

func TestRedisQuotaCache_ResetToQuotaAtomic_OverwritesExistingValue(t *testing.T) {
	client := stateful.NewMockRedisClient()
	cache := NewRedisQuotaCache(client)
	ctx := context.Background()
	key := "test-key"

	// Pre-populate with a smaller value to simulate partial consumption.
	_, err := client.IncrBy(stateful.AIUsedQuotaKey(key), 500)
	require.NoError(t, err)

	quota := float64(1000)
	err = cache.ResetToQuotaAtomic(ctx, key, &quota, nil)
	require.NoError(t, err)

	remaining, err := cache.GetRemaining(ctx, key, nil)
	require.NoError(t, err)
	assert.Equal(t, quota, remaining, "ResetToQuotaAtomic should overwrite existing value")
}

func TestRedisQuotaCache_ResetToQuotaAtomic_NilQuota(t *testing.T) {
	client := stateful.NewMockRedisClient()
	cache := NewRedisQuotaCache(client)
	ctx := context.Background()

	err := cache.ResetToQuotaAtomic(ctx, "test-key", nil, nil)
	require.NoError(t, err)

	remaining, err := cache.GetRemaining(ctx, "test-key", nil)
	require.NoError(t, err)
	assert.Equal(t, float64(0), remaining)
}

func TestRedisQuotaCache_ResetToQuotaAtomic_NilClient(t *testing.T) {
	cache := NewRedisQuotaCache(nil)
	ctx := context.Background()
	quota := float64(1000)

	err := cache.ResetToQuotaAtomic(ctx, "test-key", &quota, nil)
	require.NoError(t, err)
}
