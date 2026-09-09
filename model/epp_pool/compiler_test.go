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
	"encoding/json"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func findProfilePlugin(t *testing.T, conf *EndpointPickerConfig, pluginRef string) *ProfilePluginConfig {
	t.Helper()
	require.Len(t, conf.SchedulingProfiles, 1)
	for _, p := range conf.SchedulingProfiles[0].Plugins {
		if p.PluginRef == pluginRef {
			return p
		}
	}
	return nil
}

func findPlugin(t *testing.T, conf *EndpointPickerConfig, name string) *PluginConfig {
	t.Helper()
	for _, p := range conf.Plugins {
		if p.Name == name {
			return p
		}
	}
	return nil
}

func TestCompileEppConfig_Profiles(t *testing.T) {
	cases := []struct {
		profile string
		kv      float64
		queue   float64
	}{
		{SchedulingProfileLatencyFirst, 0.2, 1.0},
		{SchedulingProfileBalanced, 1.0, 0.5},
		{SchedulingProfileThroughputFirst, 1.0, 0.2},
	}

	for _, tc := range cases {
		conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
			SchedulingProfile: lib.PString(tc.profile),
		})

		kv := findProfilePlugin(t, conf, pluginNameKVScorer)
		require.NotNil(t, kv)
		require.NotNil(t, kv.Weight)
		assert.Equal(t, tc.kv, *kv.Weight)

		queue := findProfilePlugin(t, conf, pluginNameQueueScorer)
		require.NotNil(t, queue)
		require.NotNil(t, queue.Weight)
		assert.Equal(t, tc.queue, *queue.Weight)
	}
}

func TestCompileEppConfig_CacheAffinityOverride(t *testing.T) {
	// Explicit cache_affinity overrides the profile weights.
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		SchedulingProfile: lib.PString(SchedulingProfileBalanced),
		CacheAffinity:     lib.PString(CacheAffinityHigh),
	})
	assert.Equal(t, 1.0, *findProfilePlugin(t, conf, pluginNameKVScorer).Weight)
	assert.Equal(t, 0.2, *findProfilePlugin(t, conf, pluginNameQueueScorer).Weight)

	// Default (no cache_affinity) follows the profile.
	conf = CompileEppConfig("cluster-a", &EppConfigSimplified{
		SchedulingProfile: lib.PString(SchedulingProfileLatencyFirst),
	})
	assert.Equal(t, 0.2, *findProfilePlugin(t, conf, pluginNameKVScorer).Weight)
	assert.Equal(t, 1.0, *findProfilePlugin(t, conf, pluginNameQueueScorer).Weight)

	// Explicit medium is an override different from any profile default.
	conf = CompileEppConfig("cluster-a", &EppConfigSimplified{
		SchedulingProfile: lib.PString(SchedulingProfileBalanced),
		CacheAffinity:     lib.PString(CacheAffinityMedium),
	})
	assert.Equal(t, 0.6, *findProfilePlugin(t, conf, pluginNameKVScorer).Weight)
	assert.Equal(t, 0.6, *findProfilePlugin(t, conf, pluginNameQueueScorer).Weight)
}

func TestCompileEppConfig_PrefixCacheAffinity(t *testing.T) {
	// Default: prefix scorer injected with weight 1.0.
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{})
	require.NotNil(t, findPlugin(t, conf, pluginNamePrefixScorer))
	prefix := findProfilePlugin(t, conf, pluginNamePrefixScorer)
	require.NotNil(t, prefix)
	assert.Equal(t, 1.0, *prefix.Weight)

	// Explicit false: not injected.
	conf = CompileEppConfig("cluster-a", &EppConfigSimplified{
		PrefixCacheAffinity: lib.PBool(false),
	})
	assert.Nil(t, findPlugin(t, conf, pluginNamePrefixScorer))
	assert.Nil(t, findProfilePlugin(t, conf, pluginNamePrefixScorer))
}

func TestCompileEppConfig_SessionAffinity(t *testing.T) {
	// Disabled by default: no session scorer.
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{})
	assert.Nil(t, findPlugin(t, conf, pluginNameSessionScorer))
	assert.Nil(t, findProfilePlugin(t, conf, pluginNameSessionScorer))

	// Enabled: injected with header source and weight 1.0.
	conf = CompileEppConfig("cluster-a", &EppConfigSimplified{
		SessionAffinityEnabled: lib.PBool(true),
		SessionAffinityHeader:  lib.PString("x-session-id"),
	})
	plugin := findPlugin(t, conf, pluginNameSessionScorer)
	require.NotNil(t, plugin)
	assert.Equal(t, pluginTypeSessionScorer, plugin.Type)
	sessionIDConfig, ok := plugin.Parameters["sessionIdConfig"].(map[string]interface{})
	require.True(t, ok)
	sources, ok := sessionIDConfig["sources"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, sources, 1)
	assert.Equal(t, "x-session-id", sources[0]["header"])

	profilePlugin := findProfilePlugin(t, conf, pluginNameSessionScorer)
	require.NotNil(t, profilePlugin)
	assert.Equal(t, 1.0, *profilePlugin.Weight)
}

func TestCompileEppConfig_FixedPlugins(t *testing.T) {
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{})

	discovery := findPlugin(t, conf, pluginNameDiscovery)
	require.NotNil(t, discovery)
	assert.Equal(t, pluginTypeDiscovery, discovery.Type)
	assert.Equal(t, "cluster-a", discovery.Parameters["clusterName"])

	utilFilter := findPlugin(t, conf, pluginNameUtilFilter)
	require.NotNil(t, utilFilter)
	assert.Equal(t, pluginTypeUtilFilter, utilFilter.Type)
	conditions, ok := utilFilter.Parameters["conditions"].([]map[string]interface{})
	require.True(t, ok)
	require.Len(t, conditions, 1)
	assert.Equal(t, metricKVCacheUtilization, conditions[0]["metric"])
	assert.Equal(t, DefaultKVCacheUtilizationMax, conditions[0]["maxValue"])

	require.NotNil(t, findPlugin(t, conf, pluginNameKVScorer))
	require.NotNil(t, findPlugin(t, conf, pluginNameQueueScorer))
	require.NotNil(t, findPlugin(t, conf, pluginNameMaxScorePicker))
	require.NotNil(t, findPlugin(t, conf, pluginNameOpenAIParser))

	require.Equal(t, pluginNameDiscovery, conf.DataLayer.Discovery.Endpoints.PluginRef)
	require.Len(t, conf.RequestHandler.Parsers, 1)
	assert.Equal(t, pluginNameOpenAIParser, conf.RequestHandler.Parsers[0].PluginRef)

	// No flow_control: no flowControl section, no feature gates.
	assert.Nil(t, conf.FlowControl)
	assert.Empty(t, conf.FeatureGates)

	// Filter precedes scorers, picker is last.
	plugins := conf.SchedulingProfiles[0].Plugins
	require.Len(t, plugins, 5)
	assert.Equal(t, pluginNameUtilFilter, plugins[0].PluginRef)
	assert.Equal(t, pluginNameMaxScorePicker, plugins[len(plugins)-1].PluginRef)
}

func TestCompileEppConfig_UtilizationMaxOverride(t *testing.T) {
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		KVCacheUtilizationMax: float64Ptr(0.5),
	})
	conditions := findPlugin(t, conf, pluginNameUtilFilter).Parameters["conditions"].([]map[string]interface{})
	assert.Equal(t, 0.5, conditions[0]["maxValue"])
}

func TestCompileEppConfig_FlowControl(t *testing.T) {
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		FlowControl: &FlowControlSimplified{
			MaxRequests:        lib.PInt(1000),
			QueueTTL:           lib.PInt(30),
			NoEndpointQueueTTL: lib.PInt(600),
			EnableEviction:     lib.PBool(true),
		},
	})

	require.NotNil(t, conf.FlowControl)
	assert.Equal(t, "1000", conf.FlowControl.MaxRequests)
	assert.Equal(t, "30s", conf.FlowControl.DefaultRequestTTL)
	assert.Equal(t, "10m0s", conf.FlowControl.NoEndpointRequestTTL)
	assert.True(t, conf.FlowControl.EnableEviction)
	assert.Equal(t, []string{featureGateFlowControl}, conf.FeatureGates)
}

func TestCompileEppConfig_FlowControlMaxRequestsOmitted(t *testing.T) {
	// Unset: omitted.
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		FlowControl: &FlowControlSimplified{QueueTTL: lib.PInt(30)},
	})
	assert.Empty(t, conf.FlowControl.MaxRequests)

	// Explicit -1 (unlimited): omitted.
	conf = CompileEppConfig("cluster-a", &EppConfigSimplified{
		FlowControl: &FlowControlSimplified{MaxRequests: lib.PInt(-1)},
	})
	assert.Empty(t, conf.FlowControl.MaxRequests)
}

func TestCompileEppConfig_FlowControlZeroTTL(t *testing.T) {
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		FlowControl: &FlowControlSimplified{QueueTTL: lib.PInt(0)},
	})
	assert.Equal(t, "0s", conf.FlowControl.DefaultRequestTTL)
}

func TestCompileEppConfig_FlowControlJSONShape(t *testing.T) {
	// enableEviction carries omitempty (apix parity): false is omitted, true
	// is emitted; maxRequests stays the string form of the apix
	// resource.Quantity.
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		FlowControl: &FlowControlSimplified{
			MaxRequests: lib.PInt(1000),
			QueueTTL:    lib.PInt(30),
		},
	})
	bs, err := json.Marshal(conf.FlowControl)
	require.NoError(t, err)
	assert.JSONEq(t, `{"maxRequests":"1000","defaultRequestTTL":"30s"}`, string(bs))
	assert.NotContains(t, string(bs), "enableEviction")
	assert.NotContains(t, string(bs), "noEndpointRequestTTL")

	conf = CompileEppConfig("cluster-a", &EppConfigSimplified{
		FlowControl: &FlowControlSimplified{EnableEviction: lib.PBool(true)},
	})
	bs, err = json.Marshal(conf.FlowControl)
	require.NoError(t, err)
	assert.Contains(t, string(bs), `"enableEviction":true`)
}

func TestCompileEppConfig_WeightTagParity(t *testing.T) {
	// apix SchedulingPlugin.Weight has no omitempty: the unweighted picker
	// serializes as "weight": null, which the EPP strict decode accepts.
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{})
	bs, err := json.Marshal(conf.SchedulingProfiles[0])
	require.NoError(t, err)

	var profile struct {
		Plugins []struct {
			PluginRef string   `json:"pluginRef"`
			Weight    *float64 `json:"weight"`
		} `json:"plugins"`
	}
	require.NoError(t, json.Unmarshal(bs, &profile))

	weighted := map[string]bool{}
	for _, p := range profile.Plugins {
		if p.Weight != nil {
			weighted[p.PluginRef] = true
		}
	}
	assert.True(t, weighted[pluginNameKVScorer])
	assert.True(t, weighted[pluginNameQueueScorer])
	assert.False(t, weighted[pluginNameUtilFilter])
	assert.False(t, weighted[pluginNameMaxScorePicker])
}

func TestCompileEppConfig_DeterministicJSON(t *testing.T) {
	conf := CompileEppConfig("cluster-a", &EppConfigSimplified{
		SchedulingProfile:      lib.PString(SchedulingProfileBalanced),
		KVCacheUtilizationMax:  float64Ptr(0.9),
		SessionAffinityEnabled: lib.PBool(true),
		SessionAffinityHeader:  lib.PString("x-session-id"),
		FlowControl: &FlowControlSimplified{
			MaxRequests:        lib.PInt(1000),
			QueueTTL:           lib.PInt(30),
			NoEndpointQueueTTL: lib.PInt(600),
		},
	})

	bs1, err := json.Marshal(conf)
	require.NoError(t, err)
	bs2, err := json.Marshal(CompileEppConfig("cluster-a", &EppConfigSimplified{
		SchedulingProfile:      lib.PString(SchedulingProfileBalanced),
		KVCacheUtilizationMax:  float64Ptr(0.9),
		SessionAffinityEnabled: lib.PBool(true),
		SessionAffinityHeader:  lib.PString("x-session-id"),
		FlowControl: &FlowControlSimplified{
			MaxRequests:        lib.PInt(1000),
			QueueTTL:           lib.PInt(30),
			NoEndpointQueueTTL: lib.PInt(600),
		},
	}))
	require.NoError(t, err)
	assert.JSONEq(t, string(bs1), string(bs2))

	// The compiled JSON must decode back into the minimal struct set.
	var roundtrip EndpointPickerConfig
	require.NoError(t, json.Unmarshal(bs1, &roundtrip))
	require.Len(t, roundtrip.SchedulingProfiles, 1)
}

func TestSecondsToDuration(t *testing.T) {
	assert.Equal(t, "0s", secondsToDuration(0))
	assert.Equal(t, "30s", secondsToDuration(30))
	assert.Equal(t, "10m0s", secondsToDuration(600))
	assert.Equal(t, "1h0m0s", secondsToDuration(3600))
}
