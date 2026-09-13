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

// Package innerapi 定义 InnerAPI v1 各接口返回值的 schema。
// 由于 InnerAPI 大量字段为动态 key 的 map，本包优先校验顶层固定字段的类型，
// 对动态 map 仅校验其为 object。
package innerapi

import "github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"

// ---------- 公共/复用 schema ----------

// InnerAPIBaseSchema 所有 InnerAPI 导出配置的顶层基础字段（Version 为 string）
var InnerAPIBaseSchema = &testutil.ObjectSchema{
	Required: []string{"Version"},
	Fields: map[string]testutil.FieldSpec{
		"Version": {Type: testutil.TypeString},
	},
}

// ClusterConfSchema /configs/tls_conf/server_data_conf 中 ClusterConf 的 schema
var ClusterConfSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "Config"},
	Fields: map[string]testutil.FieldSpec{
		"Version": {Type: testutil.TypeString},
		"Config":  {Type: testutil.TypeObject},
	},
}

// ModelPriceSchema AIConf.ModelTable.Models 中单个模型价格的 schema
var ModelPriceSchema = &testutil.ObjectSchema{
	Required: []string{"Provider", "Model", "BaseModel", "Mode", "Prices"},
	Optional: []string{"Capabilities", "SupportedParameters", "Limits", "TierPrices", "Metadata"},
	Fields: map[string]testutil.FieldSpec{
		"Provider":            {Type: testutil.TypeString},
		"Model":               {Type: testutil.TypeString},
		"BaseModel":           {Type: testutil.TypeString},
		"Mode":                {Type: testutil.TypeString},
		"Capabilities":        {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString}},
		"SupportedParameters": {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString}},
		"Limits":              {Type: testutil.TypeObject},
		"Prices":              {Type: testutil.TypeObject},
		"TierPrices":          {Type: testutil.TypeObject},
		"Metadata":            {Type: testutil.TypeObject},
	},
}

// ModelTableSchema AIConf.ModelTable 的 schema
var ModelTableSchema = &testutil.ObjectSchema{
	Required: []string{"Currency", "Models"},
	Optional: []string{"TimeZone", "Tiers"},
	Fields: map[string]testutil.FieldSpec{
		"Currency": {Type: testutil.TypeString},
		"TimeZone": {Type: testutil.TypeString},
		"Tiers":    {Type: testutil.TypeArray},
		"Models":   {Type: testutil.TypeArray, Elem: ModelPriceSchema},
	},
}

// AIKeyPolicySchema AIConf.KeyPolicy 的 schema
var AIKeyPolicySchema = &testutil.ObjectSchema{
	Required: []string{
		"Strategy", "MaxRetries", "RetryBackoffInitial", "RetryBackoffMax",
		"SessionAffinity", "SessionAffinityTTL", "SessionAffinityRedisPrefix", "SessionAffinityPenaltyEnable",
	},
	Fields: map[string]testutil.FieldSpec{
		"Strategy":                     {Type: testutil.TypeString},
		"MaxRetries":                   {Type: testutil.TypeInt},
		"RetryBackoffInitial":          {Type: testutil.TypeInt},
		"RetryBackoffMax":              {Type: testutil.TypeInt},
		"SessionAffinity":              {Type: testutil.TypeBool},
		"SessionAffinityTTL":           {Type: testutil.TypeInt},
		"SessionAffinityRedisPrefix":   {Type: testutil.TypeString},
		"SessionAffinityPenaltyEnable": {Type: testutil.TypeBool},
	},
}

// AIConfSchema ClusterConf.Config.<cluster>.AIConf 的 schema
var AIConfSchema = &testutil.ObjectSchema{
	Required: []string{"KeyPolicy"},
	Optional: []string{"Keys", "ModelMappings", "ModelTable", "MatchPrefix", "StripPrefix", "ModelProtocols"},
	Fields: map[string]testutil.FieldSpec{
		"Keys":           {Type: testutil.TypeArray},
		"KeyPolicy":      {Type: testutil.TypeObject, Nested: AIKeyPolicySchema},
		"ModelMappings":  {Type: testutil.TypeObject},
		"ModelTable":     {Type: testutil.TypeObject, Nested: ModelTableSchema},
		"MatchPrefix":    {Type: testutil.TypeString},
		"StripPrefix":    {Type: testutil.TypeBool},
		"ModelProtocols": {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString}},
	},
}

// ServerDataConfSchema /configs/tls_conf/server_data_conf 返回 schema
var ServerDataConfSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "HostTable", "RouteTable", "ClusterConf"},
	Fields: map[string]testutil.FieldSpec{
		"Version":     {Type: testutil.TypeString},
		"HostTable":   {Type: testutil.TypeObject},
		"RouteTable":  {Type: testutil.TypeObject},
		"ClusterConf": {Type: testutil.TypeObject, Nested: ClusterConfSchema},
	},
}

// GSLBSchema /configs/gslb_data/gslb 返回 schema
var GSLBSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "Ts", "Hostname", "Clusters"},
	Fields: map[string]testutil.FieldSpec{
		"Version":  {Type: testutil.TypeString},
		"Ts":       {Type: testutil.TypeString},
		"Hostname": {Type: testutil.TypeString},
		"Clusters": {Type: testutil.TypeObject},
	},
}

// ClusterTableSchema /configs/gslb_data/cluster_table 返回 schema
var ClusterTableSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "Config"},
	Fields: map[string]testutil.FieldSpec{
		"Version": {Type: testutil.TypeString},
		"Config":  {Type: testutil.TypeObject},
	},
}

// ServerCertConfSchema /configs/protocol/server_cert_conf 返回 schema
var ServerCertConfSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "Config"},
	Fields: map[string]testutil.FieldSpec{
		"Version": {Type: testutil.TypeString},
		"Config":  {Type: testutil.TypeObject},
	},
}

// ModAPIKeySchema /configs/mod-api-key 返回 schema
var ModAPIKeySchema = &testutil.ObjectSchema{
	Required: []string{"version", "config", "QuotaPlans", "tokens"},
	Fields: map[string]testutil.FieldSpec{
		"version":    {Type: testutil.TypeString},
		"config":     {Type: testutil.TypeObject},
		"QuotaPlans": {Type: testutil.TypeObject},
		"tokens":     {Type: testutil.TypeObject},
	},
}

// ModBodyProcessSchema /configs/mod-body-process 返回 schema
var ModBodyProcessSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "Config"},
	Fields: map[string]testutil.FieldSpec{
		"Version": {Type: testutil.TypeString},
		"Config":  {Type: testutil.TypeObject},
	},
}

// RateLimitPolicySchema /configs/rate-limit-policy 返回 schema
var RateLimitPolicySchema = &testutil.ObjectSchema{
	Required: []string{"Config", "RateLimitPolicies", "ApikeyRateLimitPolicyBindings", "Version"},
	Fields: map[string]testutil.FieldSpec{
		"Config":                        {Type: testutil.TypeObject},
		"RateLimitPolicies":             {Type: testutil.TypeObject},
		"ApikeyRateLimitPolicyBindings": {Type: testutil.TypeObject},
		"Version":                       {Type: testutil.TypeString},
	},
}

// AIRouteSchema /configs/ai-route 返回 schema
var AIRouteSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "RouteRules", "ApikeyRouteTableBindings"},
	Fields: map[string]testutil.FieldSpec{
		"Version":                  {Type: testutil.TypeString},
		"RouteRules":               {Type: testutil.TypeObject},
		"ApikeyRouteTableBindings": {Type: testutil.TypeObject},
	},
}

// EppDataConfigBodySchema /configs/epp_data/config 返回 Config 段的 schema。
// epp_config / assignment 均为 cluster 名动态 key 的 map（见 epp-data.md §3），
// 按本包惯例仅校验为 object；条目级结构由测试中的定向断言覆盖。
var EppDataConfigBodySchema = &testutil.ObjectSchema{
	Required: []string{"epp_config", "assignment"},
	Fields: map[string]testutil.FieldSpec{
		"epp_config": {Type: testutil.TypeObject},
		"assignment": {Type: testutil.TypeObject},
	},
}

// EppDataSchema /configs/epp_data/config 返回 schema（epp-data.md §3.1）
var EppDataSchema = &testutil.ObjectSchema{
	Required: []string{"Version", "Config"},
	Fields: map[string]testutil.FieldSpec{
		"Version": {Type: testutil.TypeString},
		"Config":  {Type: testutil.TypeObject, Nested: EppDataConfigBodySchema},
	},
}
