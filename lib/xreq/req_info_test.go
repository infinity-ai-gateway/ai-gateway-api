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

package xreq

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestRequest(header map[string]string, remoteAddr string) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, "http://example.com/test", nil)
	for k, v := range header {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = remoteAddr
	return req
}

func TestClientIPFromRequest(t *testing.T) {
	tests := []struct {
		name       string
		header     map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "ClientIp header takes priority",
			header:     map[string]string{"ClientIp": "198.51.100.210"},
			remoteAddr: "10.0.0.1:8080",
			want:       "198.51.100.210",
		},
		{
			name:       "X-Forwarded-For fallback takes first entry",
			header:     map[string]string{"X-Forwarded-For": "203.0.113.5, 10.0.0.1, 10.0.0.2"},
			remoteAddr: "10.0.0.1:8080",
			want:       "203.0.113.5",
		},
		{
			name:       "X-Forwarded-For single entry",
			header:     map[string]string{"X-Forwarded-For": "203.0.113.5"},
			remoteAddr: "10.0.0.1:8080",
			want:       "203.0.113.5",
		},
		{
			name:       "RemoteAddr fallback strips port",
			header:     map[string]string{},
			remoteAddr: "10.0.0.1:8080",
			want:       "10.0.0.1",
		},
		{
			name:       "ClientIp empty falls back to X-Forwarded-For",
			header:     map[string]string{"X-Forwarded-For": "192.0.2.1, 10.0.0.1"},
			remoteAddr: "10.0.0.1:8080",
			want:       "192.0.2.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newTestRequest(tt.header, tt.remoteAddr)
			assert.Equal(t, tt.want, clientIPFromRequest(req))
		})
	}
}

func TestInitRequestInfo(t *testing.T) {
	req := newTestRequest(map[string]string{
		"User-Agent":      "test-agent/1.0",
		"X-Forwarded-For": "203.0.113.9",
	}, "10.0.0.1:8080")

	ctx, info := InitRequestInfo(context.Background(), req)
	assert.Equal(t, "test-agent/1.0", info.UserAgent)
	assert.Equal(t, "203.0.113.9", info.ClientIP)
	assert.Equal(t, "/test", info.URLPath)
	assert.Equal(t, http.MethodGet, info.Method)

	// Second call returns the existing info from context.
	_, info2 := InitRequestInfo(ctx, req)
	assert.Same(t, info, info2)
}
