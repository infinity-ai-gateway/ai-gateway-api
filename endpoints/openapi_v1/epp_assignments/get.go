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

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// ListParam carries the optional cluster filter of GET /epp-assignments.
type ListParam struct {
	Cluster string `form:"cluster"`
}

// GetRoute route
var GetEndpoint = &xreq.Endpoint{
	Path:       "/epp-assignments",
	Method:     http.MethodGet,
	Handler:    xreq.Convert(GetAction),
	Authorizer: iauth.FA(iauth.FeatureBFEPool, iauth.ActionReadAll),
}

var _ xreq.Handler = GetAction

// GetAction returns the full assignment view, optionally filtered to one
// cluster, see epp-assignments.md §2.1. Filtering by a cluster that does not
// exist or is not in EPP mode resolves to a 404 semantic error.
func GetAction(req *http.Request) (interface{}, error) {
	param := &ListParam{}
	if err := xreq.BindForm(req, param); err != nil {
		return nil, err
	}

	view, err := container.EppPoolManager.GetAssignmentsView(req.Context())
	if err != nil {
		return nil, err
	}

	data := newAssignmentsData(view)
	if param.Cluster == "" {
		return data, nil
	}

	filtered := &AssignmentsData{
		Clusters:           []*ClusterAssignmentData{},
		UnassignedClusters: []string{},
		IdleGroups:         data.IdleGroups,
	}
	for _, entry := range data.Clusters {
		if entry.Cluster != param.Cluster {
			continue
		}
		filtered.Clusters = append(filtered.Clusters, entry)
		for _, unassigned := range data.UnassignedClusters {
			if unassigned == param.Cluster {
				filtered.UnassignedClusters = append(filtered.UnassignedClusters, unassigned)
			}
		}
		return filtered, nil
	}

	return nil, xerror.WrapRecordNotExist("epp assignment", param.Cluster)
}
