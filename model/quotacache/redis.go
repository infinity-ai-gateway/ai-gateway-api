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

package quotacache

import (
	"context"
	"strings"

	"github.com/bfenetworks/bfe/bfe_util/redis_client"
	golibquota "github.com/bfenetworks/go-lib/quota"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

const setQuotaScriptSrc = `
redis.call('set', KEYS[1], ARGV[1])
return 1
`

type redisQuotaCache struct {
	client            redis_client.Client
	setQuotaScript    redis_client.RedisScript
}

// NewRedisQuotaCache creates a QuotaCache backed by Redis.
func NewRedisQuotaCache(client redis_client.Client) QuotaCache {
	c := &redisQuotaCache{client: client}
	if client != nil {
		c.setQuotaScript = client.NewScript(setQuotaScriptSrc)
	}
	return c
}

func (c *redisQuotaCache) GetRemaining(ctx context.Context, key string, unit *string) (float64, error) {
	if c.client == nil {
		return 0, nil
	}

	redisKey := stateful.AIUsedQuotaKey(key)
	remain, err := c.client.GetInt64(redisKey)
	if err != nil {
		if strings.Contains(err.Error(), "redigo: nil returned") {
			return 0, nil
		}
		return 0, err
	}

	return golibquota.PtrFromRedisValue(remain, unit), nil
}

func (c *redisQuotaCache) BatchGetRemaining(ctx context.Context, keys []string, unit *string) (map[string]float64, error) {
	if c.client == nil {
		result := make(map[string]float64, len(keys))
		for _, k := range keys {
			result[k] = 0
		}
		return result, nil
	}

	redisKeys := make([]string, len(keys))
	for i, k := range keys {
		redisKeys[i] = stateful.AIUsedQuotaKey(k)
	}

	values, err := c.client.GetInt64Batch(redisKeys)
	if err != nil {
		return nil, err
	}

	result := make(map[string]float64, len(keys))
	for i, k := range keys {
		result[k] = golibquota.PtrFromRedisValue(values[i], unit)
	}
	return result, nil
}

func (c *redisQuotaCache) SetRemaining(ctx context.Context, key string, quota *float64, unit *string) error {
	if c.client == nil || quota == nil {
		return nil
	}

	redisKey := stateful.AIUsedQuotaKey(key)
	targetValue := golibquota.PtrToRedisValue(quota, unit)

	currentValue, err := c.client.GetInt64(redisKey)
	if err != nil {
		if !strings.Contains(err.Error(), "redigo: nil returned") {
			return err
		}
		currentValue = 0
	}

	delta := targetValue - currentValue
	_, err = c.client.IncrBy(redisKey, delta)
	return err
}

func (c *redisQuotaCache) ResetToQuota(ctx context.Context, key string, quota *float64, unit *string) error {
	return c.SetRemaining(ctx, key, quota, unit)
}

func (c *redisQuotaCache) ResetToQuotaAtomic(ctx context.Context, key string, quota *float64, unit *string) error {
	if c.client == nil || quota == nil || c.setQuotaScript == nil {
		return nil
	}

	redisKey := stateful.AIUsedQuotaKey(key)
	targetValue := golibquota.PtrToRedisValue(quota, unit)
	_, err := c.setQuotaScript.Run(redisKey, targetValue)
	return err
}

func (c *redisQuotaCache) DeleteKeys(ctx context.Context, keys []string) error {
	if c.client == nil {
		return nil
	}

	for _, key := range keys {
		if err := c.client.Delete(key); err != nil {
			return err
		}
	}
	return nil
}
