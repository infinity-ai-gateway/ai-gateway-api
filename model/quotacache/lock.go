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
	"fmt"
	"time"

	"github.com/bfenetworks/bfe/bfe_util/redis_client"
)

// DistributedLock abstracts a Redis-based distributed lock with TTL and token
// safety. Acquire returns true only when the caller successfully creates the
// lock key; Release and Renew verify the caller's token before acting, so a
// crashed instance cannot delete or extend a lock now held by another instance.
type DistributedLock interface {
	Acquire(ctx context.Context, key, token string, ttl time.Duration) (bool, error)
	Release(ctx context.Context, key, token string) error
	Renew(ctx context.Context, key, token string, ttl time.Duration) error
}

// redisDistributedLock implements DistributedLock using Redis Lua scripts.
// It works with the existing redis_client.Client interface which does not
// expose SET NX EX or a plain SET command, but does expose NewScript/Run.
type redisDistributedLock struct {
	client redis_client.Client
}

// NewRedisDistributedLock creates a DistributedLock backed by the given Redis client.
func NewRedisDistributedLock(client redis_client.Client) DistributedLock {
	return &redisDistributedLock{client: client}
}

const (
	lockAcquireScript = `
if redis.call('exists', KEYS[1]) == 0 then
    redis.call('setex', KEYS[1], tonumber(ARGV[2]), ARGV[1])
    return 1
else
    return 0
end
`
	lockReleaseScript = `
if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('del', KEYS[1])
else
    return 0
end
`
	lockRenewScript = `
if redis.call('get', KEYS[1]) == ARGV[1] then
    return redis.call('expire', KEYS[1], tonumber(ARGV[2]))
else
    return 0
end
`
)

func (l *redisDistributedLock) Acquire(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	if l.client == nil {
		return false, fmt.Errorf("redis client is nil")
	}
	script := l.client.NewScript(lockAcquireScript)
	rst, err := script.Run(key, token, int64(ttl.Seconds()))
	if err != nil {
		return false, err
	}
	acquired, ok := rst.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected acquire script result type: %T", rst)
	}
	return acquired == 1, nil
}

func (l *redisDistributedLock) Release(ctx context.Context, key, token string) error {
	if l.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	script := l.client.NewScript(lockReleaseScript)
	_, err := script.Run(key, token)
	return err
}

func (l *redisDistributedLock) Renew(ctx context.Context, key, token string, ttl time.Duration) error {
	if l.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	script := l.client.NewScript(lockRenewScript)
	_, err := script.Run(key, token, int64(ttl.Seconds()))
	return err
}
