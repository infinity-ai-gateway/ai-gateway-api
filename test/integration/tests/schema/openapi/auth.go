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

// UserSchema 用户数据模型 schema
var UserSchema = &testutil.ObjectSchema{
	Required: []string{"user_name", "is_admin"},
	Fields: map[string]testutil.FieldSpec{
		"user_name": {Type: testutil.TypeString},
		"is_admin":  {Type: testutil.TypeBool},
	},
}

// SessionKeySchema 登录返回 schema
var SessionKeySchema = &testutil.ObjectSchema{
	Required: []string{"session_key", "user_name", "is_admin"},
	Fields: map[string]testutil.FieldSpec{
		"session_key": {Type: testutil.TypeString},
		"user_name":   {Type: testutil.TypeString},
		"is_admin":    {Type: testutil.TypeBool},
	},
}

// TokenSchema Token 详情/列表元素 schema
var TokenSchema = &testutil.ObjectSchema{
	Required: []string{"name", "token", "scope"},
	Fields: map[string]testutil.FieldSpec{
		"name":  {Type: testutil.TypeString},
		"token": {Type: testutil.TypeString},
		"scope": {Type: testutil.TypeString, Enum: []interface{}{"System", "Support"}},
	},
}

// CreateTokenResponseSchema 创建 Token 返回 schema（仅返回 token 字符串值）
var CreateTokenResponseSchema = &testutil.ObjectSchema{
	Required: []string{"token"},
	Fields: map[string]testutil.FieldSpec{
		"token": {Type: testutil.TypeString},
	},
}

// MetaSchema /meta 导航配置返回 schema
// icon/logo 为字符串资源路径/URL（对应配置项 UIIcon/UILogo）。
var MetaSchema = &testutil.ObjectSchema{
	Required: []string{"nav", "icon", "logo"},
	Fields: map[string]testutil.FieldSpec{
		"nav":  {Type: testutil.TypeObject},
		"icon": {Type: testutil.TypeString},
		"logo": {Type: testutil.TypeString},
	},
}
