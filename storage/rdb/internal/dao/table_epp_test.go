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

package dao

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEppTestDB(t *testing.T) (*sql.DB, lib.DBContexter) {
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

	return db, lib.NewDBContext(context.Background(), db)
}

func TestTEppInstanceCRUD(t *testing.T) {
	_, dbCtx := setupEppTestDB(t)

	affected, err := TEppInstanceCreate(dbCtx, &TEppInstanceParam{
		ID:        lib.PString("epp-a"),
		Host:      lib.PString("10.0.0.1"),
		Port:      lib.PInt(9002),
		GroupName: lib.PString("g1"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	_, err = TEppInstanceCreate(dbCtx,
		&TEppInstanceParam{ID: lib.PString("epp-b"), Host: lib.PString("10.0.0.2"), Port: lib.PInt(9002), GroupName: lib.PString("g1")},
		&TEppInstanceParam{ID: lib.PString("epp-c"), Host: lib.PString("10.0.0.3"), Port: lib.PInt(9002), GroupName: lib.PString("g2")},
	)
	require.NoError(t, err)

	one, err := TEppInstanceOne(dbCtx, &TEppInstanceParam{ID: lib.PString("epp-a")})
	require.NoError(t, err)
	require.NotNil(t, one)
	assert.Equal(t, "g1", one.GroupName)
	assert.False(t, one.CreateTime.IsZero())

	// Not found returns nil, nil.
	none, err := TEppInstanceOne(dbCtx, &TEppInstanceParam{ID: lib.PString("epp-x")})
	require.NoError(t, err)
	assert.Nil(t, none)

	// Group filter.
	list, err := TEppInstanceList(dbCtx, &TEppInstanceParam{GroupName: lib.PString("g1")})
	require.NoError(t, err)
	require.Len(t, list, 2)

	// Full list.
	list, err = TEppInstanceList(dbCtx, &TEppInstanceParam{})
	require.NoError(t, err)
	assert.Len(t, list, 3)

	// Unique(host, port) enforced.
	_, err = TEppInstanceCreate(dbCtx, &TEppInstanceParam{
		ID:        lib.PString("epp-d"),
		Host:      lib.PString("10.0.0.1"),
		Port:      lib.PInt(9002),
		GroupName: lib.PString("g1"),
	})
	require.Error(t, err)

	// Update.
	affected, err = TEppInstanceUpdate(dbCtx,
		&TEppInstanceParam{GroupName: lib.PString("g2")},
		&TEppInstanceParam{ID: lib.PString("epp-a")},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one, err = TEppInstanceOne(dbCtx, &TEppInstanceParam{ID: lib.PString("epp-a")})
	require.NoError(t, err)
	assert.Equal(t, "g2", one.GroupName)

	// Delete by filter.
	affected, err = TEppInstanceDelete(dbCtx, &TEppInstanceParam{GroupName: lib.PString("g2")})
	require.NoError(t, err)
	assert.Equal(t, int64(2), affected)

	list, err = TEppInstanceList(dbCtx, &TEppInstanceParam{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestTEppAssignmentCRUD(t *testing.T) {
	_, dbCtx := setupEppTestDB(t)

	affected, err := TEppAssignmentCreate(dbCtx, &TEppAssignmentParam{
		Cluster:           lib.PString("cluster-a"),
		GroupName:         lib.PString("g1"),
		PrimaryInstanceID: lib.PString("epp-a"),
	})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one, err := TEppAssignmentOne(dbCtx, &TEppAssignmentParam{Cluster: lib.PString("cluster-a")})
	require.NoError(t, err)
	require.NotNil(t, one)
	assert.Equal(t, "epp-a", one.PrimaryInstanceID)

	none, err := TEppAssignmentOne(dbCtx, &TEppAssignmentParam{Cluster: lib.PString("cluster-x")})
	require.NoError(t, err)
	assert.Nil(t, none)

	// Primary key conflict on duplicate cluster.
	_, err = TEppAssignmentCreate(dbCtx, &TEppAssignmentParam{
		Cluster:           lib.PString("cluster-a"),
		GroupName:         lib.PString("g2"),
		PrimaryInstanceID: lib.PString("epp-b"),
	})
	require.Error(t, err)

	list, err := TEppAssignmentList(dbCtx, &TEppAssignmentParam{})
	require.NoError(t, err)
	assert.Len(t, list, 1)

	affected, err = TEppAssignmentUpdate(dbCtx,
		&TEppAssignmentParam{PrimaryInstanceID: lib.PString("epp-b")},
		&TEppAssignmentParam{Cluster: lib.PString("cluster-a")},
	)
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one, err = TEppAssignmentOne(dbCtx, &TEppAssignmentParam{Cluster: lib.PString("cluster-a")})
	require.NoError(t, err)
	assert.Equal(t, "epp-b", one.PrimaryInstanceID)

	affected, err = TEppAssignmentDelete(dbCtx, &TEppAssignmentParam{Cluster: lib.PString("cluster-a")})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	list, err = TEppAssignmentList(dbCtx, &TEppAssignmentParam{})
	require.NoError(t, err)
	assert.Empty(t, list)
}
