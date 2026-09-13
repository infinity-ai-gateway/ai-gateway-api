// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package quota_reset

import (
	"net/http"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

// TriggerResetRoute 手动触发配额周期重置任务
var TriggerResetRoute = &xreq.Endpoint{
	Path:    "/quota/trigger-reset",
	Method:  http.MethodPost,
	Handler: xreq.Convert(TriggerResetAction),
}

var _ xreq.Handler = TriggerResetAction

// TriggerResetAction 触发一次带分布式锁保护的配额重置任务
func TriggerResetAction(req *http.Request) (interface{}, error) {
	if container.QuotaResetScheduler != nil {
		container.QuotaResetScheduler.TriggerReset()
	}
	return map[string]string{"status": "ok"}, nil
}
