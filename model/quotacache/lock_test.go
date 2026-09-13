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
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisDistributedLock_Acquire(t *testing.T) {
	client := stateful.NewMockRedisClient()
	lock := NewRedisDistributedLock(client)
	ctx := context.Background()

	acquired, err := lock.Acquire(ctx, "test-lock", "token-a", 5*time.Second)
	require.NoError(t, err)
	assert.True(t, acquired)

	// Same key with a different token should fail.
	acquired2, err := lock.Acquire(ctx, "test-lock", "token-b", 5*time.Second)
	require.NoError(t, err)
	assert.False(t, acquired2)
}

func TestRedisDistributedLock_Release(t *testing.T) {
	client := stateful.NewMockRedisClient()
	lock := NewRedisDistributedLock(client)
	ctx := context.Background()

	acquired, err := lock.Acquire(ctx, "test-lock", "token-a", 5*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)

	// Wrong token should not release.
	err = lock.Release(ctx, "test-lock", "token-b")
	require.NoError(t, err)
	acquired2, err := lock.Acquire(ctx, "test-lock", "token-c", 5*time.Second)
	require.NoError(t, err)
	assert.False(t, acquired2, "lock should still be held after wrong release")

	// Correct token should release.
	err = lock.Release(ctx, "test-lock", "token-a")
	require.NoError(t, err)
	acquired3, err := lock.Acquire(ctx, "test-lock", "token-c", 5*time.Second)
	require.NoError(t, err)
	assert.True(t, acquired3)
}

func TestRedisDistributedLock_Renew(t *testing.T) {
	client := stateful.NewMockRedisClient()
	lock := NewRedisDistributedLock(client)
	ctx := context.Background()

	acquired, err := lock.Acquire(ctx, "test-lock", "token-a", 1*time.Second)
	require.NoError(t, err)
	require.True(t, acquired)

	// Wrong token should not renew.
	err = lock.Renew(ctx, "test-lock", "token-b", 5*time.Second)
	require.NoError(t, err)
	// The lock should expire quickly because the original TTL was 1s.
	time.Sleep(1100 * time.Millisecond)
	acquired2, err := lock.Acquire(ctx, "test-lock", "token-c", 5*time.Second)
	require.NoError(t, err)
	assert.True(t, acquired2, "lock should have expired after wrong renew")

	// Correct token should extend TTL.
	acquired3, err := lock.Acquire(ctx, "test-lock2", "token-d", 1*time.Second)
	require.NoError(t, err)
	require.True(t, acquired3)
	err = lock.Renew(ctx, "test-lock2", "token-d", 5*time.Second)
	require.NoError(t, err)
	time.Sleep(1100 * time.Millisecond)
	acquired4, err := lock.Acquire(ctx, "test-lock2", "token-e", 5*time.Second)
	require.NoError(t, err)
	assert.False(t, acquired4, "lock should still be held after correct renew")
}

func TestRedisDistributedLock_NilClient(t *testing.T) {
	lock := NewRedisDistributedLock(nil)
	ctx := context.Background()

	_, err := lock.Acquire(ctx, "test-lock", "token", 5*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis client is nil")
}
