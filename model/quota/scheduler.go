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

package quota

import (
	"context"
	"runtime"
	"time"

	"github.com/google/uuid"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

// BalanceSyncer defines the operations required by QuotaResetScheduler.
type BalanceSyncer interface {
	ResetExpiredBalances(ctx context.Context) error
}

// QuotaResetScheduler 配额重置调度器
type QuotaResetScheduler struct {
	txn            itxn.TxnStorager
	balanceSyncMgr BalanceSyncer
	lockClient     quotacache.DistributedLock
	instanceToken  string
	stopCh         chan struct{}
}

// NewQuotaResetScheduler 创建配额重置调度器
func NewQuotaResetScheduler(
	txn itxn.TxnStorager,
	balanceSyncMgr BalanceSyncer,
	lockClient quotacache.DistributedLock,
) *QuotaResetScheduler {
	return &QuotaResetScheduler{
		txn:            txn,
		balanceSyncMgr: balanceSyncMgr,
		lockClient:     lockClient,
		instanceToken:  uuid.NewString(),
		stopCh:         make(chan struct{}),
	}
}

// Start 启动定时任务
func (s *QuotaResetScheduler) Start() {
	go s.run()
}

// Stop 停止定时任务
func (s *QuotaResetScheduler) Stop() {
	close(s.stopCh)
}

// TriggerReset 手动触发一次带分布式锁保护的配额重置任务。
// 仅供内部管理/测试接口使用，不影响定时任务的下次执行时间。
func (s *QuotaResetScheduler) TriggerReset() {
	s.resetQuotasWithRecover()
}

// run 运行定时任务循环
func (s *QuotaResetScheduler) run() {
	defer func() {
		if err := recover(); err != nil {
			stack := make([]byte, 1024*8)
			stack = stack[:runtime.Stack(stack, false)]
			stateful.ExceptionLogger.Error("PANIC in QuotaResetScheduler: err=%v\n%s", err, string(stack))
		}
	}()

	s.resetQuotasWithRecover()

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.resetQuotasWithRecover()
		case <-s.stopCh:
			return
		}
	}
}

func (s *QuotaResetScheduler) resetQuotasWithRecover() {
	defer func() {
		if err := recover(); err != nil {
			stack := make([]byte, 1024*8)
			stack = stack[:runtime.Stack(stack, false)]
			stateful.ExceptionLogger.Error("PANIC in resetQuotas: err=%v\n%s", err, string(stack))
		}
	}()

	s.resetQuotas()
}

// resetQuotas 重置配额
func (s *QuotaResetScheduler) resetQuotas() {
	ctx := context.Background()
	now := time.Now()

	stateful.AccessLogger.Info("Starting quota scheduler tasks at %v", now)

	if s.lockClient != nil {
		lockKey := "quota:reset:scheduler:lock"
		ttl := 5 * time.Minute

		acquired, err := s.lockClient.Acquire(ctx, lockKey, s.instanceToken, ttl)
		if err != nil {
			stateful.AccessLogger.Warn("Failed to acquire quota scheduler lock: %v", err)
			return
		}
		if !acquired {
			stateful.AccessLogger.Info("Quota scheduler lock not acquired, skip")
			return
		}

		stopRenew := s.startRenew(ctx, lockKey, ttl)
		defer stopRenew()

		defer func() {
			if err := s.lockClient.Release(ctx, lockKey, s.instanceToken); err != nil {
				stateful.AccessLogger.Warn("Failed to release quota scheduler lock: %v", err)
			}
		}()
	}

	// 查找所有 unlimited=0 的 quota_plan 中是否达到重置的时间条件，达到后执行向 Redis 重置配额余额
	if err := s.balanceSyncMgr.ResetExpiredBalances(ctx); err != nil {
		stateful.AccessLogger.Error("Failed to reset expired balances: %v", err)
	} else {
		stateful.AccessLogger.Info("Successfully checked and reset expired balances")
	}

	stateful.AccessLogger.Info("Quota scheduler tasks completed at %v", time.Now())
}

// startRenew 启动看门狗，在锁持有期间定期续期
func (s *QuotaResetScheduler) startRenew(ctx context.Context, key string, ttl time.Duration) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(ttl / 3)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := s.lockClient.Renew(ctx, key, s.instanceToken, ttl); err != nil {
					stateful.AccessLogger.Warn("Failed to renew quota scheduler lock: %v", err)
					return
				}
			case <-done:
				return
			case <-ctx.Done():
				return
			}
		}
	}()
	return func() { close(done) }
}
