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

package operation_log

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOperationLogManager struct {
	lastFilter *ioperlog.OperationLogFilter
	result     *ioperlog.OperationLogQueryResult
}

func (m *fakeOperationLogManager) Record(ctx context.Context, entry *ioperlog.OperationLogEntry) {}

func (m *fakeOperationLogManager) QueryLogs(ctx context.Context, filter *ioperlog.OperationLogFilter) (*ioperlog.OperationLogQueryResult, error) {
	m.lastFilter = filter
	return m.result, nil
}

func (m *fakeOperationLogManager) SetContextExtractor(extractor ioperlog.ContextExtractor) {}

func (m *fakeOperationLogManager) Close() error { return nil }

func TestOperationLogListAction(t *testing.T) {
	manager := &fakeOperationLogManager{
		result: &ioperlog.OperationLogQueryResult{
			Total:    1,
			Page:     1,
			PageSize: 20,
			List: []*ioperlog.OperationLogEntry{
				{
					ID:           1,
					LogID:        "log-1",
					OperatorName: "admin",
					Action:       "create",
					ResourceType: "entity",
					ResourceID:   "entity-1",
					ResourceName: "test-entity",
					Status:       ioperlog.StatusSuccess,
					CreatedAt:    time.Unix(1725091200, 0),
				},
			},
		},
	}
	container.OperationLogManager = manager

	req := httptest.NewRequest(http.MethodGet, "/open-api/v1/operation-logs?operator_name=admin&action=create&resource_type=entity&start_time=1725091100&end_time=1725091300&page=1&page_size=10", nil)
	resp, err := OperationLogListAction(req)
	require.NoError(t, err)

	listResp, ok := resp.(*operationLogListResponse)
	require.True(t, ok)
	assert.Equal(t, int64(1), listResp.Pagination.Total)
	assert.Equal(t, 1, listResp.Pagination.Page)
	assert.Equal(t, 10, listResp.Pagination.PageSize)
	require.Len(t, listResp.List, 1)
	assert.Equal(t, "admin", listResp.List[0].OperatorName)
	assert.Equal(t, "entity-1", listResp.List[0].ResourceID)

	require.NotNil(t, manager.lastFilter)
	assert.Equal(t, "admin", *manager.lastFilter.OperatorName)
	assert.Equal(t, "create", *manager.lastFilter.Action)
	assert.Equal(t, "entity", *manager.lastFilter.ResourceType)
	assert.NotNil(t, manager.lastFilter.StartTime)
	assert.NotNil(t, manager.lastFilter.EndTime)
}

func TestOperationLogListAction_DefaultPagination(t *testing.T) {
	manager := &fakeOperationLogManager{
		result: &ioperlog.OperationLogQueryResult{
			Total:    0,
			Page:     1,
			PageSize: 20,
			List:     []*ioperlog.OperationLogEntry{},
		},
	}
	container.OperationLogManager = manager

	req := httptest.NewRequest(http.MethodGet, "/open-api/v1/operation-logs", nil)
	resp, err := OperationLogListAction(req)
	require.NoError(t, err)

	listResp, ok := resp.(*operationLogListResponse)
	require.True(t, ok)
	assert.Equal(t, 1, listResp.Pagination.Page)
	assert.Equal(t, 20, listResp.Pagination.PageSize)
}
