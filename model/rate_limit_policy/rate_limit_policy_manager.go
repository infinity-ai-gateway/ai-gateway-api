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

package rate_limit_policy

import (
	"context"
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/api_key"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
)

const ConfigTopicProductRateLimitPolicy = "mod_ai_rate_limit"

// RateLimitPolicyManager 定义限流策略管理器
type RateLimitPolicyManager struct {
	txn                   itxn.TxnStorager
	storager              RateLimitPolicyStorager
	apiKeyStorager        api_key.APIKeyStorager
	entityStorager        entity.EntityStorager
	versionControlManager *iversion_control.VersionControlManager
	operationLogManager   ioperlog.OperationLogRecorder
}

// NewRateLimitPolicyManager 创建限流策略管理器
func NewRateLimitPolicyManager(txn itxn.TxnStorager, storager RateLimitPolicyStorager, apiKeyStorager api_key.APIKeyStorager, entityStorager entity.EntityStorager, versionControlManager *iversion_control.VersionControlManager) *RateLimitPolicyManager {
	return &RateLimitPolicyManager{
		txn:                   txn,
		storager:              storager,
		apiKeyStorager:        apiKeyStorager,
		entityStorager:        entityStorager,
		versionControlManager: versionControlManager,
	}
}

// SetOperationLogManager injects the operation log recorder.
func (m *RateLimitPolicyManager) SetOperationLogManager(manager ioperlog.OperationLogRecorder) {
	m.operationLogManager = manager
}

// CreateRateLimitPolicy 创建限流策略
func (m *RateLimitPolicyManager) CreateRateLimitPolicy(ctx context.Context, param *RateLimitPolicyParam) (int64, error) {
	id, err := m.storager.CreateRateLimitPolicy(ctx, param)
	if err != nil {
		m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionCreate), "", "", nil, rateLimitPolicyParamToMap(param), err)
		return 0, err
	}

	m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionCreate), rateLimitPolicyIDString(id), "", nil, rateLimitPolicyParamToMap(param), nil)
	return id, nil
}

// FetchRateLimitPolicy 查询单个限流策略
func (m *RateLimitPolicyManager) FetchRateLimitPolicy(ctx context.Context, filter *RateLimitPolicyFilter) (*RateLimitPolicyParam, error) {
	return m.storager.FetchRateLimitPolicy(ctx, filter)
}

// FetchRateLimitPolicyList 查询限流策略列表
func (m *RateLimitPolicyManager) FetchRateLimitPolicyList(ctx context.Context, filter *RateLimitPolicyFilter) ([]*RateLimitPolicyParam, error) {
	return m.storager.FetchRateLimitPolicyList(ctx, filter)
}

// UpdateRateLimitPolicy 更新限流策略
func (m *RateLimitPolicyManager) UpdateRateLimitPolicy(ctx context.Context, filter *RateLimitPolicyFilter, param *RateLimitPolicyParam) (int64, error) {
	resourceID := ""
	if filter != nil && filter.ID != nil {
		resourceID = rateLimitPolicyIDString(*filter.ID)
	}

	oldPolicy, err := m.storager.FetchRateLimitPolicy(ctx, filter)
	if err != nil {
		m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionUpdate), resourceID, "", nil, rateLimitPolicyParamToMap(param), err)
		return 0, err
	}

	affected, err := m.storager.UpdateRateLimitPolicy(ctx, filter, param)
	if err != nil {
		m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionUpdate), resourceID, "", rateLimitPolicyParamToMap(oldPolicy), rateLimitPolicyParamToMap(param), err)
		return affected, err
	}

	m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionUpdate), resourceID, "", rateLimitPolicyParamToMap(oldPolicy), rateLimitPolicyParamToMap(param), nil)
	return affected, nil
}

// DeleteRateLimitPolicy 删除限流策略
func (m *RateLimitPolicyManager) DeleteRateLimitPolicy(ctx context.Context, filter *RateLimitPolicyFilter) error {
	resourceID := ""
	if filter != nil && filter.ID != nil {
		resourceID = rateLimitPolicyIDString(*filter.ID)
	}

	oldPolicy, err := m.storager.FetchRateLimitPolicy(ctx, filter)
	if err != nil {
		m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionDelete), resourceID, "", nil, nil, err)
		return err
	}

	if err := m.storager.DeleteRateLimitPolicy(ctx, filter); err != nil {
		m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionDelete), resourceID, "", rateLimitPolicyParamToMap(oldPolicy), nil, err)
		return err
	}

	m.recordRateLimitPolicyOperation(ctx, string(ioperlog.ActionDelete), resourceID, "", rateLimitPolicyParamToMap(oldPolicy), nil, nil)
	return nil
}

// ConfigExport 导出限流策略配置供 BFE 使用
func (m *RateLimitPolicyManager) ConfigExport(ctx context.Context, lastVersion string) (*ExportRateLimitPolicyConfig, error) {
	rst, err := m.versionControlManager.ExportConfig(ctx, ConfigTopicProductRateLimitPolicy, m.RateLimitPolicyGenerator)
	if err != nil {
		return nil, err
	}

	if rst.DataWithoutVersion == nil {
		return nil, fmt.Errorf("RateLimitPolicyGenerator.DataWithoutVersion is nil")
	}

	conf, ok := rst.DataWithoutVersion.(*ExportRateLimitPolicyConfig)
	if ok {
		if conf.Version == lastVersion {
			return nil, nil
		}

		return conf, nil
	}

	return nil, fmt.Errorf("convert RateLimitPolicyGenerator.DataWithoutVersion to ExportRateLimitPolicyConfig is error")
}

// RateLimitPolicyGenerator 生成限流策略配置数据
func (m *RateLimitPolicyManager) RateLimitPolicyGenerator(ctx context.Context) (*iversion_control.ExportData, error) {
	apiKeys, err := m.apiKeyStorager.FetchAPIKeyList(ctx, &api_key.APIKeyFilter{})
	if err != nil {
		return nil, fmt.Errorf("fetch api keys error: %s", err.Error())
	}

	policies, err := m.storager.FetchRateLimitPolicyList(ctx, &RateLimitPolicyFilter{})
	if err != nil {
		return nil, fmt.Errorf("fetch rate limit policies error: %s", err.Error())
	}

	policyMap := make(map[int64]*RateLimitPolicyParam)
	for _, p := range policies {
		if p.ID != nil {
			policyMap[*p.ID] = p
		}
	}

	config := make(map[string][]*ExportRouteRule)
	rateLimitPolicies := make(map[string]*ExportRateLimitPolicy)
	bindings := make(map[string][]string)

	productName := "AI_product"
	config[productName] = []*ExportRouteRule{
		{
			Cond: "default_t()",
			HitAction: &ExportHitAction{
				Cmd:    "FINISH",
				Params: []string{},
			},
		},
	}

	for _, apiKey := range apiKeys {
		if apiKey.Key == nil {
			continue
		}

		policyIDs, err := m.fetchRateLimitPolicyIDsWithEntityHierarchy(ctx, apiKey)
		if err != nil {
			return nil, fmt.Errorf("fetch rate limit policy ids error: %s", err.Error())
		}

		for _, policyID := range policyIDs {
			policy, exists := policyMap[policyID]
			if !exists {
				continue
			}

			policyKey := fmt.Sprintf("rlp-%d", policyID)
			// Skip disabled policies: only effective policies should be exported
			// and bound to API keys.
			if policy.Enabled == nil || !*policy.Enabled {
				continue
			}
			if _, exists := rateLimitPolicies[policyKey]; !exists {
				exportPolicy := &ExportRateLimitPolicy{
					Name:    policyKey,
					Enabled: true,
					Rules: &ExportRateLimitRules{
						MaxConcurrency: 0,
						TPM:            []ExportTPMConfig{},
						RPM:            []ExportRPMConfig{},
					},
				}

				if policy.MaxConcurrency != nil {
					exportPolicy.Rules.MaxConcurrency = *policy.MaxConcurrency
				}

				for _, tpm := range policy.TpmConfigs {
					models := []string{"*"}
					if tpm.Model != "" && tpm.Model != "*" {
						models = []string{tpm.Model}
					}
					exportPolicy.Rules.TPM = append(exportPolicy.Rules.TPM, ExportTPMConfig{
						Name:          tpm.Name,
						Models:        models,
						WindowMinutes: tpm.WindowMinutes,
						MaxTokens:     tpm.MaxTokens,
						StepMinutes:   tpm.StepMinutes,
						RedisKey:      shared.BuildBFERateLimitRedisKey(policyID, "RL_TPM", tpm.Name),
					})
				}

				for _, rpm := range policy.RpmConfigs {
					models := []string{"*"}
					if rpm.Model != "" && rpm.Model != "*" {
						models = []string{rpm.Model}
					}
					exportPolicy.Rules.RPM = append(exportPolicy.Rules.RPM, ExportRPMConfig{
						Name:          rpm.Name,
						Models:        models,
						WindowMinutes: rpm.WindowMinutes,
						MaxRequests:   rpm.MaxRequests,
						Burst:         1,
						RedisKey:      shared.BuildBFERateLimitRedisKey(policyID, "RL_RPM", rpm.Name),
					})
				}

				rateLimitPolicies[policyKey] = exportPolicy
			}

			if _, exists := bindings[*apiKey.Key]; !exists {
				bindings[*apiKey.Key] = []string{}
			}
			bindings[*apiKey.Key] = append(bindings[*apiKey.Key], policyKey)
		}
	}

	conf := &ExportRateLimitPolicyConfig{
		Config:                        config,
		RateLimitPolicies:             rateLimitPolicies,
		ApikeyRateLimitPolicyBindings: bindings,
	}

	conf.UpdateVersion(iversion_control.ZeroVersion)

	return &iversion_control.ExportData{
		Topic:              ConfigTopicProductRateLimitPolicy,
		DataWithoutVersion: conf,
	}, nil
}

// fetchRateLimitPolicyIDsWithEntityHierarchy 获取 api-key 及其关联 entity 层级的所有限流策略ID
func (m *RateLimitPolicyManager) fetchRateLimitPolicyIDsWithEntityHierarchy(ctx context.Context, apiKey *api_key.APIKeyParam) ([]int64, error) {
	policyIDs := make([]int64, 0)

	if apiKey.RateLimitPolicyID != nil {
		policyIDs = append(policyIDs, *apiKey.RateLimitPolicyID)
	}

	if apiKey.EntityID != nil && *apiKey.EntityID != "" && m.entityStorager != nil {
		entity, err := m.entityStorager.FetchEntity(ctx, &entity.EntityFilter{EntityID: apiKey.EntityID})
		if err != nil {
			return nil, err
		}
		if entity != nil {
			entityPolicyIDs, err := m.fetchEntityRateLimitPolicyIDs(ctx, entity)
			if err != nil {
				return nil, err
			}
			policyIDs = append(policyIDs, entityPolicyIDs...)
		}
	}

	return policyIDs, nil
}

// fetchEntityRateLimitPolicyIDs 递归获取 entity 及其父 entity 的限流策略ID
func (m *RateLimitPolicyManager) fetchEntityRateLimitPolicyIDs(ctx context.Context, ent *entity.EntityParam) ([]int64, error) {
	policyIDs := make([]int64, 0)

	if ent.RateLimitPolicyID != nil {
		policyIDs = append(policyIDs, *ent.RateLimitPolicyID)
	}

	if ent.ParentID != nil && *ent.ParentID != "" && m.entityStorager != nil {
		parentEntity, err := m.entityStorager.FetchEntity(ctx, &entity.EntityFilter{EntityID: ent.ParentID})
		if err != nil {
			return nil, err
		}
		if parentEntity != nil {
			parentPolicyIDs, err := m.fetchEntityRateLimitPolicyIDs(ctx, parentEntity)
			if err != nil {
				return nil, err
			}
			policyIDs = append(policyIDs, parentPolicyIDs...)
		}
	}

	return policyIDs, nil
}
