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
	"sync"
	"testing"

	_ "github.com/glebarez/go-sqlite"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEntitySeqTestDB(t *testing.T) (*sql.DB, lib.DBContexter) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	// Use a single connection so the in-memory database is stable across the
	// pooled connection lifecycle.
	db.SetMaxOpenConns(1)

	_, err = db.Exec(`
CREATE TABLE entity_id_seq (
  name TEXT NOT NULL PRIMARY KEY,
  next_seq INTEGER NOT NULL DEFAULT 1
);`)
	require.NoError(t, err)

	return db, lib.NewDBContext(context.Background(), db)
}

func TestTEntityIDSeqAllocate_Basic(t *testing.T) {
	_, dbCtx := setupEntitySeqTestDB(t)

	seq, err := TEntityIDSeqAllocate(dbCtx, "entity")
	require.NoError(t, err)
	assert.Equal(t, int64(1), seq)

	seq, err = TEntityIDSeqAllocate(dbCtx, "entity")
	require.NoError(t, err)
	assert.Equal(t, int64(2), seq)

	seq, err = TEntityIDSeqAllocate(dbCtx, "entity")
	require.NoError(t, err)
	assert.Equal(t, int64(3), seq)
}

func TestTEntityIDSeqAllocate_NotReusedWithoutInsert(t *testing.T) {
	_, dbCtx := setupEntitySeqTestDB(t)

	// Simulate rolled-back creations: allocated numbers must not be reused
	// even if no entity row is ever written (issue #132).
	for want := int64(1); want <= 3; want++ {
		seq, err := TEntityIDSeqAllocate(dbCtx, "entity")
		require.NoError(t, err)
		assert.Equal(t, want, seq)
	}
}

func TestTEntityIDSeqAllocate_Concurrent(t *testing.T) {
	_, dbCtx := setupEntitySeqTestDB(t)

	const workers = 50
	var wg sync.WaitGroup
	results := make(chan int64, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			seq, err := TEntityIDSeqAllocate(dbCtx, "entity")
			require.NoError(t, err)
			results <- seq
		}()
	}
	wg.Wait()
	close(results)

	seen := make(map[int64]struct{})
	for seq := range results {
		assert.Greater(t, seq, int64(0))
		assert.NotContains(t, seen, seq, "sequence %d was allocated more than once", seq)
		seen[seq] = struct{}{}
	}
	assert.Len(t, seen, workers)
}
