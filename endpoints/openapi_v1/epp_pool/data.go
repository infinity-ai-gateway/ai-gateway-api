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
	"github.com/rainway-ai-gateway/ai-gateway-api/model/epp_pool"
)

// InstanceData is one EPP instance of the pool as exposed by the OpenAPI
// (epp-pool.md data model: only id/host/port).
type InstanceData struct {
	ID   string `json:"id"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

// GroupData is one named instance group of the pool.
type GroupData struct {
	Name      string          `json:"name"`
	Instances []*InstanceData `json:"instances"`
}

// PoolData is the singleton EPP instance pool detail.
type PoolData struct {
	Name   string       `json:"name"`
	Groups []*GroupData `json:"groups"`
}

func newInstanceData(inst *epp_pool.InstanceParam) *InstanceData {
	if inst == nil {
		return nil
	}
	return &InstanceData{
		ID:   inst.ID,
		Host: inst.Host,
		Port: inst.Port,
	}
}

func newPoolData(pool *epp_pool.EppPool) *PoolData {
	rst := &PoolData{
		Name:   pool.Name,
		Groups: []*GroupData{},
	}
	for _, group := range pool.Groups {
		g := &GroupData{
			Name:      group.Name,
			Instances: []*InstanceData{},
		}
		for _, inst := range group.Instances {
			g.Instances = append(g.Instances, newInstanceData(inst))
		}
		rst.Groups = append(rst.Groups, g)
	}
	return rst
}
