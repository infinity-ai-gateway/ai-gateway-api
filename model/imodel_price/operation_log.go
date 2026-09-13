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

package imodel_price

import (
	"context"
	"strconv"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (m *Manager) recordModelPriceOperation(ctx context.Context, action, resourceID, resourceName string, before, after map[string]interface{}, err error) {
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
		ResourceType:     string(ioperlog.ResourceTypeModelPrice),
		ResourceID:       resourceID,
		ResourceName:     resourceName,
		ResourceParentID: "",
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	m.operationLogManager.Record(ctx, entry)
}

func (m *Manager) recordModelPriceImport(ctx context.Context, mode string, imported int, entries []*ModelPrice, err error) {
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
		Action:           string(ioperlog.ActionImport),
		ResourceType:     string(ioperlog.ResourceTypeModelPrice),
		ResourceID:       "batch",
		ResourceName:     "model_price_import",
		ResourceParentID: "",
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.MaskSensitiveFields(map[string]interface{}{
		"mode":     mode,
		"imported": imported,
		"entries":  entries,
	})

	m.operationLogManager.Record(ctx, entry)
}

func modelPriceIDString(id int64) string {
	return strconv.FormatInt(id, 10)
}

func modelPriceName(price *ModelPrice) string {
	if price == nil {
		return ""
	}
	return price.Key()
}

func modelPriceToMap(price *ModelPrice) map[string]interface{} {
	if price == nil {
		return nil
	}

	m := map[string]interface{}{}
	if price.ID != 0 {
		m["id"] = price.ID
	}
	if price.Provider != "" {
		m["provider"] = price.Provider
	}
	if price.Model != "" {
		m["model"] = price.Model
	}
	if price.BaseModel != "" {
		m["base_model"] = price.BaseModel
	}
	if price.Mode != "" {
		m["mode"] = price.Mode
	}
	if len(price.Capabilities) > 0 {
		m["capabilities"] = price.Capabilities
	}
	if len(price.SupportedParameters) > 0 {
		m["supported_parameters"] = price.SupportedParameters
	}
	if len(price.Limits) > 0 {
		m["limits"] = price.Limits
	}
	if len(price.Prices) > 0 {
		m["prices"] = price.Prices
	}
	if len(price.TierPrices) > 0 {
		m["tier_prices"] = price.TierPrices
	}
	if price.PriceCurrency != "" {
		m["price_currency"] = price.PriceCurrency
	}
	if len(price.Metadata) > 0 {
		m["metadata"] = price.Metadata
	}

	return m
}
