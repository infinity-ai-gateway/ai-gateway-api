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

package model_price

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/imodel_price"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao"
)

// RDBModelPriceStorager implements imodel_price.ModelPriceStorager using RDB.
type RDBModelPriceStorager struct {
	dbCtxFactory lib.DBContextFactory
}

var _ imodel_price.ModelPriceStorager = &RDBModelPriceStorager{}

// NewRDBModelPriceStorager creates a new RDB-backed model price storager.
func NewRDBModelPriceStorager(dbCtxFactory lib.DBContextFactory) *RDBModelPriceStorager {
	return &RDBModelPriceStorager{
		dbCtxFactory: dbCtxFactory,
	}
}

func (s *RDBModelPriceStorager) CreateModelPrice(ctx context.Context, param *imodel_price.ModelPrice) (int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return 0, err
	}

	data, err := toDAOParam(param)
	if err != nil {
		return 0, err
	}

	return dao.TModelPriceCreate(dbCtx, data)
}

func (s *RDBModelPriceStorager) UpdateModelPrice(ctx context.Context, filter *imodel_price.ModelPriceFilter, param *imodel_price.ModelPrice) (int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return 0, err
	}

	data, err := toDAOParam(param)
	if err != nil {
		return 0, err
	}

	where := filterToDAOParam(filter)
	return dao.TModelPriceUpdate(dbCtx, data, where)
}

func (s *RDBModelPriceStorager) DeleteModelPrice(ctx context.Context, filter *imodel_price.ModelPriceFilter) error {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return err
	}

	where := filterToDAOParam(filter)
	_, err = dao.TModelPriceDelete(dbCtx, where)
	return err
}

func (s *RDBModelPriceStorager) DeleteAllModelPrices(ctx context.Context) error {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return err
	}

	_, err = dbCtx.Execer().ExecContext(dbCtx, fmt.Sprintf("DELETE FROM %s", dao.TModelPriceTableName()))
	return err
}

func (s *RDBModelPriceStorager) FetchModelPrice(ctx context.Context, filter *imodel_price.ModelPriceFilter) (*imodel_price.ModelPrice, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	where := filterToDAOParam(filter)
	one, err := dao.TModelPriceOne(dbCtx, where)
	if err != nil {
		return nil, err
	}
	if one == nil {
		return nil, nil
	}
	return fromDAO(one), nil
}

func (s *RDBModelPriceStorager) FetchModelPriceList(ctx context.Context, filter *imodel_price.ModelPriceFilter) ([]*imodel_price.ModelPrice, int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, 0, err
	}

	where := filterToDAOParam(filter)

	page := 1
	pageSize := 50
	if filter != nil {
		if filter.Page != nil && *filter.Page > 0 {
			page = *filter.Page
		}
		if filter.PageSize != nil && *filter.PageSize > 0 {
			pageSize = *filter.PageSize
			if pageSize > 1000 {
				pageSize = 1000
			}
		}
	}

	list, err := dao.TModelPriceListWithPagination(dbCtx, where, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	total, err := dao.TModelPriceCount(dbCtx, where)
	if err != nil {
		return nil, 0, err
	}

	rst := make([]*imodel_price.ModelPrice, 0, len(list))
	for _, one := range list {
		rst = append(rst, fromDAO(one))
	}
	return rst, total, nil
}

func (s *RDBModelPriceStorager) ListProviders(ctx context.Context) ([]string, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	rows, err := dbCtx.Execer().QueryContext(dbCtx, fmt.Sprintf("SELECT DISTINCT provider FROM %s ORDER BY provider", dao.TModelPriceTableName()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []string
	for rows.Next() {
		var provider string
		if err := rows.Scan(&provider); err != nil {
			return nil, err
		}
		providers = append(providers, provider)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return providers, nil
}

func toDAOParam(param *imodel_price.ModelPrice) (*dao.TModelPriceParam, error) {
	if param == nil {
		return nil, nil
	}

	capabilities, err := marshalJSON(param.Capabilities)
	if err != nil {
		return nil, err
	}
	supportedParameters, err := marshalJSON(param.SupportedParameters)
	if err != nil {
		return nil, err
	}
	limits, err := marshalJSON(param.Limits)
	if err != nil {
		return nil, err
	}
	prices, err := marshalJSON(param.Prices)
	if err != nil {
		return nil, err
	}
	tierPrices, err := marshalJSON(param.TierPrices)
	if err != nil {
		return nil, err
	}
	metadata, err := marshalJSON(param.Metadata)
	if err != nil {
		return nil, err
	}

	return &dao.TModelPriceParam{
		Provider:            lib.PString(param.Provider),
		Model:               lib.PString(param.Model),
		BaseModel:           lib.PString(param.BaseModel),
		Mode:                lib.PString(param.Mode),
		Capabilities:        capabilities,
		SupportedParameters: supportedParameters,
		Limits:              limits,
		Prices:              prices,
		TierPrices:          tierPrices,
		PriceCurrency:       lib.PString(param.PriceCurrency),
		Metadata:            metadata,
	}, nil
}

func filterToDAOParam(filter *imodel_price.ModelPriceFilter) *dao.TModelPriceParam {
	if filter == nil {
		return nil
	}
	return &dao.TModelPriceParam{
		ID:       filter.ID,
		Provider: filter.Provider,
		Model:    filter.Model,
		Mode:     filter.Mode,
	}
}

func fromDAO(one *dao.TModelPrice) *imodel_price.ModelPrice {
	if one == nil {
		return nil
	}

	createTime := one.CreatedAt.Unix()
	updateTime := one.UpdatedAt.Unix()

	return &imodel_price.ModelPrice{
		ID:                  one.ID,
		Provider:            one.Provider,
		Model:               one.Model,
		BaseModel:           one.BaseModel,
		Mode:                one.Mode,
		Capabilities:        unmarshalStringSlice(one.Capabilities),
		SupportedParameters: unmarshalStringSlice(one.SupportedParameters),
		Limits:              unmarshalMap(one.Limits),
		Prices:              unmarshalPriceMap(one.Prices),
		TierPrices:          unmarshalTierPrices(one.TierPrices),
		PriceCurrency:       one.PriceCurrency,
		Metadata:            unmarshalMap(one.Metadata),
		CreateTime:          &createTime,
		UpdateTime:          &updateTime,
	}
}

func marshalJSON(v interface{}) (*string, error) {
	if v == nil {
		return lib.PString("{}"), nil
	}
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return lib.PString(string(data)), nil
}

func unmarshalStringSlice(s string) []string {
	if s == "" || s == "null" {
		return nil
	}
	var rst []string
	if err := json.Unmarshal([]byte(s), &rst); err != nil {
		return nil
	}
	return rst
}

func unmarshalMap(s string) map[string]interface{} {
	if s == "" || s == "null" {
		return nil
	}
	var rst map[string]interface{}
	if err := json.Unmarshal([]byte(s), &rst); err != nil {
		return nil
	}
	return rst
}

func unmarshalPriceMap(s string) imodel_price.PriceMap {
	if s == "" || s == "null" {
		return nil
	}
	var rst imodel_price.PriceMap
	if err := json.Unmarshal([]byte(s), &rst); err != nil {
		return nil
	}
	return rst
}

func unmarshalTierPrices(s string) imodel_price.TierPriceMap {
	if s == "" || s == "null" {
		return nil
	}
	var rst imodel_price.TierPriceMap
	if err := json.Unmarshal([]byte(s), &rst); err != nil {
		return nil
	}
	return rst
}
