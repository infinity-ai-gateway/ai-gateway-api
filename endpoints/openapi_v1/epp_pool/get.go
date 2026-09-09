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

package epp_pool

import (
	"net/http"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// GetRoute route
var GetEndpoint = &xreq.Endpoint{
	Path:       "/epp-pool",
	Method:     http.MethodGet,
	Handler:    xreq.Convert(GetAction),
	Authorizer: iauth.FA(iauth.FeatureBFEPool, iauth.ActionReadAll),
}

var _ xreq.Handler = GetAction

// GetAction returns the EPP instance pool detail (groups + instances),
// see epp-pool.md §2.1.
func GetAction(req *http.Request) (interface{}, error) {
	pool, err := container.EppPoolManager.GetPool(req.Context())
	if err != nil {
		return nil, err
	}

	return newPoolData(pool), nil
}
