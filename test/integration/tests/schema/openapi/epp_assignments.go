package openapi

import "github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"

// EppClusterAssignmentSchema /epp-assignments §2.1 全量视图 clusters[] 元素 schema。
// group/primary/standby 在未分配或单实例组（无备）时为 null，故设为 Optional
// （testutil schema 校验中 Optional 字段为 null 时跳过类型校验）。
var EppClusterAssignmentSchema = &testutil.ObjectSchema{
	Required: []string{"cluster", "degraded"},
	Optional: []string{"group", "primary", "standby"},
	Fields: map[string]testutil.FieldSpec{
		"cluster":  {Type: testutil.TypeString},
		"group":    {Type: testutil.TypeString},
		"primary":  {Type: testutil.TypeObject, Nested: EppInstanceSchema},
		"standby":  {Type: testutil.TypeObject, Nested: EppInstanceSchema},
		"degraded": {Type: testutil.TypeBool},
	},
}

// EppClusterAssignmentOverrideSchema /epp-assignments §2.2 PUT 覆写响应 schema。
// 实现（ClusterAssignmentOverrideData）按契约注释不含 degraded 标记，故此处不要求。
var EppClusterAssignmentOverrideSchema = &testutil.ObjectSchema{
	Required: []string{"cluster"},
	Optional: []string{"group", "primary", "standby"},
	Fields: map[string]testutil.FieldSpec{
		"cluster": {Type: testutil.TypeString},
		"group":   {Type: testutil.TypeString},
		"primary": {Type: testutil.TypeObject, Nested: EppInstanceSchema},
		"standby": {Type: testutil.TypeObject, Nested: EppInstanceSchema},
	},
}

// EppAssignmentsViewSchema /epp-assignments 全量视图 schema
// （GET 全量视图与 GET ?cluster= 过滤响应同结构）。
var EppAssignmentsViewSchema = &testutil.ObjectSchema{
	Required: []string{"clusters", "unassigned_clusters", "idle_groups"},
	Fields: map[string]testutil.FieldSpec{
		"clusters":            {Type: testutil.TypeArray, Elem: EppClusterAssignmentSchema},
		"unassigned_clusters": {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString}},
		"idle_groups":         {Type: testutil.TypeArray, Item: &testutil.FieldSpec{Type: testutil.TypeString}},
	},
}
