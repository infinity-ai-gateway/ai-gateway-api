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

package entity

import (
	"context"
	"errors"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEntityTypeManager_CreateEntityType(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		store := &fakeEntityTypeStorager{
			fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, param *EntityTypeParam) (int64, error) {
				return 42, nil
			},
		}
		m := NewEntityTypeManager(&fakeTxn{}, store)

		id, err := m.CreateEntityType(ctx, &EntityTypeParam{
			TypeName: lib.PString("tenant"),
			Level:    lib.PInt(1),
		})

		require.NoError(t, err)
		assert.Equal(t, int64(42), id)
		assert.Len(t, store.fetched, 1)
		assert.Equal(t, lib.PString("tenant"), store.fetched[0].TypeName)
		assert.Len(t, store.created, 1)
	})

	t.Run("missing type_name", func(t *testing.T) {
		m := NewEntityTypeManager(&fakeTxn{}, &fakeEntityTypeStorager{})
		_, err := m.CreateEntityType(ctx, &EntityTypeParam{Level: lib.PInt(1)})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type_name is required")
	})

	t.Run("already existed", func(t *testing.T) {
		store := &fakeEntityTypeStorager{
			fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
				return &EntityTypeParam{TypeName: lib.PString("tenant")}, nil
			},
		}
		m := NewEntityTypeManager(&fakeTxn{}, store)

		_, err := m.CreateEntityType(ctx, &EntityTypeParam{TypeName: lib.PString("tenant")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Entity-Type Record Existed")
	})

	t.Run("fetch error", func(t *testing.T) {
		store := &fakeEntityTypeStorager{
			fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
				return nil, errors.New("db down")
			},
		}
		m := NewEntityTypeManager(&fakeTxn{}, store)

		_, err := m.CreateEntityType(ctx, &EntityTypeParam{TypeName: lib.PString("tenant")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "db down")
	})
}

func TestEntityTypeManager_FetchEntityType(t *testing.T) {
	ctx := context.Background()
	store := &fakeEntityTypeStorager{
		fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
			if filter.TypeName != nil && *filter.TypeName == "tenant" {
				return &EntityTypeParam{TypeName: lib.PString("tenant"), Level: lib.PInt(1)}, nil
			}
			return nil, nil
		},
	}
	m := NewEntityTypeManager(&fakeTxn{}, store)

	one, err := m.FetchEntityType(ctx, &EntityTypeFilter{TypeName: lib.PString("tenant")})
	require.NoError(t, err)
	require.NotNil(t, one)
	assert.Equal(t, "tenant", *one.TypeName)
	assert.Equal(t, 1, *one.Level)
}

func TestEntityTypeManager_FetchEntityTypeList(t *testing.T) {
	ctx := context.Background()
	store := &fakeEntityTypeStorager{
		listFn: func(ctx context.Context, filter *EntityTypeFilter) ([]*EntityTypeParam, error) {
			return []*EntityTypeParam{
				{TypeName: lib.PString("tenant"), Level: lib.PInt(1)},
			}, nil
		},
	}
	m := NewEntityTypeManager(&fakeTxn{}, store)

	list, err := m.FetchEntityTypeList(ctx, &EntityTypeFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
}

func TestEntityTypeManager_UpdateEntityType(t *testing.T) {
	ctx := context.Background()
	store := &fakeEntityTypeStorager{
		fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
			return &EntityTypeParam{TypeName: lib.PString("tenant"), Level: lib.PInt(1)}, nil
		},
		updateFn: func(ctx context.Context, filter *EntityTypeFilter, param *EntityTypeParam) (int64, error) {
			return 1, nil
		},
	}
	m := NewEntityTypeManager(&fakeTxn{}, store)

	affected, err := m.UpdateEntityType(ctx, &EntityTypeFilter{TypeName: lib.PString("tenant")}, &EntityTypeParam{Level: lib.PInt(2)})
	require.NoError(t, err)
	assert.Equal(t, int64(1), affected)
	assert.Len(t, store.updated, 1)
}

func TestEntityTypeManager_UpdateEntityType_NotFound(t *testing.T) {
	ctx := context.Background()
	store := &fakeEntityTypeStorager{
		fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
			return nil, nil
		},
	}
	m := NewEntityTypeManager(&fakeTxn{}, store)

	_, err := m.UpdateEntityType(ctx, &EntityTypeFilter{TypeName: lib.PString("tenant")}, &EntityTypeParam{Level: lib.PInt(2)})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Entity-Type Record Not Exist")
}

func TestEntityTypeManager_DeleteEntityType(t *testing.T) {
	ctx := context.Background()
	store := &fakeEntityTypeStorager{
		fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
			return &EntityTypeParam{TypeName: lib.PString("tenant"), Level: lib.PInt(1)}, nil
		},
		deleteFn: func(ctx context.Context, filter *EntityTypeFilter) error {
			return nil
		},
	}
	m := NewEntityTypeManager(&fakeTxn{}, store)

	require.NoError(t, m.DeleteEntityType(ctx, &EntityTypeFilter{TypeName: lib.PString("tenant")}))
	assert.Len(t, store.deleted, 1)
}

func TestEntityTypeManager_DeleteEntityType_NotFound(t *testing.T) {
	ctx := context.Background()
	store := &fakeEntityTypeStorager{
		fetchFn: func(ctx context.Context, filter *EntityTypeFilter) (*EntityTypeParam, error) {
			return nil, nil
		},
	}
	m := NewEntityTypeManager(&fakeTxn{}, store)

	err := m.DeleteEntityType(ctx, &EntityTypeFilter{TypeName: lib.PString("tenant")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Entity-Type Record Not Exist")
}
