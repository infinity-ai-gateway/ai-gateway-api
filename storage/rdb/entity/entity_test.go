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
	"context"
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStorager(t *testing.T) *EntityStorager {
	// The DAO layer consults stateful.DefaultConfig when recording SQL.
	if stateful.DefaultConfig == nil {
		stateful.DefaultConfig = &stateful.Config{}
	}

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
CREATE TABLE entities (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  entity_id TEXT NOT NULL,
  name TEXT NOT NULL,
  type TEXT NOT NULL,
  parent_id TEXT DEFAULT NULL,
  allow_models TEXT,
  block_models TEXT,
  quota_plan_id INTEGER DEFAULT NULL,
  rate_limit_policy_id INTEGER DEFAULT NULL,
  route_rules_id INTEGER DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (entity_id),
  UNIQUE (name)
);`)
	require.NoError(t, err)

	factory := lib.DBContextFactory(func(ctx context.Context, ops ...*lib.Op) (*lib.DBContext, error) {
		return lib.NewDBContext(ctx, db), nil
	})

	return NewEntityStorager(factory)
}

func fetchOne(t *testing.T, storager *EntityStorager, entityID string) *entity.EntityParam {
	one, err := storager.FetchEntity(context.Background(), &entity.EntityFilter{EntityID: &entityID})
	require.NoError(t, err)
	require.NotNil(t, one)
	return one
}

func TestUpdateEntity_OmittedModelsPreserveValues(t *testing.T) {
	storager := setupTestStorager(t)
	ctx := context.Background()

	id := "entity-1"
	name := "entity-one"
	typ := "tenant"
	_, err := storager.CreateEntity(ctx, &entity.EntityParam{
		EntityID:    &id,
		Name:        &name,
		Type:        &typ,
		AllowModels: []string{"model-a"},
		BlockModels: []string{"model-b"},
	})
	require.NoError(t, err)

	// PATCH with only name: omitted allow_models/block_models must be preserved
	newName := "entity-one-renamed"
	affected, err := storager.UpdateEntity(ctx,
		&entity.EntityFilter{EntityID: &id},
		&entity.EntityParam{Name: &newName})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one := fetchOne(t, storager, id)
	assert.Equal(t, newName, *one.Name)
	assert.Equal(t, []string{"model-a"}, one.AllowModels)
	assert.Equal(t, []string{"model-b"}, one.BlockModels)
}

func TestUpdateEntity_ProvidedModelsAreWritten(t *testing.T) {
	storager := setupTestStorager(t)
	ctx := context.Background()

	id := "entity-1"
	name := "entity-one"
	typ := "tenant"
	_, err := storager.CreateEntity(ctx, &entity.EntityParam{
		EntityID:    &id,
		Name:        &name,
		Type:        &typ,
		AllowModels: []string{"model-a"},
	})
	require.NoError(t, err)

	affected, err := storager.UpdateEntity(ctx,
		&entity.EntityFilter{EntityID: &id},
		&entity.EntityParam{
			AllowModels: []string{"model-x"},
			BlockModels: []string{"model-y"},
		})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one := fetchOne(t, storager, id)
	assert.Equal(t, []string{"model-x"}, one.AllowModels)
	assert.Equal(t, []string{"model-y"}, one.BlockModels)
}

func TestCreateEntity_OmittedModelsDefaultToEmpty(t *testing.T) {
	storager := setupTestStorager(t)
	ctx := context.Background()

	id := "entity-1"
	name := "entity-one"
	typ := "tenant"
	_, err := storager.CreateEntity(ctx, &entity.EntityParam{
		EntityID: &id,
		Name:     &name,
		Type:     &typ,
	})
	require.NoError(t, err)

	one := fetchOne(t, storager, id)
	assert.Empty(t, one.AllowModels)
	assert.Empty(t, one.BlockModels)
}
