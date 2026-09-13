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

package ioperlog

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type fakeOperationLogStorager struct {
	mu      sync.Mutex
	batches [][]*OperationLogEntry
	listFn  func(ctx context.Context, filter *OperationLogFilter) ([]*OperationLogEntry, int64, error)
}

func (s *fakeOperationLogStorager) BatchCreate(ctx context.Context, entries []*OperationLogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batches = append(s.batches, entries)
	return nil
}

func (s *fakeOperationLogStorager) List(ctx context.Context, filter *OperationLogFilter) ([]*OperationLogEntry, int64, error) {
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return nil, 0, nil
}

func (s *fakeOperationLogStorager) Batches() [][]*OperationLogEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([][]*OperationLogEntry, len(s.batches))
	copy(result, s.batches)
	return result
}

func TestOperationLogManager_RecordAsync(t *testing.T) {
	storager := &fakeOperationLogStorager{}
	manager := NewOperationLogManager(storager, 100)
	manager.flushTimeout = 50 * time.Millisecond
	defer manager.Close()

	ctx := context.Background()
	manager.Record(ctx, &OperationLogEntry{
		Action:       string(ActionCreate),
		ResourceType: string(ResourceTypeEntity),
		ResourceID:   "entity-1",
	})

	// Wait for the flush timeout to elapse.
	time.Sleep(150 * time.Millisecond)

	batches := storager.Batches()
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)
	assert.Equal(t, "entity-1", batches[0][0].ResourceID)
}

func TestOperationLogManager_RecordSync(t *testing.T) {
	storager := &fakeOperationLogStorager{}
	manager := NewOperationLogManager(storager, 100)
	manager.flushTimeout = 50 * time.Millisecond
	defer manager.Close()

	ctx := context.Background()
	entry := &OperationLogEntry{
		Action:       string(ActionUpdate),
		ResourceType: string(ResourceTypeAPIKey),
		ResourceID:   "key-1",
	}
	err := manager.RecordSync(ctx, entry)
	assert.NoError(t, err)

	batches := storager.Batches()
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)
	assert.Equal(t, "key-1", batches[0][0].ResourceID)
}

func TestOperationLogManager_BatchFlush(t *testing.T) {
	storager := &fakeOperationLogStorager{}
	manager := NewOperationLogManager(storager, 100)
	manager.batchSize = 5
	manager.flushTimeout = 1 * time.Hour // avoid timeout flush during test

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		manager.Record(ctx, &OperationLogEntry{
			Action:       string(ActionCreate),
			ResourceType: string(ResourceTypeEntity),
			ResourceID:   "entity",
		})
	}

	// Wait for async flush.
	time.Sleep(100 * time.Millisecond)

	// Close to ensure any remaining entries are flushed.
	_ = manager.Close()

	batches := storager.Batches()
	total := 0
	for _, batch := range batches {
		total += len(batch)
	}
	assert.Equal(t, 10, total)
}

func TestOperationLogManager_CloseFlush(t *testing.T) {
	storager := &fakeOperationLogStorager{}
	manager := NewOperationLogManager(storager, 100)
	manager.flushTimeout = 50 * time.Millisecond

	ctx := context.Background()
	manager.Record(ctx, &OperationLogEntry{
		Action:       string(ActionDelete),
		ResourceType: string(ResourceTypeProvider),
		ResourceID:   "provider-1",
	})

	err := manager.Close()
	assert.NoError(t, err)

	batches := storager.Batches()
	assert.Len(t, batches, 1)
	assert.Len(t, batches[0], 1)
}

func TestOperationLogManager_ContextExtractor(t *testing.T) {
	storager := &fakeOperationLogStorager{}
	manager := NewOperationLogManager(storager, 100)
	manager.flushTimeout = 50 * time.Millisecond
	defer manager.Close()

	manager.SetContextExtractor(func(ctx context.Context, entry *OperationLogEntry) {
		entry.OperatorName = "admin"
		entry.RequestPath = "/test"
	})

	ctx := context.Background()
	manager.RecordSync(ctx, &OperationLogEntry{
		Action:       string(ActionCreate),
		ResourceType: string(ResourceTypeEntity),
		ResourceID:   "entity-1",
	})

	batches := storager.Batches()
	assert.Equal(t, "admin", batches[0][0].OperatorName)
	assert.Equal(t, "/test", batches[0][0].RequestPath)
}

func TestOperationLogManager_QueryLogs(t *testing.T) {
	expected := []*OperationLogEntry{
		{ResourceID: "entity-1"},
	}
	storager := &fakeOperationLogStorager{
		listFn: func(ctx context.Context, filter *OperationLogFilter) ([]*OperationLogEntry, int64, error) {
			return expected, 1, nil
		},
	}
	manager := NewOperationLogManager(storager, 100)
	manager.flushTimeout = 50 * time.Millisecond
	defer manager.Close()

	result, err := manager.QueryLogs(context.Background(), &OperationLogFilter{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.List, 1)
}

func TestOperationLogManager_QueryLogsError(t *testing.T) {
	expectedErr := errors.New("storage error")
	storager := &fakeOperationLogStorager{
		listFn: func(ctx context.Context, filter *OperationLogFilter) ([]*OperationLogEntry, int64, error) {
			return nil, 0, expectedErr
		},
	}
	manager := NewOperationLogManager(storager, 100)
	manager.flushTimeout = 50 * time.Millisecond
	defer manager.Close()

	_, err := manager.QueryLogs(context.Background(), &OperationLogFilter{})
	assert.ErrorIs(t, err, expectedErr)
}
