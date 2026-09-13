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

	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
	"github.com/stretchr/testify/assert"
)

// spyBalanceSyncer implements BalanceSyncer and records method calls.
type spyBalanceSyncer struct {
	resetExpiredCalled bool
}

func (s *spyBalanceSyncer) ResetExpiredBalances(ctx context.Context) error {
	s.resetExpiredCalled = true
	return nil
}

// fakeDistributedLock is a test double for quotacache.DistributedLock.
type fakeDistributedLock struct {
	acquireResult bool
	acquireErr    error
	releaseErr    error
	renewErr      error

	acquireCalled bool
	releaseCalled bool
	renewCalled   bool
	lastKey       string
	lastToken     string
	lastTTL       time.Duration
}

func (f *fakeDistributedLock) Acquire(ctx context.Context, key, token string, ttl time.Duration) (bool, error) {
	f.acquireCalled = true
	f.lastKey = key
	f.lastToken = token
	f.lastTTL = ttl
	return f.acquireResult, f.acquireErr
}

func (f *fakeDistributedLock) Release(ctx context.Context, key, token string) error {
	f.releaseCalled = true
	return f.releaseErr
}

func (f *fakeDistributedLock) Renew(ctx context.Context, key, token string, ttl time.Duration) error {
	f.renewCalled = true
	return f.renewErr
}

var _ quotacache.DistributedLock = (*fakeDistributedLock)(nil)

func TestQuotaResetScheduler_ResetQuotas(t *testing.T) {
	spy := &spyBalanceSyncer{}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, spy, nil)

	scheduler.resetQuotas()

	assert.True(t, spy.resetExpiredCalled, "ResetExpiredBalances should be called")
}

func TestQuotaResetScheduler_StartStop(t *testing.T) {
	spy := &spyBalanceSyncer{}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, spy, nil)

	scheduler.Start()
	scheduler.Stop()
}

func TestQuotaResetScheduler_LockNotAcquired_Skips(t *testing.T) {
	spy := &spyBalanceSyncer{}
	lock := &fakeDistributedLock{acquireResult: false}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, spy, lock)

	scheduler.resetQuotas()

	assert.True(t, lock.acquireCalled, "Acquire should be called")
	assert.False(t, spy.resetExpiredCalled, "ResetExpiredBalances should be skipped when lock not acquired")
	assert.False(t, lock.releaseCalled, "Release should not be called when lock not acquired")
}

func TestQuotaResetScheduler_LockAcquired_ExecutesAndReleases(t *testing.T) {
	spy := &spyBalanceSyncer{}
	lock := &fakeDistributedLock{acquireResult: true}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, spy, lock)

	scheduler.resetQuotas()

	assert.True(t, lock.acquireCalled, "Acquire should be called")
	assert.True(t, spy.resetExpiredCalled, "ResetExpiredBalances should be called")
	assert.True(t, lock.releaseCalled, "Release should be called after execution")
	assert.Equal(t, "quota:reset:scheduler:lock", lock.lastKey)
	assert.Equal(t, 5*time.Minute, lock.lastTTL)
}

func TestQuotaResetScheduler_LockAcquireError_Skips(t *testing.T) {
	spy := &spyBalanceSyncer{}
	lock := &fakeDistributedLock{acquireResult: false, acquireErr: assert.AnError}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, spy, lock)

	scheduler.resetQuotas()

	assert.True(t, lock.acquireCalled)
	assert.False(t, spy.resetExpiredCalled, "ResetExpiredBalances should be skipped on acquire error")
	assert.False(t, lock.releaseCalled)
}

func TestQuotaResetScheduler_RenewsLock(t *testing.T) {
	lock := &fakeDistributedLock{acquireResult: true}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, &spyBalanceSyncer{}, lock)

	// startRenew with a small TTL so the ticker fires quickly.
	stop := scheduler.startRenew(context.Background(), "quota:reset:scheduler:lock", 30*time.Millisecond)
	defer stop()

	assert.Eventually(t, func() bool { return lock.renewCalled }, 200*time.Millisecond, 10*time.Millisecond,
		"Renew should be called by watchdog")
}

func TestQuotaResetScheduler_PanicReleasesLock(t *testing.T) {
	panicSyncer := &panicBalanceSyncer{}
	lock := &fakeDistributedLock{acquireResult: true}
	scheduler := NewQuotaResetScheduler(&fakeTxn{}, panicSyncer, lock)

	// resetQuotasWithRecover is the entry point used in production; it recovers
	// from panics in resetQuotas and ensures the lock is released via defer.
	assert.NotPanics(t, func() { scheduler.resetQuotasWithRecover() }, "resetQuotasWithRecover should recover from panic")
	assert.True(t, lock.releaseCalled, "Release should be called even when task panics")
}

// panicBalanceSyncer panics when ResetExpiredBalances is called.
type panicBalanceSyncer struct{}

func (p *panicBalanceSyncer) ResetExpiredBalances(ctx context.Context) error {
	panic("intentional panic")
}

var _ BalanceSyncer = (*panicBalanceSyncer)(nil)
