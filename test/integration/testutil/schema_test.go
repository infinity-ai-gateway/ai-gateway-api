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
	"testing"
)

func TestAssertSchema(t *testing.T) {
	schema := &ObjectSchema{
		Required: []string{"name", "count", "enabled"},
		Optional: []string{"description"},
		Fields: map[string]FieldSpec{
			"name":        {Type: TypeString},
			"count":       {Type: TypeInt},
			"enabled":     {Type: TypeBool},
			"description": {Type: TypeString},
		},
	}

	t.Run("valid object", func(t *testing.T) {
		data := map[string]interface{}{
			"name":    "test",
			"count":   float64(10),
			"enabled": true,
		}
		raw, _ := json.Marshal(data)
		resp := &APIResponse{ErrNum: 200, Data: raw}
		AssertSchema(t, resp, schema)
	})

	t.Run("missing required field", func(t *testing.T) {
		data := map[string]interface{}{
			"name":    "test",
			"enabled": true,
		}
		raw, _ := json.Marshal(data)
		resp := &APIResponse{ErrNum: 200, Data: raw}
		fakeT := &testingT{}
		AssertSchema(fakeT, resp, schema)
		if !fakeT.failed {
			t.Fatal("expected failure")
		}
		if !contains(fakeT.errors, "count") || !contains(fakeT.errors, "required field missing") {
			t.Fatalf("expected required field missing error for count, got %v", fakeT.errors)
		}
	})

	t.Run("wrong type", func(t *testing.T) {
		data := map[string]interface{}{
			"name":    "test",
			"count":   "not a number",
			"enabled": true,
		}
		raw, _ := json.Marshal(data)
		resp := &APIResponse{ErrNum: 200, Data: raw}
		fakeT := &testingT{}
		AssertSchema(fakeT, resp, schema)
		if !fakeT.failed {
			t.Fatal("expected failure")
		}
		if !contains(fakeT.errors, "expected integer") {
			t.Fatalf("expected integer type error, got %v", fakeT.errors)
		}
	})
}

func TestAssertPagedListSchema(t *testing.T) {
	itemSchema := &ObjectSchema{
		Required: []string{"id"},
		Fields: map[string]FieldSpec{
			"id": {Type: TypeString},
		},
	}

	t.Run("valid paged list", func(t *testing.T) {
		data := map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{"id": "a"},
				map[string]interface{}{"id": "b"},
			},
			"pagination": map[string]interface{}{
				"page":      float64(1),
				"page_size": float64(20),
				"total":     float64(2),
			},
		}
		raw, _ := json.Marshal(data)
		resp := &APIResponse{ErrNum: 200, Data: raw}
		AssertPagedListSchema(t, resp, itemSchema)
	})

	t.Run("missing pagination total", func(t *testing.T) {
		data := map[string]interface{}{
			"list": []interface{}{
				map[string]interface{}{"id": "a"},
			},
			"pagination": map[string]interface{}{
				"page":      float64(1),
				"page_size": float64(20),
			},
		}
		raw, _ := json.Marshal(data)
		resp := &APIResponse{ErrNum: 200, Data: raw}
		fakeT := &testingT{}
		AssertPagedListSchema(fakeT, resp, itemSchema)
		if !fakeT.failed {
			t.Fatal("expected failure")
		}
		if !contains(fakeT.errors, "total") || !contains(fakeT.errors, "required field missing") {
			t.Fatalf("expected missing total error, got %v", fakeT.errors)
		}
	})
}

func TestAssertListSchema(t *testing.T) {
	schema := &ObjectSchema{
		Required: []string{"id"},
		Fields: map[string]FieldSpec{
			"id": {Type: TypeString},
		},
	}

	t.Run("valid list", func(t *testing.T) {
		data := []interface{}{
			map[string]interface{}{"id": "a"},
			map[string]interface{}{"id": "b"},
		}
		raw, _ := json.Marshal(data)
		resp := &APIResponse{ErrNum: 200, Data: raw}
		AssertListSchema(t, resp, schema)
	})
}

// testingT 是一个简化的 testing.T 实现，用于捕获错误
type testingT struct {
	failed bool
	errors []string
	helped bool
}

func (t *testingT) Helper()                              { t.helped = true }
func (t *testingT) Error(args ...interface{})             { t.failed = true; t.errors = append(t.errors, sprint(args...)) }
func (t *testingT) Errorf(format string, args ...interface{}) { t.failed = true; t.errors = append(t.errors, sprintf(format, args...)) }
func (t *testingT) Fatal(args ...interface{})             { t.failed = true; t.errors = append(t.errors, sprint(args...)) }
func (t *testingT) Fatalf(format string, args ...interface{}) { t.failed = true; t.errors = append(t.errors, sprintf(format, args...)) }
func (t *testingT) Log(args ...interface{})               {}
func (t *testingT) Logf(format string, args ...interface{}) {}

func contains(slice []string, substr string) bool {
	for _, s := range slice {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

func sprint(args ...interface{}) string {
	return fmt.Sprint(args...)
}

func sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
