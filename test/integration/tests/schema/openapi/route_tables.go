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

// RouteTableSchema 路由表元素 schema
var RouteTableSchema = &testutil.ObjectSchema{
	Required: []string{"id", "type", "owner", "enabled"},
	Fields: map[string]testutil.FieldSpec{
		"id":      {Type: testutil.TypeInt},
		"type":    {Type: testutil.TypeString, Enum: []interface{}{"global", "entity", "api_key"}},
		"owner":   {Type: testutil.TypeString},
		"enabled": {Type: testutil.TypeBool},
	},
}
