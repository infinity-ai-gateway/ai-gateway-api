// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
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

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/icluster_conf"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
)

type fakeTxn struct{}

func (f *fakeTxn) AtomExecute(ctx context.Context, do func(context.Context) error) error {
	return do(ctx)
}

type fakeDomainStorager struct {
	fetchDomainsFn  func(ctx context.Context, param *DomainFilter) ([]*Domain, error)
	createDomainFn  func(ctx context.Context, product *ibasic.Product, param *DomainParam) error
	deleteDomainFn  func(ctx context.Context, product *ibasic.Product, domain *Domain) error
}

func (f *fakeDomainStorager) FetchDomains(ctx context.Context, param *DomainFilter) ([]*Domain, error) {
	if f.fetchDomainsFn != nil {
		return f.fetchDomainsFn(ctx, param)
	}
	return nil, nil
}

func (f *fakeDomainStorager) CreateDomain(ctx context.Context, product *ibasic.Product, param *DomainParam) error {
	if f.createDomainFn != nil {
		return f.createDomainFn(ctx, product, param)
	}
	return nil
}

func (f *fakeDomainStorager) DeleteDomain(ctx context.Context, product *ibasic.Product, domain *Domain) error {
	if f.deleteDomainFn != nil {
		return f.deleteDomainFn(ctx, product, domain)
	}
	return nil
}

type fakeRouteRuleStorager struct {
	upsertProductRuleFn func(ctx context.Context, product *ibasic.Product, rule *ProductRouteRule) error
	fetchProductRuleFn  func(ctx context.Context, product *ibasic.Product, clusterList []*icluster_conf.Cluster) (*ProductRouteRule, error)
	fetchRoutRulesFn    func(ctx context.Context, products []*ibasic.Product, clusterList []*icluster_conf.Cluster) (map[int64]*ProductRouteRule, error)
}

func (f *fakeRouteRuleStorager) UpsertProductRule(ctx context.Context, product *ibasic.Product, rule *ProductRouteRule) error {
	if f.upsertProductRuleFn != nil {
		return f.upsertProductRuleFn(ctx, product, rule)
	}
	return nil
}

func (f *fakeRouteRuleStorager) FetchProductRule(ctx context.Context, product *ibasic.Product, clusterList []*icluster_conf.Cluster) (*ProductRouteRule, error) {
	if f.fetchProductRuleFn != nil {
		return f.fetchProductRuleFn(ctx, product, clusterList)
	}
	return nil, nil
}

func (f *fakeRouteRuleStorager) FetchRoutRules(ctx context.Context, products []*ibasic.Product, clusterList []*icluster_conf.Cluster) (map[int64]*ProductRouteRule, error) {
	if f.fetchRoutRulesFn != nil {
		return f.fetchRoutRulesFn(ctx, products, clusterList)
	}
	return nil, nil
}

type fakeClusterStorager struct {
	fetchClusterFn     func(ctx context.Context, param *icluster_conf.ClusterFilter) (*icluster_conf.Cluster, error)
	fetchClusterListFn func(ctx context.Context, param *icluster_conf.ClusterFilter) ([]*icluster_conf.Cluster, error)
	clusterUpdateFn    func(ctx context.Context, product *ibasic.Product, old *icluster_conf.Cluster, param *icluster_conf.ClusterParam) error
	clusterCreateFn    func(ctx context.Context, product *ibasic.Product, param *icluster_conf.ClusterParam, subClusters []*icluster_conf.SubCluster) (int64, error)
	clusterDeleteFn    func(ctx context.Context, product *ibasic.Product, cluster *icluster_conf.Cluster) error
	bindSubClusterFn   func(ctx context.Context, cluster *icluster_conf.Cluster, appendSubClusters, unbindSubClusters []*icluster_conf.SubCluster) error
	fetchLBMatrixListFn func(ctx context.Context) (map[int64]map[string]map[string]int, error)
}

func (f *fakeClusterStorager) FetchCluster(ctx context.Context, param *icluster_conf.ClusterFilter) (*icluster_conf.Cluster, error) {
	if f.fetchClusterFn != nil {
		return f.fetchClusterFn(ctx, param)
	}
	return nil, nil
}

func (f *fakeClusterStorager) FetchClusterList(ctx context.Context, param *icluster_conf.ClusterFilter) ([]*icluster_conf.Cluster, error) {
	if f.fetchClusterListFn != nil {
		return f.fetchClusterListFn(ctx, param)
	}
	return nil, nil
}

func (f *fakeClusterStorager) ClusterUpdate(ctx context.Context, product *ibasic.Product, old *icluster_conf.Cluster, param *icluster_conf.ClusterParam) error {
	if f.clusterUpdateFn != nil {
		return f.clusterUpdateFn(ctx, product, old, param)
	}
	return nil
}

func (f *fakeClusterStorager) ClusterCreate(ctx context.Context, product *ibasic.Product, param *icluster_conf.ClusterParam, subClusters []*icluster_conf.SubCluster) (int64, error) {
	if f.clusterCreateFn != nil {
		return f.clusterCreateFn(ctx, product, param, subClusters)
	}
	return 0, nil
}

func (f *fakeClusterStorager) ClusterDelete(ctx context.Context, product *ibasic.Product, cluster *icluster_conf.Cluster) error {
	if f.clusterDeleteFn != nil {
		return f.clusterDeleteFn(ctx, product, cluster)
	}
	return nil
}

func (f *fakeClusterStorager) BindSubCluster(ctx context.Context, cluster *icluster_conf.Cluster, appendSubClusters, unbindSubClusters []*icluster_conf.SubCluster) error {
	if f.bindSubClusterFn != nil {
		return f.bindSubClusterFn(ctx, cluster, appendSubClusters, unbindSubClusters)
	}
	return nil
}

func (f *fakeClusterStorager) FetchLBMatrixList(ctx context.Context) (map[int64]map[string]map[string]int, error) {
	if f.fetchLBMatrixListFn != nil {
		return f.fetchLBMatrixListFn(ctx)
	}
	return nil, nil
}

type fakeProductStorager struct {
	fetchProductsFn func(ctx context.Context, param *ibasic.ProductFilter) ([]*ibasic.Product, error)
	deleteProductFn func(ctx context.Context, p *ibasic.Product) error
	createProductFn func(ctx context.Context, p *ibasic.ProductParam) error
	updateProductFn func(ctx context.Context, p *ibasic.Product, newVal *ibasic.ProductParam) error
}

func (f *fakeProductStorager) FetchProducts(ctx context.Context, param *ibasic.ProductFilter) ([]*ibasic.Product, error) {
	if f.fetchProductsFn != nil {
		return f.fetchProductsFn(ctx, param)
	}
	return nil, nil
}

func (f *fakeProductStorager) DeleteProduct(ctx context.Context, p *ibasic.Product) error {
	if f.deleteProductFn != nil {
		return f.deleteProductFn(ctx, p)
	}
	return nil
}

func (f *fakeProductStorager) CreateProduct(ctx context.Context, p *ibasic.ProductParam) error {
	if f.createProductFn != nil {
		return f.createProductFn(ctx, p)
	}
	return nil
}

func (f *fakeProductStorager) UpdateProduct(ctx context.Context, p *ibasic.Product, newVal *ibasic.ProductParam) error {
	if f.updateProductFn != nil {
		return f.updateProductFn(ctx, p, newVal)
	}
	return nil
}

type fakeVersionControlStorager struct {
	upsertFn func(ctx context.Context, css *iversion_control.ExportData) (string, error)
}

func (f *fakeVersionControlStorager) UpsertConfigLastExportedVersion(ctx context.Context, css *iversion_control.ExportData) (string, error) {
	if f.upsertFn != nil {
		return f.upsertFn(ctx, css)
	}
	return "", nil
}

type fakeEPPAssignmentResolver struct {
	endpointsFn func(ctx context.Context, clusterName string) ([]string, bool, error)
}

func (f *fakeEPPAssignmentResolver) GetAssignmentEndpoints(ctx context.Context, clusterName string) ([]string, bool, error) {
	if f.endpointsFn != nil {
		return f.endpointsFn(ctx, clusterName)
	}
	return nil, false, nil
}
