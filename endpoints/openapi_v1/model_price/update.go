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

package model_price

import (
	"net/http"
	"strings"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/imodel_price"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// UpdateEndpoint updates a model price record by id.
var UpdateEndpoint = &xreq.Endpoint{
	Path:       "/model-prices/{id}",
	Method:     http.MethodPut,
	Handler:    xreq.Convert(UpdateAction),
	Authorizer: iauth.FA(iauth.FeatureModelPrice, iauth.ActionUpdate),
}

// UpdateByQueryEndpoint updates a model price record by (provider, model, mode).
var UpdateByQueryEndpoint = &xreq.Endpoint{
	Path:       "/model-prices",
	Method:     http.MethodPut,
	Handler:    xreq.Convert(UpdateByQueryAction),
	Authorizer: iauth.FA(iauth.FeatureModelPrice, iauth.ActionUpdate),
}

// UpdateAction handles PUT /model-prices/{id}.
func UpdateAction(req *http.Request) (interface{}, error) {
	id := idFromURI(req)
	if id == nil {
		return nil, xerror.WrapParamErrorWithMsg("id is required")
	}

	param := &imodel_price.ModelPrice{}
	if err := xreq.BindJSON(req, param); err != nil {
		return nil, err
	}

	filter := &imodel_price.ModelPriceFilter{ID: id}
	existing, err := container.ModelPriceManager.FetchModelPrice(req.Context(), filter)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, xerror.WrapRecordNotExist("ModelPrice")
	}

	merged := mergeModelPrice(existing, param)
	if _, err := container.ModelPriceManager.UpdateModelPrice(req.Context(), filter, merged); err != nil {
		return nil, err
	}

	return container.ModelPriceManager.FetchModelPrice(req.Context(), filter)
}

// UpdateByQueryAction handles PUT /model-prices?provider=&model=&mode=.
func UpdateByQueryAction(req *http.Request) (interface{}, error) {
	filter := queryFilter(req)
	if (filter.Provider == nil || filter.Model == nil || filter.Mode == nil) && filter.ID == nil {
		return nil, xerror.WrapParamErrorWithMsg("provider, model and mode are required")
	}

	param := &imodel_price.ModelPrice{}
	if err := xreq.BindJSON(req, param); err != nil {
		return nil, err
	}

	existing, err := container.ModelPriceManager.FetchModelPrice(req.Context(), filter)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, xerror.WrapRecordNotExist("ModelPrice")
	}

	merged := mergeModelPrice(existing, param)
	if _, err := container.ModelPriceManager.UpdateModelPrice(req.Context(), filter, merged); err != nil {
		return nil, err
	}

	return container.ModelPriceManager.FetchModelPrice(req.Context(), filter)
}

// mergePriceMap returns a new PriceMap with all keys from dst, overridden by
// keys present in src. Keys absent from src keep their original values, so a
// partial update never silently drops existing price entries (issue #140).
// Neither dst nor src is modified.
func mergePriceMap(dst, src imodel_price.PriceMap) imodel_price.PriceMap {
	merged := make(imodel_price.PriceMap, len(dst)+len(src))
	for k, v := range dst {
		merged[k] = v
	}
	for k, v := range src {
		merged[k] = v
	}
	return merged
}

// mergeTierPriceMap merges tier prices two levels deep: tiers present in src
// are merged key-by-key (via mergePriceMap), tiers only in src are added
// whole, and tiers absent from src are retained whole. Neither dst nor src is
// modified.
func mergeTierPriceMap(dst, src imodel_price.TierPriceMap) imodel_price.TierPriceMap {
	merged := make(imodel_price.TierPriceMap, len(dst)+len(src))
	for tier, prices := range dst {
		// deep-copy retained tiers so merged never aliases dst's inner maps
		merged[tier] = mergePriceMap(prices, nil)
	}
	for tier, prices := range src {
		if existing, ok := merged[tier]; ok {
			merged[tier] = mergePriceMap(existing, prices)
		} else {
			merged[tier] = prices
		}
	}
	return merged
}

// mergeModelPrice merges non-empty fields from src into dst for partial update.
func mergeModelPrice(dst, src *imodel_price.ModelPrice) *imodel_price.ModelPrice {
	merged := *dst
	if strings.TrimSpace(src.Provider) != "" {
		merged.Provider = src.Provider
	}
	if strings.TrimSpace(src.Model) != "" {
		merged.Model = src.Model
	}
	if strings.TrimSpace(src.BaseModel) != "" {
		merged.BaseModel = src.BaseModel
	}
	if strings.TrimSpace(src.Mode) != "" {
		merged.Mode = src.Mode
	}
	if len(src.Capabilities) > 0 {
		merged.Capabilities = src.Capabilities
	}
	if len(src.SupportedParameters) > 0 {
		merged.SupportedParameters = src.SupportedParameters
	}
	if len(src.Limits) > 0 {
		merged.Limits = src.Limits
	}
	if len(src.Prices) > 0 {
		merged.Prices = mergePriceMap(dst.Prices, src.Prices)
	}
	if len(src.TierPrices) > 0 {
		merged.TierPrices = mergeTierPriceMap(dst.TierPrices, src.TierPrices)
	}
	if strings.TrimSpace(src.PriceCurrency) != "" {
		merged.PriceCurrency = src.PriceCurrency
	}
	if len(src.Metadata) > 0 {
		merged.Metadata = src.Metadata
	}
	return &merged
}
