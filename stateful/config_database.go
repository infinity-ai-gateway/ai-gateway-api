// Copyright (c) 2021 The BFE Authors.
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

package stateful

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/glebarez/go-sqlite"
	"github.com/go-sql-driver/mysql"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
)

const (
	DriverMySQL  = "mysql"
	DriverSQLite = "sqlite"
)

type DbConfig struct {
	mysql.Config

	Driver string `validate:"required,min=1"`

	ConnMaxIdleTimeInMs int
	ConnMaxLifetimeInMs int
	MaxOpenConns        int `validate:"required,min=0"`
	MaxIdleConns        int
}

func (c *DbConfig) FormatDSN() (string, error) {
	switch c.Driver {
	case DriverMySQL:
		return c.Config.FormatDSN(), nil
	case DriverSQLite, "sqlite-strip":
		if c.Config.DBName == "" {
			return "", fmt.Errorf("sqlite DBName is required")
		}
		return c.Config.DBName, nil
	default:
		return "", fmt.Errorf("unsupported driver: %s", c.Driver)
	}
}

func NewDB(dbConfig *DbConfig) (*sql.DB, error) {
	dsn, err := dbConfig.FormatDSN()
	if err != nil {
		return nil, err
	}

	driverName := dbConfig.Driver
	if driverName == DriverSQLite {
		driverName = "sqlite"
	}

	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxIdleTime(time.Duration(dbConfig.ConnMaxIdleTimeInMs) * time.Millisecond)
	db.SetConnMaxLifetime(time.Duration(dbConfig.ConnMaxLifetimeInMs) * time.Millisecond)

	return db, nil
}

func (d *Config) InitDB() error {
	tmp := map[string]*sql.DB{}

	for name, dbConfig := range d.Databases {
		db, err := NewDB(dbConfig)
		if err != nil {
			return err
		}
		tmp[name] = db
	}

	DBs = tmp

	return nil
}

var DBs map[string]*sql.DB

func DbGet(name string) (*sql.DB, error) {
	db, ok := DBs[name]
	if !ok {
		return nil, fmt.Errorf("no such database: %s", name)
	}

	var err error
	for i := 0; i < 3; i++ {
		err = db.Ping()
		if err == nil {
			break
		}
	}

	return db, err
}

func BFEDB(db ...*sql.DB) (*sql.DB, error) {
	if len(db) == 1 {
		return db[0], nil
	}

	return DbGet("bfe_db")
}

func NewBFEDBContext(ctx context.Context, ops ...*lib.Op) (*lib.DBContext, error) {
	dc, ok := ctx.(*lib.DBContext)
	if ok {
		return dc, nil
	}

	conn, err := BFEDB()
	if err != nil {
		return nil, err
	}

	dc = lib.NewDBContext(ctx, conn)
	if lib.WantOpenTxn(ops...) {
		if err := dc.BeginTrans(); err != nil {
			return nil, err
		}
	}

	return dc, nil
}
