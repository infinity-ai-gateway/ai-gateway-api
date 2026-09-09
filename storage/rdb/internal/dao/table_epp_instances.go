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

const tEppInstanceTableName = "epp_instances"

type TEppInstance struct {
	ID         string    `db:"id"`
	Host       string    `db:"host"`
	Port       int       `db:"port"`
	GroupName  string    `db:"group_name"`
	CreateTime time.Time `db:"create_time"`
	UpdateTime time.Time `db:"update_time"`
}

// TEppInstanceOne Query One
// return nil, nil if record not existed
func TEppInstanceOne(dbCtx lib.DBContexter, where *TEppInstanceParam) (*TEppInstance, error) {
	t := &TEppInstance{}
	err := internal.QueryOne(dbCtx, tEppInstanceTableName, where, t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TEppInstanceList Query Multiple
func TEppInstanceList(dbCtx lib.DBContexter, where *TEppInstanceParam) ([]*TEppInstance, error) {
	t := []*TEppInstance{}
	err := internal.QueryList(dbCtx, tEppInstanceTableName, where, &t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

type TEppInstanceParam struct {
	ID         *string    `db:"id"`
	Host       *string    `db:"host"`
	Port       *int       `db:"port"`
	GroupName  *string    `db:"group_name"`
	CreateTime *time.Time `db:"create_time"`
	UpdateTime *time.Time `db:"update_time"`

	OrderBy *string `db:"_orderby"`
}

// TEppInstanceCreate One/Multiple
func TEppInstanceCreate(dbCtx lib.DBContexter, data ...*TEppInstanceParam) (int64, error) {
	if len(data) == 1 {
		if data[0].CreateTime == nil {
			data[0].CreateTime = internal.PTimeNow()
		}
		return internal.Create(dbCtx, tEppInstanceTableName, data[0])
	}

	list := make([]interface{}, len(data))
	for i, one := range data {
		if one.CreateTime == nil {
			one.CreateTime = internal.PTimeNow()
		}
		list[i] = one
	}

	return internal.Create(dbCtx, tEppInstanceTableName, list...)
}

// TEppInstanceUpdate Update One
func TEppInstanceUpdate(dbCtx lib.DBContexter, val, where *TEppInstanceParam) (int64, error) {
	return internal.Update(dbCtx, tEppInstanceTableName, where, val)
}

// TEppInstanceDelete Delete One/Multiple
func TEppInstanceDelete(dbCtx lib.DBContexter, where *TEppInstanceParam) (int64, error) {
	return internal.Delete(dbCtx, tEppInstanceTableName, where)
}
