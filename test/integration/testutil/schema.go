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

package testutil

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FieldType 表示 JSON 字段的期望类型
type FieldType string

const (
	TypeString FieldType = "string"
	TypeNumber FieldType = "number"
	TypeInt    FieldType = "int"
	TypeBool   FieldType = "bool"
	TypeArray  FieldType = "array"
	TypeObject FieldType = "object"
)

// FieldSpec 描述单个字段的校验规则
type FieldSpec struct {
	Type   FieldType
	Elem   *ObjectSchema // 数组元素为对象时的 schema（TypeArray 时使用，与 Item 二选一）
	Item   *FieldSpec    // 数组元素为原始类型时的 schema（TypeArray 时使用，与 Elem 二选一）
	Nested *ObjectSchema // 嵌套对象 schema（TypeObject 时使用）
	Enum   []interface{} // 可选：枚举值校验
}

// ObjectSchema 描述一个 JSON 对象的 schema
type ObjectSchema struct {
	Required []string               // 必填字段
	Optional []string               // 可选字段（存在时校验类型）
	Fields   map[string]FieldSpec   // 字段定义
}

// schemaError 收集 schema 校验错误
type schemaError struct {
	path    string
	message string
}

func (e schemaError) Error() string {
	if e.path == "" {
		return e.message
	}
	return fmt.Sprintf("%s: %s", e.path, e.message)
}

// schemaTester 是 testing.T 的最小接口，便于测试校验器本身
type schemaTester interface {
	Helper()
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
}

// AssertSchema 校验 resp.Data 是否符合给定的 ObjectSchema
func AssertSchema(t schemaTester, resp *APIResponse, schema *ObjectSchema) {
	t.Helper()
	if resp == nil {
		t.Fatal("response is nil")
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal response Data: %v", err)
		return
	}

	errs := validateObject("", data, schema)
	for _, err := range errs {
		t.Error(err.Error())
	}
}

// AssertListSchema 校验 resp.Data 是数组，且每个元素符合 schema。
// 若 schema 为 nil，则只校验 Data 是数组，不校验元素结构（用于字符串数组等）。
func AssertListSchema(t schemaTester, resp *APIResponse, schema *ObjectSchema) {
	t.Helper()
	if resp == nil {
		t.Fatal("response is nil")
		return
	}

	var list []interface{}
	if err := json.Unmarshal(resp.Data, &list); err != nil {
		t.Fatalf("unmarshal response Data as list: %v", err)
		return
	}

	if schema == nil {
		return
	}

	for i, item := range list {
		obj, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("list[%d] is not an object", i)
			continue
		}
		errs := validateObject(fmt.Sprintf("[%d]", i), obj, schema)
		for _, err := range errs {
			t.Error(err.Error())
		}
	}
}

// AssertPagedListSchema 校验 resp.Data 包含 list + pagination 结构，且 list 元素符合 schema
func AssertPagedListSchema(t schemaTester, resp *APIResponse, itemSchema *ObjectSchema) {
	t.Helper()
	if resp == nil {
		t.Fatal("response is nil")
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		t.Fatalf("unmarshal response Data: %v", err)
		return
	}

	pageSchema := &ObjectSchema{
		Required: []string{"list", "pagination"},
		Fields: map[string]FieldSpec{
			"list":       {Type: TypeArray, Elem: itemSchema},
			"pagination": {Type: TypeObject, Nested: PaginationSchema},
		},
	}

	errs := validateObject("", data, pageSchema)
	for _, err := range errs {
		t.Error(err.Error())
	}
}

// PaginationSchema 分页信息 schema
var PaginationSchema = &ObjectSchema{
	Required: []string{"page", "page_size", "total"},
	Fields: map[string]FieldSpec{
		"page":      {Type: TypeInt},
		"page_size": {Type: TypeInt},
		"total":     {Type: TypeInt},
	},
}

// validateObject 校验对象是否符合 schema，返回所有错误
func validateObject(path string, data map[string]interface{}, schema *ObjectSchema) []schemaError {
	if schema == nil {
		return nil
	}

	var errs []schemaError

	// 检查必填字段
	for _, field := range schema.Required {
		if _, ok := data[field]; !ok {
			errs = append(errs, schemaError{path: joinPath(path, field), message: "required field missing"})
		}
	}

	requiredSet := make(map[string]bool)
	for _, f := range schema.Required {
		requiredSet[f] = true
	}

	// 校验所有已定义字段（必填+可选）
	allFields := append([]string{}, schema.Required...)
	allFields = append(allFields, schema.Optional...)
	checked := make(map[string]bool)
	for _, field := range allFields {
		if checked[field] {
			continue
		}
		checked[field] = true

		spec, ok := schema.Fields[field]
		if !ok {
			// 字段在 Required/Optional 中但没有 FieldSpec，说明 schema 定义不完整
			continue
		}

		value, exists := data[field]
		if !exists {
			continue
		}

		// 可选字段若为 null，跳过类型校验；必填字段为 null 仍需校验（会失败）
		optional := !requiredSet[field]
		fieldPath := joinPath(path, field)
		errs = append(errs, validateValue(fieldPath, value, &spec, optional)...)
	}

	return errs
}

// validateValue 校验单个值是否符合 FieldSpec
// optional 为 true 时，null 值会被接受，跳过类型校验。
func validateValue(path string, value interface{}, spec *FieldSpec, optional bool) []schemaError {
	if spec == nil {
		return nil
	}

	var errs []schemaError

	if value == nil && optional {
		return errs
	}

	// 检查枚举值
	if len(spec.Enum) > 0 {
		found := false
		for _, ev := range spec.Enum {
			if jsonEqual(ev, value) {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("value %v not in enum %v", value, spec.Enum)})
		}
	}

	switch spec.Type {
	case TypeString:
		if _, ok := value.(string); !ok {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("expected string, got %T", value)})
		}
	case TypeNumber:
		if !isNumber(value) {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("expected number, got %T", value)})
		}
	case TypeInt:
		if !isInt(value) {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("expected integer, got %T", value)})
		}
	case TypeBool:
		if _, ok := value.(bool); !ok {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("expected bool, got %T", value)})
		}
	case TypeArray:
		arr, ok := value.([]interface{})
		if !ok {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("expected array, got %T", value)})
			return errs
		}
		if spec.Elem != nil {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				if obj, ok := item.(map[string]interface{}); ok {
					errs = append(errs, validateObject(itemPath, obj, spec.Elem)...)
				} else {
					errs = append(errs, schemaError{path: itemPath, message: fmt.Sprintf("expected object, got %T", item)})
				}
			}
		}
		if spec.Item != nil {
			for i, item := range arr {
				itemPath := fmt.Sprintf("%s[%d]", path, i)
				errs = append(errs, validateValue(itemPath, item, spec.Item, false)...)
			}
		}
	case TypeObject:
		obj, ok := value.(map[string]interface{})
		if !ok {
			errs = append(errs, schemaError{path: path, message: fmt.Sprintf("expected object, got %T", value)})
			return errs
		}
		if spec.Nested != nil {
			errs = append(errs, validateObject(path, obj, spec.Nested)...)
		}
	default:
		errs = append(errs, schemaError{path: path, message: fmt.Sprintf("unknown expected type %q", spec.Type)})
	}

	return errs
}

func isNumber(value interface{}) bool {
	switch value.(type) {
	case float64, float32, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

func isInt(value interface{}) bool {
	switch v := value.(type) {
	case float64:
		return v == float64(int64(v))
	case float32:
		return v == float32(int64(v))
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	}
	return false
}

// jsonEqual 比较两个从 JSON 反序列化出来的值是否相等
func jsonEqual(a, b interface{}) bool {
	switch av := a.(type) {
	case float64:
		bv, ok := b.(float64)
		return ok && av == bv
	case string:
		bv, ok := b.(string)
		return ok && av == bv
	case bool:
		bv, ok := b.(bool)
		return ok && av == bv
	case nil:
		return b == nil
	default:
		return false
	}
}

func joinPath(base, field string) string {
	if base == "" {
		return field
	}
	if strings.HasPrefix(field, "[") {
		return base + field
	}
	return base + "." + field
}
