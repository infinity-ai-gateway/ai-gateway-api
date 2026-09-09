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

// CertificateSchema 证书数据模型 schema（创建/更新/详情/列表返回，不包含 cert_file_content/key_file_content）
var CertificateSchema = &testutil.ObjectSchema{
	Required: []string{"cert_name", "description", "is_default", "expired_date"},
	Fields: map[string]testutil.FieldSpec{
		"cert_name":   {Type: testutil.TypeString},
		"description": {Type: testutil.TypeString},
		"is_default":  {Type: testutil.TypeBool},
		"expired_date":{Type: testutil.TypeString},
	},
}
