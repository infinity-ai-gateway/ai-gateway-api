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
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (m *EntityManager) recordEntityOperation(ctx context.Context, action string, entityID, entityName, parentID string, before, after map[string]interface{}, err error) {
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
		ResourceType:     string(ioperlog.ResourceTypeEntity),
		ResourceID:       entityID,
		ResourceName:     entityName,
		ResourceParentID: parentID,
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func (m *EntityTypeManager) recordEntityTypeOperation(ctx context.Context, action string, param *EntityTypeParam, before, after map[string]interface{}, err error) {
	if m.operationLogManager == nil || param == nil {
		return
	}

	resourceID := ""
	if param.TypeName != nil {
		resourceID = *param.TypeName
	}
	resourceName := resourceID
	if param.Description != nil && *param.Description != "" {
		resourceName = *param.Description
	}

	status := ioperlog.StatusSuccess
	errorMsg := ""
	if err != nil {
		status = ioperlog.StatusFailed
		errorMsg = ioperlog.TruncateErrorMessageDefault(err)
	}

	entry := &ioperlog.OperationLogEntry{
		Action:       action,
		ResourceType: string(ioperlog.ResourceTypeEntityType),
		ResourceID:   resourceID,
		ResourceName: resourceName,
		Status:       status,
		ErrorMsg:     errorMsg,
		CreatedAt:    time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func entityTypeParamToMap(param *EntityTypeParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.TypeName != nil {
		m["type_name"] = *param.TypeName
	}
	if param.Description != nil {
		m["description"] = *param.Description
	}
	if param.Level != nil {
		m["level"] = *param.Level
	}

	return m
}

func entityParamToMap(param *EntityParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.Name != nil {
		m["name"] = *param.Name
	}
	if param.Type != nil {
		m["type"] = *param.Type
	}
	if param.ParentID != nil {
		m["parent_id"] = *param.ParentID
	}
	if param.EntityID != nil {
		m["entity_id"] = *param.EntityID
	}
	if len(param.AllowModels) > 0 {
		m["allow_models"] = param.AllowModels
	}
	if len(param.BlockModels) > 0 {
		m["block_models"] = param.BlockModels
	}

	return m
}
