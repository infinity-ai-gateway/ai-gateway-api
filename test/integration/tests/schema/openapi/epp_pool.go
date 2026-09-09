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
