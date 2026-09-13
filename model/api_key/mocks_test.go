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

package api_key

import (
	"context"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
)

type fakeTxn struct{}

func (f *fakeTxn) AtomExecute(ctx context.Context, do func(context.Context) error) error {
	return do(ctx)
}

var _ itxn.TxnStorager = (*fakeTxn)(nil)

type fakeAPIKeyStorager struct {
	fetchAPIKeyListFn      func(ctx context.Context, filter *APIKeyFilter) ([]*APIKeyParam, error)
	createAPIKeyFn         func(ctx context.Context, param *APIKeyParam) (int64, error)
	updateAPIKeyFn         func(ctx context.Context, filter *APIKeyFilter, param *APIKeyParam) (int64, error)
	deleteAPIKeyFn         func(ctx context.Context, filter *APIKeyFilter) error
	createAPIKeyTokenFn    func(ctx context.Context, param *APIKeyTokenParam) (int64, error)
	updateAPIKeyTokenFn    func(ctx context.Context, filter *APIKeyTokenFilter, param *APIKeyTokenParam) error
	fetchAPIKeyTokenListFn func(ctx context.Context, filter *APIKeyTokenFilter) ([]*APIKeyTokenParam, error)
}

func (f *fakeAPIKeyStorager) FetchAPIKeyList(ctx context.Context, filter *APIKeyFilter) ([]*APIKeyParam, error) {
	if f.fetchAPIKeyListFn != nil {
		return f.fetchAPIKeyListFn(ctx, filter)
	}
	return nil, nil
}

func (f *fakeAPIKeyStorager) CreateAPIKey(ctx context.Context, param *APIKeyParam) (int64, error) {
	if f.createAPIKeyFn != nil {
		return f.createAPIKeyFn(ctx, param)
	}
	return 0, nil
}

func (f *fakeAPIKeyStorager) UpdateAPIKey(ctx context.Context, filter *APIKeyFilter, param *APIKeyParam) (int64, error) {
	if f.updateAPIKeyFn != nil {
		return f.updateAPIKeyFn(ctx, filter, param)
	}
	return 0, nil
}

func (f *fakeAPIKeyStorager) DeleteAPIKey(ctx context.Context, filter *APIKeyFilter) error {
	if f.deleteAPIKeyFn != nil {
		return f.deleteAPIKeyFn(ctx, filter)
	}
	return nil
}

func (f *fakeAPIKeyStorager) CreateAPIKeyToken(ctx context.Context, param *APIKeyTokenParam) (int64, error) {
	if f.createAPIKeyTokenFn != nil {
		return f.createAPIKeyTokenFn(ctx, param)
	}
	return 0, nil
}

func (f *fakeAPIKeyStorager) UpdateAPIKeyToken(ctx context.Context, filter *APIKeyTokenFilter, param *APIKeyTokenParam) error {
	if f.updateAPIKeyTokenFn != nil {
		return f.updateAPIKeyTokenFn(ctx, filter, param)
	}
	return nil
}

func (f *fakeAPIKeyStorager) FetchAPIKeyTokenList(ctx context.Context, filter *APIKeyTokenFilter) ([]*APIKeyTokenParam, error) {
	if f.fetchAPIKeyTokenListFn != nil {
		return f.fetchAPIKeyTokenListFn(ctx, filter)
	}
	return nil, nil
}

var _ APIKeyStorager = (*fakeAPIKeyStorager)(nil)

type fakeQuotaPlanStorager struct {
	createQuotaPlanFn func(ctx context.Context, param *shared.QuotaPlanParam) (int64, error)
	updateQuotaPlanFn func(ctx context.Context, id int64, param *shared.QuotaPlanParam) (int64, error)
	deleteQuotaPlanFn func(ctx context.Context, id int64) error
	fetchQuotaPlanFn  func(ctx context.Context, id int64) (*shared.QuotaPlanParam, error)
}

func (f *fakeQuotaPlanStorager) CreateQuotaPlan(ctx context.Context, param *shared.QuotaPlanParam) (int64, error) {
	if f.createQuotaPlanFn != nil {
		return f.createQuotaPlanFn(ctx, param)
	}
	return 0, nil
}

func (f *fakeQuotaPlanStorager) UpdateQuotaPlan(ctx context.Context, id int64, param *shared.QuotaPlanParam) (int64, error) {
	if f.updateQuotaPlanFn != nil {
		return f.updateQuotaPlanFn(ctx, id, param)
	}
	return 0, nil
}

func (f *fakeQuotaPlanStorager) DeleteQuotaPlan(ctx context.Context, id int64) error {
	if f.deleteQuotaPlanFn != nil {
		return f.deleteQuotaPlanFn(ctx, id)
	}
	return nil
}

func (f *fakeQuotaPlanStorager) FetchQuotaPlan(ctx context.Context, id int64) (*shared.QuotaPlanParam, error) {
	if f.fetchQuotaPlanFn != nil {
		return f.fetchQuotaPlanFn(ctx, id)
	}
	return nil, nil
}

var _ QuotaPlanStorager = (*fakeQuotaPlanStorager)(nil)

type fakeRateLimitPolicyStorager struct {
	createRateLimitPolicyFn func(ctx context.Context, param *shared.RateLimitPolicyParam) (int64, error)
	updateRateLimitPolicyFn func(ctx context.Context, id int64, param *shared.RateLimitPolicyParam) (int64, error)
	deleteRateLimitPolicyFn func(ctx context.Context, id int64) error
	fetchRateLimitPolicyFn  func(ctx context.Context, id int64) (*shared.RateLimitPolicyParam, error)
}

func (f *fakeRateLimitPolicyStorager) CreateRateLimitPolicy(ctx context.Context, param *shared.RateLimitPolicyParam) (int64, error) {
	if f.createRateLimitPolicyFn != nil {
		return f.createRateLimitPolicyFn(ctx, param)
	}
	return 0, nil
}

func (f *fakeRateLimitPolicyStorager) UpdateRateLimitPolicy(ctx context.Context, id int64, param *shared.RateLimitPolicyParam) (int64, error) {
	if f.updateRateLimitPolicyFn != nil {
		return f.updateRateLimitPolicyFn(ctx, id, param)
	}
	return 0, nil
}

func (f *fakeRateLimitPolicyStorager) DeleteRateLimitPolicy(ctx context.Context, id int64) error {
	if f.deleteRateLimitPolicyFn != nil {
		return f.deleteRateLimitPolicyFn(ctx, id)
	}
	return nil
}

func (f *fakeRateLimitPolicyStorager) FetchRateLimitPolicy(ctx context.Context, id int64) (*shared.RateLimitPolicyParam, error) {
	if f.fetchRateLimitPolicyFn != nil {
		return f.fetchRateLimitPolicyFn(ctx, id)
	}
	return nil, nil
}

var _ RateLimitPolicyStorager = (*fakeRateLimitPolicyStorager)(nil)

type fakeRouteRulesStorager struct {
	createRouteRulesFn    func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error)
	fetchRouteRulesFn     func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error)
	fetchRouteRulesListFn func(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error)
	updateRouteRulesFn    func(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error)
	deleteRouteRulesFn    func(ctx context.Context, id int64) error
	fetchRouteRulesByIDFn func(ctx context.Context, id int64) (*shared.RouteRulesParam, error)
}

func (f *fakeRouteRulesStorager) CreateRouteRules(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
	if f.createRouteRulesFn != nil {
		return f.createRouteRulesFn(ctx, ruleType, owner, param)
	}
	return 0, nil
}

func (f *fakeRouteRulesStorager) FetchRouteRules(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
	if f.fetchRouteRulesFn != nil {
		return f.fetchRouteRulesFn(ctx, ruleType, owner)
	}
	return nil, nil
}

func (f *fakeRouteRulesStorager) FetchRouteRulesList(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error) {
	if f.fetchRouteRulesListFn != nil {
		return f.fetchRouteRulesListFn(ctx, filter)
	}
	return nil, 0, nil
}

func (f *fakeRouteRulesStorager) UpdateRouteRules(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error) {
	if f.updateRouteRulesFn != nil {
		return f.updateRouteRulesFn(ctx, id, param)
	}
	return 0, nil
}

func (f *fakeRouteRulesStorager) DeleteRouteRules(ctx context.Context, id int64) error {
	if f.deleteRouteRulesFn != nil {
		return f.deleteRouteRulesFn(ctx, id)
	}
	return nil
}

func (f *fakeRouteRulesStorager) FetchRouteRulesByID(ctx context.Context, id int64) (*shared.RouteRulesParam, error) {
	if f.fetchRouteRulesByIDFn != nil {
		return f.fetchRouteRulesByIDFn(ctx, id)
	}
	return nil, nil
}

func (f *fakeRouteRulesStorager) FetchAllRouteRules(ctx context.Context) ([]*shared.RouteRulesParam, error) {
	return nil, nil
}

var _ shared.RouteRulesStorager = (*fakeRouteRulesStorager)(nil)

type fakeEntityStorager struct {
	fetchEntityFn func(ctx context.Context, filter *shared.EntityFilter) (*shared.EntitySummary, error)
}

func (f *fakeEntityStorager) FetchEntity(ctx context.Context, filter *shared.EntityFilter) (*shared.EntitySummary, error) {
	if f.fetchEntityFn != nil {
		return f.fetchEntityFn(ctx, filter)
	}
	return nil, nil
}

var _ shared.EntityStorager = (*fakeEntityStorager)(nil)

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

var fixedTestTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
