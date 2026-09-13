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

package quotacache

import "context"

// QuotaCache abstracts the real-time quota cache operations for API-Key / Entity.
// The implementation hides Redis details (key generation, fixed-point conversion,
// nil handling) from the callers.
type QuotaCache interface {
	// GetRemaining returns the current remaining quota for the given owner key.
	// The returned value is already converted back from the internal Redis
	// representation based on unit.
	GetRemaining(ctx context.Context, key string, unit *string) (float64, error)

	// BatchGetRemaining returns the current remaining quota for multiple owner keys.
	// The returned values are already converted back from the internal Redis
	// representation based on unit.
	BatchGetRemaining(ctx context.Context, keys []string, unit *string) (map[string]float64, error)

	// SetRemaining sets the Redis remaining quota to the target value.
	// Internally it uses GetInt64 + IncrBy(delta) to keep the operation
	// compatible with concurrent request consumption.
	SetRemaining(ctx context.Context, key string, quota *float64, unit *string) error

	// ResetToQuota resets the Redis remaining quota to the given quota amount.
	// Semantically it is equivalent to SetRemaining.
	ResetToQuota(ctx context.Context, key string, quota *float64, unit *string) error

	// ResetToQuotaAtomic atomically sets the Redis remaining quota to the given
	// amount using Redis SET. Unlike SetRemaining, it does not read the current
	// value and avoids the read-modify-write race of IncrBy(delta). It is
	// intended for periodic quota reset only.
	ResetToQuotaAtomic(ctx context.Context, key string, quota *float64, unit *string) error

	// DeleteKeys removes the given Redis keys.
	// The caller must pass the complete Redis keys (including any prefix);
	// the implementation deletes them as-is.
	DeleteKeys(ctx context.Context, keys []string) error
}
