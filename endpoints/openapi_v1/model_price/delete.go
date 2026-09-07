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

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/imodel_price"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// DeleteEndpoint deletes a model price record by id.
var DeleteEndpoint = &xreq.Endpoint{
	Path:       "/model-prices/{id}",
	Method:     http.MethodDelete,
	Handler:    xreq.Convert(DeleteAction),
	Authorizer: iauth.FA(iauth.FeatureModelPrice, iauth.ActionDelete),
}

// DeleteByQueryEndpoint deletes a model price record by (provider, model, mode).
var DeleteByQueryEndpoint = &xreq.Endpoint{
	Path:       "/model-prices",
	Method:     http.MethodDelete,
	Handler:    xreq.Convert(DeleteByQueryAction),
	Authorizer: iauth.FA(iauth.FeatureModelPrice, iauth.ActionDelete),
}

// DeleteAction handles DELETE /model-prices/{id}.
func DeleteAction(req *http.Request) (interface{}, error) {
	id := idFromURI(req)
	if id == nil {
		return nil, xerror.WrapParamErrorWithMsg("id is required")
	}

	existing, err := container.ModelPriceManager.FetchModelPrice(req.Context(), &imodel_price.ModelPriceFilter{ID: id})
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, xerror.WrapRecordNotExist("ModelPrice")
	}

	if err := container.ModelPriceManager.DeleteModelPrice(req.Context(), &imodel_price.ModelPriceFilter{ID: id}); err != nil {
		return nil, err
	}
	return nil, nil
}

// DeleteByQueryAction handles DELETE /model-prices?provider=&model=&mode=.
func DeleteByQueryAction(req *http.Request) (interface{}, error) {
	filter := queryFilter(req)
	if (filter.Provider == nil || filter.Model == nil || filter.Mode == nil) && filter.ID == nil {
		return nil, xerror.WrapParamErrorWithMsg("provider, model and mode are required")
	}

	existing, err := container.ModelPriceManager.FetchModelPrice(req.Context(), filter)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, xerror.WrapRecordNotExist("ModelPrice")
	}

	if err := container.ModelPriceManager.DeleteModelPrice(req.Context(), filter); err != nil {
		return nil, err
	}
	return nil, nil
}
