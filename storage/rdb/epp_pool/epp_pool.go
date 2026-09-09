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

// Package epp_pool implements model/epp_pool.EppPoolStorager on top of the
// relational DAO layer (MySQL/SQLite).
package epp_pool

import (
	"context"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/epp_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao"
)

// EppPoolStorager implements epp_pool.EppPoolStorager using RDB.
type EppPoolStorager struct {
	dbCtxFactory lib.DBContextFactory
}

var _ epp_pool.EppPoolStorager = &EppPoolStorager{}

// NewEppPoolStorager creates a new RDB-backed epp pool storager.
func NewEppPoolStorager(dbCtxFactory lib.DBContextFactory) *EppPoolStorager {
	return &EppPoolStorager{
		dbCtxFactory: dbCtxFactory,
	}
}

func (s *EppPoolStorager) FetchInstanceList(ctx context.Context, filter *epp_pool.InstanceFilter) ([]*epp_pool.InstanceParam, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	orderBy := "group_name, id"
	list, err := dao.TEppInstanceList(dbCtx, &dao.TEppInstanceParam{
		ID:        instanceFilterID(filter),
		GroupName: instanceFilterGroupName(filter),
		OrderBy:   &orderBy,
	})
	if err != nil {
		return nil, err
	}

	rst := make([]*epp_pool.InstanceParam, 0, len(list))
	for _, one := range list {
		rst = append(rst, eppInstanceToParam(one))
	}
	return rst, nil
}

func (s *EppPoolStorager) CreateInstances(ctx context.Context, params []*epp_pool.InstanceParam) (int64, error) {
	if len(params) == 0 {
		return 0, nil
	}

	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return 0, err
	}

	data := make([]*dao.TEppInstanceParam, 0, len(params))
	for _, param := range params {
		data = append(data, eppInstanceToDAO(param))
	}

	return dao.TEppInstanceCreate(dbCtx, data...)
}

func (s *EppPoolStorager) DeleteInstances(ctx context.Context, filter *epp_pool.InstanceFilter) (int64, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return 0, err
	}

	return dao.TEppInstanceDelete(dbCtx, &dao.TEppInstanceParam{
		ID:        instanceFilterID(filter),
		GroupName: instanceFilterGroupName(filter),
	})
}

func (s *EppPoolStorager) FetchAssignment(ctx context.Context, cluster string) (*epp_pool.AssignmentParam, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	one, err := dao.TEppAssignmentOne(dbCtx, &dao.TEppAssignmentParam{Cluster: &cluster})
	if err != nil {
		return nil, err
	}
	if one == nil {
		return nil, nil
	}

	return eppAssignmentToParam(one), nil
}

func (s *EppPoolStorager) FetchAssignmentList(ctx context.Context) ([]*epp_pool.AssignmentParam, error) {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return nil, err
	}

	orderBy := "cluster"
	list, err := dao.TEppAssignmentList(dbCtx, &dao.TEppAssignmentParam{OrderBy: &orderBy})
	if err != nil {
		return nil, err
	}

	rst := make([]*epp_pool.AssignmentParam, 0, len(list))
	for _, one := range list {
		rst = append(rst, eppAssignmentToParam(one))
	}
	return rst, nil
}

// UpsertAssignment inserts or replaces the assignment of one cluster:
// update first, insert when no row exists; a unique-key conflict (concurrent
// insert) is resolved by one retry of the update path.
func (s *EppPoolStorager) UpsertAssignment(ctx context.Context, param *epp_pool.AssignmentParam) error {
	if param == nil || param.Cluster == "" {
		return xerror.WrapParamErrorWithMsg("epp assignment cluster is empty")
	}

	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return err
	}

	val := &dao.TEppAssignmentParam{
		GroupName:         &param.GroupName,
		PrimaryInstanceID: &param.PrimaryInstanceID,
	}
	where := &dao.TEppAssignmentParam{Cluster: &param.Cluster}

	affected, err := dao.TEppAssignmentUpdate(dbCtx, val, where)
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}

	_, err = dao.TEppAssignmentCreate(dbCtx, &dao.TEppAssignmentParam{
		Cluster:           &param.Cluster,
		GroupName:         &param.GroupName,
		PrimaryInstanceID: &param.PrimaryInstanceID,
	})
	if err != nil {
		// Unique-key conflict fallback: the row appeared concurrently, update it.
		if _, errUp := dao.TEppAssignmentUpdate(dbCtx, val, where); errUp == nil {
			return nil
		}
		return err
	}

	return nil
}

func (s *EppPoolStorager) DeleteAssignment(ctx context.Context, cluster string) error {
	dbCtx, err := s.dbCtxFactory(ctx)
	if err != nil {
		return err
	}

	_, err = dao.TEppAssignmentDelete(dbCtx, &dao.TEppAssignmentParam{Cluster: &cluster})
	return err
}

func instanceFilterID(filter *epp_pool.InstanceFilter) *string {
	if filter == nil {
		return nil
	}
	return filter.ID
}

func instanceFilterGroupName(filter *epp_pool.InstanceFilter) *string {
	if filter == nil {
		return nil
	}
	return filter.GroupName
}

func eppInstanceToParam(one *dao.TEppInstance) *epp_pool.InstanceParam {
	return &epp_pool.InstanceParam{
		ID:        one.ID,
		Host:      one.Host,
		Port:      one.Port,
		GroupName: one.GroupName,
	}
}

func eppInstanceToDAO(param *epp_pool.InstanceParam) *dao.TEppInstanceParam {
	return &dao.TEppInstanceParam{
		ID:        &param.ID,
		Host:      &param.Host,
		Port:      &param.Port,
		GroupName: &param.GroupName,
	}
}

func eppAssignmentToParam(one *dao.TEppAssignment) *epp_pool.AssignmentParam {
	return &epp_pool.AssignmentParam{
		Cluster:           one.Cluster,
		GroupName:         one.GroupName,
		PrimaryInstanceID: one.PrimaryInstanceID,
	}
}
