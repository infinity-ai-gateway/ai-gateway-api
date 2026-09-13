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
	"strings"
)

const (
	maskPlaceholder = "******"
	updatedMarker   = "[已更新]"
)

// MaskAPIKeyToken masks an API key token, keeping the first and last 4 chars.
func MaskAPIKeyToken(token string) string {
	if len(token) <= 8 {
		return maskPlaceholder
	}
	return token[:4] + "****" + token[len(token)-4:]
}

// MaskString returns a masked placeholder for any sensitive string.
func MaskString(_ string) string {
	return maskPlaceholder
}

// MaskSensitiveFields recursively masks sensitive fields in a map.
// It mutates the input map in place.
func MaskSensitiveFields(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		return nil
	}

	for key, val := range data {
		lowerKey := strings.ToLower(key)

		switch lowerKey {
		case "password", "secret", "session_key", "sessionkey", "private_key", "privatekey":
			data[key] = maskPlaceholder
		case "api_key", "apikey", "key":
			if s, ok := val.(string); ok {
				data[key] = MaskAPIKeyToken(s)
			}
		case "certificate", "cert", "cert_body", "private_key_body":
			data[key] = updatedMarker
		default:
			// Recurse into nested maps.
			if nested, ok := val.(map[string]interface{}); ok {
				data[key] = MaskSensitiveFields(nested)
			}
		}
	}

	return data
}
