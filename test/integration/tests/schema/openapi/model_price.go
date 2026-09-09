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

// ModelPriceSchema 模型定价记录数据模型 schema
// 与 /entities、/api-keys 等接口保持一致，create_time/update_time 为 int64 Unix 时间戳。
var ModelPriceSchema = &testutil.ObjectSchema{
	Required: []string{
		"id", "provider", "model", "base_model", "mode",
		"capabilities", "supported_parameters",
		"limits", "prices", "price_currency",
		"metadata", "create_time", "update_time",
	},
	Optional: []string{"tier_prices"},
	Fields: map[string]testutil.FieldSpec{
		"id":                   {Type: testutil.TypeInt},
		"provider":             {Type: testutil.TypeString},
		"model":                {Type: testutil.TypeString},
		"base_model":           {Type: testutil.TypeString},
		"mode":                 {Type: testutil.TypeString},
		"capabilities":         {Type: testutil.TypeArray},
		"supported_parameters": {Type: testutil.TypeArray},
		"limits":               {Type: testutil.TypeObject},
		"prices":               {Type: testutil.TypeObject},
		"tier_prices":          {Type: testutil.TypeObject},
		"price_currency":       {Type: testutil.TypeString},
		"metadata":             {Type: testutil.TypeObject},
		"create_time":          {Type: testutil.TypeInt},
		"update_time":          {Type: testutil.TypeInt},
	},
}

// ModelPricePaginationSchema /model-prices 列表分页信息 schema
var ModelPricePaginationSchema = &testutil.ObjectSchema{
	Required: []string{"page", "page_size", "total"},
	Fields: map[string]testutil.FieldSpec{
		"page":      {Type: testutil.TypeInt},
		"page_size": {Type: testutil.TypeInt},
		"total":     {Type: testutil.TypeInt},
	},
}

// ModelPriceListResponseSchema /model-prices GET 列表返回 schema
var ModelPriceListResponseSchema = &testutil.ObjectSchema{
	Required: []string{"list", "pagination"},
	Fields: map[string]testutil.FieldSpec{
		"list":       {Type: testutil.TypeArray, Elem: ModelPriceSchema},
		"pagination": {Type: testutil.TypeObject, Nested: ModelPricePaginationSchema},
	},
}

// ModelPriceImportResultSchema /model-prices/import 返回 schema
var ModelPriceImportResultSchema = &testutil.ObjectSchema{
	Required: []string{"imported_count", "skipped_count", "errors"},
	Fields: map[string]testutil.FieldSpec{
		"imported_count": {Type: testutil.TypeInt},
		"skipped_count":  {Type: testutil.TypeInt},
		"errors":         {Type: testutil.TypeArray},
	},
}

// ModelPriceGetProvidersResponseSchema /model-prices/actions/get-providers 返回 schema
var ModelPriceGetProvidersResponseSchema = &testutil.ObjectSchema{
	Required: []string{"providers"},
	Fields: map[string]testutil.FieldSpec{
		"providers": {Type: testutil.TypeArray},
	},
}
