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

// Package epp_data exposes the InnerAPI export endpoint of the merged EPP
// data config (epp_config + assignment, see design-docs/api-define/
// InnerAPI接口定义/epp-data.md).
package epp_data

import (
	"net/http"

	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/innerapi_v1/export_util"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// ExportRoute route
var ExportRoute = &xreq.Endpoint{
	Path:       "/configs/epp_data/config",
	Method:     http.MethodGet,
	Handler:    xreq.Convert(ExportAction),
	Authorizer: iauth.FA(iauth.FeatureRoute, iauth.ActionExport),
}

var _ xreq.Handler = ExportAction

// ExportAction exports the epp_data config. When the version carried by the
// request is still the latest one, a nil payload is returned (Data: null
// semantics, see epp-data.md §4).
func ExportAction(req *http.Request) (interface{}, error) {
	param, err := export_util.NewExportFromReq(req)
	if err != nil {
		return nil, err
	}

	return container.EppPoolManager.ExportEppData(req.Context(), param.Version)
}
