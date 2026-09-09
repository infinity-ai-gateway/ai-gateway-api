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
	"errors"
	"testing"

	"github.com/bfenetworks/bfe/bfe_config/bfe_cluster_conf/cluster_conf"
	"github.com/bfenetworks/bfe/bfe_config/bfe_route_conf/host_rule_conf"
	"github.com/bfenetworks/bfe/bfe_config/bfe_route_conf/route_rule_conf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/icluster_conf"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
)

func TestRouteRuleExportData_UpdateVersion(t *testing.T) {
	data := &RouteRuleExportData{
		RouteTable:  &route_rule_conf.RouteTableFile{},
		HostTable:   &host_rule_conf.HostTableConf{},
		ClusterConf: &cluster_conf.BfeClusterConf{},
	}
	require.NoError(t, data.UpdateVersion("v1"))
	assert.Equal(t, "v1", data.Version)
	assert.Equal(t, "v1", *data.RouteTable.Version)
	assert.Equal(t, "v1", *data.HostTable.Version)
	assert.Equal(t, "v1", *data.ClusterConf.Version)
}

func TestRouteRuleManager_ExportRouteRule(t *testing.T) {
	ctx := context.Background()

	t.Run("unchanged", func(t *testing.T) {
		rm := newManagerForExport(t, iversion_control.ZeroVersion)
		got, err := rm.ExportRouteRule(ctx, iversion_control.ZeroVersion)
		require.NoError(t, err)
		assert.Nil(t, got)
	})

	t.Run("changed", func(t *testing.T) {
		rm := newManagerForExport(t, "20240102000000")
		got, err := rm.ExportRouteRule(ctx, "old")
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, "20240102000000", got.Version)
	})

	t.Run("export error", func(t *testing.T) {
		vcs := &fakeVersionControlStorager{
			upsertFn: func(ctx context.Context, css *iversion_control.ExportData) (string, error) {
				return "", errors.New("upsert failed")
			},
		}
		vcm := iversion_control.NewVersionControllerManager(&fakeTxn{}, vcs)
		rm := NewRouteRuleManager(&fakeTxn{}, &fakeRouteRuleStorager{}, &fakeClusterStorager{}, &fakeProductStorager{}, vcm, &fakeDomainStorager{})
		_, err := rm.ExportRouteRule(ctx, "")
		require.Error(t, err)
	})
}

func TestRouteRuleManager_exportRouteRule(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		rm := newManagerForExport(t, iversion_control.ZeroVersion)
		ed, err := rm.exportRouteRule(ctx)
		require.NoError(t, err)
		require.NotNil(t, ed)
		assert.Equal(t, ConfigTopicRouteRule, ed.Topic)
		data := ed.DataWithoutVersion.(*RouteRuleExportData)
		assert.NotNil(t, data.HostTable)
		assert.NotNil(t, data.RouteTable)
		assert.NotNil(t, data.ClusterConf)
	})

	t.Run("domain fetch error", func(t *testing.T) {
		rm := NewRouteRuleManager(&fakeTxn{}, &fakeRouteRuleStorager{}, &fakeClusterStorager{}, &fakeProductStorager{}, nil, &fakeDomainStorager{
			fetchDomainsFn: func(ctx context.Context, param *DomainFilter) ([]*Domain, error) {
				return nil, errors.New("db down")
			},
		})
		_, err := rm.exportRouteRule(ctx)
		require.Error(t, err)
	})

	t.Run("orphan domain", func(t *testing.T) {
		rm := NewRouteRuleManager(&fakeTxn{}, &fakeRouteRuleStorager{}, &fakeClusterStorager{
			fetchClusterListFn: func(ctx context.Context, param *icluster_conf.ClusterFilter) ([]*icluster_conf.Cluster, error) {
				return nil, nil
			},
		}, &fakeProductStorager{
			fetchProductsFn: func(ctx context.Context, param *ibasic.ProductFilter) ([]*ibasic.Product, error) {
				return nil, nil
			},
		}, nil, &fakeDomainStorager{
			fetchDomainsFn: func(ctx context.Context, param *DomainFilter) ([]*Domain, error) {
				return []*Domain{{ProductID: 1, Name: "a.example.com"}}, nil
			},
		})
		_, err := rm.exportRouteRule(ctx)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Domain refer Not Exist Product 1")
	})

	t.Run("EPP cluster exports ordered EPPAddr from assignment", func(t *testing.T) {
		rm := newManagerForExport(t, "20240102000000")
		rm.SetEPPAssignmentResolver(&fakeEPPAssignmentResolver{
			endpointsFn: func(ctx context.Context, clusterName string) ([]string, bool, error) {
				return []string{"10.0.0.1:9002", "10.0.0.2:9002"}, true, nil
			},
		})
		rm.clusterStorager = &fakeClusterStorager{
			fetchClusterListFn: func(ctx context.Context, param *icluster_conf.ClusterFilter) ([]*icluster_conf.Cluster, error) {
				c := newTestCluster()
				c.BalanceMode = icluster_conf.BalanceModeEPP
				return []*icluster_conf.Cluster{c}, nil
			},
		}

		ed, err := rm.exportRouteRule(ctx)
		require.NoError(t, err)
		data := ed.DataWithoutVersion.(*RouteRuleExportData)
		cConf := (*data.ClusterConf.Config)["c1"]
		require.NotNil(t, cConf.GslbBasic.EPPAddr)
		assert.Equal(t, []string{"10.0.0.1:9002", "10.0.0.2:9002"}, *cConf.GslbBasic.EPPAddr)
		require.NotNil(t, cConf.GslbBasic.BalanceMode)
		assert.Equal(t, icluster_conf.BalanceModeEPP, *cConf.GslbBasic.BalanceMode)
	})

	t.Run("EPP cluster without assignment degrades to WRR without failing export", func(t *testing.T) {
		rm := newManagerForExport(t, "20240102000000")
		rm.SetEPPAssignmentResolver(&fakeEPPAssignmentResolver{
			endpointsFn: func(ctx context.Context, clusterName string) ([]string, bool, error) {
				return nil, false, nil
			},
		})
		rm.clusterStorager = &fakeClusterStorager{
			fetchClusterListFn: func(ctx context.Context, param *icluster_conf.ClusterFilter) ([]*icluster_conf.Cluster, error) {
				c := newTestCluster()
				c.BalanceMode = icluster_conf.BalanceModeEPP
				return []*icluster_conf.Cluster{c}, nil
			},
		}

		ed, err := rm.exportRouteRule(ctx)
		require.NoError(t, err)
		data := ed.DataWithoutVersion.(*RouteRuleExportData)
		cConf := (*data.ClusterConf.Config)["c1"]
		assert.Nil(t, cConf.GslbBasic.EPPAddr)
		require.NotNil(t, cConf.GslbBasic.BalanceMode)
		assert.Equal(t, icluster_conf.BalanceModeWRR, *cConf.GslbBasic.BalanceMode)
	})
}

func newTestCluster() *icluster_conf.Cluster {
	return &icluster_conf.Cluster{
		ID:   1,
		Name: "c1",
		Basic: &icluster_conf.ClusterBasic{
			Protocol: lib.PString("http"),
			Connection: &icluster_conf.ClusterBasicConnection{
				MaxIdleConnPerRs:    10,
				CancelOnClientClose: false,
			},
			Retries: &icluster_conf.ClusterBasicRetries{
				MaxRetryInSubcluster:    1,
				MaxRetryCrossSubcluster: 1,
			},
			Buffers: &icluster_conf.ClusterBasicBuffers{
				ReqWriteBufferSize: 1024,
				ReqFlushInterval:   100,
				ResFlushInterval:   100,
			},
			Timeouts: &icluster_conf.ClusterBasicTimeouts{
				TimeoutConnServ:        1000,
				TimeoutResponseHeader:  1000,
				TimeoutReadbodyClient:  1000,
				TimeoutReadClientAgain: 1000,
				TimeoutWriteClient:     1000,
			},
		},
		StickySessions: &icluster_conf.ClusterStickySessions{
			SessionSticky: false,
			HashStrategy:  0,
			HashHeader:    "",
		},
		PassiveHealthCheck: &icluster_conf.ClusterPassiveHealthCheck{
			Schema:     "http",
			Interval:   10,
			Failnum:    3,
			Statuscode: 500,
			Host:       "",
			Uri:        "/",
		},
	}
}

func newManagerForExport(t *testing.T, version string) *RouteRuleManager {
	t.Helper()
	vcs := &fakeVersionControlStorager{
		upsertFn: func(ctx context.Context, css *iversion_control.ExportData) (string, error) {
			return version, nil
		},
	}
	vcm := iversion_control.NewVersionControllerManager(&fakeTxn{}, vcs)
	clusterStore := &fakeClusterStorager{
		fetchClusterListFn: func(ctx context.Context, param *icluster_conf.ClusterFilter) ([]*icluster_conf.Cluster, error) {
			return []*icluster_conf.Cluster{newTestCluster()}, nil
		},
	}
	productStore := &fakeProductStorager{
		fetchProductsFn: func(ctx context.Context, param *ibasic.ProductFilter) ([]*ibasic.Product, error) {
			return []*ibasic.Product{{ID: 1, Name: defaultProduct}}, nil
		},
	}
	domainStore := &fakeDomainStorager{
		fetchDomainsFn: func(ctx context.Context, param *DomainFilter) ([]*Domain, error) {
			return []*Domain{{ProductID: 1, Name: "a.example.com"}}, nil
		},
	}
	ruleStore := &fakeRouteRuleStorager{
		fetchRoutRulesFn: func(ctx context.Context, products []*ibasic.Product, clusterList []*icluster_conf.Cluster) (map[int64]*ProductRouteRule, error) {
			return map[int64]*ProductRouteRule{1: {
				AdvanceRouteRules: []*AdvanceRouteRule{{Expression: "default_t()", ClusterName: "c1"}},
			}}, nil
		},
	}
	return NewRouteRuleManager(&fakeTxn{}, ruleStore, clusterStore, productStore, vcm, domainStore)
}
