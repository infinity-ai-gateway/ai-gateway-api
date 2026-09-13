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

// Package openapi 定义 OpenAPI v1 各接口返回值的 schema。
// schema 根据 ai-gateway-api/design-docs/api-define/OpenAPI接口定义 编写，
// 用于在集成测试中严格校验接口返回字段的存在性与类型。
package openapi

import "github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"

// ---------- 公共类型 schema ----------

// QuotaPlanWithoutBalanceSchema 是不含 balance 的配额计划 schema
var QuotaPlanWithoutBalanceSchema = &testutil.ObjectSchema{
	Required: []string{"unlimited", "pass_when_no_enough_quota", "quota", "unit", "reset_period"},
	Fields: map[string]testutil.FieldSpec{
		"unlimited":                 {Type: testutil.TypeBool},
		"pass_when_no_enough_quota": {Type: testutil.TypeBool},
		"quota":                     {Type: testutil.TypeNumber},
		"unit":                      {Type: testutil.TypeString, Enum: []interface{}{"total_token", "RMB"}},
		"reset_period":              {Type: testutil.TypeString, Enum: []interface{}{"never", "weekly", "monthly"}},
	},
}

// QuotaPlanWithBalanceSchema 是包含 balance 的配额计划 schema
var QuotaPlanWithBalanceSchema = &testutil.ObjectSchema{
	Required: []string{"unlimited", "pass_when_no_enough_quota", "quota", "unit", "reset_period", "balance"},
	Fields: map[string]testutil.FieldSpec{
		"unlimited":                 {Type: testutil.TypeBool},
		"pass_when_no_enough_quota": {Type: testutil.TypeBool},
		"quota":                     {Type: testutil.TypeNumber},
		"unit":                      {Type: testutil.TypeString, Enum: []interface{}{"total_token", "RMB"}},
		"reset_period":              {Type: testutil.TypeString, Enum: []interface{}{"never", "weekly", "monthly"}},
		"balance":                   {Type: testutil.TypeObject, Nested: QuotaBalanceSchema},
	},
}

// QuotaBalanceSchema 余额状态 schema
var QuotaBalanceSchema = &testutil.ObjectSchema{
	Required: []string{"used", "remaining"},
	Fields: map[string]testutil.FieldSpec{
		"used":      {Type: testutil.TypeNumber},
		"remaining": {Type: testutil.TypeNumber},
	},
}

// QuotaResetResultSchema 重置配额余额返回 schema
var QuotaResetResultSchema = &testutil.ObjectSchema{
	Required: []string{"id", "previous_quota", "new_quota", "balance"},
	Fields: map[string]testutil.FieldSpec{
		"id":             {Type: testutil.TypeString},
		"previous_quota": {Type: testutil.TypeNumber},
		"new_quota":      {Type: testutil.TypeNumber},
		"balance":        {Type: testutil.TypeObject, Nested: QuotaResetBalanceSchema},
	},
}

// QuotaResetBalanceSchema 重置后的余额变更详情 schema
var QuotaResetBalanceSchema = &testutil.ObjectSchema{
	Required: []string{"previous_remaining", "new_remaining", "used"},
	Fields: map[string]testutil.FieldSpec{
		"previous_remaining": {Type: testutil.TypeNumber},
		"new_remaining":      {Type: testutil.TypeNumber},
		"used":               {Type: testutil.TypeNumber},
	},
}

// TPMConfigSchema TPM 限流配置 schema
var TPMConfigSchema = &testutil.ObjectSchema{
	Required: []string{"name", "model", "window_minutes", "max_tokens", "step_minutes"},
	Fields: map[string]testutil.FieldSpec{
		"name":           {Type: testutil.TypeString},
		"model":          {Type: testutil.TypeString},
		"window_minutes": {Type: testutil.TypeInt},
		"max_tokens":     {Type: testutil.TypeInt},
		"step_minutes":   {Type: testutil.TypeInt},
	},
}

// RPMConfigSchema RPM 限流配置 schema
var RPMConfigSchema = &testutil.ObjectSchema{
	Required: []string{"name", "model", "window_minutes", "max_requests"},
	Fields: map[string]testutil.FieldSpec{
		"name":           {Type: testutil.TypeString},
		"model":          {Type: testutil.TypeString},
		"window_minutes": {Type: testutil.TypeInt},
		"max_requests":   {Type: testutil.TypeInt},
	},
}

// RateLimitPolicyRulesSchema 限流规则详情 schema
var RateLimitPolicyRulesSchema = &testutil.ObjectSchema{
	Required: []string{"tpm", "rpm", "max_concurrency"},
	Fields: map[string]testutil.FieldSpec{
		"tpm":             {Type: testutil.TypeArray, Elem: TPMConfigSchema},
		"rpm":             {Type: testutil.TypeArray, Elem: RPMConfigSchema},
		"max_concurrency": {Type: testutil.TypeInt},
	},
}

// RateLimitPolicySchema 限流策略 schema
var RateLimitPolicySchema = &testutil.ObjectSchema{
	Required: []string{"enabled", "rules"},
	Fields: map[string]testutil.FieldSpec{
		"enabled": {Type: testutil.TypeBool},
		"rules":   {Type: testutil.TypeObject, Nested: RateLimitPolicyRulesSchema},
	},
}

// RouteTargetSchema 路由目标 schema
var RouteTargetSchema = &testutil.ObjectSchema{
	Required: []string{"cluster_name", "model", "weight"},
	Fields: map[string]testutil.FieldSpec{
		"cluster_name": {Type: testutil.TypeString},
		"model":        {Type: testutil.TypeString},
		"weight":       {Type: testutil.TypeInt},
	},
}

// RouteFallbackSchema 路由降级目标 schema
var RouteFallbackSchema = &testutil.ObjectSchema{
	Required: []string{"cluster_name", "model"},
	Fields: map[string]testutil.FieldSpec{
		"cluster_name": {Type: testutil.TypeString},
		"model":        {Type: testutil.TypeString},
	},
}

// RouteRuleSchema 单个路由规则 schema
var RouteRuleSchema = &testutil.ObjectSchema{
	Required: []string{"name", "cond", "targets", "fallbacks"},
	Fields: map[string]testutil.FieldSpec{
		"name":      {Type: testutil.TypeString},
		"cond":      {Type: testutil.TypeString},
		"targets":   {Type: testutil.TypeArray, Elem: RouteTargetSchema},
		"fallbacks": {Type: testutil.TypeArray, Elem: RouteFallbackSchema},
	},
}

// RouteRulesSchema 路由规则集 schema
var RouteRulesSchema = &testutil.ObjectSchema{
	Required: []string{"enabled", "rules"},
	Fields: map[string]testutil.FieldSpec{
		"enabled": {Type: testutil.TypeBool},
		"rules":   {Type: testutil.TypeArray, Elem: RouteRuleSchema},
	},
}

// EntitySummarySchema Entity 摘要 schema
var EntitySummarySchema = &testutil.ObjectSchema{
	Required: []string{"id", "name", "type"},
	Fields: map[string]testutil.FieldSpec{
		"id":   {Type: testutil.TypeString},
		"name": {Type: testutil.TypeString},
		"type": {Type: testutil.TypeString},
	},
}
