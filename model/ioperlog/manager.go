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
	"sync"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

const (
	defaultBufferSize   = 4096
	defaultBatchSize    = 200
	defaultFlushTimeout = 5 * time.Second
	defaultRetryTimes   = 3
)

// OverflowStrategy defines how to handle a full buffer.
type OverflowStrategy string

const (
	// OverflowStrategySync blocks the caller and writes synchronously.
	OverflowStrategySync OverflowStrategy = "sync"
	// OverflowStrategyDiscard drops the log and emits a warning.
	OverflowStrategyDiscard OverflowStrategy = "discard"
)

// ContextExtractor extracts request-level fields from context into an entry.
type ContextExtractor func(ctx context.Context, entry *OperationLogEntry)

// OperationLogManager buffers and persists operation logs asynchronously.
type OperationLogManager struct {
	storager OperationLogStorager

	buffer           chan *OperationLogEntry
	closeCh          chan struct{}
	closeOnce        sync.Once
	wg               sync.WaitGroup
	contextExtractor ContextExtractor

	batchSize    int
	flushTimeout time.Duration
	retryTimes   int
	overflow     OverflowStrategy
}

// NewOperationLogManager creates a new OperationLogManager.
func NewOperationLogManager(storager OperationLogStorager, bufferSize int) *OperationLogManager {
	if bufferSize <= 0 {
		bufferSize = defaultBufferSize
	}

	m := &OperationLogManager{
		storager:     storager,
		buffer:       make(chan *OperationLogEntry, bufferSize),
		closeCh:      make(chan struct{}),
		batchSize:    defaultBatchSize,
		flushTimeout: defaultFlushTimeout,
		retryTimes:   defaultRetryTimes,
		overflow:     OverflowStrategySync,
	}

	m.wg.Add(1)
	go m.batchWorker()

	return m
}

// SetContextExtractor injects the function used to extract request-level fields from context.
func (m *OperationLogManager) SetContextExtractor(extractor ContextExtractor) {
	m.contextExtractor = extractor
}

// Record submits an operation log entry asynchronously.
// It does not block the caller unless the buffer is full and overflow strategy is Sync.
func (m *OperationLogManager) Record(ctx context.Context, entry *OperationLogEntry) {
	if entry == nil {
		return
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	if m.contextExtractor != nil {
		m.contextExtractor(ctx, entry)
	}

	select {
	case m.buffer <- entry:
	case <-m.closeCh:
		// Manager is closing; try sync write as a best-effort fallback.
		if err := m.RecordSync(ctx, entry); err != nil {
			stateful.AccessLogger.Warn("operation log sync write failed after close: %v", err)
		}
	default:
		switch m.overflow {
		case OverflowStrategyDiscard:
			stateful.AccessLogger.Warn("operation log buffer full, dropping log for %s/%s", entry.ResourceType, entry.ResourceID)
		default:
			// Sync fallback: block briefly and write directly.
			if err := m.RecordSync(ctx, entry); err != nil {
				stateful.AccessLogger.Warn("operation log sync write failed: %v", err)
			}
		}
	}
}

// RecordSync writes the operation log synchronously.
func (m *OperationLogManager) RecordSync(ctx context.Context, entry *OperationLogEntry) error {
	if entry == nil {
		return nil
	}

	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	if m.contextExtractor != nil {
		m.contextExtractor(ctx, entry)
	}

	return m.storager.BatchCreate(ctx, []*OperationLogEntry{entry})
}

// QueryLogs queries operation logs with the given filter.
func (m *OperationLogManager) QueryLogs(ctx context.Context, filter *OperationLogFilter) (*OperationLogQueryResult, error) {
	list, total, err := m.storager.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	page := 1
	pageSize := len(list)
	if filter != nil && filter.Page != nil {
		page = *filter.Page
	}
	if filter != nil && filter.PageSize != nil {
		pageSize = *filter.PageSize
	}

	return &OperationLogQueryResult{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		List:     list,
	}, nil
}

// Close gracefully shuts down the manager, flushing buffered logs.
func (m *OperationLogManager) Close() error {
	m.closeOnce.Do(func() {
		close(m.closeCh)
	})
	m.wg.Wait()
	return nil
}

func (m *OperationLogManager) batchWorker() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.flushTimeout)
	defer ticker.Stop()

	batch := make([]*OperationLogEntry, 0, m.batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		m.flushBatch(batch)
		batch = batch[:0]
		ticker.Reset(m.flushTimeout)
	}

	for {
		select {
		case entry, ok := <-m.buffer:
			if !ok {
				flush()
				return
			}
			batch = append(batch, entry)
			if len(batch) >= m.batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-m.closeCh:
			// Drain remaining buffer with a short timeout.
			drainTimeout := time.After(500 * time.Millisecond)
		drain:
			for {
				select {
				case entry, ok := <-m.buffer:
					if !ok {
						break drain
					}
					batch = append(batch, entry)
					if len(batch) >= m.batchSize {
						m.flushBatch(batch)
						batch = batch[:0]
					}
					// Reset timeout while entries are still arriving.
					drainTimeout = time.After(500 * time.Millisecond)
				case <-drainTimeout:
					break drain
				}
			}
			flush()
			return
		}
	}
}

func (m *OperationLogManager) flushBatch(batch []*OperationLogEntry) {
	if len(batch) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error
	for i := 0; i < m.retryTimes; i++ {
		err = m.storager.BatchCreate(ctx, batch)
		if err == nil {
			return
		}
		time.Sleep(time.Duration(i+1) * 100 * time.Millisecond)
	}

	stateful.AccessLogger.Warn("operation log batch write failed after %d retries: %v", m.retryTimes, err)
}
