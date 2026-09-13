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

package iroute_conf

import (
	"context"
	"strconv"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (rm *RouteRuleManager) recordRouteRuleOperation(ctx context.Context, action string, product *ibasic.Product, before, after map[string]interface{}, err error) {
	if rm.operationLogManager == nil || product == nil {
		return
	}

	status := ioperlog.StatusSuccess
	errorMsg := ""
	if err != nil {
		status = ioperlog.StatusFailed
		errorMsg = ioperlog.TruncateErrorMessageDefault(err)
	}

	entry := &ioperlog.OperationLogEntry{
		Action:           action,
		ResourceType:     string(ioperlog.ResourceTypeRoute),
		ResourceID:       product.Name,
		ResourceName:     product.Name,
		ResourceParentID: strconv.FormatInt(product.ID, 10),
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	rm.operationLogManager.Record(ctx, entry)
}

func productRouteRuleToMap(rule *ProductRouteRule) map[string]interface{} {
	if rule == nil {
		return nil
	}

	m := map[string]interface{}{}
	if len(rule.BasicRouteRules) > 0 {
		basic := make([]map[string]interface{}, len(rule.BasicRouteRules))
		for i, r := range rule.BasicRouteRules {
			basic[i] = map[string]interface{}{
				"hostnames":    r.HostNames,
				"paths":        r.Paths,
				"cluster_name": r.ClusterName,
			}
		}
		m["basic_rules"] = basic
	}
	if len(rule.AdvanceRouteRules) > 0 {
		adv := make([]map[string]interface{}, len(rule.AdvanceRouteRules))
		for i, r := range rule.AdvanceRouteRules {
			adv[i] = map[string]interface{}{
				"name":         r.Name,
				"expression":   r.Expression,
				"cluster_name": r.ClusterName,
			}
		}
		m["advance_rules"] = adv
	}

	return m
}

func (m *DomainManager) recordDomainOperation(ctx context.Context, action, resourceID, resourceName, parentID string, before, after map[string]interface{}, err error) {
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
		Action:           action,
		ResourceType:     string(ioperlog.ResourceTypeDomain),
		ResourceID:       resourceID,
		ResourceName:     resourceName,
		ResourceParentID: parentID,
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func domainParamToMap(param *DomainParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.ProductID != nil {
		m["product_id"] = *param.ProductID
	}
	if param.Name != nil {
		m["name"] = *param.Name
	}
	if param.UsingAdvancedRedirect != nil {
		m["using_advanced_redirect"] = *param.UsingAdvancedRedirect
	}
	if param.UsingAdvancedHsts != nil {
		m["using_advanced_hsts"] = *param.UsingAdvancedHsts
	}

	return m
}

func domainToMap(domain *Domain) map[string]interface{} {
	if domain == nil {
		return nil
	}

	return map[string]interface{}{
		"id":                      domain.ID,
		"product_id":              domain.ProductID,
		"name":                    domain.Name,
		"using_advanced_redirect": domain.UsingAdvancedRedirect,
		"using_advanced_hsts":     domain.UsingAdvancedHsts,
	}
}
