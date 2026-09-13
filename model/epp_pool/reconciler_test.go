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
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEppReconciler_RunAndStop(t *testing.T) {
	var calls int32
	r := NewEppReconciler(5, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return nil
	})

	r.Start()
	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) >= 2 }, time.Second, 5*time.Millisecond)
	r.Stop()
}

func TestEppReconciler_ReconcileErrorSwallowed(t *testing.T) {
	var calls int32
	r := NewEppReconciler(5, func(ctx context.Context) error {
		atomic.AddInt32(&calls, 1)
		return errors.New("boom")
	})

	r.Start()
	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) >= 1 }, time.Second, 5*time.Millisecond)
	r.Stop()
}

func TestEppReconciler_PanicSwallowed(t *testing.T) {
	var calls int32
	r := NewEppReconciler(5, func(ctx context.Context) error {
		if atomic.AddInt32(&calls, 1) == 1 {
			panic("boom")
		}
		return nil
	})

	r.Start()
	require.Eventually(t, func() bool { return atomic.LoadInt32(&calls) >= 2 }, time.Second, 5*time.Millisecond)
	r.Stop()
}

func TestEppReconciler_NilReconcile(t *testing.T) {
	r := NewEppReconciler(5, nil)
	r.Start()
	r.Stop()
}

func TestEppReconciler_DefaultInterval(t *testing.T) {
	r := NewEppReconciler(0, nil)
	assert.Equal(t, DefaultReconcileInterval, r.interval)
}
