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

package entity

import (
	"context"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
)

type fakeTxn struct{}

func (f *fakeTxn) AtomExecute(ctx context.Context, do func(context.Context) error) error {
	return do(ctx)
}

var _ itxn.TxnStorager = (*fakeTxn)(nil)

type fakeEntityTypeStorager struct {
	createFn func(ctx context.Context, param *EntityTypeParam) (int64, error)
	fetchFn  func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error)
	listFn   func(ctx context.Context, filter *EntityTypeFilter) ([]*EntityTypeParam, error)
	updateFn func(ctx context.Context, filter *EntityTypeFilter, param *EntityTypeParam) (int64, error)
	deleteFn func(ctx context.Context, filter *EntityTypeFilter) error

	created []*EntityTypeParam
	fetched []*EntityTypeFilter
	listed  []*EntityTypeFilter
	updated []updateEntityTypeCall
	deleted []*EntityTypeFilter
}

type updateEntityTypeCall struct {
	filter *EntityTypeFilter
	param  *EntityTypeParam
}

func (s *fakeEntityTypeStorager) CreateEntityType(ctx context.Context, param *EntityTypeParam) (int64, error) {
	s.created = append(s.created, param)
	if s.createFn != nil {
		return s.createFn(ctx, param)
	}
	return 0, nil
}

func (s *fakeEntityTypeStorager) FetchEntityType(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
	s.fetched = append(s.fetched, filter)
	if s.fetchFn != nil {
		return s.fetchFn(ctx, filter)
	}
	return nil, nil
}

func (s *fakeEntityTypeStorager) FetchEntityTypeList(ctx context.Context, filter *EntityTypeFilter) ([]*EntityTypeParam, error) {
	s.listed = append(s.listed, filter)
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return nil, nil
}

func (s *fakeEntityTypeStorager) UpdateEntityType(ctx context.Context, filter *EntityTypeFilter, param *EntityTypeParam) (int64, error) {
	s.updated = append(s.updated, updateEntityTypeCall{filter: filter, param: param})
	if s.updateFn != nil {
		return s.updateFn(ctx, filter, param)
	}
	return 0, nil
}

func (s *fakeEntityTypeStorager) DeleteEntityType(ctx context.Context, filter *EntityTypeFilter) error {
	s.deleted = append(s.deleted, filter)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, filter)
	}
	return nil
}

var _ EntityTypeStorager = (*fakeEntityTypeStorager)(nil)

type fakeEntityStorager struct {
	createFn func(ctx context.Context, param *EntityParam) (int64, error)
	fetchFn  func(ctx context.Context, filter *EntityFilter) (*EntityParam, error)
	listFn   func(ctx context.Context, filter *EntityFilter) ([]*EntityParam, error)
	updateFn func(ctx context.Context, filter *EntityFilter, param *EntityParam) (int64, error)
	deleteFn func(ctx context.Context, filter *EntityFilter) error

	created []*EntityParam
	fetched []*EntityFilter
	listed  []*EntityFilter
	updated []updateEntityCall
	deleted []*EntityFilter
}

type updateEntityCall struct {
	filter *EntityFilter
	param  *EntityParam
}

func (s *fakeEntityStorager) CreateEntity(ctx context.Context, param *EntityParam) (int64, error) {
	s.created = append(s.created, param)
	if s.createFn != nil {
		return s.createFn(ctx, param)
	}
	return 0, nil
}

func (s *fakeEntityStorager) FetchEntity(ctx context.Context, filter *EntityFilter) (*EntityParam, error) {
	s.fetched = append(s.fetched, filter)
	if s.fetchFn != nil {
		return s.fetchFn(ctx, filter)
	}
	return nil, nil
}

func (s *fakeEntityStorager) FetchEntityList(ctx context.Context, filter *EntityFilter) ([]*EntityParam, error) {
	s.listed = append(s.listed, filter)
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return nil, nil
}

func (s *fakeEntityStorager) UpdateEntity(ctx context.Context, filter *EntityFilter, param *EntityParam) (int64, error) {
	s.updated = append(s.updated, updateEntityCall{filter: filter, param: param})
	if s.updateFn != nil {
		return s.updateFn(ctx, filter, param)
	}
	return 0, nil
}

func (s *fakeEntityStorager) DeleteEntity(ctx context.Context, filter *EntityFilter) error {
	s.deleted = append(s.deleted, filter)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, filter)
	}
	return nil
}

var _ EntityStorager = (*fakeEntityStorager)(nil)

type fakeSharedQuotaPlanStorager struct {
	createFn func(ctx context.Context, param *shared.QuotaPlanParam) (int64, error)
	updateFn func(ctx context.Context, id int64, param *shared.QuotaPlanParam) (int64, error)
	deleteFn func(ctx context.Context, id int64) error
	fetchFn  func(ctx context.Context, id int64) (*shared.QuotaPlanParam, error)

	created []*shared.QuotaPlanParam
	updated []updateSharedQuotaPlanCall
	deleted []int64
	fetched []int64
}

type updateSharedQuotaPlanCall struct {
	id    int64
	param *shared.QuotaPlanParam
}

func (s *fakeSharedQuotaPlanStorager) CreateQuotaPlan(ctx context.Context, param *shared.QuotaPlanParam) (int64, error) {
	s.created = append(s.created, param)
	if s.createFn != nil {
		return s.createFn(ctx, param)
	}
	return 0, nil
}

func (s *fakeSharedQuotaPlanStorager) UpdateQuotaPlan(ctx context.Context, id int64, param *shared.QuotaPlanParam) (int64, error) {
	s.updated = append(s.updated, updateSharedQuotaPlanCall{id: id, param: param})
	if s.updateFn != nil {
		return s.updateFn(ctx, id, param)
	}
	return 0, nil
}

func (s *fakeSharedQuotaPlanStorager) DeleteQuotaPlan(ctx context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *fakeSharedQuotaPlanStorager) FetchQuotaPlan(ctx context.Context, id int64) (*shared.QuotaPlanParam, error) {
	s.fetched = append(s.fetched, id)
	if s.fetchFn != nil {
		return s.fetchFn(ctx, id)
	}
	return nil, nil
}

var _ shared.QuotaPlanStorager = (*fakeSharedQuotaPlanStorager)(nil)

type fakeSharedRateLimitPolicyStorager struct {
	createFn func(ctx context.Context, param *shared.RateLimitPolicyParam) (int64, error)
	updateFn func(ctx context.Context, id int64, param *shared.RateLimitPolicyParam) (int64, error)
	deleteFn func(ctx context.Context, id int64) error
	fetchFn  func(ctx context.Context, id int64) (*shared.RateLimitPolicyParam, error)

	created []*shared.RateLimitPolicyParam
	updated []updateSharedRateLimitPolicyCall
	deleted []int64
	fetched []int64
}

type updateSharedRateLimitPolicyCall struct {
	id    int64
	param *shared.RateLimitPolicyParam
}

func (s *fakeSharedRateLimitPolicyStorager) CreateRateLimitPolicy(ctx context.Context, param *shared.RateLimitPolicyParam) (int64, error) {
	s.created = append(s.created, param)
	if s.createFn != nil {
		return s.createFn(ctx, param)
	}
	return 0, nil
}

func (s *fakeSharedRateLimitPolicyStorager) UpdateRateLimitPolicy(ctx context.Context, id int64, param *shared.RateLimitPolicyParam) (int64, error) {
	s.updated = append(s.updated, updateSharedRateLimitPolicyCall{id: id, param: param})
	if s.updateFn != nil {
		return s.updateFn(ctx, id, param)
	}
	return 0, nil
}

func (s *fakeSharedRateLimitPolicyStorager) DeleteRateLimitPolicy(ctx context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *fakeSharedRateLimitPolicyStorager) FetchRateLimitPolicy(ctx context.Context, id int64) (*shared.RateLimitPolicyParam, error) {
	s.fetched = append(s.fetched, id)
	if s.fetchFn != nil {
		return s.fetchFn(ctx, id)
	}
	return nil, nil
}

var _ shared.RateLimitPolicyStorager = (*fakeSharedRateLimitPolicyStorager)(nil)

type fakeRouteRulesStorager struct {
	createFn    func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error)
	fetchFn     func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error)
	listFn      func(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error)
	updateFn    func(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error)
	deleteFn    func(ctx context.Context, id int64) error
	fetchByIDFn func(ctx context.Context, id int64) (*shared.RouteRulesParam, error)
	allFn       func(ctx context.Context) ([]*shared.RouteRulesParam, error)

	created     []createRouteRulesCall
	fetched     []fetchRouteRulesCall
	listed      []*shared.RouteRulesFilter
	updated     []updateRouteRulesCall
	deleted     []int64
	fetchedByID []int64
	fetchedAll  int
}

type createRouteRulesCall struct {
	ruleType string
	owner    *string
	param    *shared.RouteRulesParam
}

type fetchRouteRulesCall struct {
	ruleType string
	owner    *string
}

type updateRouteRulesCall struct {
	id    int64
	param *shared.RouteRulesParam
}

func (s *fakeRouteRulesStorager) CreateRouteRules(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
	s.created = append(s.created, createRouteRulesCall{ruleType: ruleType, owner: owner, param: param})
	if s.createFn != nil {
		return s.createFn(ctx, ruleType, owner, param)
	}
	return 0, nil
}

func (s *fakeRouteRulesStorager) FetchRouteRules(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
	s.fetched = append(s.fetched, fetchRouteRulesCall{ruleType: ruleType, owner: owner})
	if s.fetchFn != nil {
		return s.fetchFn(ctx, ruleType, owner)
	}
	return nil, nil
}

func (s *fakeRouteRulesStorager) FetchRouteRulesList(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error) {
	s.listed = append(s.listed, filter)
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return nil, 0, nil
}

func (s *fakeRouteRulesStorager) UpdateRouteRules(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error) {
	s.updated = append(s.updated, updateRouteRulesCall{id: id, param: param})
	if s.updateFn != nil {
		return s.updateFn(ctx, id, param)
	}
	return 0, nil
}

func (s *fakeRouteRulesStorager) DeleteRouteRules(ctx context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *fakeRouteRulesStorager) FetchRouteRulesByID(ctx context.Context, id int64) (*shared.RouteRulesParam, error) {
	s.fetchedByID = append(s.fetchedByID, id)
	if s.fetchByIDFn != nil {
		return s.fetchByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *fakeRouteRulesStorager) FetchAllRouteRules(ctx context.Context) ([]*shared.RouteRulesParam, error) {
	s.fetchedAll++
	if s.allFn != nil {
		return s.allFn(ctx)
	}
	return nil, nil
}

var _ shared.RouteRulesStorager = (*fakeRouteRulesStorager)(nil)

// fakeQuotaCache 实现 quotacache.QuotaCache，用于单元测试记录调用。
type fakeQuotaCache struct {
	setRemainingCalls   []quotaCacheSetRemainingCall
	resetToQuotaCalls   []quotaCacheResetToQuotaCall
	deleteKeysCalls     [][]string
	getRemainingFn      func(ctx context.Context, key string, unit *string) (float64, error)
	batchGetRemainingFn func(ctx context.Context, keys []string, unit *string) (map[string]float64, error)
	setRemainingFn      func(ctx context.Context, key string, quota *float64, unit *string) error
	resetToQuotaFn      func(ctx context.Context, key string, quota *float64, unit *string) error
	deleteKeysFn        func(ctx context.Context, keys []string) error
}

type quotaCacheSetRemainingCall struct {
	key   string
	quota *float64
	unit  *string
}

type quotaCacheResetToQuotaCall struct {
	key   string
	quota *float64
	unit  *string
}

func (c *fakeQuotaCache) GetRemaining(ctx context.Context, key string, unit *string) (float64, error) {
	if c.getRemainingFn != nil {
		return c.getRemainingFn(ctx, key, unit)
	}
	return 0, nil
}

func (c *fakeQuotaCache) BatchGetRemaining(ctx context.Context, keys []string, unit *string) (map[string]float64, error) {
	if c.batchGetRemainingFn != nil {
		return c.batchGetRemainingFn(ctx, keys, unit)
	}
	result := make(map[string]float64, len(keys))
	for _, k := range keys {
		result[k] = 0
	}
	return result, nil
}

func (c *fakeQuotaCache) SetRemaining(ctx context.Context, key string, quota *float64, unit *string) error {
	c.setRemainingCalls = append(c.setRemainingCalls, quotaCacheSetRemainingCall{key: key, quota: quota, unit: unit})
	if c.setRemainingFn != nil {
		return c.setRemainingFn(ctx, key, quota, unit)
	}
	return nil
}

func (c *fakeQuotaCache) ResetToQuota(ctx context.Context, key string, quota *float64, unit *string) error {
	c.resetToQuotaCalls = append(c.resetToQuotaCalls, quotaCacheResetToQuotaCall{key: key, quota: quota, unit: unit})
	if c.resetToQuotaFn != nil {
		return c.resetToQuotaFn(ctx, key, quota, unit)
	}
	return nil
}

func (c *fakeQuotaCache) ResetToQuotaAtomic(ctx context.Context, key string, quota *float64, unit *string) error {
	return c.ResetToQuota(ctx, key, quota, unit)
}

func (c *fakeQuotaCache) DeleteKeys(ctx context.Context, keys []string) error {
	c.deleteKeysCalls = append(c.deleteKeysCalls, keys)
	if c.deleteKeysFn != nil {
		return c.deleteKeysFn(ctx, keys)
	}
	return nil
}

var _ quotacache.QuotaCache = (*fakeQuotaCache)(nil)
