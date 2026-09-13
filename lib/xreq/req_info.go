// Copyright (c) 2021 The BFE Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package xreq

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type requestInfoKey string

var (
	_requestInfoKey requestInfoKey = "request_info"
)

type RequestInfo struct {
	StartTime time.Time
	Duration  time.Duration

	URLPath   string
	Method    string
	ClientIP  string
	UserAgent string
	LogID     string

	URLPattern string

	StatusCode int
	RetMsg     string
	ErrDetail  string
}

// clientIPFromRequest extracts the real client IP from the request.
// The custom ClientIp header is preferred for backward compatibility,
// falling back to the first X-Forwarded-For entry, and finally to
// RemoteAddr (with the port stripped).
func clientIPFromRequest(req *http.Request) string {
	if v := req.Header.Get("ClientIp"); v != "" {
		return v
	}
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.IndexByte(xff, ','); i >= 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if host, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		return host
	}
	return req.RemoteAddr
}

func InitRequestInfo(ctx context.Context, req *http.Request) (context.Context, *RequestInfo) {
	requestInfo := GetRequestInfo(ctx)
	if requestInfo != nil {
		return ctx, requestInfo
	}
	requestInfo = &RequestInfo{
		StartTime:  time.Now(),
		URLPath:    req.URL.Path,
		ClientIP:   clientIPFromRequest(req),
		UserAgent:  req.Header.Get("User-Agent"),
		Method:     req.Method,
		StatusCode: 200,
	}
	return context.WithValue(ctx, _requestInfoKey, requestInfo), requestInfo
}

func GetRequestInfo(ctx context.Context) *RequestInfo {
	if v := ctx.Value(_requestInfoKey); v != nil {
		return v.(*RequestInfo)
	}

	return nil
}

func (requestInfo *RequestInfo) String() string {
	return fmt.Sprintf("[%s]cost_ms[%d] method[%s] pattern[%s] path[%s] client_ip[%s] status_code[%d] ret_msg[%s] err_detail[%s]]",
		requestInfo.LogID, requestInfo.Duration.Milliseconds(), requestInfo.Method, requestInfo.URLPattern, requestInfo.URLPath, requestInfo.ClientIP,
		requestInfo.StatusCode, requestInfo.RetMsg, requestInfo.ErrDetail)
}
