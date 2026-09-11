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

package route_rules

import (
	"context"
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/validate"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/icluster_conf"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
)

// RouteRulesManager manages route rules with transaction support
type RouteRulesManager struct {
	txn                 itxn.TxnStorager
	storager            shared.RouteRulesStorager
	operationLogManager ioperlog.OperationLogRecorder
}

// NewRouteRulesManager creates a new RouteRulesManager instance
func NewRouteRulesManager(txn itxn.TxnStorager, storager shared.RouteRulesStorager) *RouteRulesManager {
	return &RouteRulesManager{
		txn:      txn,
		storager: storager,
	}
}

// SetOperationLogManager injects the operation log recorder.
func (m *RouteRulesManager) SetOperationLogManager(manager ioperlog.OperationLogRecorder) {
	m.operationLogManager = manager
}

// CreateRouteRules creates route rules for a given type and owner
func (m *RouteRulesManager) CreateRouteRules(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
	var id int64
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		if err := m.validateRouteRules(param); err != nil {
			return err
		}

		var err error
		id, err = m.storager.CreateRouteRules(ctx, ruleType, owner, param)
		return err
	})

	return id, err
}

// UpdateRouteRules updates route rules by type and owner
// If no existing rules, creates a new one
func (m *RouteRulesManager) UpdateRouteRules(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
	var id int64
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		if err := m.validateRouteRules(param); err != nil {
			return err
		}

		existing, err := m.storager.FetchRouteRules(ctx, ruleType, owner)
		if err != nil {
			return err
		}

		if existing == nil || existing.ID == nil {
			id, err = m.storager.CreateRouteRules(ctx, ruleType, owner, param)
			return err
		}

		id = *existing.ID
		_, err = m.storager.UpdateRouteRules(ctx, id, param)
		return err
	})

	return id, err
}

// FetchRouteRules fetches route rules by type and owner
func (m *RouteRulesManager) FetchRouteRules(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
	var result *shared.RouteRulesParam
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.storager.FetchRouteRules(ctx, ruleType, owner)
		return err
	})

	return result, err
}

// DeleteRouteRules deletes route rules by type and owner
func (m *RouteRulesManager) DeleteRouteRules(ctx context.Context, ruleType string, owner *string) error {
	return m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		existing, err := m.storager.FetchRouteRules(ctx, ruleType, owner)
		if err != nil {
			return err
		}
		if existing == nil || existing.ID == nil {
			return nil
		}

		return m.storager.DeleteRouteRules(ctx, *existing.ID)
	})
}

// GetGlobalRouteRules fetches global route rules
func (m *RouteRulesManager) GetGlobalRouteRules(ctx context.Context) (*shared.RouteRulesParam, error) {
	owner := shared.RouteRulesTypeGlobal
	return m.FetchRouteRules(ctx, shared.RouteRulesTypeGlobal, &owner)
}

// SetGlobalRouteRules sets global route rules
func (m *RouteRulesManager) SetGlobalRouteRules(ctx context.Context, param *shared.RouteRulesParam) (*shared.RouteRulesParam, error) {
	owner := shared.RouteRulesTypeGlobal

	// Fetch existing rules for change summary; ignore errors here because
	// UpdateRouteRules will re-fetch inside its transaction.
	before, _ := m.storager.FetchRouteRules(ctx, shared.RouteRulesTypeGlobal, &owner)
	beforeMap := routeRulesParamToMap(before)

	id, err := m.UpdateRouteRules(ctx, shared.RouteRulesTypeGlobal, &owner, param)
	if err != nil {
		m.recordGlobalRouteRulesOperation(ctx, string(ioperlog.ActionUpdate), beforeMap, routeRulesParamToMap(param), err)
		return nil, err
	}

	updated, err := m.storager.FetchRouteRulesByID(ctx, id)
	if err != nil {
		m.recordGlobalRouteRulesOperation(ctx, string(ioperlog.ActionUpdate), beforeMap, routeRulesParamToMap(param), err)
		return nil, err
	}
	if updated == nil {
		err = xerror.WrapRecordNotExist("global route rules")
		m.recordGlobalRouteRulesOperation(ctx, string(ioperlog.ActionUpdate), beforeMap, routeRulesParamToMap(param), err)
		return nil, err
	}

	m.recordGlobalRouteRulesOperation(ctx, string(ioperlog.ActionUpdate), beforeMap, routeRulesParamToMap(updated), nil)
	return updated, nil
}

// EnsureGlobalRouteRules ensures the global route table exists.
// If it does not exist, a default global route table (disabled, empty rules) is created.
func (m *RouteRulesManager) EnsureGlobalRouteRules(ctx context.Context) error {
	return m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		owner := shared.RouteRulesTypeGlobal
		existing, err := m.storager.FetchRouteRules(ctx, shared.RouteRulesTypeGlobal, &owner)
		if err != nil {
			return err
		}
		if existing != nil {
			return nil
		}

		enabled := false
		param := &shared.RouteRulesParam{
			Enabled: &enabled,
			Rules:   []*shared.AiRouteRuleParam{},
		}
		_, err = m.storager.CreateRouteRules(ctx, shared.RouteRulesTypeGlobal, &owner, param)
		return err
	})
}

// FetchRouteRulesByID fetches route rules by id
func (m *RouteRulesManager) FetchRouteRulesByID(ctx context.Context, id int64) (*shared.RouteRulesParam, error) {
	var result *shared.RouteRulesParam
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.storager.FetchRouteRulesByID(ctx, id)
		return err
	})

	return result, err
}

// ListRouteTables lists route tables with pagination
func (m *RouteRulesManager) ListRouteTables(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error) {
	var result []*shared.RouteTableParam
	var total int64
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		var err error
		result, total, err = m.storager.FetchRouteRulesList(ctx, filter)
		return err
	})

	return result, total, err
}

// ClusterDeleteChecker checks whether the cluster is referenced by any route rule
// in the route_rules table (global/entity/api-key level).
// Note: this checker is invoked inside a transaction by the cluster manager, so
// it should not start another transaction here to avoid premature commit of the
// outer transaction.
func (m *RouteRulesManager) ClusterDeleteChecker(ctx context.Context, product *ibasic.Product, cluster *icluster_conf.Cluster) error {
	allRouteRules, err := m.storager.FetchAllRouteRules(ctx)
	if err != nil {
		return err
	}

	return checkClusterReferencedByRouteRules(cluster.Name, allRouteRules)
}

func checkClusterReferencedByRouteRules(clusterName string, routeRules []*shared.RouteRulesParam) error {
	for _, routeRulesParam := range routeRules {
		if routeRulesParam == nil {
			continue
		}

		for _, rule := range routeRulesParam.Rules {
			if rule == nil {
				continue
			}

			ruleName := ""
			if rule.Name != nil {
				ruleName = *rule.Name
			}

			for _, target := range rule.Targets {
				if target != nil && target.ClusterName != nil && *target.ClusterName == clusterName {
					return xerror.WrapConflictErrorWithMsg("Rule %s Refer To This Cluster", ruleName)
				}
			}

			for _, fallback := range rule.Fallbacks {
				if fallback != nil && fallback.ClusterName != nil && *fallback.ClusterName == clusterName {
					return xerror.WrapConflictErrorWithMsg("Rule %s Refer To This Cluster", ruleName)
				}
			}
		}
	}

	return nil
}

// ClusterModelUpdateChecker checks whether any model being removed from a cluster
// is still referenced by route rules (targets or fallbacks). The checker is
// invoked inside a transaction by the cluster manager, so it should not start
// another transaction here.
func (m *RouteRulesManager) ClusterModelUpdateChecker(ctx context.Context, product *ibasic.Product, cluster *icluster_conf.Cluster, param *icluster_conf.ClusterParam) error {
	if param.LLMConfig == nil {
		return nil
	}

	removedModels := removedModels(cluster.LLMConfig, param.LLMConfig)
	if len(removedModels) == 0 {
		return nil
	}

	allRouteRules, err := m.storager.FetchAllRouteRules(ctx)
	if err != nil {
		return err
	}

	return checkClusterModelsReferenced(cluster.Name, removedModels, allRouteRules)
}

func removedModels(oldConfig, newConfig *icluster_conf.LLMConfig) []string {
	oldModels := map[string]struct{}{}
	if oldConfig != nil {
		for _, model := range oldConfig.Models {
			oldModels[model] = struct{}{}
		}
	}

	newModels := map[string]struct{}{}
	for _, model := range newConfig.Models {
		newModels[model] = struct{}{}
	}

	var removed []string
	for model := range oldModels {
		if _, ok := newModels[model]; !ok {
			removed = append(removed, model)
		}
	}
	return removed
}

func checkClusterModelsReferenced(clusterName string, models []string, routeRules []*shared.RouteRulesParam) error {
	modelSet := lib.StringSlice2Map(models)

	for _, routeRulesParam := range routeRules {
		if routeRulesParam == nil {
			continue
		}

		for _, rule := range routeRulesParam.Rules {
			if rule == nil {
				continue
			}

			ruleName := ""
			if rule.Name != nil {
				ruleName = *rule.Name
			}

			for _, target := range rule.Targets {
				if target != nil && target.ClusterName != nil && *target.ClusterName == clusterName &&
					target.Model != nil && modelSet[*target.Model] {
					return xerror.WrapConflictErrorWithMsg("Rule %s Refer To Model %s In Cluster %s", ruleName, *target.Model, clusterName)
				}
			}

			for _, fallback := range rule.Fallbacks {
				if fallback != nil && fallback.ClusterName != nil && *fallback.ClusterName == clusterName &&
					fallback.Model != nil && modelSet[*fallback.Model] {
					return xerror.WrapConflictErrorWithMsg("Rule %s Refer To Model %s In Cluster %s", ruleName, *fallback.Model, clusterName)
				}
			}
		}
	}

	return nil
}

func (m *RouteRulesManager) validateRouteRules(param *shared.RouteRulesParam) error {
	if param == nil {
		return nil
	}

	nameSet := make(map[string]struct{})
	for _, rule := range param.Rules {
		if rule.Name == nil || *rule.Name == "" {
			return xerror.WrapParamErrorWithMsg("rule name is required")
		}
		if _, ok := nameSet[*rule.Name]; ok {
			return xerror.WrapParamErrorWithMsg("%s", fmt.Sprintf("duplicate rule name: %s", *rule.Name))
		}
		nameSet[*rule.Name] = struct{}{}

		if rule.Cond == nil || *rule.Cond == "" {
			return xerror.WrapParamErrorWithMsg("rule Cond is required")
		}
		if err := validate.ConditionExpression(*rule.Cond); err != nil {
			return err
		}

		if len(rule.Targets) == 0 {
			return xerror.WrapParamErrorWithMsg("targets cannot be empty")
		}

		totalWeight := 0
		for _, target := range rule.Targets {
			if target.Weight == nil {
				return xerror.WrapParamErrorWithMsg("target weight is required")
			}
			totalWeight += *target.Weight
		}
		if totalWeight != 100 {
			return xerror.WrapParamErrorWithMsg("targets total weight must be 100")
		}

		for _, fallback := range rule.Fallbacks {
			if fallback.ClusterName == nil || *fallback.ClusterName == "" {
				return xerror.WrapParamErrorWithMsg("fallback ClusterName cannot be empty")
			}
		}
	}

	return nil
}
