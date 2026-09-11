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

package entity

import (
	"context"
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/quotacache"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

// EntityManager 定义 Entity 管理器
type EntityManager struct {
	txn                     itxn.TxnStorager
	storager                EntityStorager
	entityTypeStorager      EntityTypeStorager
	quotaPlanStorager       shared.QuotaPlanStorager
	rateLimitPolicyStorager shared.RateLimitPolicyStorager
	routeRulesStorager      shared.RouteRulesStorager
	quotaCache              quotacache.QuotaCache
	operationLogManager     ioperlog.OperationLogRecorder
}

// NewEntityManager 创建 Entity 管理器
func NewEntityManager(txn itxn.TxnStorager, storager EntityStorager,
	entityTypeStorager EntityTypeStorager,
	quotaPlanStorager shared.QuotaPlanStorager,
	rateLimitPolicyStorager shared.RateLimitPolicyStorager,
	routeRulesStorager shared.RouteRulesStorager,
	quotaCache quotacache.QuotaCache) *EntityManager {
	return &EntityManager{
		txn:                     txn,
		storager:                storager,
		entityTypeStorager:      entityTypeStorager,
		quotaPlanStorager:       quotaPlanStorager,
		rateLimitPolicyStorager: rateLimitPolicyStorager,
		routeRulesStorager:      routeRulesStorager,
		quotaCache:              quotaCache,
	}
}

// SetOperationLogManager injects the operation log recorder.
func (m *EntityManager) SetOperationLogManager(manager ioperlog.OperationLogRecorder) {
	m.operationLogManager = manager
}

// CreateEntity 创建 Entity
func (m *EntityManager) CreateEntity(ctx context.Context, param *EntityParam) (int64, error) {
	if param.Type != nil && m.entityTypeStorager != nil {
		entityTypeInfo, err := m.entityTypeStorager.FetchEntityType(ctx, &EntityTypeFilter{TypeName: param.Type})
		if err != nil {
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), err)
			return 0, err
		}
		if entityTypeInfo == nil {
			err := xerror.WrapParamError(fmt.Errorf("entity type not found: %s", *param.Type))
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), err)
			return 0, err
		}
	}

	if param.Name != nil && *param.Name != "" {
		existing, err := m.storager.FetchEntity(ctx, &EntityFilter{Name: param.Name})
		if err != nil {
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), err)
			return 0, err
		}
		if existing != nil {
			err := xerror.WrapRecordExisted("Entity")
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), err)
			return 0, err
		}
	}

	if param.ParentID != nil && *param.ParentID != "" && param.Type != nil && m.entityTypeStorager != nil {
		if err := m.checkEntityLevel(ctx, *param.Type, *param.ParentID); err != nil {
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), err)
			return 0, err
		}
	}

	var id int64
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error

		if param.QuotaPlan != nil && m.quotaPlanStorager != nil {
			quotaPlanID, err := m.quotaPlanStorager.CreateQuotaPlan(ctx, param.QuotaPlan)
			if err != nil {
				return err
			}
			param.QuotaPlanID = &quotaPlanID
		}

		if param.RateLimitPolicy != nil && m.rateLimitPolicyStorager != nil {
			rateLimitPolicyID, err := m.rateLimitPolicyStorager.CreateRateLimitPolicy(ctx, param.RateLimitPolicy)
			if err != nil {
				return err
			}
			param.RateLimitPolicyID = &rateLimitPolicyID
		}

		if param.RouteRules != nil && m.routeRulesStorager != nil {
			routeRulesID, err := m.routeRulesStorager.CreateRouteRules(ctx, shared.RouteRulesTypeEntity, param.EntityID, param.RouteRules)
			if err != nil {
				return err
			}
			param.RouteRulesID = &routeRulesID
		}

		id, err = m.storager.CreateEntity(ctx, param)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		entityID, entityName, parentID := entityParamIdentifiers(param)
		m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), err)
		return 0, err
	}

	// Sync Redis remaining quota after DB transaction commits (best-effort).
	if param.EntityID != nil && param.QuotaPlan != nil && m.quotaCache != nil {
		if param.QuotaPlan.Unlimited != nil && *param.QuotaPlan.Unlimited {
			defaultQuota := float64(100000000)
			if cacheErr := m.quotaCache.SetRemaining(ctx, *param.EntityID, &defaultQuota, param.QuotaPlan.Unit); cacheErr != nil {
				stateful.AccessLogger.Warn("failed to set quota cache for entity %s: %v", *param.EntityID, cacheErr)
			}
		} else if param.QuotaPlan.Quota != nil {
			if cacheErr := m.quotaCache.SetRemaining(ctx, *param.EntityID, param.QuotaPlan.Quota, param.QuotaPlan.Unit); cacheErr != nil {
				stateful.AccessLogger.Warn("failed to set quota cache for entity %s: %v", *param.EntityID, cacheErr)
			}
		}
	}

	entityName := ""
	if param.Name != nil {
		entityName = *param.Name
	}
	entityID := ""
	if param.EntityID != nil {
		entityID = *param.EntityID
	}
	parentID := ""
	if param.ParentID != nil {
		parentID = *param.ParentID
	}
	m.recordEntityOperation(ctx, string(ioperlog.ActionCreate), entityID, entityName, parentID, nil, entityParamToMap(param), nil)

	return id, nil
}

// FetchEntity 查询单个 Entity
func (m *EntityManager) FetchEntity(ctx context.Context, filter *EntityFilter) (*EntityParam, error) {
	var one *EntityParam
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error
		one, err = m.storager.FetchEntity(ctx, filter)
		if err != nil {
			return err
		}
		if one == nil {
			return nil
		}
		return m.populateAssociatedData(ctx, one)
	})
	if err != nil {
		return nil, err
	}

	// 事务外从 Redis 读取实时余额（最终一致，失败不影响主数据返回）。
	if one != nil {
		if err := m.populateQuotaBalance(ctx, one); err != nil {
			stateful.AccessLogger.Warn("failed to populate quota balance for entity: %v", err)
		}
	}

	return one, nil
}

// FetchEntityList 查询 Entity 列表
func (m *EntityManager) FetchEntityList(ctx context.Context, filter *EntityFilter) ([]*EntityParam, error) {
	var list []*EntityParam
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error
		list, err = m.storager.FetchEntityList(ctx, filter)
		if err != nil {
			return err
		}
		for _, one := range list {
			if err := m.populateAssociatedData(ctx, one); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 事务外批量从 Redis 读取实时余额（最终一致，失败不影响主数据返回）。
	if err := m.populateQuotaBalances(ctx, list); err != nil {
		stateful.AccessLogger.Warn("failed to populate quota balances for entity list: %v", err)
	}

	return list, nil
}

// UpdateEntity 更新 Entity
func (m *EntityManager) UpdateEntity(ctx context.Context, filter *EntityFilter, param *EntityParam) (int64, error) {
	if param.ParentID != nil && *param.ParentID != "" && param.Type != nil && m.entityTypeStorager != nil {
		if err := m.checkEntityLevel(ctx, *param.Type, *param.ParentID); err != nil {
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, entityParamToMap(param), nil, err)
			return 0, err
		}
	} else if param.ParentID != nil && *param.ParentID != "" && m.entityTypeStorager != nil {
		list, err := m.storager.FetchEntityList(ctx, filter)
		if err != nil {
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, entityParamToMap(param), nil, err)
			return 0, err
		}
		if len(list) == 0 {
			err := xerror.WrapRecordNotExist("Entity")
			entityID, entityName, parentID := entityParamIdentifiers(param)
			m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, entityParamToMap(param), nil, err)
			return 0, err
		}
		if list[0].Type != nil {
			if err := m.checkEntityLevel(ctx, *list[0].Type, *param.ParentID); err != nil {
				entityID, entityName, parentID := entityParamIdentifiers(param)
				m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, entityParamToMap(param), nil, err)
				return 0, err
			}
		}
	}

	var (
		affected              int64
		rateLimitKeysToDelete []string
		oldEntity             *EntityParam
	)
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error

		list, err := m.storager.FetchEntityList(ctx, filter)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return xerror.WrapRecordNotExist("Entity")
		}

		one := list[0]
		oldEntity = one

		if param.QuotaPlan != nil && m.quotaPlanStorager != nil {
			if one.QuotaPlanID != nil {
				_, err = m.quotaPlanStorager.UpdateQuotaPlan(ctx, *one.QuotaPlanID, param.QuotaPlan)
				if err != nil {
					return err
				}
			} else {
				quotaPlanID, err := m.quotaPlanStorager.CreateQuotaPlan(ctx, param.QuotaPlan)
				if err != nil {
					return err
				}
				param.QuotaPlanID = &quotaPlanID
			}
		}

		if param.RateLimitPolicy != nil && m.rateLimitPolicyStorager != nil {
			if one.RateLimitPolicyID != nil {
				oldPolicy, err := m.rateLimitPolicyStorager.FetchRateLimitPolicy(ctx, *one.RateLimitPolicyID)
				if err != nil {
					return err
				}
				_, err = m.rateLimitPolicyStorager.UpdateRateLimitPolicy(ctx, *one.RateLimitPolicyID, param.RateLimitPolicy)
				if err != nil {
					return err
				}
				if oldPolicy != nil && oldPolicy.Rules != nil {
					rateLimitKeysToDelete = shared.DiffRateLimitRedisKeys(*one.RateLimitPolicyID, oldPolicy.Rules, param.RateLimitPolicy.Rules)
				}
			} else {
				rateLimitPolicyID, err := m.rateLimitPolicyStorager.CreateRateLimitPolicy(ctx, param.RateLimitPolicy)
				if err != nil {
					return err
				}
				param.RateLimitPolicyID = &rateLimitPolicyID
			}
		}

		if param.RouteRules != nil && m.routeRulesStorager != nil {
			if one.RouteRulesID != nil {
				_, err = m.routeRulesStorager.UpdateRouteRules(ctx, *one.RouteRulesID, param.RouteRules)
				if err != nil {
					return err
				}
			} else {
				routeRulesID, err := m.routeRulesStorager.CreateRouteRules(ctx, shared.RouteRulesTypeEntity, one.EntityID, param.RouteRules)
				if err != nil {
					return err
				}
				param.RouteRulesID = &routeRulesID
			}
		}

		affected, err = m.storager.UpdateEntity(ctx, &EntityFilter{EntityID: one.EntityID}, param)
		return err
	})
	if err != nil {
		entityID, entityName, parentID := entityParamIdentifiers(param)
		if oldEntity != nil {
			entityID, entityName, parentID = entityParamIdentifiers(oldEntity)
		}
		m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, entityParamToMap(oldEntity), entityParamToMap(param), err)
		return affected, err
	}

	if len(rateLimitKeysToDelete) > 0 {
		m.cleanupRedisKeys(ctx, "", rateLimitKeysToDelete)
	}

	entityID := ""
	entityName := ""
	parentID := ""
	if oldEntity != nil {
		if oldEntity.EntityID != nil {
			entityID = *oldEntity.EntityID
		}
		if oldEntity.Name != nil {
			entityName = *oldEntity.Name
		}
		if oldEntity.ParentID != nil {
			parentID = *oldEntity.ParentID
		}
	}
	m.recordEntityOperation(ctx, string(ioperlog.ActionUpdate), entityID, entityName, parentID, entityParamToMap(oldEntity), entityParamToMap(param), nil)

	return affected, nil
}

// DeleteEntity 删除 Entity
func (m *EntityManager) DeleteEntity(ctx context.Context, filter *EntityFilter) error {
	var (
		quotaKey      string
		rateLimitKeys []string
		oldEntity     *EntityParam
	)

	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		list, err := m.storager.FetchEntityList(ctx, filter)
		if err != nil {
			return err
		}
		if len(list) == 0 {
			return xerror.WrapRecordNotExist("Entity")
		}

		one := list[0]
		oldEntity = one

		if one.EntityID != nil && *one.EntityID != "" {
			quotaKey = *one.EntityID
		}

		if one.RateLimitPolicyID != nil && m.rateLimitPolicyStorager != nil {
			policy, err := m.rateLimitPolicyStorager.FetchRateLimitPolicy(ctx, *one.RateLimitPolicyID)
			if err != nil {
				return err
			}
			if policy != nil && policy.Rules != nil {
				rateLimitKeys = shared.BuildRateLimitRedisKeys(*one.RateLimitPolicyID, policy.Rules)
			}
		}

		children, err := m.storager.FetchEntityList(ctx, &EntityFilter{ParentID: one.EntityID})
		if err != nil {
			return err
		}
		if len(children) > 0 {
			return xerror.WrapConflictErrorWithMsg("cannot delete entity with children")
		}

		if one.QuotaPlanID != nil && m.quotaPlanStorager != nil {
			if err := m.quotaPlanStorager.DeleteQuotaPlan(ctx, *one.QuotaPlanID); err != nil {
				return err
			}
		}

		if one.RateLimitPolicyID != nil && m.rateLimitPolicyStorager != nil {
			if err := m.rateLimitPolicyStorager.DeleteRateLimitPolicy(ctx, *one.RateLimitPolicyID); err != nil {
				return err
			}
		}

		if one.RouteRulesID != nil && m.routeRulesStorager != nil {
			if err := m.routeRulesStorager.DeleteRouteRules(ctx, *one.RouteRulesID); err != nil {
				return err
			}
		}

		return m.storager.DeleteEntity(ctx, filter)
	})
	if err != nil {
		entityID, entityName, parentID := entityParamIdentifiers(oldEntity)
		m.recordEntityOperation(ctx, string(ioperlog.ActionDelete), entityID, entityName, parentID, entityParamToMap(oldEntity), nil, err)
		return err
	}

	// 事务提交成功后清理 Redis Key
	m.cleanupRedisKeys(ctx, quotaKey, rateLimitKeys)

	entityID := ""
	entityName := ""
	parentID := ""
	if oldEntity != nil {
		if oldEntity.EntityID != nil {
			entityID = *oldEntity.EntityID
		}
		if oldEntity.Name != nil {
			entityName = *oldEntity.Name
		}
		if oldEntity.ParentID != nil {
			parentID = *oldEntity.ParentID
		}
	}
	m.recordEntityOperation(ctx, string(ioperlog.ActionDelete), entityID, entityName, parentID, entityParamToMap(oldEntity), nil, nil)

	return nil
}

// cleanupRedisKeys 清理 Quota Key 与 Rate-Limit Key，错误仅记录日志不返回。
func (m *EntityManager) cleanupRedisKeys(ctx context.Context, quotaKey string, rateLimitKeys []string) {
	if m.quotaCache == nil {
		return
	}

	var keysToDelete []string
	if quotaKey != "" {
		keysToDelete = append(keysToDelete, stateful.AIUsedQuotaKey(quotaKey))
	}
	if len(rateLimitKeys) > 0 {
		keysToDelete = append(keysToDelete, rateLimitKeys...)
	}
	if len(keysToDelete) == 0 {
		return
	}

	if err := m.quotaCache.DeleteKeys(ctx, keysToDelete); err != nil {
		stateful.AccessLogger.Warn("failed to cleanup redis keys for entity: %v", err)
	}
}

func (m *EntityManager) populateAssociatedData(ctx context.Context, one *EntityParam) error {
	if one.QuotaPlan == nil {
		one.QuotaPlan = &shared.QuotaPlanParam{}
	}
	if one.RateLimitPolicy == nil {
		one.RateLimitPolicy = &shared.RateLimitPolicyParam{}
	}

	if one.QuotaPlanID != nil && m.quotaPlanStorager != nil {
		quotaPlan, err := m.quotaPlanStorager.FetchQuotaPlan(ctx, *one.QuotaPlanID)
		if err != nil {
			return err
		}
		one.QuotaPlan = quotaPlan
	}

	if one.RateLimitPolicyID != nil && m.rateLimitPolicyStorager != nil {
		rateLimitPolicy, err := m.rateLimitPolicyStorager.FetchRateLimitPolicy(ctx, *one.RateLimitPolicyID)
		if err != nil {
			return err
		}
		one.RateLimitPolicy = rateLimitPolicy
	}

	if one.RouteRules == nil {
		one.RouteRules = &shared.RouteRulesParam{}
	}
	if one.RouteRules.Enabled == nil {
		enabled := false
		one.RouteRules.Enabled = &enabled
	}
	if one.RouteRules.Rules == nil {
		one.RouteRules.Rules = []*shared.AiRouteRuleParam{}
	}

	if one.RouteRulesID != nil && m.routeRulesStorager != nil {
		routeRules, err := m.routeRulesStorager.FetchRouteRulesByID(ctx, *one.RouteRulesID)
		if err != nil {
			return err
		}
		if routeRules != nil {
			one.RouteRules = routeRules
		}
	}

	return nil
}

const unlimitedSentinel = float64(100000000)

// fillQuotaBalance 根据 Redis 实时剩余量填充 quotaPlan.Balance。
func fillQuotaBalance(quotaPlan *shared.QuotaPlanParam, remaining float64) {
	if quotaPlan == nil || quotaPlan.Quota == nil {
		return
	}
	if remaining < 0 {
		remaining = 0
	}
	used := *quotaPlan.Quota - remaining
	if used < 0 {
		used = 0
	}
	if quotaPlan.Balance == nil {
		quotaPlan.Balance = &shared.BalanceSummary{}
	}
	quotaPlan.Balance.Used = &used
	quotaPlan.Balance.Remaining = &remaining
}

// fillUnlimitedQuotaBalance 为无限配额填充 sentinel 余额（used=0, remaining=100000000）。
func fillUnlimitedQuotaBalance(quotaPlan *shared.QuotaPlanParam) {
	if quotaPlan == nil {
		return
	}
	used := float64(0)
	remaining := unlimitedSentinel
	if quotaPlan.Balance == nil {
		quotaPlan.Balance = &shared.BalanceSummary{}
	}
	quotaPlan.Balance.Used = &used
	quotaPlan.Balance.Remaining = &remaining
}

// populateQuotaBalance 为单个 Entity 从 Redis 实时读取剩余量并填充 Balance。
func (m *EntityManager) populateQuotaBalance(ctx context.Context, one *EntityParam) error {
	if m.quotaCache == nil {
		return nil
	}
	if one.QuotaPlan == nil || one.QuotaPlan.Quota == nil || one.EntityID == nil {
		return nil
	}
	if one.QuotaPlan.Unlimited != nil && *one.QuotaPlan.Unlimited {
		fillUnlimitedQuotaBalance(one.QuotaPlan)
		return nil
	}

	remaining, err := m.quotaCache.GetRemaining(ctx, *one.EntityID, one.QuotaPlan.Unit)
	if err != nil {
		return fmt.Errorf("get %s from cache is error:%s", *one.EntityID, err.Error())
	}
	fillQuotaBalance(one.QuotaPlan, remaining)
	return nil
}

// populateQuotaBalances 为 Entity 列表批量从 Redis 读取剩余量并填充 Balance。
func (m *EntityManager) populateQuotaBalances(ctx context.Context, list []*EntityParam) error {
	if m.quotaCache == nil {
		return nil
	}

	type item struct {
		one *EntityParam
		key string
	}
	groups := make(map[string][]item)
	for _, one := range list {
		if one.QuotaPlan == nil || one.QuotaPlan.Quota == nil || one.EntityID == nil {
			continue
		}
		if one.QuotaPlan.Unlimited != nil && *one.QuotaPlan.Unlimited {
			fillUnlimitedQuotaBalance(one.QuotaPlan)
			continue
		}
		unit := ""
		if one.QuotaPlan.Unit != nil {
			unit = *one.QuotaPlan.Unit
		}
		groups[unit] = append(groups[unit], item{one: one, key: *one.EntityID})
	}

	for unit, items := range groups {
		keys := make([]string, len(items))
		for i, it := range items {
			keys[i] = it.key
		}
		var unitPtr *string
		if unit != "" {
			unitPtr = &unit
		}
		result, err := m.quotaCache.BatchGetRemaining(ctx, keys, unitPtr)
		if err != nil {
			return err
		}
		for _, it := range items {
			fillQuotaBalance(it.one.QuotaPlan, result[it.key])
		}
	}
	return nil
}

// FetchEntitySummary 获取 Entity 摘要
func (m *EntityManager) FetchEntitySummary(ctx context.Context, entityID string) (*shared.EntitySummary, error) {
	entity, err := m.FetchEntity(ctx, &EntityFilter{
		EntityID: &entityID,
	})
	if err != nil {
		return nil, err
	}

	if entity == nil {
		return nil, nil
	}

	return &shared.EntitySummary{
		ID:   entity.EntityID,
		Name: entity.Name,
		Type: entity.Type,
	}, nil
}

// checkEntityLevel 检查子 Entity 的 level 是否高于父 Entity 的 level
func (m *EntityManager) checkEntityLevel(ctx context.Context, entityType string, parentID string) error {
	if m.entityTypeStorager == nil {
		return nil
	}

	entityTypeInfo, err := m.entityTypeStorager.FetchEntityType(ctx, &EntityTypeFilter{TypeName: &entityType})
	if err != nil {
		return err
	}
	if entityTypeInfo == nil {
		return xerror.WrapParamErrorWithMsg("%s", "entity type not found: "+entityType)
	}
	if entityTypeInfo.Level == nil {
		return xerror.WrapParamErrorWithMsg("%s", "entity type level not set: "+entityType)
	}

	parentEntity, err := m.storager.FetchEntity(ctx, &EntityFilter{EntityID: &parentID})
	if err != nil {
		return err
	}
	if parentEntity == nil {
		return xerror.WrapParamErrorWithMsg("%s", "parent entity not found: "+parentID)
	}
	if parentEntity.Type == nil {
		return xerror.WrapParamErrorWithMsg("%s", "parent entity type not set: "+parentID)
	}

	parentEntityTypeInfo, err := m.entityTypeStorager.FetchEntityType(ctx, &EntityTypeFilter{TypeName: parentEntity.Type})
	if err != nil {
		return err
	}
	if parentEntityTypeInfo == nil {
		return xerror.WrapParamErrorWithMsg("%s", "parent entity type not found: "+*parentEntity.Type)
	}
	if parentEntityTypeInfo.Level == nil {
		return xerror.WrapParamErrorWithMsg("%s", "parent entity type level not set: "+*parentEntity.Type)
	}

	if *entityTypeInfo.Level <= *parentEntityTypeInfo.Level {
		return xerror.WrapParamErrorWithMsg("%s", fmt.Sprintf("entity type level (%d) must be higher than parent entity type level (%d)",
			*entityTypeInfo.Level, *parentEntityTypeInfo.Level))
	}

	return nil
}

// entityStoragerAdapter 适配 EntityStorager 为 shared.EntityStorager
type entityStoragerAdapter struct {
	entityStorager EntityStorager
}

// NewEntityStoragerAdapter 创建 EntityStorager 适配器
func NewEntityStoragerAdapter(entityStorager EntityStorager) shared.EntityStorager {
	return &entityStoragerAdapter{
		entityStorager: entityStorager,
	}
}

func (a *entityStoragerAdapter) FetchEntity(ctx context.Context, filter *shared.EntityFilter) (*shared.EntitySummary, error) {
	entity, err := a.entityStorager.FetchEntity(ctx, &EntityFilter{
		EntityID: filter.EntityID,
		Name:     filter.Name,
		Type:     filter.Type,
		ParentID: filter.ParentID,
	})
	if err != nil {
		return nil, err
	}
	if entity == nil {
		return nil, nil
	}
	return &shared.EntitySummary{
		ID:   entity.EntityID,
		Name: entity.Name,
		Type: entity.Type,
	}, nil
}

func entityParamIdentifiers(param *EntityParam) (entityID, entityName, parentID string) {
	if param == nil {
		return "", "", ""
	}
	if param.EntityID != nil {
		entityID = *param.EntityID
	}
	if param.Name != nil {
		entityName = *param.Name
	}
	if param.ParentID != nil {
		parentID = *param.ParentID
	}
	return
}
