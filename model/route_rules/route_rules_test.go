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

package route_rules

import (
	"context"
	"errors"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/icluster_conf"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/shared"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeTxn implements itxn.TxnStorager for unit tests
type fakeTxn struct{}

func (f *fakeTxn) AtomExecute(ctx context.Context, do func(context.Context) error) error {
	return do(ctx)
}

var _ itxn.TxnStorager = (*fakeTxn)(nil)

// fakeRouteRulesStorager implements shared.RouteRulesStorager
type fakeRouteRulesStorager struct {
	createFn    func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error)
	fetchFn     func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error)
	listFn      func(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error)
	updateFn    func(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error)
	deleteFn    func(ctx context.Context, id int64) error
	fetchByIDFn func(ctx context.Context, id int64) (*shared.RouteRulesParam, error)
	allFn       func(ctx context.Context) ([]*shared.RouteRulesParam, error)

	created     []createRouteRulesCall
	fetched     []fetchRouteRulesCall
	updated     []updateRouteRulesCall
	deleted     []int64
	fetchedByID []int64
	fetchedAll  int
}

type createRouteRulesCall struct {
	ruleType string
	owner    *string
	param    *shared.RouteRulesParam
}

type fetchRouteRulesCall struct {
	ruleType string
	owner    *string
}

type updateRouteRulesCall struct {
	id    int64
	param *shared.RouteRulesParam
}

func (s *fakeRouteRulesStorager) CreateRouteRules(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
	s.created = append(s.created, createRouteRulesCall{ruleType: ruleType, owner: owner, param: param})
	if s.createFn != nil {
		return s.createFn(ctx, ruleType, owner, param)
	}
	return 0, nil
}

func (s *fakeRouteRulesStorager) FetchRouteRules(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
	s.fetched = append(s.fetched, fetchRouteRulesCall{ruleType: ruleType, owner: owner})
	if s.fetchFn != nil {
		return s.fetchFn(ctx, ruleType, owner)
	}
	return nil, nil
}

func (s *fakeRouteRulesStorager) FetchRouteRulesList(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error) {
	if s.listFn != nil {
		return s.listFn(ctx, filter)
	}
	return nil, 0, nil
}

func (s *fakeRouteRulesStorager) UpdateRouteRules(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error) {
	s.updated = append(s.updated, updateRouteRulesCall{id: id, param: param})
	if s.updateFn != nil {
		return s.updateFn(ctx, id, param)
	}
	return 0, nil
}

func (s *fakeRouteRulesStorager) DeleteRouteRules(ctx context.Context, id int64) error {
	s.deleted = append(s.deleted, id)
	if s.deleteFn != nil {
		return s.deleteFn(ctx, id)
	}
	return nil
}

func (s *fakeRouteRulesStorager) FetchRouteRulesByID(ctx context.Context, id int64) (*shared.RouteRulesParam, error) {
	s.fetchedByID = append(s.fetchedByID, id)
	if s.fetchByIDFn != nil {
		return s.fetchByIDFn(ctx, id)
	}
	return nil, nil
}

func (s *fakeRouteRulesStorager) FetchAllRouteRules(ctx context.Context) ([]*shared.RouteRulesParam, error) {
	s.fetchedAll++
	if s.allFn != nil {
		return s.allFn(ctx)
	}
	return nil, nil
}

var _ shared.RouteRulesStorager = (*fakeRouteRulesStorager)(nil)

// fakeOperationLogRecorder captures operation log entries for tests.
type fakeOperationLogRecorder struct {
	entries []*ioperlog.OperationLogEntry
}

func (r *fakeOperationLogRecorder) Record(ctx context.Context, entry *ioperlog.OperationLogEntry) {
	r.entries = append(r.entries, entry)
}

func TestRouteRulesManager_CreateRouteRules(t *testing.T) {
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			createFn: func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
				return 1, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		id, err := m.CreateRouteRules(ctx, shared.RouteRulesTypeAPIKey, lib.PString("owner-1"), validRouteRulesParam())
		require.NoError(t, err)
		assert.Equal(t, int64(1), id)
		require.Len(t, store.created, 1)
		assert.Equal(t, shared.RouteRulesTypeAPIKey, store.created[0].ruleType)
	})

	t.Run("validation error", func(t *testing.T) {
		m := NewRouteRulesManager(&fakeTxn{}, &fakeRouteRulesStorager{})

		_, err := m.CreateRouteRules(ctx, shared.RouteRulesTypeAPIKey, nil, &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{{}},
		})
		require.Error(t, err)
	})
}

func TestRouteRulesManager_UpdateRouteRules(t *testing.T) {
	ctx := context.Background()

	t.Run("create when not exists", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
				return 2, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		id, err := m.UpdateRouteRules(ctx, shared.RouteRulesTypeAPIKey, lib.PString("owner-1"), validRouteRulesParam())
		require.NoError(t, err)
		assert.Equal(t, int64(2), id)
		assert.Len(t, store.created, 1)
	})

	t.Run("update when exists", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return &shared.RouteRulesParam{ID: lib.PInt64(3)}, nil
			},
			updateFn: func(ctx context.Context, id int64, param *shared.RouteRulesParam) (int64, error) {
				return 1, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		id, err := m.UpdateRouteRules(ctx, shared.RouteRulesTypeAPIKey, lib.PString("owner-1"), validRouteRulesParam())
		require.NoError(t, err)
		assert.Equal(t, int64(3), id)
		assert.Len(t, store.updated, 1)
	})

	t.Run("validation error", func(t *testing.T) {
		m := NewRouteRulesManager(&fakeTxn{}, &fakeRouteRulesStorager{})

		_, err := m.UpdateRouteRules(ctx, shared.RouteRulesTypeAPIKey, nil, &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{{}},
		})
		require.Error(t, err)
	})
}

func TestRouteRulesManager_FetchRouteRules(t *testing.T) {
	ctx := context.Background()
	store := &fakeRouteRulesStorager{
		fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
			return &shared.RouteRulesParam{Enabled: lib.PBool(true)}, nil
		},
	}
	m := NewRouteRulesManager(&fakeTxn{}, store)

	rule, err := m.FetchRouteRules(ctx, shared.RouteRulesTypeAPIKey, lib.PString("owner-1"))
	require.NoError(t, err)
	require.NotNil(t, rule)
	assert.True(t, *rule.Enabled)
}

func TestRouteRulesManager_DeleteRouteRules(t *testing.T) {
	ctx := context.Background()

	t.Run("delete existing", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return &shared.RouteRulesParam{ID: lib.PInt64(5)}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.DeleteRouteRules(ctx, shared.RouteRulesTypeAPIKey, lib.PString("owner-1")))
		assert.Equal(t, []int64{5}, store.deleted)
	})

	t.Run("delete not existing", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return nil, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.DeleteRouteRules(ctx, shared.RouteRulesTypeAPIKey, lib.PString("owner-1")))
		assert.Empty(t, store.deleted)
	})
}

func TestRouteRulesManager_FetchRouteRulesByID(t *testing.T) {
	ctx := context.Background()
	store := &fakeRouteRulesStorager{
		fetchByIDFn: func(ctx context.Context, id int64) (*shared.RouteRulesParam, error) {
			return &shared.RouteRulesParam{ID: lib.PInt64(7)}, nil
		},
	}
	m := NewRouteRulesManager(&fakeTxn{}, store)

	rule, err := m.FetchRouteRulesByID(ctx, 7)
	require.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, int64(7), *rule.ID)
}

func TestRouteRulesManager_ListRouteTables(t *testing.T) {
	ctx := context.Background()
	store := &fakeRouteRulesStorager{
		listFn: func(ctx context.Context, filter *shared.RouteRulesFilter) ([]*shared.RouteTableParam, int64, error) {
			return []*shared.RouteTableParam{{Type: "apikey"}}, 1, nil
		},
	}
	m := NewRouteRulesManager(&fakeTxn{}, store)

	list, total, err := m.ListRouteTables(ctx, &shared.RouteRulesFilter{})
	require.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, int64(1), total)
}

func TestRouteRulesManager_GetGlobalRouteRules(t *testing.T) {
	ctx := context.Background()
	store := &fakeRouteRulesStorager{
		fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
			assert.Equal(t, shared.RouteRulesTypeGlobal, ruleType)
			return &shared.RouteRulesParam{Enabled: lib.PBool(true)}, nil
		},
	}
	m := NewRouteRulesManager(&fakeTxn{}, store)

	rule, err := m.GetGlobalRouteRules(ctx)
	require.NoError(t, err)
	require.NotNil(t, rule)
}

func TestRouteRulesManager_SetGlobalRouteRules(t *testing.T) {
	ctx := context.Background()
	recorder := &fakeOperationLogRecorder{}

	store := &fakeRouteRulesStorager{
		fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
			return 9, nil
		},
		fetchByIDFn: func(ctx context.Context, id int64) (*shared.RouteRulesParam, error) {
			return &shared.RouteRulesParam{ID: lib.PInt64(9)}, nil
		},
	}
	m := NewRouteRulesManager(&fakeTxn{}, store)
	m.SetOperationLogManager(recorder)

	rule, err := m.SetGlobalRouteRules(ctx, validRouteRulesParam())
	require.NoError(t, err)
	require.NotNil(t, rule)
	assert.Equal(t, int64(9), *rule.ID)

	require.Len(t, recorder.entries, 1)
	entry := recorder.entries[0]
	assert.Equal(t, string(ioperlog.ActionUpdate), entry.Action)
	assert.Equal(t, string(ioperlog.ResourceTypeRoute), entry.ResourceType)
	assert.Equal(t, "global", entry.ResourceID)
	assert.Equal(t, "global", entry.ResourceName)
	assert.Equal(t, ioperlog.StatusSuccess, entry.Status)
	assert.NotNil(t, entry.ChangeSummary)
}

func TestRouteRulesManager_SetGlobalRouteRules_RecordsFailedOperationLog(t *testing.T) {
	ctx := context.Background()
	recorder := &fakeOperationLogRecorder{}

	store := &fakeRouteRulesStorager{
		fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
			return nil, nil
		},
		createFn: func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
			return 0, errors.New("db error")
		},
	}
	m := NewRouteRulesManager(&fakeTxn{}, store)
	m.SetOperationLogManager(recorder)

	_, err := m.SetGlobalRouteRules(ctx, validRouteRulesParam())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "db error")

	require.Len(t, recorder.entries, 1)
	entry := recorder.entries[0]
	assert.Equal(t, string(ioperlog.ActionUpdate), entry.Action)
	assert.Equal(t, string(ioperlog.ResourceTypeRoute), entry.ResourceType)
	assert.Equal(t, "global", entry.ResourceID)
	assert.Equal(t, ioperlog.StatusFailed, entry.Status)
	assert.Contains(t, entry.ErrorMsg, "db error")
}

func TestRouteRulesManager_EnsureGlobalRouteRules(t *testing.T) {
	ctx := context.Background()

	t.Run("creates default global route table when not exists", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return nil, nil
			},
			createFn: func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
				return 10, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.EnsureGlobalRouteRules(ctx))
		require.Len(t, store.created, 1)
		assert.Equal(t, shared.RouteRulesTypeGlobal, store.created[0].ruleType)
		require.NotNil(t, store.created[0].owner)
		assert.Equal(t, shared.RouteRulesTypeGlobal, *store.created[0].owner)
		require.NotNil(t, store.created[0].param.Enabled)
		assert.False(t, *store.created[0].param.Enabled)
		assert.Empty(t, store.created[0].param.Rules)
	})

	t.Run("does nothing when global route table already exists", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return &shared.RouteRulesParam{ID: lib.PInt64(1)}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.EnsureGlobalRouteRules(ctx))
		assert.Empty(t, store.created)
	})
}

func TestRouteRulesManager_ClusterDeleteChecker(t *testing.T) {
	ctx := context.Background()
	product := &ibasic.Product{Name: "AI"}
	cluster := &icluster_conf.Cluster{Name: "c1"}

	t.Run("no route rules", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return nil, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.ClusterDeleteChecker(ctx, product, cluster))
	})

	t.Run("route rule target refers cluster", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(1),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule1"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		err := m.ClusterDeleteChecker(ctx, product, cluster)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rule rule1 Refer To This Cluster")
		assert.Equal(t, 409, xerror.Resolve(err).ErrNo)
	})

	t.Run("route rule fallback refers cluster", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(2),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule2"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c2"), Weight: lib.PInt(100)},
								},
								Fallbacks: []*shared.AiRouteFallbackParam{
									{ClusterName: lib.PString("c1")},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		err := m.ClusterDeleteChecker(ctx, product, cluster)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rule rule2 Refer To This Cluster")
	})

	t.Run("route rule refers other cluster", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(3),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule3"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c2"), Weight: lib.PInt(100)},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.ClusterDeleteChecker(ctx, product, cluster))
	})

	t.Run("route rule beyond first page is still checked", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(21),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule21"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		err := m.ClusterDeleteChecker(ctx, product, cluster)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rule rule21 Refer To This Cluster")
	})
}

func TestRouteRulesManager_ClusterModelUpdateChecker(t *testing.T) {
	ctx := context.Background()
	product := &ibasic.Product{Name: "AI"}
	cluster := &icluster_conf.Cluster{
		Name: "c1",
		LLMConfig: &icluster_conf.LLMConfig{
			Models: []string{"m1", "m2"},
		},
	}

	t.Run("no llm_config change", func(t *testing.T) {
		m := NewRouteRulesManager(&fakeTxn{}, &fakeRouteRulesStorager{})
		require.NoError(t, m.ClusterModelUpdateChecker(ctx, product, cluster, &icluster_conf.ClusterParam{}))
	})

	t.Run("no model removed", func(t *testing.T) {
		m := NewRouteRulesManager(&fakeTxn{}, &fakeRouteRulesStorager{})
		require.NoError(t, m.ClusterModelUpdateChecker(ctx, product, cluster, &icluster_conf.ClusterParam{
			LLMConfig: &icluster_conf.LLMConfig{Models: []string{"m1", "m2", "m3"}},
		}))
	})

	t.Run("route rule target refers removed model", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(1),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule1"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c1"), Model: lib.PString("m1"), Weight: lib.PInt(100)},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		err := m.ClusterModelUpdateChecker(ctx, product, cluster, &icluster_conf.ClusterParam{
			LLMConfig: &icluster_conf.LLMConfig{Models: []string{"m2"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rule rule1 Refer To Model m1 In Cluster c1")
	})

	t.Run("route rule fallback refers removed model", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(2),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule2"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c2"), Model: lib.PString("m1"), Weight: lib.PInt(100)},
								},
								Fallbacks: []*shared.AiRouteFallbackParam{
									{ClusterName: lib.PString("c1"), Model: lib.PString("m2")},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		err := m.ClusterModelUpdateChecker(ctx, product, cluster, &icluster_conf.ClusterParam{
			LLMConfig: &icluster_conf.LLMConfig{Models: []string{"m1"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rule rule2 Refer To Model m2 In Cluster c1")
	})

	t.Run("route rule refers other cluster model", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(3),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule3"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c2"), Model: lib.PString("m1"), Weight: lib.PInt(100)},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		require.NoError(t, m.ClusterModelUpdateChecker(ctx, product, cluster, &icluster_conf.ClusterParam{
			LLMConfig: &icluster_conf.LLMConfig{Models: []string{"m2"}},
		}))
	})

	t.Run("route rule beyond first page is still checked", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			allFn: func(ctx context.Context) ([]*shared.RouteRulesParam, error) {
				return []*shared.RouteRulesParam{
					{
						ID:      lib.PInt64(21),
						Enabled: lib.PBool(true),
						Rules: []*shared.AiRouteRuleParam{
							{
								Name: lib.PString("rule21"),
								Cond: lib.PString("default_t()"),
								Targets: []*shared.AiRouteTargetParam{
									{ClusterName: lib.PString("c1"), Model: lib.PString("m1"), Weight: lib.PInt(100)},
								},
							},
						},
					},
				}, nil
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		err := m.ClusterModelUpdateChecker(ctx, product, cluster, &icluster_conf.ClusterParam{
			LLMConfig: &icluster_conf.LLMConfig{Models: []string{"m2"}},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "Rule rule21 Refer To Model m1 In Cluster c1")
	})
}

func TestRouteRulesManager_validateRouteRules(t *testing.T) {
	m := NewRouteRulesManager(&fakeTxn{}, &fakeRouteRulesStorager{})

	t.Run("nil param", func(t *testing.T) {
		assert.NoError(t, m.validateRouteRules(nil))
	})

	t.Run("missing rule name", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{{
				Cond: lib.PString("default_t()"),
				Targets: []*shared.AiRouteTargetParam{{
					ClusterName: lib.PString("cluster-1"),
					Weight:      lib.PInt(100),
				}},
			}},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("duplicate rule name", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{Name: lib.PString("rule-1"), Cond: lib.PString("cond1"), Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)}}},
				{Name: lib.PString("rule-1"), Cond: lib.PString("cond2"), Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c2"), Weight: lib.PInt(100)}}},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("missing cond", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{Name: lib.PString("rule-1"), Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)}}},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("invalid cond syntax", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{Name: lib.PString("rule-1"), Cond: lib.PString("req_path_in(/v1, false)"), Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)}}},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("empty targets", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{Name: lib.PString("rule-1"), Cond: lib.PString("cond1")},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("missing target weight", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{Name: lib.PString("rule-1"), Cond: lib.PString("cond1"), Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c1")}}},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("total weight not 100", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{Name: lib.PString("rule-1"), Cond: lib.PString("cond1"), Targets: []*shared.AiRouteTargetParam{
					{ClusterName: lib.PString("c1"), Weight: lib.PInt(60)},
					{ClusterName: lib.PString("c2"), Weight: lib.PInt(30)},
				}},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("empty fallback cluster name", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{
					Name:      lib.PString("rule-1"),
					Cond:      lib.PString("cond1"),
					Targets:   []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)}},
					Fallbacks: []*shared.AiRouteFallbackParam{{}},
				},
			},
		}
		assert.Error(t, m.validateRouteRules(param))
	})

	t.Run("valid rules", func(t *testing.T) {
		param := &shared.RouteRulesParam{
			Rules: []*shared.AiRouteRuleParam{
				{
					Name:    lib.PString("rule-1"),
					Cond:    lib.PString("default_t()"),
					Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("c1"), Weight: lib.PInt(100)}},
				},
			},
		}
		assert.NoError(t, m.validateRouteRules(param))
	})
}

func TestRouteRulesManager_PropagateErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateRouteRules storage error", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			createFn: func(ctx context.Context, ruleType string, owner *string, param *shared.RouteRulesParam) (int64, error) {
				return 0, errors.New("db error")
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		_, err := m.CreateRouteRules(ctx, shared.RouteRulesTypeAPIKey, nil, validRouteRulesParam())
		require.Error(t, err)
	})

	t.Run("FetchRouteRules storage error", func(t *testing.T) {
		store := &fakeRouteRulesStorager{
			fetchFn: func(ctx context.Context, ruleType string, owner *string) (*shared.RouteRulesParam, error) {
				return nil, errors.New("db error")
			},
		}
		m := NewRouteRulesManager(&fakeTxn{}, store)

		_, err := m.FetchRouteRules(ctx, shared.RouteRulesTypeAPIKey, nil)
		require.Error(t, err)
	})
}

func validRouteRulesParam() *shared.RouteRulesParam {
	return &shared.RouteRulesParam{
		Enabled: lib.PBool(true),
		Rules: []*shared.AiRouteRuleParam{
			{
				Name:    lib.PString("rule-1"),
				Cond:    lib.PString("default_t()"),
				Targets: []*shared.AiRouteTargetParam{{ClusterName: lib.PString("cluster-1"), Weight: lib.PInt(100)}},
			},
		},
	}
}
