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
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib"
	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
)

const (
	MaxProviderNameLength        = 64
	MaxProviderKeyLength         = 512
	MaxProviderDescriptionLength = 256
)

var (
	providerNameToken = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

	ValidModelProtocols = map[string]bool{
		"openai":    true,
		"anthropic": true,
	}
)

// ProviderEndpoint describes the provider's model-discovery endpoint.
type ProviderEndpoint struct {
	Schema string `json:"schema"`
	URI    string `json:"uri"`
}

// ProviderKey is an API key defined on a provider.
type ProviderKey struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// ProviderInstance is a single upstream instance owned by a provider.
// It mirrors icluster_conf.Instance so the provider package can stay free of
// cluster-conf dependencies and avoid an import cycle.
type ProviderInstance struct {
	Addr    string `json:"addr"`
	Port    int    `json:"port"`
	Weight  int64  `json:"weight"`
	Disable bool   `json:"disable"`
}

// TimeRange defines a time window within a day for a pricing tier.
type TimeRange struct {
	Weekdays []int  `json:"weekdays,omitempty" yaml:"weekdays,omitempty"`
	Start    string `json:"start" yaml:"start"`
	End      string `json:"end" yaml:"end"`
}

// PricingTier defines a named pricing tier (e.g., "peak") with time ranges.
type PricingTier struct {
	Name       string      `json:"name" yaml:"name"`
	TimeRanges []TimeRange `json:"time_ranges" yaml:"time_ranges"`
}

// Provider represents a model provider.
type Provider struct {
	ID             int64              `json:"id"`
	Name           string             `json:"name"`
	Description    string             `json:"description"`
	ModelEndpoint  *ProviderEndpoint  `json:"model_endpoint"`
	Models         []string           `json:"models"`
	Keys           []ProviderKey      `json:"keys"`
	InstancePool   []ProviderInstance `json:"instance_pool"`
	ModelProtocols []string           `json:"model_protocols"`
	TimeZone       string             `json:"time_zone"`
	Tiers          []PricingTier      `json:"tiers"`
	CreateTime     int64              `json:"create_time"`
	UpdateTime     int64              `json:"update_time"`
}

// ProviderParam is used to create or update a provider.
type ProviderParam struct {
	ID             *int64             `json:"id,omitempty"`
	Name           *string            `json:"name"`
	Description    *string            `json:"description,omitempty"`
	ModelEndpoint  *ProviderEndpoint  `json:"model_endpoint,omitempty"`
	Models         []string           `json:"models,omitempty"`
	Keys           []ProviderKey      `json:"keys,omitempty"`
	InstancePool   []ProviderInstance `json:"instance_pool"`
	ModelProtocols []string           `json:"model_protocols"`
	TimeZone       *string            `json:"time_zone,omitempty"`
	Tiers          []PricingTier      `json:"tiers,omitempty"`
}

// PricingTiersParam is used to update a provider's pricing tiers.
type PricingTiersParam struct {
	TimeZone string        `json:"time_zone" yaml:"time_zone"`
	Tiers    []PricingTier `json:"tiers" yaml:"tiers"`
}

// ProviderFilter is used to query providers.
type ProviderFilter struct {
	ID            *int64
	Name          *string
	Names         []string
	ModelProtocol *string
	Page          *int
	PageSize      *int
}

// ProviderStorager defines persistence operations for providers.
type ProviderStorager interface {
	CreateProvider(ctx context.Context, param *ProviderParam) (int64, error)
	UpdateProvider(ctx context.Context, name string, param *ProviderParam) error
	DeleteProvider(ctx context.Context, name string) error
	FetchProvider(ctx context.Context, filter *ProviderFilter) (*Provider, error)
	FetchProviderList(ctx context.Context, filter *ProviderFilter) ([]*Provider, int64, error)
	FetchProviderNames(ctx context.Context) ([]string, error)
}

// NewProviderManager creates a ProviderManager.
func NewProviderManager(txn itxn.TxnStorager, storager ProviderStorager) *ProviderManager {
	return &ProviderManager{
		txn:      txn,
		storager: storager,
	}
}

// ProviderManager provides business-level operations for providers.
type ProviderManager struct {
	txn                 itxn.TxnStorager
	storager            ProviderStorager
	operationLogManager ioperlog.OperationLogRecorder
}

// SetOperationLogManager injects the operation log recorder.
func (m *ProviderManager) SetOperationLogManager(manager ioperlog.OperationLogRecorder) {
	m.operationLogManager = manager
}

// CreateProvider creates a new provider after validation.
func (m *ProviderManager) CreateProvider(ctx context.Context, param *ProviderParam) (int64, error) {
	if err := ValidateProviderParam(param); err != nil {
		name := ""
		if param != nil && param.Name != nil {
			name = *param.Name
		}
		m.recordProviderOperation(ctx, string(ioperlog.ActionCreate), name, nil, providerParamToMap(param), err)
		return 0, err
	}

	var id int64
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		existing, err := m.storager.FetchProvider(ctx, &ProviderFilter{Name: param.Name})
		if err != nil {
			return err
		}
		if existing != nil {
			return xerror.WrapRecordExisted("provider")
		}

		id, err = m.storager.CreateProvider(ctx, param)
		return err
	})
	if err != nil {
		m.recordProviderOperation(ctx, string(ioperlog.ActionCreate), *param.Name, nil, providerParamToMap(param), err)
		return 0, err
	}

	m.recordProviderOperation(ctx, string(ioperlog.ActionCreate), *param.Name, nil, providerParamToMap(param), nil)
	return id, nil
}

// UpdateProvider updates an existing provider.
// Optional syncHooks are invoked within the same transaction when instance_pool
// is explicitly provided and differs from the existing one, allowing downstream
// managers (e.g. cluster_conf) to propagate the new pool to clusters that
// reference this provider.
func (m *ProviderManager) UpdateProvider(ctx context.Context, name string,
	param *ProviderParam,
	syncHooks ...func(ctx context.Context, oldProvider, newProvider *Provider) error) error {

	if err := ValidateProviderParam(param); err != nil {
		m.recordProviderOperation(ctx, string(ioperlog.ActionUpdate), name, nil, providerParamToMap(param), err)
		return err
	}

	var oldProvider *Provider
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		existing, err := m.storager.FetchProvider(ctx, &ProviderFilter{Name: &name})
		if err != nil {
			return err
		}
		if existing == nil {
			return xerror.WrapRecordNotExist("provider")
		}
		oldProvider = existing

		// Capture which cluster-relevant fields are explicitly provided before
		// the storager applies defaults (FillDefaults mutates nil slices into
		// empty slices, which would be mistaken for an intentional clear).
		origInstancePool := param.InstancePool
		origKeys := param.Keys
		origModels := param.Models

		if err := m.storager.UpdateProvider(ctx, name, param); err != nil {
			return err
		}

		// Invoke hooks only when an explicitly provided cluster-relevant field
		// actually changed. Each hook is responsible for checking whether its own
		// concern needs action.
		clusterConfigChanged := false
		if origInstancePool != nil && !providerInstancePoolEqual(existing.InstancePool, origInstancePool) {
			clusterConfigChanged = true
		}
		if origKeys != nil && !providerKeysEqual(existing.Keys, origKeys) {
			clusterConfigChanged = true
		}
		if origModels != nil && !providerModelsEqual(existing.Models, origModels) {
			clusterConfigChanged = true
		}

		if len(syncHooks) > 0 && clusterConfigChanged {
			// Restore nil for unchanged fields so applyProviderUpdate preserves
			// the existing provider values when building the hook snapshot.
			if origInstancePool == nil {
				param.InstancePool = nil
			}
			if origKeys == nil {
				param.Keys = nil
			}
			if origModels == nil {
				param.Models = nil
			}
			newProvider := applyProviderUpdate(existing, param)
			for _, hook := range syncHooks {
				if err := hook(ctx, existing, newProvider); err != nil {
					return err
				}
			}
		}

		return nil
	})
	if err != nil {
		m.recordProviderOperation(ctx, string(ioperlog.ActionUpdate), name, providerToMap(oldProvider), providerParamToMap(param), err)
		return err
	}

	m.recordProviderOperation(ctx, string(ioperlog.ActionUpdate), name, providerToMap(oldProvider), providerParamToMap(param), nil)
	return nil
}

// UpdatePricingTiers updates the time zone and pricing tiers of a provider.
// It supports both JSON (parsed into PricingTiersParam) and YAML bodies at the endpoint layer.
func (m *ProviderManager) UpdatePricingTiers(ctx context.Context, name string, param *PricingTiersParam) error {
	if err := ValidatePricingTiersParam(param); err != nil {
		m.recordProviderOperation(ctx, string(ioperlog.ActionUpdate), name, nil, nil, err)
		return err
	}

	var oldProvider *Provider
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		existing, err := m.storager.FetchProvider(ctx, &ProviderFilter{Name: &name})
		if err != nil {
			return err
		}
		if existing == nil {
			return xerror.WrapRecordNotExist("provider")
		}
		oldProvider = existing

		// Preserve all existing provider fields; only update time_zone and tiers.
		updateParam := &ProviderParam{
			Name:           &existing.Name,
			Description:    &existing.Description,
			ModelEndpoint:  existing.ModelEndpoint,
			Models:         existing.Models,
			Keys:           existing.Keys,
			InstancePool:   existing.InstancePool,
			ModelProtocols: existing.ModelProtocols,
			TimeZone:       &param.TimeZone,
			Tiers:          param.Tiers,
		}
		return m.storager.UpdateProvider(ctx, name, updateParam)
	})
	if err != nil {
		m.recordProviderOperation(ctx, string(ioperlog.ActionUpdate), name, providerToMap(oldProvider), nil, err)
		return err
	}

	after := map[string]interface{}{
		"time_zone": param.TimeZone,
		"tiers":     param.Tiers,
	}
	m.recordProviderOperation(ctx, string(ioperlog.ActionUpdate), name, providerToMap(oldProvider), after, nil)
	return nil
}

// DeleteProvider deletes a provider if it is not referenced.
func (m *ProviderManager) DeleteProvider(ctx context.Context, name string,
	refCheckers ...func(context.Context, string) error) error {

	var oldProvider *Provider
	err := m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		existing, err := m.storager.FetchProvider(ctx, &ProviderFilter{Name: &name})
		if err != nil {
			return err
		}
		if existing == nil {
			return xerror.WrapRecordNotExist("provider")
		}
		oldProvider = existing

		for _, checker := range refCheckers {
			if err := checker(ctx, name); err != nil {
				return err
			}
		}

		return m.storager.DeleteProvider(ctx, name)
	})
	if err != nil {
		m.recordProviderOperation(ctx, string(ioperlog.ActionDelete), name, providerToMap(oldProvider), nil, err)
		return err
	}

	m.recordProviderOperation(ctx, string(ioperlog.ActionDelete), name, providerToMap(oldProvider), nil, nil)
	return nil
}

// FetchProvider fetches a single provider by filter.
func (m *ProviderManager) FetchProvider(ctx context.Context, filter *ProviderFilter) (*Provider, error) {
	return m.storager.FetchProvider(ctx, filter)
}

// FetchProviderList fetches a paginated list of providers.
func (m *ProviderManager) FetchProviderList(ctx context.Context, filter *ProviderFilter) ([]*Provider, int64, error) {
	if filter == nil || filter.ModelProtocol == nil || *filter.ModelProtocol == "" {
		return m.storager.FetchProviderList(ctx, filter)
	}

	// model_protocols is stored as a JSON array in RDB. Cross-database JSON
	// containment queries differ between MySQL and SQLite, so we apply the
	// filter in memory for correctness and portability. This is acceptable
	// because the provider table is small.
	proto := *filter.ModelProtocol
	storageFilter := &ProviderFilter{}
	all, _, err := m.storager.FetchProviderList(ctx, storageFilter)
	if err != nil {
		return nil, 0, err
	}

	matched := make([]*Provider, 0, len(all))
	for _, p := range all {
		for _, mp := range p.ModelProtocols {
			if mp == proto {
				matched = append(matched, p)
				break
			}
		}
	}

	// No pagination requested: return all matched records.
	if filter.Page == nil && filter.PageSize == nil {
		return matched, int64(len(matched)), nil
	}

	page := 1
	pageSize := 50
	if filter.Page != nil && *filter.Page > 0 {
		page = *filter.Page
	}
	if filter.PageSize != nil && *filter.PageSize > 0 {
		pageSize = *filter.PageSize
		if pageSize > 1000 {
			pageSize = 1000
		}
	}

	start := (page - 1) * pageSize
	if start > len(matched) {
		start = len(matched)
	}
	end := start + pageSize
	if end > len(matched) {
		end = len(matched)
	}

	return matched[start:end], int64(len(matched)), nil
}

func intPtr(v int) *int {
	return &v
}

// ListProviderNames returns all provider names sorted in ascending order.
func (m *ProviderManager) ListProviderNames(ctx context.Context) ([]string, error) {
	names, err := m.storager.FetchProviderNames(ctx)
	if err != nil {
		return nil, err
	}
	return names, nil
}

// ValidateProviderParam validates a provider create/update request.
func ValidateProviderParam(param *ProviderParam) error {
	if param == nil {
		return xerror.WrapParamErrorWithMsg("provider is required")
	}
	if param.Name == nil || *param.Name == "" {
		return xerror.WrapParamErrorWithMsg("provider name is required")
	}
	if err := ProviderName(*param.Name); err != nil {
		return err
	}

	if param.Description != nil {
		if err := validateProviderDescription(*param.Description); err != nil {
			return err
		}
	}

	if err := validateProviderInstancePool(param.InstancePool); err != nil {
		return err
	}

	if len(param.ModelProtocols) == 0 {
		return xerror.WrapParamErrorWithMsg("model_protocols is required")
	}
	seenProtocol := map[string]bool{}
	for i, p := range param.ModelProtocols {
		if p == "" {
			return xerror.WrapParamErrorWithMsg("model_protocols[%d] cannot be empty", i)
		}
		if !ValidModelProtocols[p] {
			return xerror.WrapParamErrorWithMsg("invalid model_protocols[%d]: %s", i, p)
		}
		if seenProtocol[p] {
			return xerror.WrapParamErrorWithMsg("duplicate model_protocol: %s", p)
		}
		seenProtocol[p] = true
	}

	if param.ModelEndpoint != nil {
		if param.ModelEndpoint.Schema == "" {
			param.ModelEndpoint.Schema = "https"
		}
		if param.ModelEndpoint.Schema != "http" && param.ModelEndpoint.Schema != "https" {
			return xerror.WrapParamErrorWithMsg("model_endpoint.schema must be http or https")
		}
		if param.ModelEndpoint.URI == "" {
			param.ModelEndpoint.URI = "/v1/models"
		}
		if !strings.HasPrefix(param.ModelEndpoint.URI, "/") {
			return xerror.WrapParamErrorWithMsg("model_endpoint.uri must start with '/'")
		}
	}

	if len(param.Models) > 0 {
		seenModel := map[string]bool{}
		for i, m := range param.Models {
			if strings.TrimSpace(m) == "" {
				return xerror.WrapParamErrorWithMsg("models[%d] cannot be empty", i)
			}
			if seenModel[m] {
				return xerror.WrapParamErrorWithMsg("duplicate model: %s", m)
			}
			seenModel[m] = true
		}
	}

	if len(param.Keys) > 0 {
		seenKey := map[string]bool{}
		for i, k := range param.Keys {
			if strings.TrimSpace(k.Name) == "" {
				return xerror.WrapParamErrorWithMsg("keys[%d].name is required", i)
			}
			if len(k.Name) < 1 || len(k.Name) > 128 {
				return xerror.WrapParamErrorWithMsg("keys[%d].name length must be between 1 and 128", i)
			}
			if seenKey[k.Name] {
				return xerror.WrapParamErrorWithMsg("duplicate key name: %s", k.Name)
			}
			seenKey[k.Name] = true

			if strings.TrimSpace(k.Key) == "" {
				return xerror.WrapParamErrorWithMsg("keys[%d].key is required", i)
			}
			if len(k.Key) > MaxProviderKeyLength {
				return xerror.WrapParamErrorWithMsg("keys[%d].key length must be <= %d", i, MaxProviderKeyLength)
			}
		}
	}

	if param.TimeZone != nil {
		if err := validateTimeZone(*param.TimeZone); err != nil {
			return err
		}
	}

	if err := validatePricingTiers(param.Tiers); err != nil {
		return err
	}

	return nil
}

// ValidatePricingTiersParam validates a pricing tiers update request.
func ValidatePricingTiersParam(param *PricingTiersParam) error {
	if param == nil {
		return xerror.WrapParamErrorWithMsg("pricing tiers param is required")
	}
	if err := validateTimeZone(param.TimeZone); err != nil {
		return err
	}
	return validatePricingTiers(param.Tiers)
}

func validatePricingTiers(tiers []PricingTier) error {
	seenTier := map[string]bool{}
	for i, tier := range tiers {
		if strings.TrimSpace(tier.Name) == "" {
			return xerror.WrapParamErrorWithMsg("tiers[%d].name is required", i)
		}
		// 初期只支持 peak tier
		if tier.Name != "peak" {
			return xerror.WrapParamErrorWithMsg("tiers[%d].name: unsupported tier name %s, only 'peak' is allowed", i, tier.Name)
		}
		if seenTier[tier.Name] {
			return xerror.WrapParamErrorWithMsg("duplicate tier name: %s", tier.Name)
		}
		seenTier[tier.Name] = true

		if len(tier.TimeRanges) == 0 {
			return xerror.WrapParamErrorWithMsg("tiers[%d].time_ranges is required", i)
		}

		seenRange := map[string]bool{}
		for j, tr := range tier.TimeRanges {
			key := fmt.Sprintf("%v|%s|%s", tr.Weekdays, tr.Start, tr.End)
			if seenRange[key] {
				return xerror.WrapParamErrorWithMsg("tiers[%d].time_ranges[%d]: duplicate time range", i, j)
			}
			seenRange[key] = true

			if err := validateTimeRange(tr); err != nil {
				return xerror.WrapParamErrorWithMsg("tiers[%d].time_ranges[%d]: %v", i, j, err)
			}

			for k := 0; k < j; k++ {
				if timeRangesOverlap(tr, tier.TimeRanges[k]) {
					return xerror.WrapParamErrorWithMsg("tiers[%d].time_ranges[%d] overlaps with time_ranges[%d]", i, j, k)
				}
			}
		}
	}
	return nil
}

func validateTimeZone(tz string) error {
	if strings.TrimSpace(tz) == "" {
		return nil
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return xerror.WrapParamErrorWithMsg("invalid time_zone: %s", tz)
	}
	return nil
}

// timeRangesOverlap reports whether two time ranges overlap.
// It assumes each range has been validated (end > start, weekdays in [0,6]).
// Two ranges overlap iff they share at least one weekday and their time intervals overlap.
func timeRangesOverlap(a, b TimeRange) bool {
	// Check weekday intersection.
	sharedWeekday := false
	if len(a.Weekdays) == 0 || len(b.Weekdays) == 0 {
		sharedWeekday = true
	} else {
		for _, wdA := range a.Weekdays {
			for _, wdB := range b.Weekdays {
				if wdA == wdB {
					sharedWeekday = true
					break
				}
			}
			if sharedWeekday {
				break
			}
		}
	}
	if !sharedWeekday {
		return false
	}

	ast, _ := parseHHMM(a.Start)
	aet, _ := parseHHMM(a.End)
	bst, _ := parseHHMM(b.Start)
	bet, _ := parseHHMM(b.End)

	// Intervals are left-closed, right-open.
	return ast < bet && bst < aet
}

func validateTimeRange(tr TimeRange) error {
	seenWeekday := map[int]bool{}
	for _, wd := range tr.Weekdays {
		if wd < 0 || wd > 6 {
			return fmt.Errorf("weekdays must be between 0 and 6")
		}
		if seenWeekday[wd] {
			return fmt.Errorf("duplicate weekday: %d", wd)
		}
		seenWeekday[wd] = true
	}

	start, err := parseHHMM(tr.Start)
	if err != nil {
		return fmt.Errorf("invalid start time: %v", err)
	}
	end, err := parseHHMM(tr.End)
	if err != nil {
		return fmt.Errorf("invalid end time: %v", err)
	}
	if end <= start {
		return fmt.Errorf("end must be greater than start")
	}
	return nil
}

func parseHHMM(s string) (int, error) {
	if len(s) != 5 || s[2] != ':' {
		return 0, fmt.Errorf("time must be in HH:MM format")
	}
	hour, err := strconv.Atoi(s[0:2])
	if err != nil {
		return 0, fmt.Errorf("invalid hour")
	}
	min, err := strconv.Atoi(s[3:5])
	if err != nil {
		return 0, fmt.Errorf("invalid minute")
	}
	if hour < 0 || hour > 23 || min < 0 || min > 59 {
		return 0, fmt.Errorf("time out of range")
	}
	return hour*60 + min, nil
}

// ProviderName validates a provider name.
func ProviderName(s string) error {
	if len(s) < 1 || len(s) > MaxProviderNameLength {
		return xerror.WrapParamErrorWithMsg("provider name length must be between 1 and %d", MaxProviderNameLength)
	}
	if !providerNameToken.MatchString(s) {
		return xerror.WrapParamErrorWithMsg("provider name contains invalid characters")
	}
	first := s[0]
	last := s[len(s)-1]
	if first == '.' || first == '-' || first == '_' || last == '.' || last == '-' || last == '_' {
		return xerror.WrapParamErrorWithMsg("provider name cannot start or end with '.', '-', or '_'")
	}
	return nil
}

func validateProviderDescription(s string) error {
	if len(s) > MaxProviderDescriptionLength {
		return xerror.WrapParamErrorWithMsg("description length must be <= %d", MaxProviderDescriptionLength)
	}
	for _, r := range s {
		if r < 32 || r == 127 {
			return xerror.WrapParamErrorWithMsg("description contains control characters")
		}
	}
	return nil
}

func validateProviderInstancePool(instances []ProviderInstance) error {
	if len(instances) == 0 {
		return xerror.WrapParamErrorWithMsg("instance_pool is required")
	}
	comboSet := map[string]struct{}{}
	hasPositiveWeight := false
	for i, inst := range instances {
		if strings.TrimSpace(inst.Addr) == "" {
			return xerror.WrapParamErrorWithMsg("instance_pool[%d]: instance addr is required", i)
		}
		if inst.Port <= 0 || inst.Port > 65535 {
			return xerror.WrapParamErrorWithMsg("instance_pool[%d]: instance port must be between 1 and 65535", i)
		}
		if inst.Weight < 0 || inst.Weight > 100 {
			return xerror.WrapParamErrorWithMsg("instance_pool[%d]: instance weight must be between 0 and 100", i)
		}
		if inst.Weight > 0 {
			hasPositiveWeight = true
		}
		key := fmt.Sprintf("%s|%d", inst.Addr, inst.Port)
		if _, ok := comboSet[key]; ok {
			return xerror.WrapParamErrorWithMsg("instance_pool: duplicate instance addr/port combination: %s", key)
		}
		comboSet[key] = struct{}{}
	}
	if !hasPositiveWeight {
		return xerror.WrapParamErrorWithMsg("instance_pool must have at least one instance with weight > 0")
	}
	return nil
}

// DefaultModelEndpoint returns the default model discovery endpoint.
func DefaultModelEndpoint() *ProviderEndpoint {
	return &ProviderEndpoint{Schema: "https", URI: "/v1/models"}
}

// FillDefaults fills default values for a provider param.
func FillDefaults(param *ProviderParam) {
	if param == nil {
		return
	}
	if param.ModelEndpoint == nil {
		param.ModelEndpoint = DefaultModelEndpoint()
	} else {
		if param.ModelEndpoint.Schema == "" {
			param.ModelEndpoint.Schema = "https"
		}
		if param.ModelEndpoint.URI == "" {
			param.ModelEndpoint.URI = "/v1/models"
		}
	}
	if param.Models == nil {
		param.Models = []string{}
	}
	if param.Keys == nil {
		param.Keys = []ProviderKey{}
	}
	if param.TimeZone == nil {
		defaultTZ := "Asia/Shanghai"
		param.TimeZone = &defaultTZ
	}
}

// NowUnix returns the current Unix timestamp in seconds.
func NowUnix() int64 {
	return time.Now().Unix()
}

// KeyMap builds a map from key name to key value.
func KeyMap(keys []ProviderKey) map[string]string {
	m := make(map[string]string, len(keys))
	for _, k := range keys {
		m[k.Name] = k.Key
	}
	return m
}

// HasModel reports whether model is in the provider's models list.
func HasModel(provider *Provider, model string) bool {
	if provider == nil {
		return false
	}
	for _, m := range provider.Models {
		if m == model {
			return true
		}
	}
	return false
}

// BuildAuthHeader returns the auth header name/value for the given protocol.
func BuildAuthHeader(protocol, key string) (string, string) {
	switch protocol {
	case "anthropic":
		return "x-api-key", key
	case "openai":
		fallthrough
	default:
		return "Authorization", "Bearer " + key
	}
}

// providerInstancePoolEqual reports whether two instance pools are identical.
// Order matters because the pool is a snapshot; a reordered list is still a
// different snapshot.
func providerInstancePoolEqual(a, b []ProviderInstance) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Addr != b[i].Addr ||
			a[i].Port != b[i].Port ||
			a[i].Weight != b[i].Weight ||
			a[i].Disable != b[i].Disable {
			return false
		}
	}
	return true
}

// providerKeysEqual reports whether two provider key lists are identical.
// Order matters because keys are replaced as an ordered array.
func providerKeysEqual(a, b []ProviderKey) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Name != b[i].Name || a[i].Key != b[i].Key {
			return false
		}
	}
	return true
}

// providerModelsEqual reports whether two provider model lists are identical.
// Order matters because models are replaced as an ordered array.
func providerModelsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// applyProviderUpdate returns a Provider that reflects the existing provider
// merged with the update param. It is used to build the provider snapshot passed
// to sync hooks without an extra DB round-trip.
func applyProviderUpdate(existing *Provider, param *ProviderParam) *Provider {
	if existing == nil {
		return nil
	}

	newProvider := *existing

	if param.Name != nil {
		newProvider.Name = *param.Name
	}
	if param.Description != nil {
		newProvider.Description = *param.Description
	}
	if param.ModelEndpoint != nil {
		newProvider.ModelEndpoint = param.ModelEndpoint
	}
	if param.Models != nil {
		newProvider.Models = param.Models
	}
	if param.Keys != nil {
		newProvider.Keys = param.Keys
	}
	if param.InstancePool != nil {
		newProvider.InstancePool = param.InstancePool
	}
	if param.ModelProtocols != nil {
		newProvider.ModelProtocols = param.ModelProtocols
	}
	if param.TimeZone != nil {
		newProvider.TimeZone = *param.TimeZone
	}
	if param.Tiers != nil {
		newProvider.Tiers = param.Tiers
	}

	return &newProvider
}

// ProviderNamePtr returns a pointer to a provider name for use in filters.
func ProviderNamePtr(s string) *string {
	return lib.PString(s)
}
