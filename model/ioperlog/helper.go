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

// DefaultErrorMessageMaxLen is the maximum length of operation_logs.error_msg
// column (varchar(1024)).
const DefaultErrorMessageMaxLen = 1024

// TruncateErrorMessage converts an error to a string and truncates it to the
// given maximum length so that it can be safely stored in the database. If the
// error is nil, an empty string is returned. If truncation occurs, a trailing
// ellipsis is appended to indicate that the message was shortened.
func TruncateErrorMessage(err error, maxLen int) string {
	if err == nil {
		return ""
	}

	msg := err.Error()
	if maxLen <= 0 {
		return msg
	}

	if len(msg) <= maxLen {
		return msg
	}

	ellipsis := "..."
	if maxLen <= len(ellipsis) {
		return msg[:maxLen]
	}

	return msg[:maxLen-len(ellipsis)] + ellipsis
}

// TruncateErrorMessageDefault truncates an error message to the default column
// length (1024 characters).
func TruncateErrorMessageDefault(err error) string {
	return TruncateErrorMessage(err, DefaultErrorMessageMaxLen)
}
