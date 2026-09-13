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

func TestBuildChangeSummary_UpdateWithDiff(t *testing.T) {
	before := map[string]interface{}{
		"allow_models": []string{"gpt-4"},
		"name":         "old-name",
	}
	after := map[string]interface{}{
		"allow_models": []string{"gpt-4", "gpt-4o"},
		"name":         "old-name",
	}

	summary := BuildChangeSummary(string(ActionUpdate), before, after)

	assert.NotNil(t, summary)
	assert.Contains(t, summary, "before")
	assert.Contains(t, summary, "after")
	assert.Equal(t, []string{"allow_models"}, summary["diff_keys"])
}

func TestBuildChangeSummary_UpdatePartialRequest(t *testing.T) {
	// Simulates a PATCH update where only "allow_models" is sent.
	before := map[string]interface{}{
		"allow_models": []string{"gpt-4"},
		"block_models": []string{"gpt-3"},
		"name":         "entity-1",
	}
	after := map[string]interface{}{
		"allow_models": []string{"gpt-4", "gpt-4o"},
	}

	summary := BuildChangeSummary(string(ActionUpdate), before, after)

	assert.NotNil(t, summary)
	assert.Equal(t, []string{"allow_models"}, summary["diff_keys"])
}

func TestBuildChangeSummary_UpdateNoDiff(t *testing.T) {
	before := map[string]interface{}{"name": "same"}
	after := map[string]interface{}{"name": "same"}

	summary := BuildChangeSummary(string(ActionUpdate), before, after)

	assert.NotNil(t, summary)
	assert.Equal(t, []string{}, summary["diff_keys"])
}

func TestBuildChangeSummary_CreateHasNoDiffKeys(t *testing.T) {
	after := map[string]interface{}{"name": "new"}

	summary := BuildChangeSummary(string(ActionCreate), nil, after)

	assert.NotNil(t, summary)
	assert.Contains(t, summary, "after")
	assert.NotContains(t, summary, "diff_keys")
}

func TestBuildChangeSummary_DeleteHasNoDiffKeys(t *testing.T) {
	before := map[string]interface{}{"name": "old"}

	summary := BuildChangeSummary(string(ActionDelete), before, nil)

	assert.NotNil(t, summary)
	assert.Contains(t, summary, "before")
	assert.NotContains(t, summary, "diff_keys")
}

func TestBuildChangeSummary_MasksSensitiveFields(t *testing.T) {
	before := map[string]interface{}{
		"password": "old-pass",
	}
	after := map[string]interface{}{
		"password": "new-pass",
	}

	summary := BuildChangeSummary(string(ActionUpdate), before, after)

	assert.NotNil(t, summary)
	assert.Equal(t, maskPlaceholder, summary["before"].(map[string]interface{})["password"])
	assert.Equal(t, maskPlaceholder, summary["after"].(map[string]interface{})["password"])
	assert.Equal(t, []string{"password"}, summary["diff_keys"])
}

func TestBuildChangeSummary_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, BuildChangeSummary(string(ActionUpdate), nil, nil))
	assert.Nil(t, BuildChangeSummary(string(ActionUpdate), map[string]interface{}{}, map[string]interface{}{}))
}
