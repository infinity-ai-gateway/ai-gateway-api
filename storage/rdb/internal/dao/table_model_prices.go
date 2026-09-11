// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dao

import (
	"time"

	"github.com/didi/gendry/scanner"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao/internal"
)

const tModelPriceTableName = "model_prices"

// TModelPriceTableName returns the table name for model_prices.
func TModelPriceTableName() string {
	return tModelPriceTableName
}

// TModelPrice is the DAO representation of a model_prices row.
type TModelPrice struct {
	ID                  int64     `db:"id"`
	Provider            string    `db:"provider"`
	Model               string    `db:"model"`
	BaseModel           string    `db:"base_model"`
	Mode                string    `db:"mode"`
	Capabilities        string    `db:"capabilities"`
	SupportedParameters string    `db:"supported_parameters"`
	Limits              string    `db:"limits"`
	Prices              string    `db:"prices"`
	TierPrices          string    `db:"tier_prices"`
	PriceCurrency       string    `db:"price_currency"`
	Metadata            string    `db:"metadata"`
	CreatedAt           time.Time `db:"created_at"`
	UpdatedAt           time.Time `db:"updated_at"`
}

// TModelPriceParam is used for create/update/delete/where conditions.
type TModelPriceParam struct {
	ID                  *int64     `db:"id"`
	Provider            *string    `db:"provider"`
	Model               *string    `db:"model"`
	BaseModel           *string    `db:"base_model"`
	Mode                *string    `db:"mode"`
	Capabilities        *string    `db:"capabilities"`
	SupportedParameters *string    `db:"supported_parameters"`
	Limits              *string    `db:"limits"`
	Prices              *string    `db:"prices"`
	TierPrices          *string    `db:"tier_prices"`
	PriceCurrency       *string    `db:"price_currency"`
	Metadata            *string    `db:"metadata"`
	CreatedAt           *time.Time `db:"created_at"`
	UpdatedAt           *time.Time `db:"updated_at"`

	OrderBy *string `db:"_orderby"`
}

// TModelPriceOne queries one record.
func TModelPriceOne(dbCtx lib.DBContexter, where *TModelPriceParam) (*TModelPrice, error) {
	t := &TModelPrice{}
	err := internal.QueryOne(dbCtx, tModelPriceTableName, where, t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TModelPriceList queries multiple records.
func TModelPriceList(dbCtx lib.DBContexter, where *TModelPriceParam) ([]*TModelPrice, error) {
	t := []*TModelPrice{}
	err := internal.QueryList(dbCtx, tModelPriceTableName, where, &t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TModelPriceListWithPagination queries multiple records with pagination.
func TModelPriceListWithPagination(dbCtx lib.DBContexter, where *TModelPriceParam, page, pageSize int) ([]*TModelPrice, error) {
	whereMap := internal.Struct2Where(where)
	whereMap["_limit"] = []uint{uint((page - 1) * pageSize), uint(pageSize)}
	whereMap["_orderby"] = "id"

	build := internal.NewSelectBuilder(tModelPriceTableName, whereMap, nil)
	sql, args, err := build.Compile()
	if err != nil {
		return nil, xerror.WrapDaoError(err)
	}

	rows, err := dbCtx.Execer().QueryContext(dbCtx, sql, args...)
	if err != nil {
		return nil, xerror.WrapDaoError(err)
	}
	defer rows.Close()

	t := []*TModelPrice{}
	if err := scanner.Scan(rows, &t); err != nil {
		if err == scanner.ErrEmptyResult {
			return nil, nil
		}
		return nil, xerror.WrapDaoError(err)
	}
	return t, nil
}

// TModelPriceCount returns the total count matched by where.
func TModelPriceCount(dbCtx lib.DBContexter, where *TModelPriceParam) (int64, error) {
	type countResult struct {
		Count int64 `db:"count(*)"`
	}
	list := []*countResult{}
	whereMap := internal.Struct2Where(where)
	build := internal.NewSelectBuilder(tModelPriceTableName, whereMap, []string{"count(*)"})
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

// TModelPriceCreate creates one or more records.
func TModelPriceCreate(dbCtx lib.DBContexter, data ...*TModelPriceParam) (int64, error) {
	if len(data) == 1 {
		if data[0].CreatedAt == nil {
			data[0].CreatedAt = internal.PTimeNow()
		}
		if data[0].UpdatedAt == nil {
			data[0].UpdatedAt = data[0].CreatedAt
		}
		return internal.Create(dbCtx, tModelPriceTableName, data[0])
	}

	list := make([]interface{}, len(data))
	for i, one := range data {
		if one.CreatedAt == nil {
			one.CreatedAt = internal.PTimeNow()
		}
		if one.UpdatedAt == nil {
			one.UpdatedAt = one.CreatedAt
		}
		list[i] = one
	}
	return internal.Create(dbCtx, tModelPriceTableName, list...)
}

// TModelPriceUpdate updates records.
func TModelPriceUpdate(dbCtx lib.DBContexter, val, where *TModelPriceParam) (int64, error) {
	val.UpdatedAt = lib.PTimeNow()
	return internal.Update(dbCtx, tModelPriceTableName, where, val)
}

// TModelPriceDelete deletes records matched by where.
func TModelPriceDelete(dbCtx lib.DBContexter, where *TModelPriceParam) (int64, error) {
	return internal.Delete(dbCtx, tModelPriceTableName, where)
}
