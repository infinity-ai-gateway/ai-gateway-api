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
//limitations under the License. All rights reserved.

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

package iroute_conf

import (
	"context"

	"github.com/bfenetworks/bfe/bfe_config/bfe_cluster_conf/cluster_conf"
	"github.com/bfenetworks/bfe/bfe_config/bfe_route_conf/host_rule_conf"
	"github.com/bfenetworks/bfe/bfe_config/bfe_route_conf/route_rule_conf"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/icluster_conf"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/imodel_price"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iprovider"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
)

type RouteRuleExportData struct {
	Version     string
	HostTable   *host_rule_conf.HostTableConf
	RouteTable  *route_rule_conf.RouteTableFile
	ClusterConf *cluster_conf.BfeClusterConf
}

func (rred *RouteRuleExportData) UpdateVersion(version string) error {
	rred.Version = version
	rred.RouteTable.Version = &version
	rred.HostTable.Version = &version
	rred.ClusterConf.Version = &version

	return nil
}

const (
	ConfigTopicRouteRule = "route_rule"
)

func (rm *RouteRuleManager) ExportRouteRule(ctx context.Context, lastVersion string) (*RouteRuleExportData, error) {
	ed, err := rm.versionControlManager.ExportConfig(ctx, ConfigTopicRouteRule, rm.exportRouteRule)
	if err != nil {
		return nil, err
	}

	conf := ed.DataWithoutVersion.(*RouteRuleExportData)
	if conf.Version == lastVersion {
		return nil, nil
	}

	return conf, nil
}

func (rm *RouteRuleManager) exportRouteRule(ctx context.Context) (*iversion_control.ExportData, error) {
	domains, err := rm.domainStorager.FetchDomains(ctx, nil)
	if err != nil {
		return nil, err
	}

	clusters, err := rm.clusterStorager.FetchClusterList(ctx, nil)
	if err != nil {
		return nil, err
	}
	clusters = icluster_conf.AppendAdvancedRuleCluster(clusters)

	providerModelTable := map[string][]*imodel_price.ModelPrice{}
	if rm.modelPriceStorager != nil {
		allPrices, _, err := rm.modelPriceStorager.FetchModelPriceList(ctx, &imodel_price.ModelPriceFilter{})
		if err != nil {
			return nil, err
		}
		providerModelTable = imodel_price.GroupByProvider(allPrices)
	}

	providerKeyTable := map[string][]iprovider.ProviderKey{}
	providerProtocolTable := map[string][]string{}
	providerPricingTable := map[string]icluster_conf.ProviderPricingInfo{}
	if rm.providerStorager != nil {
		providers, _, err := rm.providerStorager.FetchProviderList(ctx, &iprovider.ProviderFilter{})
		if err != nil {
			return nil, err
		}
		for _, p := range providers {
			if p != nil {
				providerKeyTable[p.Name] = p.Keys
				providerProtocolTable[p.Name] = p.ModelProtocols
				providerPricingTable[p.Name] = icluster_conf.ProviderPricingInfo{
					TimeZone: p.TimeZone,
					Tiers:    convertPricingTiers(p.Tiers),
				}
			}
		}
	}

	products, err := rm.productStorager.FetchProducts(ctx, nil)
	if err != nil {
		return nil, err
	}

	productMapID2Name := map[int64]string{}
	tmp := ibasic.ProductIDMap(products)
	for _, domain := range domains {
		product, ok := tmp[domain.ProductID]
		if !ok {
			return nil, xerror.WrapDirtyDataErrorWithMsg("Domain refer Not Exist Product %d", domain.ProductID)
		}
		productMapID2Name[domain.ProductID] = product.Name
	}

	//insert default product
	for _, product := range products {
		if product.Name == defaultProduct {
			productMapID2Name[product.ID] = defaultProduct
			defaultDomain := &Domain{
				ProductID: product.ID,
				Name:      "127.0.0.1",
			}
			domains = append(domains, defaultDomain)
		}
	}

	routeRules, err := rm.storager.FetchRoutRules(ctx, products, clusters)
	if err != nil {
		return nil, err
	}

	emptyVersion := iversion_control.ZeroVersion
	rred := &RouteRuleExportData{
		Version:     emptyVersion,
		RouteTable:  newRouteTableFile(emptyVersion, productMapID2Name, routeRules),
		HostTable:   newHostTableConf(emptyVersion, productMapID2Name, domains),
		ClusterConf: icluster_conf.NewBfeClusterConf(ctx, emptyVersion, clusters, providerModelTable, providerKeyTable, providerProtocolTable, providerPricingTable, rm.eppAssignmentResolver),
	}

	return &iversion_control.ExportData{
		Topic:              ConfigTopicRouteRule,
		DataWithoutVersion: rred,
	}, nil
}

var defaultProduct = "AI_product"

func newRouteTableFile(version string, productMapID2Name map[int64]string,
	routeRules map[int64]*ProductRouteRule) *route_rule_conf.RouteTableFile {

	basicRule := route_rule_conf.ProductBasicRouteRuleFile{}
	advanceRule := route_rule_conf.ProductAdvancedRouteRuleFile{}

	newAdvanceRouteRuleFiles := func(arrs []*AdvanceRouteRule) (as []route_rule_conf.AdvancedRouteRuleFile) {
		for _, arr := range arrs {
			as = append(as, route_rule_conf.AdvancedRouteRuleFile{
				Cond:        &arr.Expression,
				ClusterName: &arr.ClusterName,
			})
		}

		return as
	}

	// sort by product_id
	if len(routeRules) > 0 {
		maxProductID := len(routeRules)
		for pid := range routeRules {
			maxProductID += int(pid)
			break
		}

		for _, pid := range lib.SortMapInt642String(productMapID2Name) {
			rule, ok := routeRules[pid]
			if !ok {
				continue
			}

			pName, ok := productMapID2Name[pid]
			if !ok {
				continue
			}

			advanceRule[pName] = newAdvanceRouteRuleFiles(rule.AdvanceRouteRules)
		}
	}

	return &route_rule_conf.RouteTableFile{
		Version:     &version,
		BasicRule:   &basicRule,
		ProductRule: &advanceRule,
	}
}

func convertPricingTiers(tiers []iprovider.PricingTier) []cluster_conf.PriceTier {
	rst := make([]cluster_conf.PriceTier, 0, len(tiers))
	for _, t := range tiers {
		ranges := make([]cluster_conf.TimeRange, 0, len(t.TimeRanges))
		for _, r := range t.TimeRanges {
			ranges = append(ranges, cluster_conf.TimeRange{
				Weekdays: r.Weekdays,
				Start:    r.Start,
				End:      r.End,
			})
		}
		rst = append(rst, cluster_conf.PriceTier{
			Name:       t.Name,
			TimeRanges: ranges,
		})
	}
	return rst
}
