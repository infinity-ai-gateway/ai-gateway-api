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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func inst(id, group, host string, port int) *InstanceParam {
	return &InstanceParam{ID: id, Host: host, Port: port, GroupName: group}
}

func makePool(groups ...*InstanceGroup) *EppPool {
	return &EppPool{Name: "EPP.pool", Groups: groups}
}

func TestAllocateCluster_Deterministic(t *testing.T) {
	pool := makePool(
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002), inst("epp-b", "g1", "10.0.0.2", 9002)}},
		&InstanceGroup{Name: "g2", Instances: []*InstanceParam{inst("epp-c", "g2", "10.0.0.3", 9002), inst("epp-d", "g2", "10.0.0.4", 9002)}},
	)

	first, err := allocateCluster(pool, nil, "cluster-a", ValidationModeProduction)
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		got, err := allocateCluster(pool, nil, "cluster-a", ValidationModeProduction)
		require.NoError(t, err)
		assert.Equal(t, first, got)
	}
	// Equal load: group name tie-break; instance id tie-break.
	assert.Equal(t, "g1", first.GroupName)
	assert.Equal(t, "epp-a", first.PrimaryInstanceID)
}

func TestAllocateCluster_GroupLoadBalancing(t *testing.T) {
	pool := makePool(
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002), inst("epp-b", "g1", "10.0.0.2", 9002)}},
		&InstanceGroup{Name: "g2", Instances: []*InstanceParam{inst("epp-c", "g2", "10.0.0.3", 9002), inst("epp-d", "g2", "10.0.0.4", 9002)}},
	)

	// g1 already carries two primaries -> the next cluster goes to g2.
	assignments := []*AssignmentParam{
		{Cluster: "cluster-x", GroupName: "g1", PrimaryInstanceID: "epp-a"},
		{Cluster: "cluster-y", GroupName: "g1", PrimaryInstanceID: "epp-b"},
	}
	got, err := allocateCluster(pool, assignments, "cluster-z", ValidationModeProduction)
	require.NoError(t, err)
	assert.Equal(t, "g2", got.GroupName)

	// Equal total load: group name tie-break even when ids would differ.
	assignments = []*AssignmentParam{
		{Cluster: "cluster-x", GroupName: "g2", PrimaryInstanceID: "epp-c"},
	}
	got, err = allocateCluster(pool, assignments, "cluster-z", ValidationModeProduction)
	require.NoError(t, err)
	// g1 load = 0, g2 load = 1 -> g1.
	assert.Equal(t, "g1", got.GroupName)
}

func TestAllocateCluster_PrimaryLoadBalancing(t *testing.T) {
	pool := makePool(
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002), inst("epp-b", "g1", "10.0.0.2", 9002)}},
	)

	// epp-a already primary elsewhere -> epp-b picked within the same group.
	assignments := []*AssignmentParam{
		{Cluster: "cluster-x", GroupName: "g2", PrimaryInstanceID: "epp-a"},
	}
	got, err := allocateCluster(pool, assignments, "cluster-z", ValidationModeProduction)
	require.NoError(t, err)
	assert.Equal(t, "g1", got.GroupName)
	assert.Equal(t, "epp-b", got.PrimaryInstanceID)
}

func TestAllocateCluster_CandidateFilter(t *testing.T) {
	pool := makePool(
		// Undersized for production mode.
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002)}},
		// Valid production group.
		&InstanceGroup{Name: "g2", Instances: []*InstanceParam{inst("epp-c", "g2", "10.0.0.3", 9002), inst("epp-d", "g2", "10.0.0.4", 9002)}},
	)

	got, err := allocateCluster(pool, nil, "cluster-a", ValidationModeProduction)
	require.NoError(t, err)
	assert.Equal(t, "g2", got.GroupName)

	// Test mode accepts single instance groups (smallest name first).
	got, err = allocateCluster(pool, nil, "cluster-a", ValidationModeTest)
	require.NoError(t, err)
	assert.Equal(t, "g1", got.GroupName)
	assert.Equal(t, "epp-a", got.PrimaryInstanceID)

	// No candidates at all.
	empty := makePool(&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002)}})
	_, err = allocateCluster(empty, nil, "cluster-a", ValidationModeProduction)
	require.Error(t, err)
}

func TestRepairOne(t *testing.T) {
	pool := makePool(
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002), inst("epp-b", "g1", "10.0.0.2", 9002)}},
		&InstanceGroup{Name: "g2", Instances: []*InstanceParam{inst("epp-c", "g2", "10.0.0.3", 9002), inst("epp-d", "g2", "10.0.0.4", 9002)}},
	)

	t.Run("valid assignment kept", func(t *testing.T) {
		assignment := &AssignmentParam{Cluster: "c1", GroupName: "g1", PrimaryInstanceID: "epp-a"}
		_, action := repairOne(pool, []*AssignmentParam{assignment}, assignment, ValidationModeProduction)
		assert.Equal(t, repairKeep, action)
	})

	t.Run("same group reselect when primary removed", func(t *testing.T) {
		assignment := &AssignmentParam{Cluster: "c1", GroupName: "g1", PrimaryInstanceID: "epp-x"}
		repaired, action := repairOne(pool, []*AssignmentParam{assignment}, assignment, ValidationModeProduction)
		require.Equal(t, repairReselectSameGroup, action)
		assert.Equal(t, "g1", repaired.GroupName)
		assert.Equal(t, "epp-a", repaired.PrimaryInstanceID)
	})

	t.Run("cross group reallocate when group gone", func(t *testing.T) {
		assignment := &AssignmentParam{Cluster: "c1", GroupName: "g9", PrimaryInstanceID: "epp-x"}
		repaired, action := repairOne(pool, []*AssignmentParam{assignment}, assignment, ValidationModeProduction)
		require.Equal(t, repairReallocate, action)
		assert.Equal(t, "g1", repaired.GroupName)
		assert.Equal(t, "epp-a", repaired.PrimaryInstanceID)
	})

	t.Run("clear when no candidate groups", func(t *testing.T) {
		empty := makePool(&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002)}})
		assignment := &AssignmentParam{Cluster: "c1", GroupName: "g9", PrimaryInstanceID: "epp-x"}
		_, action := repairOne(empty, []*AssignmentParam{assignment}, assignment, ValidationModeProduction)
		assert.Equal(t, repairClear, action)
	})

	t.Run("cross group when group undersized", func(t *testing.T) {
		poolUnder := makePool(
			&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002)}},
			&InstanceGroup{Name: "g2", Instances: []*InstanceParam{inst("epp-c", "g2", "10.0.0.3", 9002), inst("epp-d", "g2", "10.0.0.4", 9002)}},
		)
		assignment := &AssignmentParam{Cluster: "c1", GroupName: "g1", PrimaryInstanceID: "epp-a"}
		repaired, action := repairOne(poolUnder, []*AssignmentParam{assignment}, assignment, ValidationModeProduction)
		require.Equal(t, repairReallocate, action)
		assert.Equal(t, "g2", repaired.GroupName)
	})
}

func TestBuildAssignmentsView(t *testing.T) {
	pool := makePool(
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002), inst("epp-b", "g1", "10.0.0.2", 9002)}},
		&InstanceGroup{Name: "g2", Instances: []*InstanceParam{inst("epp-c", "g2", "10.0.0.3", 9002), inst("epp-d", "g2", "10.0.0.4", 9002)}},
		&InstanceGroup{Name: "g3", Instances: []*InstanceParam{inst("epp-e", "g3", "10.0.0.5", 9002), inst("epp-f", "g3", "10.0.0.6", 9002)}},
	)

	eppClusters := []*EPPClusterInfo{
		{Name: "cluster-a"},
		{Name: "cluster-b"},
		{Name: "cluster-c"},
		{Name: "cluster-d"},
	}
	assignments := []*AssignmentParam{
		{Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-a"},
		{Cluster: "cluster-b", GroupName: "g1", PrimaryInstanceID: "epp-b"},
		{Cluster: "cluster-c", GroupName: "g9", PrimaryInstanceID: "epp-x"},
	}

	view := buildAssignmentsView(pool, eppClusters, assignments)
	require.Len(t, view.Clusters, 4)

	assert.Equal(t, "g1", *view.Clusters[0].Group)
	assert.Equal(t, "epp-a", view.Clusters[0].Primary.ID)
	require.NotNil(t, view.Clusters[0].Standby)
	assert.Equal(t, "epp-b", view.Clusters[0].Standby.ID)
	assert.False(t, view.Clusters[0].Degraded)

	// Mutual primary/standby across clusters of the same group is legal.
	assert.Equal(t, "epp-b", view.Clusters[1].Primary.ID)
	assert.Equal(t, "epp-a", view.Clusters[1].Standby.ID)

	// Dangling assignment: degraded, counted as unassigned.
	assert.True(t, view.Clusters[2].Degraded)
	assert.Nil(t, view.Clusters[2].Primary)
	assert.Nil(t, view.Clusters[2].Group)

	// No assignment at all.
	assert.Nil(t, view.Clusters[3].Primary)
	assert.False(t, view.Clusters[3].Degraded)

	assert.Equal(t, []string{"cluster-c", "cluster-d"}, view.UnassignedClusters)
	// g2 and g3 carry no primary.
	assert.Equal(t, []string{"g2", "g3"}, view.IdleGroups)
}

func TestBuildAssignmentsView_SingleInstanceGroup(t *testing.T) {
	pool := makePool(
		&InstanceGroup{Name: "g1", Instances: []*InstanceParam{inst("epp-a", "g1", "10.0.0.1", 9002)}},
	)
	eppClusters := []*EPPClusterInfo{{Name: "cluster-a"}}
	assignments := []*AssignmentParam{{Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-a"}}

	view := buildAssignmentsView(pool, eppClusters, assignments)
	require.Len(t, view.Clusters, 1)
	assert.Equal(t, "epp-a", view.Clusters[0].Primary.ID)
	assert.Nil(t, view.Clusters[0].Standby)
	assert.Empty(t, view.UnassignedClusters)
}

func TestEppPoolManager_GetAssignmentEndpoints(t *testing.T) {
	ctx := context.Background()

	newManager := func(store *memoryEppPoolStorager) *EppPoolManager {
		return NewEppPoolManager(&fakeTxn{}, store, nil, nil, &ManagerOptions{ValidationMode: ValidationModeTest})
	}

	t.Run("ordered primary standby joined by net.JoinHostPort", func(t *testing.T) {
		store := newMemoryEppPoolStorager()
		store.seedInstances(
			&InstanceParam{ID: "epp-a", Host: "10.0.0.1", Port: 9002, GroupName: "g1"},
			&InstanceParam{ID: "epp-b", Host: "2001:db8::10", Port: 9002, GroupName: "g1"},
		)
		store.seedAssignments(&AssignmentParam{Cluster: "c1", GroupName: "g1", PrimaryInstanceID: "epp-b"})

		endpoints, found, err := newManager(store).GetAssignmentEndpoints(ctx, "c1")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, []string{"[2001:db8::10]:9002", "10.0.0.1:9002"}, endpoints)
	})

	t.Run("single instance group yields primary only", func(t *testing.T) {
		store := newMemoryEppPoolStorager()
		store.seedInstances(
			&InstanceParam{ID: "epp-a", Host: "10.0.0.1", Port: 9002, GroupName: "g1"},
		)
		store.seedAssignments(&AssignmentParam{Cluster: "c1", GroupName: "g1", PrimaryInstanceID: "epp-a"})

		endpoints, found, err := newManager(store).GetAssignmentEndpoints(ctx, "c1")
		require.NoError(t, err)
		require.True(t, found)
		assert.Equal(t, []string{"10.0.0.1:9002"}, endpoints)
	})

	t.Run("no assignment record", func(t *testing.T) {
		store := newMemoryEppPoolStorager()
		store.seedInstances(
			&InstanceParam{ID: "epp-a", Host: "10.0.0.1", Port: 9002, GroupName: "g1"},
		)

		_, found, err := newManager(store).GetAssignmentEndpoints(ctx, "c1")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("dangling assignment", func(t *testing.T) {
		store := newMemoryEppPoolStorager()
		store.seedInstances(
			&InstanceParam{ID: "epp-a", Host: "10.0.0.1", Port: 9002, GroupName: "g1"},
		)
		store.seedAssignments(&AssignmentParam{Cluster: "c1", GroupName: "g1", PrimaryInstanceID: "epp-removed"})

		_, found, err := newManager(store).GetAssignmentEndpoints(ctx, "c1")
		require.NoError(t, err)
		assert.False(t, found)
	})

	t.Run("empty cluster name", func(t *testing.T) {
		store := newMemoryEppPoolStorager()
		_, _, err := newManager(store).GetAssignmentEndpoints(ctx, "")
		require.Error(t, err)
	})
}
