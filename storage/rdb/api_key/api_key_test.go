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
	"database/sql"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/api_key"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestStorager(t *testing.T) *APIKeyStorager {
	// The DAO layer consults stateful.DefaultConfig when recording SQL.
	if stateful.DefaultConfig == nil {
		stateful.DefaultConfig = &stateful.Config{}
	}

	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
CREATE TABLE api_keys (
  inner_id INTEGER PRIMARY KEY AUTOINCREMENT,
  id TEXT NOT NULL DEFAULT '',
  enable INTEGER NOT NULL DEFAULT 0,
  api_key TEXT NOT NULL DEFAULT '',
  description TEXT DEFAULT '',
  unlimited_quota INTEGER DEFAULT 0,
  product_name TEXT NOT NULL DEFAULT '',
  expired_time INTEGER NOT NULL DEFAULT -1,
  allowed_models TEXT,
  subnet TEXT,
  entity_id TEXT DEFAULT NULL,
  quota_plan_id INTEGER DEFAULT NULL,
  rate_limit_policy_id INTEGER DEFAULT NULL,
  route_rules_id INTEGER DEFAULT NULL,
  created_at DATETIME NOT NULL DEFAULT '0000-01-01 00:00:00',
  updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE (id),
  UNIQUE (api_key)
);`)
	require.NoError(t, err)

	factory := lib.DBContextFactory(func(ctx context.Context, ops ...*lib.Op) (*lib.DBContext, error) {
		return lib.NewDBContext(ctx, db), nil
	})

	return NewAPIKeyStorager(factory)
}

func fetchOne(t *testing.T, storager *APIKeyStorager, id string) *api_key.APIKeyParam {
	list, err := storager.FetchAPIKeyList(context.Background(), &api_key.APIKeyFilter{ID: &id})
	require.NoError(t, err)
	require.Len(t, list, 1)
	return list[0]
}

func TestUpdateAPIKey_OmittedModelsSubnetPreserveValues(t *testing.T) {
	storager := setupTestStorager(t)
	ctx := context.Background()

	id := "key-1"
	desc := "created"
	_, err := storager.CreateAPIKey(ctx, &api_key.APIKeyParam{
		ID:          &id,
		Description: &desc,
		Models:      []string{"controlled-model-a"},
		Subnet:      []string{"10.0.0.0/24"},
	})
	require.NoError(t, err)

	// PATCH with only description: omitted models/subnet must be preserved
	newDesc := "updated"
	affected, err := storager.UpdateAPIKey(ctx,
		&api_key.APIKeyFilter{ID: &id},
		&api_key.APIKeyParam{Description: &newDesc})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one := fetchOne(t, storager, id)
	assert.Equal(t, newDesc, *one.Description)
	assert.Equal(t, []string{"controlled-model-a"}, one.Models)
	assert.Equal(t, []string{"10.0.0.0/24"}, one.Subnet)
}

func TestUpdateAPIKey_ProvidedModelsSubnetAreWritten(t *testing.T) {
	storager := setupTestStorager(t)
	ctx := context.Background()

	id := "key-1"
	desc := "created"
	_, err := storager.CreateAPIKey(ctx, &api_key.APIKeyParam{
		ID:          &id,
		Description: &desc,
		Models:      []string{"controlled-model-a"},
	})
	require.NoError(t, err)

	affected, err := storager.UpdateAPIKey(ctx,
		&api_key.APIKeyFilter{ID: &id},
		&api_key.APIKeyParam{
			Models: []string{"model-x", "model-y"},
			Subnet: []string{"192.168.0.0/16"},
		})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)

	one := fetchOne(t, storager, id)
	assert.Equal(t, []string{"model-x", "model-y"}, one.Models)
	assert.Equal(t, []string{"192.168.0.0/16"}, one.Subnet)
}

func TestCreateAPIKey_OmittedModelsSubnetDefaultToAll(t *testing.T) {
	storager := setupTestStorager(t)
	ctx := context.Background()

	id := "key-1"
	desc := "created"
	_, err := storager.CreateAPIKey(ctx, &api_key.APIKeyParam{
		ID:          &id,
		Description: &desc,
	})
	require.NoError(t, err)

	one := fetchOne(t, storager, id)
	assert.Equal(t, []string{"*"}, one.Models)
	assert.Equal(t, []string{"*"}, one.Subnet)
}
