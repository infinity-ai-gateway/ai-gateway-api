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
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
)

var errFakeConflict = errors.New("fake unique key conflict")

type fakeTxn struct{}

func (f *fakeTxn) AtomExecute(ctx context.Context, do func(context.Context) error) error {
	return do(ctx)
}

// memoryEppPoolStorager is a hand-written in-memory implementation of
// EppPoolStorager for manager tests.
type memoryEppPoolStorager struct {
	mu        sync.Mutex
	instances map[string]*InstanceParam
	// assignments keyed by cluster; nil value models a unique-key conflict on insert.
	assignments      map[string]*AssignmentParam
	upsertConflictAt int
	upsertCalls      int
}

func newMemoryEppPoolStorager() *memoryEppPoolStorager {
	return &memoryEppPoolStorager{
		instances:   map[string]*InstanceParam{},
		assignments: map[string]*AssignmentParam{},
	}
}

func (s *memoryEppPoolStorager) seedInstances(instances ...*InstanceParam) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, inst := range instances {
		s.instances[inst.ID] = inst
	}
}

func (s *memoryEppPoolStorager) seedAssignments(assignments ...*AssignmentParam) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, assignment := range assignments {
		s.assignments[assignment.Cluster] = assignment
	}
}

func (s *memoryEppPoolStorager) FetchInstanceList(ctx context.Context, filter *InstanceFilter) ([]*InstanceParam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rst := []*InstanceParam{}
	for _, inst := range s.instances {
		if filter != nil {
			if filter.ID != nil && *filter.ID != inst.ID {
				continue
			}
			if filter.GroupName != nil && *filter.GroupName != inst.GroupName {
				continue
			}
		}
		rst = append(rst, inst)
	}
	sort.Slice(rst, func(i, j int) bool { return rst[i].ID < rst[j].ID })
	return rst, nil
}

func (s *memoryEppPoolStorager) CreateInstances(ctx context.Context, params []*InstanceParam) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, param := range params {
		s.instances[param.ID] = param
	}
	return int64(len(params)), nil
}

func (s *memoryEppPoolStorager) DeleteInstances(ctx context.Context, filter *InstanceFilter) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var affected int64
	for id, inst := range s.instances {
		if filter != nil {
			if filter.ID != nil && *filter.ID != inst.ID {
				continue
			}
			if filter.GroupName != nil && *filter.GroupName != inst.GroupName {
				continue
			}
		}
		delete(s.instances, id)
		affected++
	}
	return affected, nil
}

func (s *memoryEppPoolStorager) FetchAssignment(ctx context.Context, cluster string) (*AssignmentParam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if assignment, ok := s.assignments[cluster]; ok {
		return assignment, nil
	}
	return nil, nil
}

func (s *memoryEppPoolStorager) FetchAssignmentList(ctx context.Context) ([]*AssignmentParam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rst := []*AssignmentParam{}
	for _, assignment := range s.assignments {
		rst = append(rst, assignment)
	}
	sort.Slice(rst, func(i, j int) bool { return rst[i].Cluster < rst[j].Cluster })
	return rst, nil
}

func (s *memoryEppPoolStorager) UpsertAssignment(ctx context.Context, param *AssignmentParam) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.upsertCalls++
	if s.upsertConflictAt > 0 && s.upsertCalls == s.upsertConflictAt {
		s.upsertConflictAt = 0
		return errFakeConflict
	}

	s.assignments[param.Cluster] = param
	return nil
}

func (s *memoryEppPoolStorager) DeleteAssignment(ctx context.Context, cluster string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.assignments, cluster)
	return nil
}

type fakeClusterSource struct {
	clusters []*EPPClusterInfo
	err      error
}

func (f *fakeClusterSource) FetchEPPClusters(ctx context.Context) ([]*EPPClusterInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.clusters, nil
}

type fakeVersionControlStorager struct {
	upsertFn func(ctx context.Context, css *iversion_control.ExportData) (string, error)
}

func (s *fakeVersionControlStorager) UpsertConfigLastExportedVersion(ctx context.Context, css *iversion_control.ExportData) (string, error) {
	if s.upsertFn != nil {
		return s.upsertFn(ctx, css)
	}
	return "20260906120000", nil
}

var _ iversion_control.VersionControlStorager = (*fakeVersionControlStorager)(nil)
