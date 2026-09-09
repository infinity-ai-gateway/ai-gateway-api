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
	"github.com/rainway-ai-gateway/ai-gateway-api/model/epp_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// PatchParam is the full-replacement body of PATCH /epp-pool
// (epp-pool.md §2.2). Field-level validation lives in the model layer
// (EppPoolManager.PatchPool); invalid payloads surface as 422.
type PatchParam struct {
	Groups []*GroupParam `json:"groups"`
}

// GroupParam is one instance group of the replacement body.
type GroupParam struct {
	Name      string           `json:"name"`
	Instances []*InstanceParam `json:"instances"`
}

// InstanceParam is one instance of the replacement body.
type InstanceParam struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// PatchRoute route
var PatchEndpoint = &xreq.Endpoint{
	Path:       "/epp-pool",
	Method:     http.MethodPatch,
	Handler:    xreq.Convert(PatchAction),
	Authorizer: iauth.FA(iauth.FeatureBFEPool, iauth.ActionUpdate),
}

var _ xreq.Handler = PatchAction

// PatchAction fully replaces the EPP instance pool and returns the updated
// pool detail, see epp-pool.md §2.2.
func PatchAction(req *http.Request) (interface{}, error) {
	param := &PatchParam{}
	if err := xreq.BindJSON(req, param); err != nil {
		return nil, err
	}

	pool, err := container.EppPoolManager.PatchPool(req.Context(), toModelGroups(param.Groups))
	if err != nil {
		return nil, err
	}

	return newPoolData(pool), nil
}

func toModelGroups(groups []*GroupParam) []*epp_pool.InstanceGroup {
	rst := []*epp_pool.InstanceGroup{}
	for _, group := range groups {
		if group == nil {
			rst = append(rst, nil)
			continue
		}
		g := &epp_pool.InstanceGroup{
			Name:      group.Name,
			Instances: []*epp_pool.InstanceParam{},
		}
		for _, inst := range group.Instances {
			if inst == nil {
				g.Instances = append(g.Instances, nil)
				continue
			}
			g.Instances = append(g.Instances, &epp_pool.InstanceParam{
				ID:   inst.ID,
				Host: inst.Host,
				Port: inst.Port,
			})
		}
		rst = append(rst, g)
	}
	return rst
}
