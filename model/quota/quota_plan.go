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
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
)

// QuotaPlanParam 定义配额计划参数
type QuotaPlanParam struct {
	ID                    *int64     `json:"id"`
	Unlimited             *bool      `json:"unlimited"`
	PassWhenNoEnoughQuota *bool      `json:"pass_when_no_enough_quota"`
	Quota                 *float64   `json:"quota"`
	Unit                  *string    `json:"unit"`
	ResetPeriod           *string    `json:"reset_period"`
	LastResetAt           *time.Time `json:"last_reset_at"`
	CreateTime            *int64     `json:"create_time"`
}

// QuotaPlanFilter 定义配额计划过滤条件
type QuotaPlanFilter struct {
	ID                *int64
	Unlimited         *bool
	ResetPeriod       []string
	LastResetAtBefore *time.Time
}

// QuotaPlanStorager 定义配额计划存储接口
type QuotaPlanStorager interface {
	CreateQuotaPlan(ctx context.Context, param *QuotaPlanParam) (int64, error)
	FetchQuotaPlan(ctx context.Context, filter *QuotaPlanFilter) (*QuotaPlanParam, error)
	FetchQuotaPlanList(ctx context.Context, filter *QuotaPlanFilter) ([]*QuotaPlanParam, error)
	UpdateQuotaPlan(ctx context.Context, filter *QuotaPlanFilter, param *QuotaPlanParam) (int64, error)
	DeleteQuotaPlan(ctx context.Context, filter *QuotaPlanFilter) error
}

var _ shared.QuotaPlanStorager = (*quotaPlanStoragerAdapter)(nil)

type quotaPlanStoragerAdapter struct {
	storager QuotaPlanStorager
}

func (a *quotaPlanStoragerAdapter) CreateQuotaPlan(ctx context.Context, param *shared.QuotaPlanParam) (int64, error) {
	return a.storager.CreateQuotaPlan(ctx, &QuotaPlanParam{
		Unlimited:             param.Unlimited,
		PassWhenNoEnoughQuota: param.PassWhenNoEnoughQuota,
		Quota:                 param.Quota,
		Unit:                  param.Unit,
		ResetPeriod:           param.ResetPeriod,
		LastResetAt:           param.LastResetAt,
	})
}

func (a *quotaPlanStoragerAdapter) UpdateQuotaPlan(ctx context.Context, id int64, param *shared.QuotaPlanParam) (int64, error) {
	return a.storager.UpdateQuotaPlan(ctx, &QuotaPlanFilter{ID: &id}, &QuotaPlanParam{
		Unlimited:             param.Unlimited,
		PassWhenNoEnoughQuota: param.PassWhenNoEnoughQuota,
		Quota:                 param.Quota,
		Unit:                  param.Unit,
		ResetPeriod:           param.ResetPeriod,
		LastResetAt:           param.LastResetAt,
	})
}

func (a *quotaPlanStoragerAdapter) DeleteQuotaPlan(ctx context.Context, id int64) error {
	return a.storager.DeleteQuotaPlan(ctx, &QuotaPlanFilter{ID: &id})
}

func (a *quotaPlanStoragerAdapter) FetchQuotaPlan(ctx context.Context, id int64) (*shared.QuotaPlanParam, error) {
	result, err := a.storager.FetchQuotaPlan(ctx, &QuotaPlanFilter{ID: &id})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, nil
	}
	return &shared.QuotaPlanParam{
		Unlimited:             result.Unlimited,
		PassWhenNoEnoughQuota: result.PassWhenNoEnoughQuota,
		Quota:                 result.Quota,
		Unit:                  result.Unit,
		ResetPeriod:           result.ResetPeriod,
		LastResetAt:           result.LastResetAt,
	}, nil
}

func NewQuotaPlanStoragerAdapter(storager QuotaPlanStorager) shared.QuotaPlanStorager {
	return &quotaPlanStoragerAdapter{storager: storager}
}
