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

package openapi_v1

import (
	"github.com/gorilla/mux"

	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/middleware"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/api_key"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/auth"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/bfe_cluster"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/bfe_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/certificate"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/domain"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/entity_type"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/epp_assignments"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/epp_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/global_route_rules"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/model_price"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/operation_log"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/product"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/product_cluster"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/product_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/provider"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/route"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/route_tables"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/subcluster"
	"github.com/rainway-ai-gateway/ai-gateway-api/endpoints/openapi_v1/traffic"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xreq"
)

func RegisterEndpoints(router *mux.Router) *mux.Router {
	openAPIV1Router := router.PathPrefix("/open-api/v1").Subrouter()
	openAPIV1Router.Use(middleware.McProductProbe, middleware.McUserProbe)
	for _, one := range endpoints() {
		one.Register(openAPIV1Router)
	}
	return openAPIV1Router
}

func endpoints() []*xreq.Endpoint {
	return merge(
		product.Routers,
		product_cluster.Endpoints,
		certificate.Endpoints,
		product_pool.Endpoints,
		subcluster.Endpoints,
		bfe_pool.Endpoints,
		epp_pool.Endpoints,
		epp_assignments.Endpoints,
		auth.Endpoints,
		traffic.Endpoints,
		bfe_cluster.Endpoints,
		route.Endpoints,
		domain.Endpoints,
		api_key.Endpoints,
		entity_type.Endpoints,
		entity.Endpoints,
		global_route_rules.Endpoints,
		route_tables.Endpoints,
		model_price.Endpoints,
		operation_log.Endpoints,
		provider.Endpoints,
	)
}

func merge(rss ...[]*xreq.Endpoint) (rs []*xreq.Endpoint) {
	for _, r := range rss {
		rs = append(rs, r...)
	}
	return
}
