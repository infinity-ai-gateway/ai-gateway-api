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
	"reflect"
	"sort"
)

// BuildChangeSummary constructs the change_summary map for an operation log.
// It applies MaskSensitiveFields to before/after, and for update actions it
// also computes the list of changed field keys (diff_keys).
//
// diff_keys is computed from the keys present in the "after" map:
//   - keys that exist in both before and after but have different values
//   - keys that only exist in after (newly added fields)
//
// Keys that only exist in "before" are intentionally excluded, because many
// update requests are partial and omit unchanged fields. Including them would
// produce false-positive diffs.
func BuildChangeSummary(action string, before, after map[string]interface{}) map[string]interface{} {
	var diffKeys []string
	if ActionType(action) == ActionUpdate {
		diffKeys = computeDiffKeys(before, after)
	}

	changeSummary := map[string]interface{}{}
	if len(before) > 0 {
		changeSummary["before"] = MaskSensitiveFields(before)
	}
	if len(after) > 0 {
		changeSummary["after"] = MaskSensitiveFields(after)
	}
	if len(changeSummary) == 0 {
		return nil
	}

	if ActionType(action) == ActionUpdate {
		changeSummary["diff_keys"] = diffKeys
	}

	return changeSummary
}

func computeDiffKeys(before, after map[string]interface{}) []string {
	if len(after) == 0 {
		return []string{}
	}

	diffs := make([]string, 0, len(after))
	for key, afterVal := range after {
		beforeVal, ok := before[key]
		if !ok {
			diffs = append(diffs, key)
			continue
		}
		if !reflect.DeepEqual(beforeVal, afterVal) {
			diffs = append(diffs, key)
		}
	}

	sort.Strings(diffs)
	return diffs
}
