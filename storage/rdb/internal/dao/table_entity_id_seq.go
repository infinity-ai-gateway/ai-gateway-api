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

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao/internal"
)

const tEntityIDSeqTableName = "entity_id_seq"

// TEntityIDSeq maps to entity_id_seq table.
type TEntityIDSeq struct {
	Name    string `db:"name"`
	NextSeq int64  `db:"next_seq"`
}

// TEntityIDSeqParam is used for query/update conditions.
type TEntityIDSeqParam struct {
	Name    *string `db:"name"`
	NextSeq *int64  `db:"next_seq"`
}

// TEntityIDSeqOne returns the sequence row for the given key, or nil if not exists.
func TEntityIDSeqOne(dbCtx lib.DBContexter, where *TEntityIDSeqParam) (*TEntityIDSeq, error) {
	t := &TEntityIDSeq{}
	err := internal.QueryOne(dbCtx, tEntityIDSeqTableName, where, t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TEntityIDSeqCreate inserts a new sequence row.
func TEntityIDSeqCreate(dbCtx lib.DBContexter, data *TEntityIDSeqParam) (int64, error) {
	return internal.Create(dbCtx, tEntityIDSeqTableName, data)
}

// TEntityIDSeqUpdate updates a sequence row.
func TEntityIDSeqUpdate(dbCtx lib.DBContexter, val, where *TEntityIDSeqParam) (int64, error) {
	return internal.Update(dbCtx, tEntityIDSeqTableName, where, val)
}

// TEntityIDSeqAllocate atomically allocates and returns the next available
// Entity sequence number. The returned number can be used directly to build
// an ID such as "entity-{seq}". Allocated numbers are never reused, even if
// the caller rolls back afterwards (see issue #132).
//
// It uses the same allocation strategy as TAPIKeyIDSeqAllocate: a single
// atomic statement on MySQL and an INSERT OR IGNORE + UPDATE on SQLite,
// avoiding the deadlock-prone compare-and-set retry loop (see issue #99).
//
// Like TAPIKeyIDSeqAllocate, it intentionally uses the connection pool
// (dbCtx.Conn()) rather than dbCtx.Execer(): the allocation must commit
// independently of any enclosing transaction, and it is only invoked at the
// endpoint layer, outside any AtomExecute transaction.
func TEntityIDSeqAllocate(dbCtx lib.DBContexter, name string) (int64, error) {
	conn := dbCtx.Conn()
	return allocateEntityIDSeq(conn, name)
}

func allocateEntityIDSeq(conn *sql.DB, name string) (int64, error) {
	var allocated int64
	useMySQL := isMySQL(conn)
	err := lib.Transaction(conn, func(tx *sql.Tx) error {
		if useMySQL {
			// MySQL: a single INSERT ... ON DUPLICATE KEY UPDATE atomically
			// initializes the row or increments next_seq. LAST_INSERT_ID is
			// set to the new next_seq value, so the allocation result is
			// LAST_INSERT_ID() - 1.
			_, err := tx.ExecContext(context.Background(),
				"INSERT INTO entity_id_seq (name, next_seq) VALUES (?, LAST_INSERT_ID(2)) "+
					"ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1)",
				name)
			if err != nil {
				return err
			}
			row := tx.QueryRowContext(context.Background(), "SELECT LAST_INSERT_ID()")
			if err := row.Scan(&allocated); err != nil {
				return err
			}
			// The table stores the next available sequence; the value just
			// incremented is the one allocated to this caller.
			allocated--
			return nil
		}

		// SQLite: ensure the row exists, then atomically increment and read
		// back the new value. The write lock is database-level, so a plain
		// UPDATE followed by a SELECT inside the same transaction is safe.
		if _, err := tx.ExecContext(context.Background(),
			"INSERT OR IGNORE INTO entity_id_seq (name, next_seq) VALUES (?, 1)",
			name); err != nil {
			return err
		}
		_, err := tx.ExecContext(context.Background(),
			"UPDATE entity_id_seq SET next_seq = next_seq + 1 WHERE name = ?",
			name)
		if err != nil {
			return err
		}
		row := tx.QueryRowContext(context.Background(),
			"SELECT next_seq FROM entity_id_seq WHERE name = ?",
			name)
		if err := row.Scan(&allocated); err != nil {
			return err
		}
		// The table stores the next available sequence; the value just
		// incremented is the one allocated to this caller.
		allocated--
		return nil
	})
	return allocated, err
}
