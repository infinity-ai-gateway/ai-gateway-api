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

package ioperlog

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMaskAPIKeyToken(t *testing.T) {
	assert.Equal(t, "******", MaskAPIKeyToken("short"))
	assert.Equal(t, "abcd****ijkl", MaskAPIKeyToken("abcdefghijkl"))
	assert.Equal(t, "abcd****fghi", MaskAPIKeyToken("abcdefghi"))
}

func TestMaskSensitiveFields(t *testing.T) {
	input := map[string]interface{}{
		"name":        "test",
		"password":    "secret123",
		"api_key":     "abcdefghijkl",
		"certificate": "-----BEGIN CERTIFICATE-----",
		"nested": map[string]interface{}{
			"secret": "nested-secret",
		},
	}

	result := MaskSensitiveFields(input)
	assert.Equal(t, "test", result["name"])
	assert.Equal(t, "******", result["password"])
	assert.Equal(t, "abcd****ijkl", result["api_key"])
	assert.Equal(t, "[已更新]", result["certificate"])

	nested, ok := result["nested"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "******", nested["secret"])
}
