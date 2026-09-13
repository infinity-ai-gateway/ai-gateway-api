// Copyright(c) 2026 The Rainway AI Gateway (壬远AI网关) Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package iprovider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
)

// DiscoverModelsParam holds the input for a stateless model discovery request.
type DiscoverModelsParam struct {
	ModelProtocol string `json:"model_protocol"`
	Schema        string `json:"schema"`
	Addr          string `json:"addr"`
	Port          int    `json:"port"`
	URI           string `json:"uri"`
	APIKey        string `json:"apikey"`
}

// DiscoverCaller abstracts the HTTP call used by DiscoverModels.
type DiscoverCaller interface {
	Call(ctx context.Context, method, url string, headers map[string]string) ([]byte, error)
}

// HTTPDiscoverCaller is the default DiscoverCaller using net/http.
type HTTPDiscoverCaller struct {
	Client *http.Client
}

// NewHTTPDiscoverCaller creates a default HTTPDiscoverCaller.
func NewHTTPDiscoverCaller() *HTTPDiscoverCaller {
	return &HTTPDiscoverCaller{
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Call performs an HTTP GET request and returns the response body.
func (c *HTTPDiscoverCaller) Call(ctx context.Context, method, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("model discovery returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// DiscoverModels triggers a stateless model discovery and returns the model name list.
// It does not read or write any provider record.
func (m *ProviderManager) DiscoverModels(ctx context.Context, param *DiscoverModelsParam) ([]string, error) {
	return m.DiscoverModelsWithCaller(ctx, param, NewHTTPDiscoverCaller())
}

// DiscoverModelsWithCaller triggers model discovery using the supplied caller (for tests).
func (m *ProviderManager) DiscoverModelsWithCaller(ctx context.Context, param *DiscoverModelsParam, caller DiscoverCaller) ([]string, error) {
	if err := ValidateDiscoverModelsParam(param); err != nil {
		return nil, err
	}

	uri := param.URI
	if uri == "" {
		uri = defaultModelDiscoveryURI(param.ModelProtocol)
	}
	url := fmt.Sprintf("%s://%s:%d%s", param.Schema, param.Addr, param.Port, uri)

	headers := make(map[string]string)
	if param.APIKey != "" {
		headerName, headerValue := BuildAuthHeader(param.ModelProtocol, param.APIKey)
		headers[headerName] = headerValue
	}

	body, err := caller.Call(ctx, http.MethodGet, url, headers)
	if err != nil {
		return nil, xerror.WrapParamErrorWithMsg("model discovery request failed: %v", err)
	}

	models, err := ParseModelDiscoveryResponse(body, param.ModelProtocol)
	if err != nil {
		return nil, xerror.WrapParamErrorWithMsg("model discovery response parse failed: %v", err)
	}

	return models, nil
}

// ValidateDiscoverModelsParam validates a model discovery request.
func ValidateDiscoverModelsParam(param *DiscoverModelsParam) error {
	if param == nil {
		return xerror.WrapParamErrorWithMsg("request body is required")
	}
	if param.ModelProtocol == "" {
		return xerror.WrapParamErrorWithMsg("model_protocol is required")
	}
	if !ValidModelProtocols[param.ModelProtocol] {
		return xerror.WrapParamErrorWithMsg("invalid model_protocol: %s", param.ModelProtocol)
	}
	if param.Schema == "" {
		return xerror.WrapParamErrorWithMsg("schema is required")
	}
	if param.Schema != "http" && param.Schema != "https" {
		return xerror.WrapParamErrorWithMsg("schema must be http or https")
	}
	if param.Addr == "" {
		return xerror.WrapParamErrorWithMsg("addr is required")
	}
	if param.Port <= 0 || param.Port > 65535 {
		return xerror.WrapParamErrorWithMsg("port must be between 1 and 65535")
	}
	if param.URI != "" && !startsWithSlash(param.URI) {
		return xerror.WrapParamErrorWithMsg("uri must start with '/'")
	}
	if len(param.APIKey) > MaxProviderKeyLength {
		return xerror.WrapParamErrorWithMsg("apikey length must be <= %d", MaxProviderKeyLength)
	}
	return nil
}

func startsWithSlash(s string) bool {
	return len(s) > 0 && s[0] == '/'
}

// modelParser describes how to extract model names from a provider discovery response.
type modelParser struct {
	ListPath    string // JSON key of the array containing model entries
	IDField     string // field used as the canonical model identifier
	NameField   string // fallback field used when IDField is absent or empty
	DefaultURI  string // default discovery endpoint when the request does not specify uri
	StripPrefix string // prefix stripped from extracted model ids (e.g. gemini "models/")
}

// modelProtocolParsers maps model protocol names to their discovery response parsers.
// The definitions formerly lived in conf/ai/model_definition.json; they are now
// maintained in code so that /providers/tools/discover-models can select the right
// parser based on model_protocol.
var modelProtocolParsers = map[string]modelParser{
	"openai": {
		ListPath:   "data",
		IDField:    "id",
		NameField:  "object",
		DefaultURI: "/v1/models",
	},
	"anthropic": {
		ListPath:   "models",
		IDField:    "model_id",
		NameField:  "display_name",
		DefaultURI: "/v1/models",
	},
	"gemini": {
		ListPath:    "models",
		IDField:     "name",
		NameField:   "",
		DefaultURI:  "/v1beta/models",
		StripPrefix: "models/",
	},
}

// defaultModelDiscoveryURI returns the default discovery endpoint for the given
// protocol, falling back to /v1/models for unknown protocols.
func defaultModelDiscoveryURI(protocol string) string {
	if parser, ok := modelProtocolParsers[protocol]; ok && parser.DefaultURI != "" {
		return parser.DefaultURI
	}
	return "/v1/models"
}

// ParseModelDiscoveryResponse extracts a model name list from a provider discovery response.
func ParseModelDiscoveryResponse(body []byte, protocol string) ([]string, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	// Use protocol-specific parser when available.
	if parser, ok := modelProtocolParsers[protocol]; ok {
		return parseWithModelParser(data, parser, protocol)
	}

	// Fallback to generic discovery heuristics for unknown protocols.
	return parseModelDiscoveryResponseGeneric(data, protocol)
}

func parseWithModelParser(data map[string]interface{}, parser modelParser, protocol string) ([]string, error) {
	list, ok := data[parser.ListPath].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unable to extract models from %s response: %q array not found", protocol, parser.ListPath)
	}

	models := make([]string, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if id, ok := m[parser.IDField].(string); ok && id != "" {
			if parser.StripPrefix != "" {
				id = strings.TrimPrefix(id, parser.StripPrefix)
			}
			if id != "" {
				models = append(models, id)
				continue
			}
		}
		if parser.NameField != "" {
			if name, ok := m[parser.NameField].(string); ok && name != "" {
				models = append(models, name)
			}
		}
	}

	if len(models) == 0 {
		return nil, fmt.Errorf("unable to extract models from %s response: no valid entries in %q array", protocol, parser.ListPath)
	}
	return models, nil
}

func parseModelDiscoveryResponseGeneric(data map[string]interface{}, protocol string) ([]string, error) {
	// Try common OpenAI-style "data" array first.
	if list, ok := data["data"].([]interface{}); ok {
		models := make([]string, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				if id, ok := m["id"].(string); ok && id != "" {
					models = append(models, id)
					continue
				}
				if name, ok := m["name"].(string); ok && name != "" {
					models = append(models, name)
				}
			}
		}
		if len(models) > 0 {
			return models, nil
		}
	}

	// Fallback to "models" array.
	if list, ok := data["models"].([]interface{}); ok {
		models := make([]string, 0, len(list))
		for _, item := range list {
			if m, ok := item.(map[string]interface{}); ok {
				if name, ok := m["name"].(string); ok && name != "" {
					models = append(models, name)
					continue
				}
				if id, ok := m["id"].(string); ok && id != "" {
					models = append(models, id)
				}
			} else if s, ok := item.(string); ok && s != "" {
				models = append(models, s)
			}
		}
		if len(models) > 0 {
			return models, nil
		}
	}

	// Some providers return { "models": { "name": ... } } for a single model.
	if single, ok := data["models"].(map[string]interface{}); ok {
		if name, ok := single["name"].(string); ok && name != "" {
			return []string{name}, nil
		}
	}

	return nil, fmt.Errorf("unable to extract models from %s response", protocol)
}
