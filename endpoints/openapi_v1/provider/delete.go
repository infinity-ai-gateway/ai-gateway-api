// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package provider

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// DeleteEndpoint deletes a provider.
var DeleteEndpoint = &xreq.Endpoint{
	Path:       "/providers/{provider_name}",
	Method:     http.MethodDelete,
	Handler:    xreq.Convert(DeleteAction),
	Authorizer: iauth.FA(iauth.FeatureProvider, iauth.ActionDelete),
}

// DeleteAction handles DELETE /providers/{provider_name}.
func DeleteAction(req *http.Request) (interface{}, error) {
	name := mux.Vars(req)["provider_name"]
	if name == "" {
		return nil, xerror.WrapParamErrorWithMsg("provider_name is required")
	}

	err := container.ProviderManager.DeleteProvider(req.Context(), name,
		container.ClusterManager.ProviderDeleteChecker,
	)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

// ProviderDeleteCheckerFunc adapts a manager method to the signature expected by ProviderManager.
type ProviderDeleteCheckerFunc func(ctx context.Context, providerName string) error
