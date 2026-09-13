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

// EppInstanceSchema EPP 实例 schema（/epp-pool 与 /epp-assignments 共用：
// 仅 id/host/port 三个字段，见 epp-pool.md 数据模型）。
var EppInstanceSchema = &testutil.ObjectSchema{
	Required: []string{"id", "host", "port"},
	Fields: map[string]testutil.FieldSpec{
		"id":   {Type: testutil.TypeString},
		"host": {Type: testutil.TypeString},
		"port": {Type: testutil.TypeInt},
	},
}

// EppPoolGroupSchema /epp-pool groups 元素 schema
var EppPoolGroupSchema = &testutil.ObjectSchema{
	Required: []string{"name", "instances"},
	Fields: map[string]testutil.FieldSpec{
		"name":      {Type: testutil.TypeString},
		"instances": {Type: testutil.TypeArray, Elem: EppInstanceSchema},
	},
}

// EppPoolSchema /epp-pool（GET/PATCH 响应）schema
var EppPoolSchema = &testutil.ObjectSchema{
	Required: []string{"name", "groups"},
	Fields: map[string]testutil.FieldSpec{
		"name":   {Type: testutil.TypeString},
		"groups": {Type: testutil.TypeArray, Elem: EppPoolGroupSchema},
	},
}
