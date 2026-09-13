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
	"encoding/json"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao"
)

// OperationLogStorager implements ioperlog.OperationLogStorager.
type OperationLogStorager struct {
	dbCtxFactory lib.DBContextFactory
}

// NewOperationLogStorager creates a new OperationLogStorager.
func NewOperationLogStorager(dbCtxFactory lib.DBContextFactory) *OperationLogStorager {
	return &OperationLogStorager{
		dbCtxFactory: dbCtxFactory,
	}
}

var _ ioperlog.OperationLogStorager = (*OperationLogStorager)(nil)

// BatchCreate persists a batch of operation log entries.
func (s *OperationLogStorager) BatchCreate(ctx context.Context, entries []*ioperlog.OperationLogEntry) error {
	if len(entries) == 0 {
		return nil
	}

	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return err
	}

	params := make([]*dao.TOperationLogParam, 0, len(entries))
	for _, entry := range entries {
		params = append(params, entryToParam(entry))
	}

	_, err = dao.TOperationLogCreate(dbCtx, params...)
	return err
}

// List queries operation logs with the given filter.
func (s *OperationLogStorager) List(ctx context.Context, filter *ioperlog.OperationLogFilter) ([]*ioperlog.OperationLogEntry, int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, 0, err
	}

	where := filterToParam(filter)

	total, err := dao.TOperationLogCount(dbCtx, where)
	if err != nil {
		return nil, 0, err
	}

	list, err := dao.TOperationLogList(dbCtx, where)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*ioperlog.OperationLogEntry, 0, len(list))
	for _, one := range list {
		result = append(result, paramToEntry(one))
	}

	return result, total, nil
}

func entryToParam(entry *ioperlog.OperationLogEntry) *dao.TOperationLogParam {
	param := &dao.TOperationLogParam{
		LogID:            &entry.LogID,
		OperatorType:     (*int8)(&entry.OperatorType),
		OperatorID:       &entry.OperatorID,
		OperatorName:     &entry.OperatorName,
		Action:           &entry.Action,
		ResourceType:     &entry.ResourceType,
		ResourceID:       &entry.ResourceID,
		ResourceName:     &entry.ResourceName,
		ResourceParentID: &entry.ResourceParentID,
		Status:           &entry.Status,
		ErrorMsg:         &entry.ErrorMsg,
		RequestPath:      &entry.RequestPath,
		RequestMethod:    &entry.RequestMethod,
		ClientIP:         &entry.ClientIP,
		UserAgent:        &entry.UserAgent,
		CreatedAt:        &entry.CreatedAt,
	}

	if len(entry.ChangeSummary) > 0 {
		bs, _ := json.Marshal(entry.ChangeSummary)
		changeSummary := string(bs)
		param.ChangeSummary = &changeSummary
	}

	return param
}

func paramToEntry(one *dao.TOperationLog) *ioperlog.OperationLogEntry {
	entry := &ioperlog.OperationLogEntry{
		ID:               one.ID,
		LogID:            one.LogID,
		OperatorType:     ioperlog.OperatorType(one.OperatorType),
		OperatorID:       one.OperatorID,
		OperatorName:     one.OperatorName,
		Action:           one.Action,
		ResourceType:     one.ResourceType,
		ResourceID:       one.ResourceID,
		ResourceName:     one.ResourceName,
		ResourceParentID: one.ResourceParentID,
		Status:           one.Status,
		ErrorMsg:         one.ErrorMsg,
		RequestPath:      one.RequestPath,
		RequestMethod:    one.RequestMethod,
		ClientIP:         one.ClientIP,
		UserAgent:        one.UserAgent,
		CreatedAt:        one.CreatedAt,
	}

	if one.ChangeSummary != "" {
		_ = json.Unmarshal([]byte(one.ChangeSummary), &entry.ChangeSummary)
	}

	return entry
}

func filterToParam(filter *ioperlog.OperationLogFilter) *dao.TOperationLogParam {
	if filter == nil {
		return nil
	}

	param := &dao.TOperationLogParam{}

	if filter.LogID != nil {
		param.LogID = filter.LogID
	}
	if filter.OperatorType != nil {
		operatorType := int8(*filter.OperatorType)
		param.OperatorType = &operatorType
	}
	if filter.OperatorID != nil {
		param.OperatorID = filter.OperatorID
	}
	if filter.OperatorName != nil {
		param.OperatorName = filter.OperatorName
	}
	if filter.Action != nil {
		param.Action = filter.Action
	}
	if filter.ResourceType != nil {
		param.ResourceType = filter.ResourceType
	}
	if filter.ResourceID != nil {
		param.ResourceID = filter.ResourceID
	}
	if filter.ResourceName != nil {
		param.ResourceName = filter.ResourceName
	}
	if filter.ResourceParentID != nil {
		param.ResourceParentID = filter.ResourceParentID
	}
	if filter.Status != nil {
		param.Status = filter.Status
	}
	if filter.StartTime != nil {
		param.CreatedAtGTE = filter.StartTime
	}
	if filter.EndTime != nil {
		param.CreatedAtLTE = filter.EndTime
	}

	if filter.Page != nil && filter.PageSize != nil {
		offset := (*filter.Page - 1) * (*filter.PageSize)
		if offset < 0 {
			offset = 0
		}
		param.Limit = []uint{uint(offset), uint(*filter.PageSize)}
		orderBy := "created_at DESC"
		param.OrderBy = &orderBy
	}

	return param
}
