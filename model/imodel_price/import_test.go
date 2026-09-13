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

package imodel_price

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseModelListYAML(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    capabilities:
      - chat
      - vision
    supported_parameters:
      - temperature
      - top_p
    limits:
      context_window: 128000
      max_input_tokens: 64000
    prices:
      input_cost_per_token: 0.001
      output_cost_per_token: 0.002
    metadata:
      source: test
      notes: note1
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.NotNil(t, file)
	assert.Equal(t, "v1.0", file.Version)
	assert.Equal(t, "RMB", file.DefaultCurrency)
	require.Len(t, file.Models, 1)

	m := file.Models[0]
	assert.Equal(t, "openai", m.Provider)
	assert.Equal(t, "gpt-4", m.Model)
	assert.Equal(t, "gpt-4", m.BaseModel)
	assert.Equal(t, "chat", m.Mode)
	assert.Equal(t, []string{"chat", "vision"}, m.Capabilities)
	assert.Equal(t, []string{"temperature", "top_p"}, m.SupportedParameters)
	assert.Equal(t, 128000, m.Limits["context_window"])
	assert.Equal(t, 64000, m.Limits["max_input_tokens"])
	assert.Equal(t, float64(0.001), m.Prices["input_cost_per_token"])
	assert.Equal(t, float64(0.002), m.Prices["output_cost_per_token"])
	assert.Equal(t, "test", m.Metadata["source"])
	assert.Equal(t, "note1", m.Metadata["notes"])
	assert.Equal(t, "RMB", m.PriceCurrency)
}

func TestParseModelListYAMLFillsDefaultCurrency(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    prices:
      input_cost_per_token: 0.001
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)
	assert.Equal(t, "RMB", file.Models[0].PriceCurrency)
}

func TestParseModelListYAMLKeepsExplicitCurrency(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    price_currency: USD
    prices:
      input_cost_per_token: 0.001
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)
	// Parse should keep the explicit value; validation later rejects non-RMB.
	assert.Equal(t, "USD", file.Models[0].PriceCurrency)
}

func TestParseModelListYAMLEmpty(t *testing.T) {
	file, err := ParseModelListYAML(strings.NewReader(""))
	require.NoError(t, err)
	assert.NotNil(t, file)
	assert.Empty(t, file.Version)
	assert.Empty(t, file.DefaultCurrency)
	assert.Empty(t, file.Models)
}

func TestParseModelListYAMLInvalid(t *testing.T) {
	_, err := ParseModelListYAML(strings.NewReader("not yaml: ["))
	assert.Error(t, err)
}

func TestParseModelListYAMLReaderError(t *testing.T) {
	_, err := ParseModelListYAML(&errReader{})
	assert.Error(t, err)
}

func TestParseModelListYAMLMultipleModels(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    prices:
      input_cost_per_token: 0.001
  - provider: anthropic
    model: claude-3
    base_model: claude-3
    mode: chat
    price_currency: RMB
    prices:
      output_cost_per_token: 0.003
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 2)

	first := file.Models[0]
	assert.Equal(t, "openai", first.Provider)
	assert.Equal(t, "gpt-4", first.Model)
	assert.Equal(t, "chat", first.Mode)
	assert.Equal(t, "RMB", first.PriceCurrency)

	second := file.Models[1]
	assert.Equal(t, "anthropic", second.Provider)
	assert.Equal(t, "claude-3", second.Model)
	assert.Equal(t, "RMB", second.PriceCurrency)
}

func TestParseModelListYAMLEmptyModels(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models: []
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Empty(t, file.Models)
}

func TestParseModelListYAMLUnknownFieldsIgnored(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
extra_top: ignored
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    unknown_field: ignored
    prices:
      input_cost_per_token: 0.001
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)
	assert.Equal(t, "openai", file.Models[0].Provider)
}

func TestParseModelListYAMLIntegerPrices(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    prices:
      input_cost_per_token: 1
      output_cost_per_token: 2
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)
	assert.Equal(t, float64(1), file.Models[0].Prices["input_cost_per_token"])
	assert.Equal(t, float64(2), file.Models[0].Prices["output_cost_per_token"])
}

func TestParseModelListYAMLMissingTopLevelFields(t *testing.T) {
	yaml := `
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
    prices:
      input_cost_per_token: 0.001
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	assert.Empty(t, file.Version)
	assert.Empty(t, file.DefaultCurrency)
	// Without default_currency, PriceCurrency should remain empty.
	assert.Empty(t, file.Models[0].PriceCurrency)
}

func TestParseModelListYAMLNoPrices(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: openai
    model: gpt-4
    base_model: gpt-4
    mode: chat
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)
	assert.Nil(t, file.Models[0].Prices)
}

type errReader struct{}

func (e *errReader) Read(p []byte) (int, error) {
	return 0, errors.New("read error")
}

var _ io.Reader = (*errReader)(nil)

func TestParseModelListYAMLEightDecimalPlaces(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: deepseek
    model: deepseek-v3
    base_model: deepseek-v3
    mode: chat
    prices:
      input_cost_per_token: 0.0000015
      output_cost_per_token: 0.0000045
      cache_read_input_token_cost: 0.00000005
    tier_prices:
      peak:
        input_cost_per_token: 0.000003
        output_cost_per_token: 0.000009
        cache_read_input_token_cost: 0.0000001
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)

	m := file.Models[0]
	assert.Equal(t, 0.0000015, m.Prices["input_cost_per_token"])
	assert.Equal(t, 0.0000045, m.Prices["output_cost_per_token"])
	assert.Equal(t, 0.00000005, m.Prices["cache_read_input_token_cost"])
	assert.Equal(t, 0.000003, m.TierPrices["peak"]["input_cost_per_token"])
	assert.Equal(t, 0.000009, m.TierPrices["peak"]["output_cost_per_token"])
	assert.Equal(t, 0.0000001, m.TierPrices["peak"]["cache_read_input_token_cost"])

	// JSON round-trip preserves values regardless of textual notation
	// (the standard encoder may emit scientific notation for small values).
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var back ModelPrice
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, 0.0000015, back.Prices["input_cost_per_token"])
	assert.Equal(t, 0.0000045, back.Prices["output_cost_per_token"])
	assert.Equal(t, 0.00000005, back.Prices["cache_read_input_token_cost"])
	assert.Equal(t, 0.000003, back.TierPrices["peak"]["input_cost_per_token"])
	assert.Equal(t, 0.000009, back.TierPrices["peak"]["output_cost_per_token"])
	assert.Equal(t, 0.0000001, back.TierPrices["peak"]["cache_read_input_token_cost"])
}

func TestParseModelListYAMLScientificNotation(t *testing.T) {
	yaml := `
version: v1.0
default_currency: RMB
models:
  - provider: example-provider
    model: qwen2.5-omni-7b
    base_model: qwen2.5-omni-7b
    mode: chat
    prices:
      input_cost_per_token: 6.0168984e-09
      output_cost_per_token: 7.6234102728e-08
    tier_prices:
      peak:
        input_cost_per_token: 4.141631732e-06
`
	file, err := ParseModelListYAML(strings.NewReader(yaml))
	require.NoError(t, err)
	require.Len(t, file.Models, 1)

	m := file.Models[0]
	assert.Equal(t, 6.0168984e-09, m.Prices["input_cost_per_token"])
	assert.Equal(t, 7.6234102728e-08, m.Prices["output_cost_per_token"])
	assert.Equal(t, 4.141631732e-06, m.TierPrices["peak"]["input_cost_per_token"])

	// Scientific notation and decimal notation are equivalent: JSON
	// round-trip must preserve the float64 values.
	data, err := json.Marshal(m)
	require.NoError(t, err)
	var back ModelPrice
	require.NoError(t, json.Unmarshal(data, &back))
	assert.Equal(t, 6.0168984e-09, back.Prices["input_cost_per_token"])
	assert.Equal(t, 7.6234102728e-08, back.Prices["output_cost_per_token"])
	assert.Equal(t, 4.141631732e-06, back.TierPrices["peak"]["input_cost_per_token"])
}
