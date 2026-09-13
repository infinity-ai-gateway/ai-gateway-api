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
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
)

const globalRouteRulesResourceID = "global"

func (m *RouteRulesManager) recordGlobalRouteRulesOperation(ctx context.Context, action string, before, after map[string]interface{}, err error) {
	if m.operationLogManager == nil {
		return
	}

	status := ioperlog.StatusSuccess
	errorMsg := ""
	if err != nil {
		status = ioperlog.StatusFailed
		errorMsg = ioperlog.TruncateErrorMessageDefault(err)
	}

	entry := &ioperlog.OperationLogEntry{
		Action:       action,
		ResourceType: string(ioperlog.ResourceTypeRoute),
		ResourceID:   globalRouteRulesResourceID,
		ResourceName: globalRouteRulesResourceID,
		Status:       status,
		ErrorMsg:     errorMsg,
		CreatedAt:    time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func routeRulesParamToMap(param *shared.RouteRulesParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.ID != nil {
		m["id"] = *param.ID
	}
	if param.Enabled != nil {
		m["enabled"] = *param.Enabled
	}
	if len(param.Rules) > 0 {
		rules := make([]map[string]interface{}, len(param.Rules))
		for i, r := range param.Rules {
			rules[i] = aiRouteRuleParamToMap(r)
		}
		m["rules"] = rules
	}

	return m
}

func aiRouteRuleParamToMap(rule *shared.AiRouteRuleParam) map[string]interface{} {
	if rule == nil {
		return nil
	}

	m := map[string]interface{}{}
	if rule.Name != nil {
		m["name"] = *rule.Name
	}
	if rule.Cond != nil {
		m["cond"] = *rule.Cond
	}
	if len(rule.Targets) > 0 {
		targets := make([]map[string]interface{}, len(rule.Targets))
		for i, t := range rule.Targets {
			targets[i] = aiRouteTargetParamToMap(t)
		}
		m["targets"] = targets
	}
	if len(rule.Fallbacks) > 0 {
		fallbacks := make([]map[string]interface{}, len(rule.Fallbacks))
		for i, f := range rule.Fallbacks {
			fallbacks[i] = aiRouteFallbackParamToMap(f)
		}
		m["fallbacks"] = fallbacks
	}

	return m
}

func aiRouteTargetParamToMap(target *shared.AiRouteTargetParam) map[string]interface{} {
	if target == nil {
		return nil
	}

	m := map[string]interface{}{}
	if target.ClusterName != nil {
		m["cluster_name"] = *target.ClusterName
	}
	if target.Model != nil {
		m["model"] = *target.Model
	}
	if target.Weight != nil {
		m["weight"] = *target.Weight
	}

	return m
}

func aiRouteFallbackParamToMap(fallback *shared.AiRouteFallbackParam) map[string]interface{} {
	if fallback == nil {
		return nil
	}

	m := map[string]interface{}{}
	if fallback.ClusterName != nil {
		m["cluster_name"] = *fallback.ClusterName
	}
	if fallback.Model != nil {
		m["model"] = *fallback.Model
	}

	return m
}
