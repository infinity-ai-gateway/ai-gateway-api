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

package entity

import (
	"net/http"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iauth"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful/container"
)

var EntityCreateRoute = &xreq.Endpoint{
	Path:       "/entities",
	Method:     http.MethodPost,
	Handler:    xreq.Convert(EntityCreateAction),
	Authorizer: iauth.FA(iauth.FeatureEntity, iauth.ActionCreate),
}

func EntityCreateAction(req *http.Request) (interface{}, error) {
	param := &entity.EntityParam{}
	if err := xreq.BindJSON(req, param); err != nil {
		return nil, err
	}

	if err := validateEntityParam(param, true); err != nil {
		return nil, err
	}

	entityType, err := container.EntityTypeStorager.FetchEntityType(req.Context(), &entity.EntityTypeFilter{TypeName: param.Type})
	if err != nil {
		return nil, err
	}
	if entityType == nil {
		return nil, xerror.WrapParamErrorWithMsg("type does not exist")
	}

	if param.ParentID != nil && *param.ParentID != "" {
		parentEntity, err := container.EntityManager.FetchEntity(req.Context(), &entity.EntityFilter{EntityID: param.ParentID})
		if err != nil {
			return nil, err
		}
		if parentEntity == nil {
			return nil, xerror.WrapParamErrorWithMsg("parent_id does not exist")
		}
		if parentEntity.Type == nil {
			return nil, xerror.WrapParamErrorWithMsg("parent entity has no type")
		}
		parentType, err := container.EntityTypeStorager.FetchEntityType(req.Context(), &entity.EntityTypeFilter{TypeName: parentEntity.Type})
		if err != nil {
			return nil, err
		}
		if parentType == nil || parentType.Level == nil {
			return nil, xerror.WrapParamErrorWithMsg("parent entity type is invalid")
		}
		if *parentType.Level >= *entityType.Level {
			return nil, xerror.WrapParamErrorWithMsg("parent level must be lower than current level")
		}
	}

	existingEntities, err := container.EntityManager.FetchEntityList(req.Context(), &entity.EntityFilter{Type: param.Type, Name: param.Name})
	if err != nil {
		return nil, err
	}
	if len(existingEntities) > 0 {
		return nil, xerror.WrapDuplicateData("entity")
	}

	if param.EntityID == nil || *param.EntityID == "" {
		generatedID, err := container.EntityIDGenerator.Generate(req.Context())
		if err != nil {
			return nil, xerror.WrapModelError(err)
		}
		param.EntityID = &generatedID
	}

	if param.QuotaPlan == nil {
		param.QuotaPlan = &shared.QuotaPlanParam{
			Unlimited:             lib.PBool(true),
			PassWhenNoEnoughQuota: lib.PBool(false),
		}
	}

	if param.RateLimitPolicy == nil {
		param.RateLimitPolicy = &shared.RateLimitPolicyParam{
			Enabled: lib.PBool(false),
			Rules: &shared.RateLimitRules{
				TpmConfigs:     []shared.TPMConfig{},
				RpmConfigs:     []shared.RPMConfig{},
				MaxConcurrency: lib.PInt(0),
			},
		}
	}

	if param.RouteRules == nil {
		param.RouteRules = &shared.RouteRulesParam{
			Enabled: lib.PBool(false),
			Rules:   []*shared.AiRouteRuleParam{},
		}
	}

	if _, err := container.EntityManager.CreateEntity(req.Context(), param); err != nil {
		return nil, err
	}

	return container.EntityManager.FetchEntity(req.Context(), &entity.EntityFilter{EntityID: param.EntityID})
}
