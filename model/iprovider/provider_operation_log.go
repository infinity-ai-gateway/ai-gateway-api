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

package iprovider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (m *ProviderManager) recordProviderOperation(ctx context.Context, action, name string, before, after map[string]interface{}, err error) {
	if m.operationLogManager == nil || name == "" {
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
		ResourceType: string(ioperlog.ResourceTypeProvider),
		ResourceID:   name,
		ResourceName: name,
		Status:       status,
		ErrorMsg:     errorMsg,
		CreatedAt:    time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func providerParamToMap(param *ProviderParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	data, err := json.Marshal(param)
	if err != nil {
		return nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	return m
}

func providerToMap(provider *Provider) map[string]interface{} {
	if provider == nil {
		return nil
	}

	data, err := json.Marshal(provider)
	if err != nil {
		return nil
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil
	}

	return m
}
