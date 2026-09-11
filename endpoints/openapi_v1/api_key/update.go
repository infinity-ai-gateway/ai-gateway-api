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

package api_key

import (
	"context"
	"fmt"
	"net/http"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/api_key"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

var _ xreq.Handler = APIKeyUpdateAction

var APIKeyUpdateRoute = &xreq.Endpoint{
	Path:       "/api-keys/{id}",
	Method:     http.MethodPatch,
	Handler:    xreq.Convert(APIKeyUpdateAction),
	Authorizer: iauth.FA(iauth.FeatureAPIKey, iauth.ActionUpdate),
}

func APIKeyUpdateAction(req *http.Request) (interface{}, error) {
	// uri param
	oneReq, err := newReq4One(req)
	if err != nil {
		return nil, err
	}

	// body param
	param := &api_key.APIKeyParam{}
	if err := xreq.BindJSON(req, param); err != nil {
		return nil, err
	}

	param.ID = lib.PString(*oneReq.ID)

	return APIKeyUpdateProcess(req.Context(), param, defaultProduct())
}

func APIKeyUpdateProcess(ctx context.Context, param *api_key.APIKeyParam, product *ibasic.Product) (*api_key.APIKeyParam, error) {
	if err := checkUpdateAPIKey(param, product.Name); err != nil {
		return nil, xerror.WrapParamError(err)
	}

	existing, err := container.APIKeyManager.FetchAPIKey(ctx, &api_key.APIKeyFilter{
		ID:          param.ID,
		ProductName: &product.Name,
	})
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, xerror.WrapRecordNotExist("API-Key")
	}

	// 检查 entity_id 是否存在（如果传入的话）
	if param.EntityID != nil && *param.EntityID != "" {
		entity, err := container.EntityManager.FetchEntity(ctx, &entity.EntityFilter{EntityID: param.EntityID})
		if err != nil {
			return nil, err
		}
		if entity == nil {
			return nil, xerror.WrapParamErrorWithMsg("%s", fmt.Sprintf("Entity not found: %s", *param.EntityID))
		}
	}

	err = container.APIKeyManager.UpdateAPIKey(ctx, &api_key.APIKeyFilter{
		ID:          param.ID,
		ProductName: &product.Name,
	}, &api_key.APIKeyParam{
		Enable:          param.Enable,
		Key:             param.Key,
		Description:     param.Description,
		UnlimitedQuota:  param.UnlimitedQuota,
		ExpiredTime:     param.ExpiredTime,
		Models:          param.Models,
		Subnet:          param.Subnet,
		EntityID:        param.EntityID,
		ProductName:     &product.Name,
		QuotaPlan:       param.QuotaPlan,
		RateLimitPolicy: param.RateLimitPolicy,
		RouteRules:      param.RouteRules,
	})

	if err != nil {
		return nil, err
	}

	// 获取更新后的 API-Key
	updated, err := container.APIKeyManager.FetchAPIKey(ctx, &api_key.APIKeyFilter{
		ID:          param.ID,
		ProductName: &product.Name,
	})
	if err != nil {
		return nil, err
	}

	// 当 quota_plan 发生变更时，按需调整配额余额（普通属性修改不重置使用量）。
	if param.QuotaPlan != nil && updated != nil && updated.QuotaPlanID != nil {
		if err := container.QuotaPlanManager.ApplyQuotaPlanChange(ctx, *updated.QuotaPlanID, existing.QuotaPlan, param.QuotaPlan); err != nil {
			return nil, err
		}
	}

	return updated, nil
}
