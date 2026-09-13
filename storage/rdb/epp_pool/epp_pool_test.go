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

package epp_pool

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/epp_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStorager(t *testing.T) (*EppPoolStorager, *sql.DB) {
	// The DAO layer consults stateful.DefaultConfig when recording SQL.
	if stateful.DefaultConfig == nil {
		stateful.DefaultConfig = &stateful.Config{}
	}

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
CREATE TABLE epp_instances (
  id TEXT NOT NULL PRIMARY KEY,
  host TEXT NOT NULL,
  port INTEGER NOT NULL,
  group_name TEXT NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (host, port)
);
CREATE TABLE epp_assignments (
  cluster TEXT NOT NULL PRIMARY KEY,
  group_name TEXT NOT NULL,
  primary_instance_id TEXT NOT NULL,
  create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  update_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);`)
	require.NoError(t, err)

	factory := lib.DBContextFactory(func(ctx context.Context, ops ...*lib.Op) (*lib.DBContext, error) {
		return lib.NewDBContext(ctx, db), nil
	})

	return NewEppPoolStorager(factory), db
}

func TestEppPoolStorager_Instances(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestStorager(t)

	affected, err := s.CreateInstances(ctx, []*epp_pool.InstanceParam{
		{ID: "epp-a", Host: "10.0.0.1", Port: 9002, GroupName: "g1"},
		{ID: "epp-b", Host: "10.0.0.2", Port: 9002, GroupName: "g1"},
		{ID: "epp-c", Host: "10.0.0.3", Port: 9002, GroupName: "g2"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(3), affected)

	list, err := s.FetchInstanceList(ctx, &epp_pool.InstanceFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 3)

	grouped, err := s.FetchInstanceList(ctx, &epp_pool.InstanceFilter{GroupName: lib.PString("g1")})
	require.NoError(t, err)
	assert.Len(t, grouped, 2)

	one, err := s.FetchInstanceList(ctx, &epp_pool.InstanceFilter{ID: lib.PString("epp-c")})
	require.NoError(t, err)
	require.Len(t, one, 1)
	assert.Equal(t, "g2", one[0].GroupName)

	affected, err = s.DeleteInstances(ctx, &epp_pool.InstanceFilter{})
	require.NoError(t, err)
	assert.Equal(t, int64(3), affected)

	list, err = s.FetchInstanceList(ctx, &epp_pool.InstanceFilter{})
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestEppPoolStorager_Assignments(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestStorager(t)

	// Upsert inserts when absent.
	err := s.UpsertAssignment(ctx, &epp_pool.AssignmentParam{
		Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-a",
	})
	require.NoError(t, err)

	assignment, err := s.FetchAssignment(ctx, "cluster-a")
	require.NoError(t, err)
	require.NotNil(t, assignment)
	assert.Equal(t, "epp-a", assignment.PrimaryInstanceID)

	// Upsert updates when present.
	err = s.UpsertAssignment(ctx, &epp_pool.AssignmentParam{
		Cluster: "cluster-a", GroupName: "g1", PrimaryInstanceID: "epp-b",
	})
	require.NoError(t, err)

	list, err := s.FetchAssignmentList(ctx)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "epp-b", list[0].PrimaryInstanceID)

	require.NoError(t, s.DeleteAssignment(ctx, "cluster-a"))
	assignment, err = s.FetchAssignment(ctx, "cluster-a")
	require.NoError(t, err)
	assert.Nil(t, assignment)
}

func TestEppPoolStorager_UpsertValidation(t *testing.T) {
	ctx := context.Background()
	s, _ := setupTestStorager(t)

	require.Error(t, s.UpsertAssignment(ctx, nil))
	require.Error(t, s.UpsertAssignment(ctx, &epp_pool.AssignmentParam{Cluster: ""}))
}
