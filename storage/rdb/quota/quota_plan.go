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

package quota

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quota"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao"
)

type QuotaPlanStorager struct {
	dbCtxFactory lib.DBContextFactory
}

func NewQuotaPlanStorager(dbCtxFactory lib.DBContextFactory) *QuotaPlanStorager {
	return &QuotaPlanStorager{
		dbCtxFactory: dbCtxFactory,
	}
}

var _ quota.QuotaPlanStorager = &QuotaPlanStorager{}

func (s *QuotaPlanStorager) CreateQuotaPlan(ctx context.Context, param *quota.QuotaPlanParam) (int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return 0, err
	}

	data := quotaPlanDataToParam(param)
	data.CreatedAt = lib.PTimeNow()

	return dao.TQuotaPlanCreate(dbCtx, data)
}

func (s *QuotaPlanStorager) FetchQuotaPlan(ctx context.Context, filter *quota.QuotaPlanFilter) (*quota.QuotaPlanParam, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	where := quotaPlanFilterToParam(filter)
	one, err := dao.TQuotaPlanOne(dbCtx, where)
	if err != nil {
		return nil, err
	}
	if one == nil {
		return nil, nil
	}

	return quotaPlanParamToData(one), nil
}

func (s *QuotaPlanStorager) FetchQuotaPlanList(ctx context.Context, filter *quota.QuotaPlanFilter) ([]*quota.QuotaPlanParam, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	where := quotaPlanFilterToParam(filter)
	list, err := dao.TQuotaPlanList(dbCtx, where)
	if err != nil {
		return nil, err
	}

	var rst []*quota.QuotaPlanParam
	for _, one := range list {
		rst = append(rst, quotaPlanParamToData(one))
	}

	return rst, nil
}

func (s *QuotaPlanStorager) UpdateQuotaPlan(ctx context.Context, filter *quota.QuotaPlanFilter, param *quota.QuotaPlanParam) (int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return 0, err
	}

	data := quotaPlanDataToParam(param)
	data.UpdatedAt = lib.PTimeNow()

	return dao.TQuotaPlanUpdate(dbCtx, data, quotaPlanFilterToParam(filter))
}

func (s *QuotaPlanStorager) DeleteQuotaPlan(ctx context.Context, filter *quota.QuotaPlanFilter) error {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return err
	}

	_, err = dao.TQuotaPlanDelete(dbCtx, quotaPlanFilterToParam(filter))
	return err
}

func quotaPlanFilterToParam(filter *quota.QuotaPlanFilter) *dao.TQuotaPlanParam {
	if filter == nil {
		return nil
	}

	return &dao.TQuotaPlanParam{
		ID:                filter.ID,
		LastResetAtBefore: filter.LastResetAtBefore,
	}
}

func quotaPlanDataToParam(param *quota.QuotaPlanParam) *dao.TQuotaPlanParam {
	return &dao.TQuotaPlanParam{
		Unlimited:             param.Unlimited,
		PassWhenNoEnoughQuota: param.PassWhenNoEnoughQuota,
		Quota:                 float64PtrToDecimalPtr(param.Quota),
		Unit:                  param.Unit,
		ResetPeriod:           param.ResetPeriod,
		LastResetAt:           param.LastResetAt,
	}
}

func quotaPlanParamToData(one *dao.TQuotaPlan) *quota.QuotaPlanParam {
	createTime := one.CreatedAt.Unix()
	return &quota.QuotaPlanParam{
		ID:                    &one.ID,
		Unlimited:             &one.Unlimited,
		PassWhenNoEnoughQuota: &one.PassWhenNoEnoughQuota,
		Quota:                 decimalToFloat64Ptr(one.Quota),
		Unit:                  &one.Unit,
		ResetPeriod:           &one.ResetPeriod,
		LastResetAt:           one.LastResetAt,
		CreateTime:            &createTime,
	}
}

func float64PtrToDecimalPtr(v *float64) *decimal.Decimal {
	if v == nil {
		return nil
	}
	d := decimal.NewFromFloat(*v)
	return &d
}

func decimalToFloat64Ptr(d decimal.Decimal) *float64 {
	return lib.PFloat64(d.InexactFloat64())
}
