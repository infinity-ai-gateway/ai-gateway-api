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

// ProviderEndpointSchema provider model_endpoint schema
var ProviderEndpointSchema = &testutil.ObjectSchema{
	Required: []string{"schema", "uri"},
	Fields: map[string]testutil.FieldSpec{
		"schema": {Type: testutil.TypeString},
		"uri":    {Type: testutil.TypeString},
	},
}

// ProviderKeySchema provider keys element schema
var ProviderKeySchema = &testutil.ObjectSchema{
	Required: []string{"name", "key"},
	Fields: map[string]testutil.FieldSpec{
		"name": {Type: testutil.TypeString},
		"key":  {Type: testutil.TypeString},
	},
}

// ProviderInstanceSchema provider instance_pool element schema
var ProviderInstanceSchema = &testutil.ObjectSchema{
	Required: []string{"addr", "weight", "port"},
	Fields: map[string]testutil.FieldSpec{
		"addr":   {Type: testutil.TypeString},
		"weight": {Type: testutil.TypeInt},
		"port":   {Type: testutil.TypeInt},
	},
}

// TimeRangeSchema provider tiers.time_ranges element schema
var TimeRangeSchema = &testutil.ObjectSchema{
	Required: []string{"start", "end"},
	Fields: map[string]testutil.FieldSpec{
		"weekdays": {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeInt}},
		"start":    {Type: testutil.TypeString},
		"end":      {Type: testutil.TypeString},
	},
}

// PricingTierSchema provider tiers element schema
var PricingTierSchema = &testutil.ObjectSchema{
	Required: []string{"name", "time_ranges"},
	Fields: map[string]testutil.FieldSpec{
		"name":        {Type: testutil.TypeString},
		"time_ranges": {Type: testutil.TypeArray, Elem: TimeRangeSchema},
	},
}

// ProviderSchema Provider 数据模型 schema
var ProviderSchema = &testutil.ObjectSchema{
	Required: []string{
		"id", "name", "description", "model_endpoint", "models", "keys",
		"instance_pool", "model_protocols", "time_zone", "tiers",
		"create_time", "update_time",
	},
	Fields: map[string]testutil.FieldSpec{
		"id":              {Type: testutil.TypeInt},
		"name":            {Type: testutil.TypeString},
		"description":     {Type: testutil.TypeString},
		"model_endpoint":  {Type: testutil.TypeObject, Nested: ProviderEndpointSchema},
		"models":          {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString}},
		"keys":            {Type: testutil.TypeArray, Elem: ProviderKeySchema},
		"instance_pool":   {Type: testutil.TypeArray, Elem: ProviderInstanceSchema},
		"model_protocols": {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString, Enum: []interface{}{"openai", "anthropic"}}},
		"time_zone":       {Type: testutil.TypeString},
		"tiers":           {Type: testutil.TypeArray, Elem: PricingTierSchema},
		"create_time":     {Type: testutil.TypeInt},
		"update_time":     {Type: testutil.TypeInt},
	},
}
