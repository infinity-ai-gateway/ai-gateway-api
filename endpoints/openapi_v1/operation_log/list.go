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
	"net/http"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

var OperationLogListRoute = &xreq.Endpoint{
	Path:       "/operation-logs",
	Method:     http.MethodGet,
	Handler:    xreq.Convert(OperationLogListAction),
	Authorizer: iauth.FA(iauth.FeatureOperationLog, iauth.ActionReadAll),
}

type operationLogListResponse struct {
	List       []*operationLogItem `json:"list"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}

type operationLogItem struct {
	ID               int64                  `json:"id"`
	LogID            string                 `json:"log_id"`
	OperatorType     int8                   `json:"operator_type"`
	OperatorID       int64                  `json:"operator_id"`
	OperatorName     string                 `json:"operator_name"`
	Action           string                 `json:"action"`
	ResourceType     string                 `json:"resource_type"`
	ResourceID       string                 `json:"resource_id"`
	ResourceName     string                 `json:"resource_name"`
	ResourceParentID string                 `json:"resource_parent_id"`
	Status           int8                   `json:"status"`
	ErrorMsg         string                 `json:"error_msg"`
	ChangeSummary    map[string]interface{} `json:"change_summary,omitempty"`
	RequestPath      string                 `json:"request_path"`
	RequestMethod    string                 `json:"request_method"`
	ClientIP         string                 `json:"client_ip"`
	UserAgent        string                 `json:"user_agent"`
	CreatedAt        int64                  `json:"created_at"`
}

type operationLogListFilter struct {
	OperatorName     *string `form:"operator_name"`
	Action           *string `form:"action"`
	ResourceType     *string `form:"resource_type"`
	ResourceID       *string `form:"resource_id"`
	ResourceName     *string `form:"resource_name"`
	ResourceParentID *string `form:"resource_parent_id"`
	Status           *int8   `form:"status"`
	StartTime        *int64  `form:"start_time"`
	EndTime          *int64  `form:"end_time"`
	Page             *int    `form:"page"`
	PageSize         *int    `form:"page_size"`
}

func OperationLogListAction(req *http.Request) (interface{}, error) {
	filter := &operationLogListFilter{}
	if err := xreq.BindForm(req, filter); err != nil {
		return nil, err
	}

	page := 1
	pageSize := 20
	if filter.Page != nil && *filter.Page > 0 {
		page = *filter.Page
	}
	if filter.PageSize != nil && *filter.PageSize > 0 {
		pageSize = *filter.PageSize
	}
	if pageSize > 100 {
		pageSize = 100
	}

	managerFilter := &ioperlog.OperationLogFilter{
		OperatorName:     filter.OperatorName,
		Action:           filter.Action,
		ResourceType:     filter.ResourceType,
		ResourceID:       filter.ResourceID,
		ResourceName:     filter.ResourceName,
		ResourceParentID: filter.ResourceParentID,
		Status:           filter.Status,
		Page:             &page,
		PageSize:         &pageSize,
	}

	if filter.StartTime != nil {
		t := time.Unix(*filter.StartTime, 0)
		managerFilter.StartTime = &t
	}
	if filter.EndTime != nil {
		t := time.Unix(*filter.EndTime, 0)
		managerFilter.EndTime = &t
	}

	result, err := container.OperationLogManager.QueryLogs(req.Context(), managerFilter)
	if err != nil {
		return nil, err
	}

	resp := &operationLogListResponse{
		List: convertToResponseItems(result.List),
	}
	resp.Pagination.Page = page
	resp.Pagination.PageSize = pageSize
	resp.Pagination.Total = result.Total

	return resp, nil
}

func convertToResponseItems(entries []*ioperlog.OperationLogEntry) []*operationLogItem {
	items := make([]*operationLogItem, 0, len(entries))
	for _, entry := range entries {
		items = append(items, &operationLogItem{
			ID:               entry.ID,
			LogID:            entry.LogID,
			OperatorType:     int8(entry.OperatorType),
			OperatorID:       entry.OperatorID,
			OperatorName:     entry.OperatorName,
			Action:           entry.Action,
			ResourceType:     entry.ResourceType,
			ResourceID:       entry.ResourceID,
			ResourceName:     entry.ResourceName,
			ResourceParentID: entry.ResourceParentID,
			Status:           entry.Status,
			ErrorMsg:         entry.ErrorMsg,
			ChangeSummary:    entry.ChangeSummary,
			RequestPath:      entry.RequestPath,
			RequestMethod:    entry.RequestMethod,
			ClientIP:         entry.ClientIP,
			UserAgent:        entry.UserAgent,
			CreatedAt:        entry.CreatedAt.Unix(),
		})
	}
	return items
}
