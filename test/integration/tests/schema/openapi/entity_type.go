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

// EntityTypeSchema Entity-Type 数据模型 schema
var EntityTypeSchema = &testutil.ObjectSchema{
	Required: []string{"type_name", "description", "level", "create_time"},
	Fields: map[string]testutil.FieldSpec{
		"type_name":   {Type: testutil.TypeString},
		"description": {Type: testutil.TypeString},
		"level":       {Type: testutil.TypeInt},
		"create_time": {Type: testutil.TypeInt},
	},
}
