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
	"encoding/json"
	"errors"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEppDataGenerator(t *testing.T) {
	ctx := context.Background()
	store := newMemoryEppPoolStorager()
	store.seedInstances(
		inst("epp-a", "g1", "10.0.0.1", 9002),
		inst("epp-b", "g1", "10.0.0.2", 9002),
	)
	source := &fakeClusterSource{clusters: []*EPPClusterInfo{
		{Name: "cluster-a", EppConfigJSON: `{"scheduling_profile":"balanced","kv_cache_utilization_max":0.9}`},
		{Name: "cluster-b", EppConfigJSON: `{"scheduling_profile":"latency-first"}`},
		{Name: "cluster-c", EppConfigJSON: `{"prefix_cache_affinity":true}`},
	}}
	store.seedAssignments(
		&AssignmentParam{Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-a"},
		// Dangling: cluster-b unassigned in the export.
		&AssignmentParam{Cluster: "cluster-b", GroupName: "g9", PrimaryInstanceID: "epp-x"},
	)

	m := testManager(store, source)

	exportData, err := m.EppDataGenerator(ctx)
	require.NoError(t, err)
	assert.Equal(t, ConfigTopicEppData, exportData.Topic)

	conf, ok := exportData.DataWithoutVersion.(*ExportEppDataConfig)
	require.True(t, ok)

	// epp_config section: every EPP cluster compiled.
	require.Len(t, conf.Config.EppConfig, 3)
	compiledA := conf.Config.EppConfig["cluster-a"]
	require.NotNil(t, compiledA)
	assert.Equal(t, "cluster-a", compiledA.Plugins[0].Parameters["clusterName"])
	compiledB := conf.Config.EppConfig["cluster-b"]
	require.NotNil(t, compiledB)
	require.NotNil(t, findProfilePlugin(t, compiledB, pluginNameQueueScorer))
	assert.Equal(t, 1.0, *findProfilePlugin(t, compiledB, pluginNameQueueScorer).Weight)

	// assignment section: only validly assigned clusters.
	require.Len(t, conf.Config.Assignment, 1)
	entry := conf.Config.Assignment["cluster-a"]
	require.NotNil(t, entry)
	assert.Equal(t, "epp-a", entry.Primary)
	require.NotNil(t, entry.Standby)
	assert.Equal(t, "epp-b", *entry.Standby)
	_, hasB := conf.Config.Assignment["cluster-b"]
	assert.False(t, hasB)

	// The whole payload marshals to JSON for the InnerAPI response.
	_, err = json.Marshal(conf)
	require.NoError(t, err)
}

func TestEppDataGenerator_SingleInstanceStandbyNull(t *testing.T) {
	ctx := context.Background()
	store := newMemoryEppPoolStorager()
	store.seedInstances(inst("epp-a", "g1", "10.0.0.1", 9002))
	source := &fakeClusterSource{clusters: []*EPPClusterInfo{
		{Name: "cluster-a", EppConfigJSON: `{}`},
	}}
	store.seedAssignments(&AssignmentParam{Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-a"})

	m := NewEppPoolManager(&fakeTxn{}, store, source, nil, &ManagerOptions{ValidationMode: ValidationModeTest})

	exportData, err := m.EppDataGenerator(ctx)
	require.NoError(t, err)
	conf := exportData.DataWithoutVersion.(*ExportEppDataConfig)

	entry := conf.Config.Assignment["cluster-a"]
	require.NotNil(t, entry)
	assert.Equal(t, "epp-a", entry.Primary)
	assert.Nil(t, entry.Standby)
}

func TestEppDataGenerator_Errors(t *testing.T) {
	ctx := context.Background()

	t.Run("nil cluster source", func(t *testing.T) {
		m := testManager(newMemoryEppPoolStorager(), nil)
		_, err := m.EppDataGenerator(ctx)
		require.Error(t, err)
	})

	t.Run("cluster without epp_config", func(t *testing.T) {
		source := &fakeClusterSource{clusters: []*EPPClusterInfo{{Name: "cluster-a"}}}
		m := testManager(newMemoryEppPoolStorager(), source)
		_, err := m.EppDataGenerator(ctx)
		require.Error(t, err)
	})

	t.Run("invalid stored epp_config", func(t *testing.T) {
		source := &fakeClusterSource{clusters: []*EPPClusterInfo{{Name: "cluster-a", EppConfigJSON: `{"scheduling_profile":"nope"}`}}}
		m := testManager(newMemoryEppPoolStorager(), source)
		_, err := m.EppDataGenerator(ctx)
		require.NoError(t, err) // parse ok; field validation happens at write time
	})
}

func TestExportEppData_VersionSemantics(t *testing.T) {
	ctx := context.Background()
	store := newMemoryEppPoolStorager()
	store.seedInstances(
		inst("epp-a", "g1", "10.0.0.1", 9002),
		inst("epp-b", "g1", "10.0.0.2", 9002),
	)
	source := &fakeClusterSource{clusters: []*EPPClusterInfo{
		{Name: "cluster-a", EppConfigJSON: `{"scheduling_profile":"balanced"}`},
	}}
	store.seedAssignments(&AssignmentParam{Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-a"})

	vcStore := &fakeVersionControlStorager{}
	vc := iversion_control.NewVersionControllerManager(&fakeTxn{}, vcStore)
	m := NewEppPoolManager(&fakeTxn{}, store, source, vc, &ManagerOptions{})

	t.Run("new version returned", func(t *testing.T) {
		conf, err := m.ExportEppData(ctx, iversion_control.ZeroVersion)
		require.NoError(t, err)
		require.NotNil(t, conf)
		assert.Equal(t, "20260906120000", conf.Version)
		require.NotNil(t, conf.Config)
		assert.Contains(t, conf.Config.EppConfig, "cluster-a")
	})

	t.Run("unchanged version returns nil", func(t *testing.T) {
		conf, err := m.ExportEppData(ctx, "20260906120000")
		require.NoError(t, err)
		assert.Nil(t, conf)
	})

	t.Run("nil version control manager", func(t *testing.T) {
		m := testManager(store, source)
		_, err := m.ExportEppData(ctx, "")
		require.Error(t, err)
	})
}

func TestOverrideAssignment_TriggersExport(t *testing.T) {
	ctx := context.Background()
	store := newMemoryEppPoolStorager()
	store.seedInstances(
		inst("epp-a", "g1", "10.0.0.1", 9002),
		inst("epp-b", "g1", "10.0.0.2", 9002),
	)
	source := &fakeClusterSource{clusters: []*EPPClusterInfo{
		{Name: "cluster-a", EppConfigJSON: `{"scheduling_profile":"balanced"}`},
	}}

	var exported []*iversion_control.ExportData
	vcStore := &fakeVersionControlStorager{
		upsertFn: func(ctx context.Context, css *iversion_control.ExportData) (string, error) {
			exported = append(exported, css)
			return "20260906120001", nil
		},
	}
	vc := iversion_control.NewVersionControllerManager(&fakeTxn{}, vcStore)
	m := NewEppPoolManager(&fakeTxn{}, store, source, vc, &ManagerOptions{})

	_, err := m.OverrideAssignment(ctx, "cluster-a", "g1", "epp-b")
	require.NoError(t, err)
	require.NotEmpty(t, exported)
	assert.Equal(t, ConfigTopicEppData, exported[0].Topic)
}

func TestEppDataGenerator_ClusterSourceError(t *testing.T) {
	m := testManager(newMemoryEppPoolStorager(), &fakeClusterSource{err: errors.New("boom")})
	_, err := m.EppDataGenerator(context.Background())
	require.Error(t, err)
}

func TestCompileIntegration_FromStoredJSON(t *testing.T) {
	// End-to-end: stored raw JSON -> parse -> validate -> compile -> marshal.
	raw := `{
		"scheduling_profile": "throughput-first",
		"cache_affinity": "high",
		"session_affinity_enabled": true,
		"session_affinity_header": "x-session-id",
		"kv_cache_utilization_max": 0.85,
		"flow_control": {"max_requests": -1, "queue_ttl": 45, "no_endpoint_queue_ttl": 0, "enable_eviction": true}
	}`
	conf, err := ParseEppConfig(raw)
	require.NoError(t, err)
	require.NoError(t, conf.Validate())

	compiled := CompileEppConfig("llm-cluster-a", conf)
	bs, err := json.Marshal(compiled)
	require.NoError(t, err)

	var decoded map[string]interface{}
	require.NoError(t, json.Unmarshal(bs, &decoded))

	assert.Equal(t, []interface{}{featureGateFlowControl}, decoded["featureGates"])
	flowControl := decoded["flowControl"].(map[string]interface{})
	assert.NotContains(t, flowControl, "maxRequests") // -1 not generated
	assert.Equal(t, "45s", flowControl["defaultRequestTTL"])
	assert.Equal(t, "0s", flowControl["noEndpointRequestTTL"])
	assert.Equal(t, true, flowControl["enableEviction"])

	assert.Equal(t, "llm-cluster-a", decoded["plugins"].([]interface{})[0].(map[string]interface{})["parameters"].(map[string]interface{})["clusterName"])
}
