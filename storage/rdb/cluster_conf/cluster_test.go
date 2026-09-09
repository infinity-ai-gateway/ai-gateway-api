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

package cluster_conf

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/icluster_conf"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testClustersSchema = `
CREATE TABLE clusters (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  description TEXT NOT NULL DEFAULT 'no desc',
  product_id INTEGER NOT NULL,
  protocol TEXT NOT NULL DEFAULT 'http',
  max_idle_conn_per_host INTEGER NOT NULL DEFAULT 2,
  timeout_conn_serv INTEGER NOT NULL DEFAULT 50000,
  timeout_response_header INTEGER NOT NULL DEFAULT 50000,
  timeout_readbody_client INTEGER NOT NULL DEFAULT 30000,
  timeout_read_client_again INTEGER NOT NULL DEFAULT 30000,
  timeout_write_client INTEGER NOT NULL DEFAULT 60000,
  healthcheck_schem TEXT NOT NULL DEFAULT 'http',
  healthcheck_interval INTEGER NOT NULL DEFAULT 1000,
  healthcheck_failnum INTEGER NOT NULL DEFAULT 10,
  healthcheck_host TEXT NOT NULL DEFAULT '',
  healthcheck_uri TEXT NOT NULL DEFAULT '/',
  healthcheck_statuscode INTEGER NOT NULL DEFAULT 200,
  clientip_carry INTEGER NOT NULL DEFAULT 0,
  port_carry INTEGER NOT NULL DEFAULT 0,
  max_retry_in_cluster INTEGER NOT NULL DEFAULT 3,
  max_retry_cross_cluster INTEGER NOT NULL DEFAULT 0,
  ready INTEGER NOT NULL DEFAULT 1,
  hash_strategy INTEGER NOT NULL DEFAULT 0,
  cookie_key TEXT NOT NULL DEFAULT 'BAIDUID',
  hash_header TEXT NOT NULL DEFAULT 'Cookie:BAIDUID',
  session_sticky INTEGER NOT NULL DEFAULT 0,
  req_write_buffer_size INTEGER NOT NULL DEFAULT 512,
  req_flush_interval INTEGER NOT NULL DEFAULT 0,
  res_flush_interval INTEGER NOT NULL DEFAULT 20,
  cancel_on_client_close INTEGER NOT NULL DEFAULT 0,
  failure_status INTEGER NOT NULL DEFAULT 0,
  max_conns_per_host INTEGER NOT NULL DEFAULT 0,
  llm_config TEXT,
  balance_mode TEXT NOT NULL DEFAULT 'WRR',
  epp_config TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (name)
);
CREATE TABLE lb_matrices (
  cluster_id INTEGER PRIMARY KEY AUTOINCREMENT,
  lb_matrix TEXT NOT NULL,
  product_id INTEGER NOT NULL,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`

type fakeSubClusterStorager struct{}

func (f *fakeSubClusterStorager) FetchSubClusterList(ctx context.Context, param *icluster_conf.SubClusterFilter) ([]*icluster_conf.SubCluster, error) {
	return nil, nil
}

func (f *fakeSubClusterStorager) CreateSubCluster(ctx context.Context, param *icluster_conf.SubClusterParam) error {
	return nil
}

func (f *fakeSubClusterStorager) DeleteSubCluster(ctx context.Context, param *icluster_conf.SubCluster) error {
	return nil
}

func (f *fakeSubClusterStorager) UpdateSubCluster(ctx context.Context, one *icluster_conf.SubCluster, param *icluster_conf.SubClusterParam) error {
	return nil
}

func setupTestClusterStorager(t *testing.T) *RDBClusterStorager {
	if stateful.DefaultConfig == nil {
		stateful.DefaultConfig = &stateful.Config{}
	}

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	_, err = db.Exec(testClustersSchema)
	require.NoError(t, err)

	factory := lib.DBContextFactory(func(ctx context.Context, ops ...*lib.Op) (*lib.DBContext, error) {
		return lib.NewDBContext(ctx, db), nil
	})

	return NewRDBClusterStorager(factory, &fakeSubClusterStorager{})
}

func TestRDBClusterStorager_BalanceModeEppConfig(t *testing.T) {
	ctx := context.Background()
	product := &ibasic.Product{ID: 2, Name: "test"}
	s := setupTestClusterStorager(t)

	t.Run("create with EPP mode and epp_config round-trips", func(t *testing.T) {
		eppConfig := `{"scheduling_profile":"balanced"}`
		id, err := s.ClusterCreate(ctx, product, &icluster_conf.ClusterParam{
			Name:        lib.PString("c-epp"),
			ProductID:   lib.PInt64(product.ID),
			BalanceMode: lib.PString(icluster_conf.BalanceModeEPP),
			EppConfig:   lib.PString(eppConfig),
		}, nil)
		require.NoError(t, err)

		cluster, err := s.FetchCluster(ctx, &icluster_conf.ClusterFilter{Name: lib.PString("c-epp")})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		assert.Equal(t, id, cluster.ID)
		assert.Equal(t, icluster_conf.BalanceModeEPP, cluster.BalanceMode)
		assert.Equal(t, eppConfig, cluster.EppConfig)
	})

	t.Run("create without fields defaults to WRR and empty config", func(t *testing.T) {
		_, err := s.ClusterCreate(ctx, product, &icluster_conf.ClusterParam{
			Name:      lib.PString("c-wrr"),
			ProductID: lib.PInt64(product.ID),
		}, nil)
		require.NoError(t, err)

		cluster, err := s.FetchCluster(ctx, &icluster_conf.ClusterFilter{Name: lib.PString("c-wrr")})
		require.NoError(t, err)
		require.NotNil(t, cluster)
		assert.Equal(t, icluster_conf.BalanceModeWRR, cluster.BalanceMode)
		assert.Equal(t, "", cluster.EppConfig)
	})

	t.Run("update without carrying fields retains stored values", func(t *testing.T) {
		cluster, err := s.FetchCluster(ctx, &icluster_conf.ClusterFilter{Name: lib.PString("c-epp")})
		require.NoError(t, err)
		require.NotNil(t, cluster)

		// EPP -> WRR with neither balance_mode nor epp_config carried: the row
		// keeps its stored values (dormant retention).
		require.NoError(t, s.ClusterUpdate(ctx, product, cluster, &icluster_conf.ClusterParam{}))

		updated, err := s.FetchCluster(ctx, &icluster_conf.ClusterFilter{Name: lib.PString("c-epp")})
		require.NoError(t, err)
		assert.Equal(t, icluster_conf.BalanceModeEPP, updated.BalanceMode)
		assert.NotEmpty(t, updated.EppConfig)
	})

	t.Run("update carrying fields overwrites them", func(t *testing.T) {
		cluster, err := s.FetchCluster(ctx, &icluster_conf.ClusterFilter{Name: lib.PString("c-epp")})
		require.NoError(t, err)
		require.NotNil(t, cluster)

		require.NoError(t, s.ClusterUpdate(ctx, product, cluster, &icluster_conf.ClusterParam{
			BalanceMode: lib.PString(icluster_conf.BalanceModeWRR),
			EppConfig:   lib.PString(`{"scheduling_profile":"latency-first"}`),
		}))

		updated, err := s.FetchCluster(ctx, &icluster_conf.ClusterFilter{Name: lib.PString("c-epp")})
		require.NoError(t, err)
		assert.Equal(t, icluster_conf.BalanceModeWRR, updated.BalanceMode)
		assert.Equal(t, `{"scheduling_profile":"latency-first"}`, updated.EppConfig)
	})
}
