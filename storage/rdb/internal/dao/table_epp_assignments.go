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

const tEppAssignmentTableName = "epp_assignments"

type TEppAssignment struct {
	Cluster           string    `db:"cluster"`
	GroupName         string    `db:"group_name"`
	PrimaryInstanceID string    `db:"primary_instance_id"`
	CreateTime        time.Time `db:"create_time"`
	UpdateTime        time.Time `db:"update_time"`
}

// TEppAssignmentOne Query One
// return nil, nil if record not existed
func TEppAssignmentOne(dbCtx lib.DBContexter, where *TEppAssignmentParam) (*TEppAssignment, error) {
	t := &TEppAssignment{}
	err := internal.QueryOne(dbCtx, tEppAssignmentTableName, where, t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TEppAssignmentList Query Multiple
func TEppAssignmentList(dbCtx lib.DBContexter, where *TEppAssignmentParam) ([]*TEppAssignment, error) {
	t := []*TEppAssignment{}
	err := internal.QueryList(dbCtx, tEppAssignmentTableName, where, &t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

type TEppAssignmentParam struct {
	Cluster           *string    `db:"cluster"`
	GroupName         *string    `db:"group_name"`
	PrimaryInstanceID *string    `db:"primary_instance_id"`
	CreateTime        *time.Time `db:"create_time"`
	UpdateTime        *time.Time `db:"update_time"`

	OrderBy *string `db:"_orderby"`
}

// TEppAssignmentCreate One/Multiple
func TEppAssignmentCreate(dbCtx lib.DBContexter, data ...*TEppAssignmentParam) (int64, error) {
	if len(data) == 1 {
		if data[0].CreateTime == nil {
			data[0].CreateTime = internal.PTimeNow()
		}
		return internal.Create(dbCtx, tEppAssignmentTableName, data[0])
	}

	list := make([]interface{}, len(data))
	for i, one := range data {
		if one.CreateTime == nil {
			one.CreateTime = internal.PTimeNow()
		}
		list[i] = one
	}

	return internal.Create(dbCtx, tEppAssignmentTableName, list...)
}

// TEppAssignmentUpdate Update One
func TEppAssignmentUpdate(dbCtx lib.DBContexter, val, where *TEppAssignmentParam) (int64, error) {
	return internal.Update(dbCtx, tEppAssignmentTableName, where, val)
}

// TEppAssignmentDelete Delete One/Multiple
func TEppAssignmentDelete(dbCtx lib.DBContexter, where *TEppAssignmentParam) (int64, error) {
	return internal.Delete(dbCtx, tEppAssignmentTableName, where)
}
