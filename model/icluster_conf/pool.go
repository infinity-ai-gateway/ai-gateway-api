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
//limitations under the License. All rights reserved.
// Copyright (c) 2021 The BFE Authors.
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

package icluster_conf

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
)

var (
	PoolTagBFE     int8 = 1
	PoolTagProduct int8 = 2
)

type PoolFilter struct {
	Name      *string
	IDs       []int64
	ID        *int64
	ProductID *int64
}

type PoolParam struct {
	ID        *int64
	Name      *string
	ProductID *int64
	Instances []Instance

	Tag *int8

	EPPServer *EPPServer
	Role      *string
}

type Pool struct {
	ID        int64
	Name      string
	Ready     bool
	Product   *ibasic.Product
	Instances []Instance
	Tag       int8
	Role      string
	EPPServer *EPPServer
}

type Instance struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Port    int    `json:"port"`
	Weight  int64  `json:"weight"`
	Disable bool   `json:"disable"`
}

// instanceCompat is used to decode both the new lowercase JSON format and the
// legacy PascalCase JSON stored in the database.
type instanceCompat struct {
	Name    string `json:"name"`
	Addr    string `json:"addr"`
	Port    int    `json:"port"`
	Weight  int64  `json:"weight"`
	Disable bool   `json:"disable"`

	OldName    string         `json:"Name"`
	OldAddr    string         `json:"Addr"`
	OldPort    int            `json:"Port"`
	OldPorts   map[string]int `json:"Ports,omitempty"`
	OldWeight  int64          `json:"Weight"`
	OldDisable bool           `json:"Disable"`
}

func (i *Instance) UnmarshalJSON(data []byte) error {
	var c instanceCompat
	if err := json.Unmarshal(data, &c); err != nil {
		return err
	}

	// Prefer the new lowercase fields; fallback to the legacy PascalCase fields
	// so that historical data in pools.instance_detail remains readable.
	if c.Name != "" || c.Addr != "" {
		i.Name = c.Name
		i.Addr = c.Addr
		i.Port = c.Port
		i.Weight = c.Weight
		i.Disable = c.Disable
	} else {
		i.Name = c.OldName
		i.Addr = c.OldAddr
		i.Port = c.OldPort
		i.Weight = c.OldWeight
		i.Disable = c.OldDisable
		if i.Port == 0 {
			i.Port = c.OldPorts["Default"]
		}
	}

	return nil
}

const (
	ProductPoolRoleEPP    = "EPP"
	ProductPoolRoleCommon = "COMMON"
)

type EPPServer struct {
	Domain    *string        `json:"domain"`
	Port      *int           `json:"port"`
	Endpoints []*EPPEndpoint `json:"endpoints"`
}

type EPPEndpoint struct {
	IP   *string `json:"ip"`
	Port *int    `json:"port"`
}

func (i *Instance) IPWithPort() string {
	return fmt.Sprintf("%s:%d", i.Addr, i.Port)
}

type PoolStorager interface {
	FetchPool(ctx context.Context, name string) (*Pool, error)
	FetchPools(ctx context.Context, param *PoolFilter) ([]*Pool, error)

	CreatePool(ctx context.Context, product *ibasic.Product, data *PoolParam) (*Pool, error)
	UpdatePool(ctx context.Context, oldData *Pool, diff *PoolParam) error
	DeletePool(ctx context.Context, pool *Pool) error
}

type PoolManager struct {
	storager           PoolStorager
	bfeClusterStorager ibasic.BFEClusterStorager
	subClusterStorager SubClusterStorager
	txn                itxn.TxnStorager
}

func NewPoolManager(txn itxn.TxnStorager, storager PoolStorager,
	bfeClusterStorager ibasic.BFEClusterStorager, subClusterStorager SubClusterStorager) *PoolManager {

	return &PoolManager{
		txn:                txn,
		storager:           storager,
		bfeClusterStorager: bfeClusterStorager,
		subClusterStorager: subClusterStorager,
	}
}

func (rppm *PoolManager) FetchPoolByName(ctx context.Context, name string) (one *Pool, err error) {
	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		one, err = rppm.storager.FetchPool(ctx, name)
		return err
	})

	return
}

func (rppm *PoolManager) FetchBFEPool(ctx context.Context, name string) (one *Pool, err error) {
	return rppm.FetchProductPool(ctx, ibasic.BuildinProduct, name)
}

func (rppm *PoolManager) FetchProductPool(ctx context.Context, product *ibasic.Product, name string) (one *Pool, err error) {
	name, err = poolNameJudger(product.Name, name)
	if err != nil {
		return
	}

	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		one, err = rppm.storager.FetchPool(ctx, name)
		return err
	})

	return
}

func (rppm *PoolManager) FetchBFEPools(ctx context.Context) (list []*Pool, err error) {
	return rppm.FetchProductPools(ctx, ibasic.BuildinProduct)
}

func (rppm *PoolManager) FetchProductPools(ctx context.Context, product *ibasic.Product) (list []*Pool, err error) {
	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		list, err = rppm.storager.FetchPools(ctx, &PoolFilter{
			ProductID: &product.ID,
		})
		return err
	})

	return
}

func poolNameJudger(productName string, poolName string) (realName string, err error) {
	ss := strings.SplitN(poolName, ".", 2)
	if len(ss) == 2 {
		if ss[0] != productName {
			return "", xerror.WrapParamErrorWithMsg("Pool Name Must Use Product Name as Prefix")
		}

		return poolName, nil
	}

	return productName + "." + poolName, nil
}

// CanDelete check whether pool can be deleted, Check Logic:
// 1. Not BFE Cluster Refer To
// 2. Not SubCluster Refer To
func (rppm *PoolManager) CanDelete(ctx context.Context, pool *Pool) error {
	bfeClusters, err := rppm.bfeClusterStorager.FetchBFEClusters(ctx, &ibasic.BFEClusterFilter{
		Pool: &pool.Name,
	})
	if err != nil {
		return err
	}
	if len(bfeClusters) != 0 {
		return xerror.WrapConflictErrorWithMsg("BFECluster %s Refer To This Pool", bfeClusters[0].Name)
	}

	subClusters, err := rppm.subClusterStorager.FetchSubClusterList(ctx, &SubClusterFilter{
		InstancePool: pool,
	})
	if err != nil {
		return err
	}
	if len(subClusters) != 0 {
		return xerror.WrapConflictErrorWithMsg("SubCluster %s Refer To This Pool", subClusters[0].Name)
	}

	return nil
}

func (rppm *PoolManager) DeleteBFEPool(ctx context.Context, name string) (one *Pool, err error) {
	return rppm.DeleteProductPool(ctx, ibasic.BuildinProduct, name)
}

func (rppm *PoolManager) DeleteProductPool(ctx context.Context, product *ibasic.Product, name string) (one *Pool, err error) {
	name, err = poolNameJudger(product.Name, name)
	if err != nil {
		return
	}

	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		one, err = rppm.storager.FetchPool(ctx, name)
		if err != nil {
			return err
		}

		if one == nil {
			return xerror.WrapRecordNotExist("Pool")
		}

		if err = rppm.CanDelete(ctx, one); err != nil {
			return err
		}

		return rppm.storager.DeletePool(ctx, one)
	})

	return
}

func (rppm *PoolManager) CreateBFEPool(ctx context.Context, pool *PoolParam) (one *Pool, err error) {
	pool.Tag = &PoolTagBFE
	return rppm.CreateProductPool(ctx, ibasic.BuildinProduct, pool)
}

func (rppm *PoolManager) CreateProductPool(ctx context.Context, product *ibasic.Product, pool *PoolParam) (one *Pool, err error) {
	var pN string
	pN, err = poolNameJudger(product.Name, *pool.Name)
	if err != nil {
		return
	}
	pool.Name = &pN
	if pool.Tag == nil {
		pool.Tag = &PoolTagProduct
	}

	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		old, err := rppm.storager.FetchPool(ctx, *pool.Name)
		if err != nil {
			return err
		}
		if old != nil {
			return xerror.WrapRecordExisted()
		}

		one, err = rppm.storager.CreatePool(ctx, product, pool)
		return err
	})

	return
}

func (rppm *PoolManager) UpdateBFEPool(ctx context.Context, pool *Pool, diff *PoolParam) (err error) {
	return rppm.UpdateProductPool(ctx, ibasic.BuildinProduct, pool, diff)
}

func (rppm *PoolManager) UpdateProductPool(ctx context.Context, product *ibasic.Product, pool *Pool, diff *PoolParam) (err error) {
	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		return rppm.storager.UpdatePool(ctx, pool, diff)
	})

	return
}

func PoolList2Map(list []*Pool) map[int64]*Pool {
	m := map[int64]*Pool{}
	for _, one := range list {
		m[one.ID] = one
	}

	return m
}

func (rppm *PoolManager) GetPoolByName(ctx context.Context, poolName *string) (pool *Pool, err error) {
	err = rppm.txn.AtomExecute(ctx, func(ctx context.Context) error {
		if poolName == nil || *poolName == "" {
			return xerror.WrapParamErrorWithMsg("Pool Name Illegal")
		}

		pool, err = rppm.storager.FetchPool(ctx, *poolName)
		return err
	})

	return
}
