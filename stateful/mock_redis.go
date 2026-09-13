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

package stateful

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bfenetworks/bfe/bfe_util/redis_client"
)

// MockRedisClient 内存 Redis Mock 实现
// 用于测试环境替代真实 Redis，避免依赖外部 Redis 服务
type MockRedisClient struct {
	mu   sync.Mutex
	data map[string]interface{}
	ttls map[string]time.Time
}

var _ redis_client.Client = (*MockRedisClient)(nil)

// NewMockRedisClient 创建新的 Mock Redis 客户端
func NewMockRedisClient() *MockRedisClient {
	return &MockRedisClient{
		data: make(map[string]interface{}),
		ttls: make(map[string]time.Time),
	}
}

// Setex 设置 key 的值并设置过期时间（mock 中 expire 以秒为单位）
func (m *MockRedisClient) Setex(key string, value []byte, expire int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = m.parseValue(value)
	if expire > 0 {
		m.ttls[key] = time.Now().Add(time.Duration(expire) * time.Second)
	}
	return nil
}

// Get 获取 key 的值
func (m *MockRedisClient) Get(key string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isExpiredLocked(key) {
		delete(m.data, key)
		delete(m.ttls, key)
		return nil, nil
	}
	return m.data[key], nil
}

// Expire 设置 key 的过期时间（mock 中 expire 以秒为单位）
func (m *MockRedisClient) Expire(key string, expire int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if expire > 0 {
		m.ttls[key] = time.Now().Add(time.Duration(expire) * time.Second)
	} else {
		delete(m.ttls, key)
	}
	return nil
}

// Incr 将 key 对应的值增加 1
func (m *MockRedisClient) Incr(key string) (int64, error) {
	return m.IncrBy(key, 1)
}

// IncrAndExpire Incr 并设置过期时间（mock 忽略过期时间）
func (m *MockRedisClient) IncrAndExpire(key string, expire int) (int64, error) {
	return m.IncrBy(key, 1)
}

// Decr 将 key 对应的值减少 1
func (m *MockRedisClient) Decr(key string) (int64, error) {
	return m.IncrBy(key, -1)
}

// PIncr 批量增加多个 key
func (m *MockRedisClient) PIncr(keys []string) ([]int64, error) {
	results := make([]int64, len(keys))
	for i, key := range keys {
		val, err := m.IncrBy(key, 1)
		if err != nil {
			return nil, err
		}
		results[i] = val
	}
	return results, nil
}

// GetInt64 获取 key 对应的 int64 值
func (m *MockRedisClient) GetInt64(key string) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.isExpiredLocked(key) {
		delete(m.data, key)
		delete(m.ttls, key)
		return 0, nil
	}
	return m.toInt64Locked(m.data[key]), nil
}

// GetInt64Batch 批量获取多个 key 对应的 int64 值
func (m *MockRedisClient) GetInt64Batch(keys []string) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	results := make([]int64, len(keys))
	for i, key := range keys {
		if m.isExpiredLocked(key) {
			delete(m.data, key)
			delete(m.ttls, key)
			results[i] = 0
			continue
		}
		results[i] = m.toInt64Locked(m.data[key])
	}
	return results, nil
}

// IncrBy 将 key 对应的值增加 delta
func (m *MockRedisClient) IncrBy(key string, delta int64) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	current := m.toInt64Locked(m.data[key])
	current += delta
	m.data[key] = current
	return current, nil
}

// Delete 删除 key
func (m *MockRedisClient) Delete(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
	delete(m.ttls, key)
	return nil
}

// Reset 清空所有数据
func (m *MockRedisClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = make(map[string]interface{})
	m.ttls = make(map[string]time.Time)
}

// NewScript 创建 Redis 脚本（mock 实现支持本期用到的 Lua 脚本）
func (m *MockRedisClient) NewScript(src string) redis_client.RedisScript {
	return &mockRedisScript{client: m, src: src}
}

type mockRedisScript struct {
	client *MockRedisClient
	src    string
}

// Run 执行 Redis 脚本（mock 实现，按脚本关键字匹配执行）
func (s *mockRedisScript) Run(key string, args ...interface{}) (interface{}, error) {
	s.client.mu.Lock()
	defer s.client.mu.Unlock()

	src := strings.TrimSpace(s.src)

	// Atomic SET for ResetToQuotaAtomic.
	if strings.Contains(src, "redis.call('set', KEYS[1], ARGV[1])") {
		if len(args) < 1 {
			return nil, fmt.Errorf("set script expects 1 arg")
		}
		val := s.client.toInt64Locked(args[0])
		s.client.data[key] = val
		return int64(1), nil
	}

	// Lock acquire: exists + setex.
	if strings.Contains(src, "exists") && strings.Contains(src, "setex") {
		if len(args) < 2 {
			return nil, fmt.Errorf("acquire script expects 2 args")
		}
		token := fmt.Sprintf("%v", args[0])
		expire := s.client.toInt64Locked(args[1])
		if s.client.existsLocked(key) {
			return int64(0), nil
		}
		s.client.data[key] = token
		if expire > 0 {
			s.client.ttls[key] = time.Now().Add(time.Duration(expire) * time.Second)
		}
		return int64(1), nil
	}

	// Lock release: get + del.
	if strings.Contains(src, "get") && strings.Contains(src, "del") && !strings.Contains(src, "expire") {
		if len(args) < 1 {
			return nil, fmt.Errorf("release script expects 1 arg")
		}
		token := fmt.Sprintf("%v", args[0])
		if !s.client.existsLocked(key) {
			return int64(0), nil
		}
		if fmt.Sprintf("%v", s.client.data[key]) != token {
			return int64(0), nil
		}
		delete(s.client.data, key)
		delete(s.client.ttls, key)
		return int64(1), nil
	}

	// Lock renew: get + expire.
	if strings.Contains(src, "get") && strings.Contains(src, "expire") {
		if len(args) < 2 {
			return nil, fmt.Errorf("renew script expects 2 args")
		}
		token := fmt.Sprintf("%v", args[0])
		expire := s.client.toInt64Locked(args[1])
		if !s.client.existsLocked(key) {
			return int64(0), nil
		}
		if fmt.Sprintf("%v", s.client.data[key]) != token {
			return int64(0), nil
		}
		if expire > 0 {
			s.client.ttls[key] = time.Now().Add(time.Duration(expire) * time.Second)
		}
		return int64(1), nil
	}

	return nil, fmt.Errorf("unsupported lua script in mock: %s", src)
}

func (m *MockRedisClient) isExpiredLocked(key string) bool {
	exp, ok := m.ttls[key]
	return ok && time.Now().After(exp)
}

func (m *MockRedisClient) existsLocked(key string) bool {
	if m.isExpiredLocked(key) {
		delete(m.data, key)
		delete(m.ttls, key)
		return false
	}
	_, ok := m.data[key]
	return ok
}

func (m *MockRedisClient) toInt64Locked(v interface{}) int64 {
	switch val := v.(type) {
	case int64:
		return val
	case int:
		return int64(val)
	case int32:
		return int64(val)
	case string:
		if i, err := strconv.ParseInt(val, 10, 64); err == nil {
			return i
		}
		return 0
	case []byte:
		if i, err := strconv.ParseInt(string(val), 10, 64); err == nil {
			return i
		}
		return 0
	default:
		return 0
	}
}

func (m *MockRedisClient) parseValue(v []byte) interface{} {
	if i, err := strconv.ParseInt(string(v), 10, 64); err == nil {
		return i
	}
	return string(v)
}
