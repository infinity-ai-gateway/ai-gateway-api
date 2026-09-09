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

package epp_pool

import (
	"context"
	"runtime"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

// EppReconciler periodically reconciles EPP cluster assignments
// (design-changes.md §4.3: idempotent scan plus back-fill allocation).
// Start/Stop follow the process lifecycle like QuotaResetScheduler.
type EppReconciler struct {
	interval    time.Duration
	reconcileFn func(ctx context.Context) error
	stopCh      chan struct{}
}

// NewEppReconciler creates the reconciler. reconcile is invoked every interval.
func NewEppReconciler(interval time.Duration, reconcile func(ctx context.Context) error) *EppReconciler {
	if interval <= 0 {
		interval = DefaultReconcileInterval
	}
	return &EppReconciler{
		interval:    interval,
		reconcileFn: reconcile,
		stopCh:      make(chan struct{}),
	}
}

// Start launches the reconcile loop in the background.
func (r *EppReconciler) Start() {
	go r.run()
}

// Stop terminates the reconcile loop.
func (r *EppReconciler) Stop() {
	close(r.stopCh)
}

func (r *EppReconciler) run() {
	defer func() {
		if err := recover(); err != nil {
			stack := make([]byte, 1024*8)
			stack = stack[:runtime.Stack(stack, false)]
			stateful.ExceptionLogger.Error("PANIC in EppReconciler: err=%v\n%s", err, string(stack))
		}
	}()

	r.reconcileWithRecover()

	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.reconcileWithRecover()
		case <-r.stopCh:
			return
		}
	}
}

func (r *EppReconciler) reconcileWithRecover() {
	defer func() {
		if err := recover(); err != nil {
			stack := make([]byte, 1024*8)
			stack = stack[:runtime.Stack(stack, false)]
			stateful.ExceptionLogger.Error("PANIC in EppReconciler reconcile: err=%v\n%s", err, string(stack))
		}
	}()

	if r.reconcileFn == nil {
		return
	}

	if err := r.reconcileFn(context.Background()); err != nil {
		stateful.AccessLogger.Error("EppReconciler reconcile failed: %v", err)
	}
}

// eppLogError logs a non-fatal epp pool error.
func eppLogError(format string, args ...interface{}) {
	stateful.AccessLogger.Error(format, args...)
}
