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
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/entity"
	"github.com/rainway-ai-gateway/ai-gateway-api/storage/rdb/internal/dao"
)

// EntityIDSeqName is the fixed sequence key for Entity IDs.
const EntityIDSeqName = "entity"

// RDBEntityIDGenerator generates entity-{seq} IDs using the database sequence
// table entity_id_seq. It is safe for concurrent use across processes.
type RDBEntityIDGenerator struct {
	dbCtxFactory lib.DBContextFactory
}

var _ entity.EntityIDGenerator = (*RDBEntityIDGenerator)(nil)

// NewRDBEntityIDGenerator creates a new generator backed by the RDB sequence table.
func NewRDBEntityIDGenerator(dbCtxFactory lib.DBContextFactory) *RDBEntityIDGenerator {
	return &RDBEntityIDGenerator{dbCtxFactory: dbCtxFactory}
}

// Generate allocates the next sequence number from the database and returns an
// ID in the form "entity-{seq}".
func (g *RDBEntityIDGenerator) Generate(ctx context.Context) (string, error) {
	dbCtx, err := g.dbCtxFactory(ctx)
	if err != nil {
		return "", err
	}

	seq, err := dao.TEntityIDSeqAllocate(dbCtx, EntityIDSeqName)
	if err != nil {
		return "", fmt.Errorf("allocate entity id sequence: %w", err)
	}

	return fmt.Sprintf("entity-%d", seq), nil
}
