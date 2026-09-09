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

package openapi

import "github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"

// EntitySchema Entity 数据模型 schema（不含 balance）
// parent_id 对根节点为 null，因此设为可选。
var EntitySchema = &testutil.ObjectSchema{
	Required: []string{
		"id", "name", "type",
		"allow_models", "block_models",
		"quota_plan", "rate_limit_policy", "route_rules",
		"create_time", "update_time",
	},
	Optional: []string{"parent_id"},
	Fields: map[string]testutil.FieldSpec{
		"id":                {Type: testutil.TypeString},
		"name":              {Type: testutil.TypeString},
		"type":              {Type: testutil.TypeString},
		"parent_id":         {Type: testutil.TypeString},
		"allow_models":      {Type: testutil.TypeArray},
		"block_models":      {Type: testutil.TypeArray},
		"quota_plan":        {Type: testutil.TypeObject, Nested: QuotaPlanWithoutBalanceSchema},
		"rate_limit_policy": {Type: testutil.TypeObject, Nested: RateLimitPolicySchema},
		"route_rules":       {Type: testutil.TypeObject, Nested: RouteRulesSchema},
		"create_time":       {Type: testutil.TypeInt},
		"update_time":       {Type: testutil.TypeInt},
	},
}

// EntityListItemSchema Entity 列表元素 schema（不含 balance）
var EntityListItemSchema = EntitySchema
