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

	"github.com/go-sql-driver/mysql"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao/internal"
)

const tAPIKeyIDSeqTableName = "api_key_id_seq"

// TAPIKeyIDSeq maps to api_key_id_seq table.
type TAPIKeyIDSeq struct {
	ProductName string `db:"product_name"`
	NextSeq     int64  `db:"next_seq"`
}

// TAPIKeyIDSeqParam is used for query/update conditions.
type TAPIKeyIDSeqParam struct {
	ProductName *string `db:"product_name"`
	NextSeq     *int64  `db:"next_seq"`
}

// TAPIKeyIDSeqOne returns the sequence row for a product, or nil if not exists.
func TAPIKeyIDSeqOne(dbCtx lib.DBContexter, where *TAPIKeyIDSeqParam) (*TAPIKeyIDSeq, error) {
	t := &TAPIKeyIDSeq{}
	err := internal.QueryOne(dbCtx, tAPIKeyIDSeqTableName, where, t)
	if err == nil {
		return t, nil
	}
	if xerror.Cause(err) == internal.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

// TAPIKeyIDSeqCreate inserts a new sequence row.
func TAPIKeyIDSeqCreate(dbCtx lib.DBContexter, data *TAPIKeyIDSeqParam) (int64, error) {
	return internal.Create(dbCtx, tAPIKeyIDSeqTableName, data)
}

// TAPIKeyIDSeqUpdate updates a sequence row.
func TAPIKeyIDSeqUpdate(dbCtx lib.DBContexter, val, where *TAPIKeyIDSeqParam) (int64, error) {
	return internal.Update(dbCtx, tAPIKeyIDSeqTableName, where, val)
}

// TAPIKeyIDSeqAllocate atomically allocates and returns the next available
// API-Key sequence number for the given product. The returned number can be
// used directly to build an ID such as "api-key-{seq}".
//
// It performs a single atomic UPDATE to increment next_seq and return the new
// value, avoiding the compare-and-set retry loop that caused lock wait timeouts
// under MySQL's default REPEATABLE READ isolation level (see issue #99).
//
// It intentionally uses the connection pool (dbCtx.Conn()) rather than
// dbCtx.Execer(): the allocation is a self-contained mini-transaction that
// must commit independently, and it is only invoked at the endpoint layer,
// outside any enclosing AtomExecute transaction.
func TAPIKeyIDSeqAllocate(dbCtx lib.DBContexter, productName string) (int64, error) {
	conn := dbCtx.Conn()
	return allocateAPIKeyIDSeq(conn, productName)
}

func allocateAPIKeyIDSeq(conn *sql.DB, productName string) (int64, error) {
	var allocated int64
	useMySQL := isMySQL(conn)
	err := lib.Transaction(conn, func(tx *sql.Tx) error {
		if useMySQL {
			// MySQL: a single INSERT ... ON DUPLICATE KEY UPDATE atomically
			// initializes the row or increments next_seq. LAST_INSERT_ID is
			// set to the new next_seq value, so the allocation result is
			// LAST_INSERT_ID() - 1. This avoids the deadlock-prone
			// "INSERT IGNORE + UPDATE" sequence (issue #99).
			_, err := tx.ExecContext(context.Background(),
				"INSERT INTO api_key_id_seq (product_name, next_seq) VALUES (?, LAST_INSERT_ID(2)) "+
					"ON DUPLICATE KEY UPDATE next_seq = LAST_INSERT_ID(next_seq + 1)",
				productName)
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
			"INSERT OR IGNORE INTO api_key_id_seq (product_name, next_seq) VALUES (?, 1)",
			productName); err != nil {
			return err
		}
		_, err := tx.ExecContext(context.Background(),
			"UPDATE api_key_id_seq SET next_seq = next_seq + 1 WHERE product_name = ?",
			productName)
		if err != nil {
			return err
		}
		row := tx.QueryRowContext(context.Background(),
			"SELECT next_seq FROM api_key_id_seq WHERE product_name = ?",
			productName)
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

// isMySQL reports whether the underlying database connection uses the MySQL
// driver.
func isMySQL(conn *sql.DB) bool {
	_, ok := conn.Driver().(*mysql.MySQLDriver)
	return ok
}
