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

package epp_data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"sync"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/innerapi_v1/internal/testutil"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/epp_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeEppPoolStorager is a hand-written in-memory EppPoolStorager for
// handler tests.
type fakeEppPoolStorager struct {
	mu          sync.Mutex
	instances   map[string]*epp_pool.InstanceParam
	assignments map[string]*epp_pool.AssignmentParam
}

func newFakeEppPoolStorager() *fakeEppPoolStorager {
	return &fakeEppPoolStorager{
		instances:   map[string]*epp_pool.InstanceParam{},
		assignments: map[string]*epp_pool.AssignmentParam{},
	}
}

func (s *fakeEppPoolStorager) FetchInstanceList(ctx context.Context, filter *epp_pool.InstanceFilter) ([]*epp_pool.InstanceParam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rst := []*epp_pool.InstanceParam{}
	for _, inst := range s.instances {
		rst = append(rst, inst)
	}
	sort.Slice(rst, func(i, j int) bool { return rst[i].ID < rst[j].ID })
	return rst, nil
}

func (s *fakeEppPoolStorager) CreateInstances(ctx context.Context, params []*epp_pool.InstanceParam) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, param := range params {
		s.instances[param.ID] = param
	}
	return int64(len(params)), nil
}

func (s *fakeEppPoolStorager) DeleteInstances(ctx context.Context, filter *epp_pool.InstanceFilter) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var affected int64
	for id := range s.instances {
		delete(s.instances, id)
		affected++
	}
	return affected, nil
}

func (s *fakeEppPoolStorager) FetchAssignment(ctx context.Context, cluster string) (*epp_pool.AssignmentParam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.assignments[cluster], nil
}

func (s *fakeEppPoolStorager) FetchAssignmentList(ctx context.Context) ([]*epp_pool.AssignmentParam, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	rst := []*epp_pool.AssignmentParam{}
	for _, assignment := range s.assignments {
		rst = append(rst, assignment)
	}
	sort.Slice(rst, func(i, j int) bool { return rst[i].Cluster < rst[j].Cluster })
	return rst, nil
}

func (s *fakeEppPoolStorager) UpsertAssignment(ctx context.Context, param *epp_pool.AssignmentParam) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[param.Cluster] = param
	return nil
}

func (s *fakeEppPoolStorager) DeleteAssignment(ctx context.Context, cluster string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.assignments, cluster)
	return nil
}

type fakeClusterSource struct {
	clusters []*epp_pool.EPPClusterInfo
}

func (f *fakeClusterSource) FetchEPPClusters(ctx context.Context) ([]*epp_pool.EPPClusterInfo, error) {
	return f.clusters, nil
}

const testExportedVersion = "20260906120000"

// setupManager swaps container.EppPoolManager with one backed by in-memory
// fakes plus a fake version-control manager pinned to testExportedVersion.
func setupManager(t *testing.T) func() {
	old := container.EppPoolManager
	storager := newFakeEppPoolStorager()

	instances := []*epp_pool.InstanceParam{
		{ID: "epp-a", Host: "10.0.0.1", Port: 9002, GroupName: "g1"},
		{ID: "epp-b", Host: "10.0.0.2", Port: 9002, GroupName: "g1"},
		{ID: "epp-c", Host: "10.0.0.3", Port: 9002, GroupName: "g2"},
	}
	if _, err := storager.CreateInstances(context.Background(), instances); err != nil {
		t.Fatalf("seed instances: %v", err)
	}
	if err := storager.UpsertAssignment(context.Background(), &epp_pool.AssignmentParam{
		Cluster:           "cluster-a",
		GroupName:         "g1",
		PrimaryInstanceID: "epp-a",
	}); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}
	if err := storager.UpsertAssignment(context.Background(), &epp_pool.AssignmentParam{
		Cluster:           "cluster-b",
		GroupName:         "g2",
		PrimaryInstanceID: "epp-c",
	}); err != nil {
		t.Fatalf("seed assignment: %v", err)
	}

	clusterSource := &fakeClusterSource{clusters: []*epp_pool.EPPClusterInfo{
		{Name: "cluster-a", EppConfigJSON: `{"scheduling_profile":"balanced","kv_cache_utilization_max":0.9}`},
		{Name: "cluster-b", EppConfigJSON: `{"scheduling_profile":"latency-first"}`},
	}}

	container.EppPoolManager = epp_pool.NewEppPoolManager(
		&testutil.FakeTxn{},
		storager,
		clusterSource,
		testutil.NewVersionControlManager(testExportedVersion),
		&epp_pool.ManagerOptions{ValidationMode: epp_pool.ValidationModeTest},
	)
	return func() { container.EppPoolManager = old }
}

func TestExportAction(t *testing.T) {
	defer setupManager(t)()

	req := httptest.NewRequest(http.MethodGet, "/configs/epp_data/config?version=", nil)
	data, err := ExportAction(req)

	require.NoError(t, err)
	require.NotNil(t, data)

	conf, ok := data.(*epp_pool.ExportEppDataConfig)
	require.True(t, ok)
	assert.Equal(t, testExportedVersion, conf.Version)
	require.NotNil(t, conf.Config)

	// epp_config section: one compiled entry per EPP-mode cluster.
	require.Len(t, conf.Config.EppConfig, 2)
	assert.Contains(t, conf.Config.EppConfig, "cluster-a")
	assert.Contains(t, conf.Config.EppConfig, "cluster-b")

	// assignment section: standby is expanded at read time; a single
	// instance group exports a null standby.
	require.Len(t, conf.Config.Assignment, 2)
	entryA := conf.Config.Assignment["cluster-a"]
	require.NotNil(t, entryA)
	assert.Equal(t, "epp-a", entryA.Primary)
	require.NotNil(t, entryA.Standby)
	assert.Equal(t, "epp-b", *entryA.Standby)

	entryB := conf.Config.Assignment["cluster-b"]
	require.NotNil(t, entryB)
	assert.Equal(t, "epp-c", entryB.Primary)
	assert.Nil(t, entryB.Standby)
}

func TestExportAction_VersionNotChanged(t *testing.T) {
	defer setupManager(t)()

	req := httptest.NewRequest(http.MethodGet, "/configs/epp_data/config?version="+testExportedVersion, nil)
	data, err := ExportAction(req)

	require.NoError(t, err)
	assert.Nil(t, data)
}
