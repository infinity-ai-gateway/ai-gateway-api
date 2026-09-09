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
	"strconv"
	"time"
)

// Minimal struct set mirroring the llm-d apix v1alpha1 EndpointPickerConfig
// JSON shape (no llm-d dependency). Field names and json tags follow
// ai-gateway-epp/docs/zh_cn/configuration/EPP配置定义说明-epp_config.md.
//
// The full example:
//
//	{
//	  "featureGates": ["flowControl"],
//	  "plugins": [
//	    {"name": "ep-discover", "type": "cluster-table-discovery", "parameters": {"clusterName": "<cluster>"}},
//	    {"name": "util-filter", "type": "utilization-filter", "parameters": {"conditions": [{"metric": "kv-cache-utilization", "maxValue": 0.9}]}},
//	    {"name": "kv-scorer", "type": "kv-cache-utilization-scorer", "parameters": {}},
//	    {"name": "queue-scorer", "type": "queue-scorer", "parameters": {}},
//	    {"name": "prefix-scorer", "type": "prefix-cache-scorer", "parameters": {}},
//	    {"name": "session-scorer", "type": "session-affinity-scorer", "parameters": {"sessionIdConfig": {"sources": [{"header": "x-session-id"}]}}},
//	    {"name": "max-score", "type": "max-score-picker", "parameters": {}},
//	    {"name": "openai-parser", "type": "openai-parser", "parameters": {}}
//	  ],
//	  "schedulingProfiles": [
//	    {"name": "default", "plugins": [
//	      {"pluginRef": "util-filter"},
//	      {"pluginRef": "kv-scorer", "weight": 1.0},
//	      {"pluginRef": "queue-scorer", "weight": 0.5},
//	      {"pluginRef": "prefix-scorer", "weight": 1.0},
//	      {"pluginRef": "session-scorer", "weight": 1.0},
//	      {"pluginRef": "max-score"}
//	    ]}
//	  ],
//	  "dataLayer": {"discovery": {"endpoints": {"pluginRef": "ep-discover"}}},
//	  "flowControl": {"maxRequests": "1000", "defaultRequestTTL": "30s", "noEndpointRequestTTL": "10m0s"},
//	  "requestHandler": {"parsers": [{"pluginRef": "openai-parser"}]}
//	}
//
// JSON tags follow ai-gateway-epp/docs/zh_cn/configuration/EPP配置定义说明-epp_config.md
// and the llm-d apix v1alpha1 EndpointPickerConfig tags exactly
// (llm-d-router/apix/config/v1alpha1/endpointpickerconfig_types.go): maxRequests
// is a Kubernetes resource.Quantity in apix, whose canonical JSON form is the
// string kept here (e.g. "1000"); enableEviction is omitted when false
// (omitempty, as in apix); weight carries no omitempty in apix, so an
// unweighted profile plugin serializes as "weight": null.

// EndpointPickerConfig is the compiled per-cluster EPP scheduling config.
type EndpointPickerConfig struct {
	FeatureGates       []string                   `json:"featureGates,omitempty"`
	Plugins            []*PluginConfig            `json:"plugins"`
	SchedulingProfiles []*SchedulingProfileConfig `json:"schedulingProfiles"`
	DataLayer          *DataLayerConfig           `json:"dataLayer"`
	FlowControl        *FlowControlConfig         `json:"flowControl,omitempty"`
	RequestHandler     *RequestHandlerConfig      `json:"requestHandler,omitempty"`
}

// PluginConfig declares one plugin instance.
type PluginConfig struct {
	Name       string                 `json:"name"`
	Type       string                 `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

// SchedulingProfileConfig declares one scheduling profile.
type SchedulingProfileConfig struct {
	Name    string                 `json:"name"`
	Plugins []*ProfilePluginConfig `json:"plugins"`
}

// ProfilePluginConfig references a plugin instance inside a profile.
// Weight mirrors the apix SchedulingPlugin tag: no omitempty, so an
// unweighted plugin serializes as "weight": null (accepted by the EPP
// strict decode).
type ProfilePluginConfig struct {
	PluginRef string   `json:"pluginRef"`
	Weight    *float64 `json:"weight"`
}

// DataLayerConfig declares the data layer (endpoint discovery).
type DataLayerConfig struct {
	Discovery *DiscoveryConfig `json:"discovery,omitempty"`
}

// DiscoveryConfig declares the endpoint discovery plugin reference.
type DiscoveryConfig struct {
	Endpoints *PluginRefConfig `json:"endpoints,omitempty"`
}

// PluginRefConfig is a bare plugin reference.
type PluginRefConfig struct {
	PluginRef string `json:"pluginRef"`
}

// FlowControlConfig is the compiled flow-control section. MaxRequests is a
// string because the apix FlowControlConfig types it as resource.Quantity,
// whose canonical JSON form is a string ("1000").
type FlowControlConfig struct {
	MaxRequests          string `json:"maxRequests,omitempty"`
	DefaultRequestTTL    string `json:"defaultRequestTTL,omitempty"`
	NoEndpointRequestTTL string `json:"noEndpointRequestTTL,omitempty"`
	EnableEviction       bool   `json:"enableEviction,omitempty"`
}

// RequestHandlerConfig declares request parsers.
type RequestHandlerConfig struct {
	Parsers []*PluginRefConfig `json:"parsers,omitempty"`
}

// Fixed plugin instance names of the compile template.
const (
	pluginNameDiscovery      = "ep-discover"
	pluginNameUtilFilter     = "util-filter"
	pluginNameKVScorer       = "kv-scorer"
	pluginNameQueueScorer    = "queue-scorer"
	pluginNamePrefixScorer   = "prefix-scorer"
	pluginNameSessionScorer  = "session-scorer"
	pluginNameMaxScorePicker = "max-score"
	pluginNameOpenAIParser   = "openai-parser"
)

// Fixed plugin types of the compile template.
const (
	pluginTypeDiscovery      = "cluster-table-discovery"
	pluginTypeUtilFilter     = "utilization-filter"
	pluginTypeKVScorer       = "kv-cache-utilization-scorer"
	pluginTypeQueueScorer    = "queue-scorer"
	pluginTypePrefixScorer   = "prefix-cache-scorer"
	pluginTypeSessionScorer  = "session-affinity-scorer"
	pluginTypeMaxScorePicker = "max-score-picker"
	pluginTypeOpenAIParser   = "openai-parser"
)

const metricKVCacheUtilization = "kv-cache-utilization"

const featureGateFlowControl = "flowControl"

// scorerWeights maps scheduling profiles / cache affinities to (kv, queue) weights.
var scorerWeights = map[string][2]float64{
	SchedulingProfileLatencyFirst:    {0.2, 1.0},
	SchedulingProfileBalanced:        {1.0, 0.5},
	SchedulingProfileThroughputFirst: {1.0, 0.2},

	CacheAffinityLow:    {0.2, 1.0},
	CacheAffinityMedium: {0.6, 0.6},
	CacheAffinityHigh:   {1.0, 0.2},
}

// CompileEppConfig deterministically compiles the simplified epp_config of
// one cluster into the full EndpointPickerConfig (api-changes.md §3.2.1).
// The result is always structurally valid; defaults are applied for unset fields.
func CompileEppConfig(clusterName string, conf *EppConfigSimplified) *EndpointPickerConfig {
	kvWeight, queueWeight := compileScorerWeights(conf)

	plugins := []*PluginConfig{
		{
			Name: pluginNameDiscovery,
			Type: pluginTypeDiscovery,
			Parameters: map[string]interface{}{
				"clusterName": clusterName,
			},
		},
		{
			Name: pluginNameUtilFilter,
			Type: pluginTypeUtilFilter,
			Parameters: map[string]interface{}{
				"conditions": []map[string]interface{}{
					{
						"metric":   metricKVCacheUtilization,
						"maxValue": conf.EffectiveKVCacheUtilizationMax(),
					},
				},
			},
		},
		{
			Name:       pluginNameKVScorer,
			Type:       pluginTypeKVScorer,
			Parameters: map[string]interface{}{},
		},
		{
			Name:       pluginNameQueueScorer,
			Type:       pluginTypeQueueScorer,
			Parameters: map[string]interface{}{},
		},
	}

	profilePlugins := []*ProfilePluginConfig{
		{PluginRef: pluginNameUtilFilter},
		{PluginRef: pluginNameKVScorer, Weight: float64Ptr(kvWeight)},
		{PluginRef: pluginNameQueueScorer, Weight: float64Ptr(queueWeight)},
	}

	if conf.EffectivePrefixCacheAffinity() {
		plugins = append(plugins, &PluginConfig{
			Name:       pluginNamePrefixScorer,
			Type:       pluginTypePrefixScorer,
			Parameters: map[string]interface{}{},
		})
		profilePlugins = append(profilePlugins, &ProfilePluginConfig{
			PluginRef: pluginNamePrefixScorer,
			Weight:    float64Ptr(1.0),
		})
	}

	if conf.EffectiveSessionAffinityEnabled() && conf.SessionAffinityHeader != nil {
		plugins = append(plugins, &PluginConfig{
			Name: pluginNameSessionScorer,
			Type: pluginTypeSessionScorer,
			Parameters: map[string]interface{}{
				"sessionIdConfig": map[string]interface{}{
					"sources": []map[string]interface{}{
						{"header": *conf.SessionAffinityHeader},
					},
				},
			},
		})
		profilePlugins = append(profilePlugins, &ProfilePluginConfig{
			PluginRef: pluginNameSessionScorer,
			Weight:    float64Ptr(1.0),
		})
	}

	plugins = append(plugins,
		&PluginConfig{
			Name:       pluginNameMaxScorePicker,
			Type:       pluginTypeMaxScorePicker,
			Parameters: map[string]interface{}{},
		},
		&PluginConfig{
			Name:       pluginNameOpenAIParser,
			Type:       pluginTypeOpenAIParser,
			Parameters: map[string]interface{}{},
		},
	)

	profilePlugins = append(profilePlugins, &ProfilePluginConfig{PluginRef: pluginNameMaxScorePicker})

	compiled := &EndpointPickerConfig{
		Plugins: plugins,
		SchedulingProfiles: []*SchedulingProfileConfig{
			{
				Name:    "default",
				Plugins: profilePlugins,
			},
		},
		DataLayer: &DataLayerConfig{
			Discovery: &DiscoveryConfig{
				Endpoints: &PluginRefConfig{PluginRef: pluginNameDiscovery},
			},
		},
		RequestHandler: &RequestHandlerConfig{
			Parsers: []*PluginRefConfig{{PluginRef: pluginNameOpenAIParser}},
		},
	}

	if conf.FlowControl != nil {
		compiled.FeatureGates = []string{featureGateFlowControl}
		compiled.FlowControl = compileFlowControl(conf.FlowControl)
	}

	return compiled
}

// compileScorerWeights resolves the (kv, queue) scorer weights: an explicit
// cache_affinity overrides the scheduling profile; otherwise the profile
// weights are used.
func compileScorerWeights(conf *EppConfigSimplified) (float64, float64) {
	if conf != nil && conf.CacheAffinity != nil {
		weights, ok := scorerWeights[*conf.CacheAffinity]
		if ok {
			return weights[0], weights[1]
		}
	}

	weights, ok := scorerWeights[conf.EffectiveSchedulingProfile()]
	if !ok {
		weights = scorerWeights[DefaultSchedulingProfile]
	}
	return weights[0], weights[1]
}

// compileFlowControl expands the simplified flow_control section. Durations
// are converted from seconds to Go duration strings (30 -> "30s", 600 -> "10m0s").
func compileFlowControl(fc *FlowControlSimplified) *FlowControlConfig {
	compiled := &FlowControlConfig{}

	if fc.MaxRequests != nil && *fc.MaxRequests > 0 {
		compiled.MaxRequests = strconv.Itoa(*fc.MaxRequests)
	}

	if fc.QueueTTL != nil {
		compiled.DefaultRequestTTL = secondsToDuration(*fc.QueueTTL)
	}

	if fc.NoEndpointQueueTTL != nil {
		compiled.NoEndpointRequestTTL = secondsToDuration(*fc.NoEndpointQueueTTL)
	}

	if fc.EnableEviction != nil {
		compiled.EnableEviction = *fc.EnableEviction
	}

	return compiled
}

// secondsToDuration converts seconds to a Go duration string ("30s", "10m0s", "0s").
func secondsToDuration(seconds int) string {
	return (time.Duration(seconds) * time.Second).String()
}

func float64Ptr(v float64) *float64 {
	return &v
}
