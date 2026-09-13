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

package epp_assignments

import (
	"net/http"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// OverrideParam is the body of PUT /epp-assignments/{cluster}
// (epp-assignments.md §2.2). The cluster must reference an existing
// balance_mode=EPP cluster; the group and the primary instance must exist
// in the current EPP instance pool.
type OverrideParam struct {
	Cluster           string `uri:"cluster"`
	GroupName         string `json:"group_name"`
	PrimaryInstanceID string `json:"primary_instance_id"`
}

// OverrideRoute route
var OverrideEndpoint = &xreq.Endpoint{
	Path:       "/epp-assignments/{cluster}",
	Method:     http.MethodPut,
	Handler:    xreq.Convert(OverrideAction),
	Authorizer: iauth.FA(iauth.FeatureBFEPool, iauth.ActionUpdate),
}

var _ xreq.Handler = OverrideAction

// OverrideAction manually overwrites the assignment of one cluster and
// returns the updated per-cluster assignment view, see epp-assignments.md §2.2.
// A cluster that does not exist or is not in EPP mode surfaces the 404
// semantic error from the model layer.
func OverrideAction(req *http.Request) (interface{}, error) {
	param := &OverrideParam{}
	if err := xreq.Bind(req, param); err != nil {
		return nil, err
	}

	view, err := container.EppPoolManager.OverrideAssignment(req.Context(), param.Cluster, param.GroupName, param.PrimaryInstanceID)
	if err != nil {
		return nil, err
	}

	return newClusterAssignmentOverrideData(view), nil
}
