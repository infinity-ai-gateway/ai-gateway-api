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

package model_price

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/imodel_price"
)

// TestMergeModelPricePricesKeyLevelMerge verifies that a PUT carrying only one
// price key keeps the other keys of the prices map (issue #140).
func TestMergeModelPricePricesKeyLevelMerge(t *testing.T) {
	dst := &imodel_price.ModelPrice{
		Provider: "p1",
		Model:    "m1",
		Mode:     "chat",
		Prices: imodel_price.PriceMap{
			"input_cost_per_token":  5.1e-06,
			"output_cost_per_token": 6.2e-06,
		},
	}
	src := &imodel_price.ModelPrice{
		Prices: imodel_price.PriceMap{
			"input_cost_per_token": 3.1e-06,
		},
	}

	merged := mergeModelPrice(dst, src)

	assert.Equal(t, 3.1e-06, merged.Prices["input_cost_per_token"])
	assert.Equal(t, 6.2e-06, merged.Prices["output_cost_per_token"], "unsubmitted key must keep its value")
	assert.Len(t, merged.Prices, 2)
}

// TestMergeModelPriceTierPricesTwoLevelMerge verifies two-level tier merging:
// submitted tier keys override, unsubmitted keys of the same tier are kept,
// unsubmitted tiers are retained whole, and new tiers are added (issue #140).
func TestMergeModelPriceTierPricesTwoLevelMerge(t *testing.T) {
	dst := &imodel_price.ModelPrice{
		TierPrices: imodel_price.TierPriceMap{
			"peak": {
				"input_cost_per_token":        9.1e-06,
				"output_cost_per_token":       1.02e-05,
				"cache_read_input_token_cost": 3.1e-06,
			},
			"offpeak": {
				"input_cost_per_token": 1.0e-06,
			},
		},
	}
	src := &imodel_price.ModelPrice{
		TierPrices: imodel_price.TierPriceMap{
			"peak": {
				"input_cost_per_token": 1.11e-05,
			},
			"night": {
				"input_cost_per_token": 5.0e-07,
			},
		},
	}

	merged := mergeModelPrice(dst, src)

	peak := merged.TierPrices["peak"]
	assert.Equal(t, 1.11e-05, peak["input_cost_per_token"], "submitted key overridden")
	assert.Equal(t, 1.02e-05, peak["output_cost_per_token"], "same-tier unsubmitted key kept")
	assert.Equal(t, 3.1e-06, peak["cache_read_input_token_cost"], "same-tier unsubmitted key kept")

	assert.Equal(t, map[string]float64{"input_cost_per_token": 1.0e-06}, merged.TierPrices["offpeak"],
		"tier absent from src retained whole")
	assert.Equal(t, map[string]float64{"input_cost_per_token": 5.0e-07}, merged.TierPrices["night"],
		"new tier added whole")
	assert.Len(t, merged.TierPrices, 3)
}

// TestMergeModelPriceEmptySrcMapsUnchanged verifies that omitting the maps
// leaves the existing maps untouched.
func TestMergeModelPriceEmptySrcMapsUnchanged(t *testing.T) {
	dst := &imodel_price.ModelPrice{
		Prices: imodel_price.PriceMap{
			"input_cost_per_token": 5.1e-06,
		},
		TierPrices: imodel_price.TierPriceMap{
			"peak": {"input_cost_per_token": 9.1e-06},
		},
	}
	src := &imodel_price.ModelPrice{Mode: "chat"}

	merged := mergeModelPrice(dst, src)

	assert.Equal(t, dst.Prices, merged.Prices)
	assert.Equal(t, dst.TierPrices, merged.TierPrices)
	assert.Equal(t, "chat", merged.Mode)
}

// TestMergeModelPriceDoesNotMutateDst verifies the merge never writes into the
// maps held by the existing record (merged := *dst is a shallow copy; maps
// must be rebuilt instead of edited in place).
func TestMergeModelPriceDoesNotMutateDst(t *testing.T) {
	dst := &imodel_price.ModelPrice{
		Prices: imodel_price.PriceMap{
			"input_cost_per_token": 5.1e-06,
		},
		TierPrices: imodel_price.TierPriceMap{
			"peak": {
				"input_cost_per_token":  9.1e-06,
				"output_cost_per_token": 1.02e-05,
			},
		},
	}
	src := &imodel_price.ModelPrice{
		Prices: imodel_price.PriceMap{
			"output_cost_per_token": 6.2e-06,
		},
		TierPrices: imodel_price.TierPriceMap{
			"peak": {
				"input_cost_per_token": 1.11e-05,
			},
		},
	}

	merged := mergeModelPrice(dst, src)

	// mutate every merged map; the existing record must stay intact
	merged.Prices["input_cost_per_token"] = -1
	merged.TierPrices["peak"]["output_cost_per_token"] = -1
	merged.TierPrices["peak"]["cache_read_input_token_cost"] = -1

	assert.Equal(t, imodel_price.PriceMap{"input_cost_per_token": 5.1e-06}, dst.Prices)
	assert.Equal(t, imodel_price.TierPriceMap{
		"peak": {
			"input_cost_per_token":  9.1e-06,
			"output_cost_per_token": 1.02e-05,
		},
	}, dst.TierPrices)
}

// TestMergeModelPriceScalarFields verifies scalar merge semantics are
// unaffected by the map merge fix.
func TestMergeModelPriceScalarFields(t *testing.T) {
	dst := &imodel_price.ModelPrice{
		Provider: "p1",
		Model:    "m1",
		Mode:     "chat",
	}
	src := &imodel_price.ModelPrice{
		Model: "m2",
		Mode:  " ",
	}

	merged := mergeModelPrice(dst, src)

	assert.Equal(t, "p1", merged.Provider, "empty src keeps dst value")
	assert.Equal(t, "m2", merged.Model, "non-empty src overrides")
	assert.Equal(t, "chat", merged.Mode, "blank src keeps dst value")
}
