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
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncateErrorMessage(t *testing.T) {
	t.Run("nil error returns empty string", func(t *testing.T) {
		assert.Equal(t, "", TruncateErrorMessage(nil, 1024))
	})

	t.Run("short message is kept intact", func(t *testing.T) {
		msg := "route in use"
		assert.Equal(t, msg, TruncateErrorMessage(errors.New(msg), 1024))
	})

	t.Run("long message is truncated with ellipsis", func(t *testing.T) {
		msg := strings.Repeat("a", 2000)
		got := TruncateErrorMessage(errors.New(msg), 1024)
		assert.Len(t, got, 1024)
		assert.True(t, strings.HasSuffix(got, "..."))
	})

	t.Run("maxLen zero returns full message", func(t *testing.T) {
		msg := "any error"
		assert.Equal(t, msg, TruncateErrorMessage(errors.New(msg), 0))
	})

	t.Run("maxLen smaller than ellipsis returns raw prefix", func(t *testing.T) {
		msg := "hello world"
		assert.Equal(t, "he", TruncateErrorMessage(errors.New(msg), 2))
	})
}

func TestTruncateErrorMessageDefault(t *testing.T) {
	t.Run("uses default max length", func(t *testing.T) {
		msg := strings.Repeat("b", DefaultErrorMessageMaxLen+100)
		got := TruncateErrorMessageDefault(errors.New(msg))
		assert.Len(t, got, DefaultErrorMessageMaxLen)
	})

	t.Run("nil error returns empty string", func(t *testing.T) {
		assert.Equal(t, "", TruncateErrorMessageDefault(nil))
	})
}
