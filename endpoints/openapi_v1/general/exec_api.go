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

package general

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

const (
	timeOutSecond    = 10
	sleepMicroSecond = 10
	retry            = 1
)

var _ xreq.Handler = ExecAPIAction

var ExecAPIRoute = &xreq.Endpoint{
	Path:       "/general/actions/exec-api",
	Method:     http.MethodPost,
	Handler:    xreq.Convert(ExecAPIAction),
	Authorizer: iauth.FA(iauth.FeatureGeneral, iauth.ActionCreate),
}

type RequestParams struct {
	Schema  string            `json:"schema" validate:"required,oneof=http https"`
	URI     string            `json:"uri" validate:"required,min=1"`
	Hosts   []string          `json:"hosts" validate:"required,min=1"`
	Headers map[string]string `json:"headers"`
}

func ExecAPIAction(req *http.Request) (interface{}, error) {
	param := &RequestParams{}
	err := xreq.BindJSON(req, param)
	if err != nil {
		return nil, err
	}

	return execAPIProcess(req.Context(), param)
}

func execAPIProcess(ctx context.Context, param *RequestParams) (interface{}, error) {
	url := param.Schema
	var errStr string

	for i, host := range param.Hosts {
		reqURL := host
		if param.URI == "/" {
			reqURL = reqURL + param.URI
		} else {
			if param.URI != "" {
				reqURL = path.Join(reqURL, param.URI)
			}
		}
		reqURL = fmt.Sprintf("%s://%s", url, reqURL)

		response, err := lib.ReadWithRetry(reqURL, timeOutSecond, param.Headers, retry, sleepMicroSecond, nil)
		if err != nil {
			eStr := fmt.Sprintf("exec-api, request index:%d url:%s is error:%s", i, reqURL, err.Error())
			stateful.AccessLogger.Error(eStr)
			if errStr == "" {
				errStr = eStr
			} else {
				errStr += "\n" + eStr
			}
			continue
		}

		resp := make(map[string]interface{})
		resp["result"] = string(response)
		return resp, nil
	}

	return nil, xerror.WrapParamError(fmt.Errorf("%s", errStr))
}
