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

package dao

import (
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao/internal"
)

const tOperationLogTableName = "operation_logs"

type TOperationLog struct {
	ID               int64     `db:"id"`
	LogID            string    `db:"log_id"`
	OperatorType     int8      `db:"operator_type"`
	OperatorID       int64     `db:"operator_id"`
	OperatorName     string    `db:"operator_name"`
	Action           string    `db:"action"`
	ResourceType     string    `db:"resource_type"`
	ResourceID       string    `db:"resource_id"`
	ResourceName     string    `db:"resource_name"`
	ResourceParentID string    `db:"resource_parent_id"`
	Status           int8      `db:"status"`
	ErrorMsg         string    `db:"error_msg"`
	ChangeSummary    string    `db:"change_summary"`
	RequestPath      string    `db:"request_path"`
	RequestMethod    string    `db:"request_method"`
	ClientIP         string    `db:"client_ip"`
	UserAgent        string    `db:"user_agent"`
	CreatedAt        time.Time `db:"created_at"`
}

// TOperationLogOne Query One
// return nil, nil if record not existed
func TOperationLogOne(dbCtx lib.DBContexter, where *TOperationLogParam) (*TOperationLog, error) {
	t := &TOperationLog{}
	err := internal.QueryOne(dbCtx, tOperationLogTableName, where, t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TOperationLogList Query Multiple
func TOperationLogList(dbCtx lib.DBContexter, where *TOperationLogParam) ([]*TOperationLog, error) {
	t := []*TOperationLog{}
	err := internal.QueryList(dbCtx, tOperationLogTableName, where, &t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

type TOperationLogParam struct {
	ID               *int64     `db:"id"`
	LogID            *string    `db:"log_id"`
	OperatorType     *int8      `db:"operator_type"`
	OperatorID       *int64     `db:"operator_id"`
	OperatorName     *string    `db:"operator_name"`
	Action           *string    `db:"action"`
	ResourceType     *string    `db:"resource_type"`
	ResourceID       *string    `db:"resource_id"`
	ResourceName     *string    `db:"resource_name"`
	ResourceParentID *string    `db:"resource_parent_id"`
	Status           *int8      `db:"status"`
	ErrorMsg         *string    `db:"error_msg"`
	ChangeSummary    *string    `db:"change_summary"`
	RequestPath      *string    `db:"request_path"`
	RequestMethod    *string    `db:"request_method"`
	ClientIP         *string    `db:"client_ip"`
	UserAgent        *string    `db:"user_agent"`
	CreatedAt        *time.Time `db:"created_at"`
	CreatedAtGTE     *time.Time `db:"created_at,>="`
	CreatedAtLTE     *time.Time `db:"created_at,<="`

	OrderBy *string `db:"_orderby"`
	Limit   []uint  `db:"_limit"`
}

// TOperationLogCount returns the total count matched by where.
func TOperationLogCount(dbCtx lib.DBContexter, where *TOperationLogParam) (int64, error) {
	type countResult struct {
		Count int64 `db:"count(*)"`
	}
	list := []*countResult{}
	whereMap := internal.Struct2Where(where)
	// Count should not be constrained by pagination/order clauses.
	delete(whereMap, "_limit")
	delete(whereMap, "_orderby")
	build := internal.NewSelectBuilder(tOperationLogTableName, whereMap, []string{"count(*)"})
	sql, args, err := build.Compile()
	if err != nil {
		return 0, xerror.WrapDaoError(err)
	}

	rows, err := dbCtx.Execer().QueryContext(dbCtx, sql, args...)
	if err != nil {
		return 0, xerror.WrapDaoError(err)
	}
	defer rows.Close()

	for rows.Next() {
		var c countResult
		if err := rows.Scan(&c.Count); err != nil {
			return 0, xerror.WrapDaoError(err)
		}
		list = append(list, &c)
	}
	if len(list) == 0 {
		return 0, nil
	}
	return list[0].Count, nil
}

// TOperationLogCreate One/Multiple
func TOperationLogCreate(dbCtx lib.DBContexter, data ...*TOperationLogParam) (int64, error) {
	if len(data) == 1 {
		if data[0].CreatedAt == nil {
			data[0].CreatedAt = internal.PTimeNow()
		}
		return internal.Create(dbCtx, tOperationLogTableName, data[0])
	}

	list := make([]interface{}, len(data))
	for i, one := range data {
		if one.CreatedAt == nil {
			one.CreatedAt = internal.PTimeNow()
		}
		list[i] = one
	}

	return internal.Create(dbCtx, tOperationLogTableName, list...)
}
