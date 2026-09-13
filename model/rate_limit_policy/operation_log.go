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
	"strconv"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (m *RateLimitPolicyManager) recordRateLimitPolicyOperation(ctx context.Context, action string, policyID, parentID string, before, after map[string]interface{}, err error) {
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
		ResourceType:     string(ioperlog.ResourceTypeRateLimitPolicy),
		ResourceID:       policyID,
		ResourceName:     "",
		ResourceParentID: parentID,
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func rateLimitPolicyIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func rateLimitPolicyParamToMap(param *RateLimitPolicyParam) map[string]interface{} {
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
	if param.MaxConcurrency != nil {
		m["max_concurrency"] = *param.MaxConcurrency
	}
	if len(param.TpmConfigs) > 0 {
		m["tpm_configs"] = param.TpmConfigs
	}
	if len(param.RpmConfigs) > 0 {
		m["rpm_configs"] = param.RpmConfigs
	}

	return m
}
