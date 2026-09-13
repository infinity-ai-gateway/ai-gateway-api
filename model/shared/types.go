package shared

import (
	"context"
	"encoding/json"
	"time"
)

type EntitySummary struct {
	ID   *string `json:"id"`
	Name *string `json:"name"`
	Type *string `json:"type"`
}

type EntityFilter struct {
	EntityID *string `form:"id"`
	Name     *string `form:"name"`
	Type     *string `form:"type"`
	ParentID *string `form:"parent_id"`
	Page     *int    `form:"page"`
	PageSize *int    `form:"page_size"`
}

type EntityStorager interface {
	FetchEntity(ctx context.Context, filter *EntityFilter) (*EntitySummary, error)
}

type BalanceSummary struct {
	Used      *float64 `json:"used"`
	Remaining *float64 `json:"remaining"`
}

type QuotaPlanParam struct {
	Unlimited             *bool           `json:"unlimited"`
	PassWhenNoEnoughQuota *bool           `json:"pass_when_no_enough_quota"`
	Quota                 *float64        `json:"quota"`
	Unit                  *string         `json:"unit"`
	ResetPeriod           *string         `json:"reset_period"`
	LastResetAt           *time.Time      `json:"last_reset_at,omitempty"`
	Balance               *BalanceSummary `json:"balance,omitempty"`
}

type TPMConfig struct {
	Name          string `json:"name"`
	Model         string `json:"model"`
	WindowMinutes int    `json:"window_minutes"`
	MaxTokens     int    `json:"max_tokens"`
	StepMinutes   int    `json:"step_minutes"`
}

type RPMConfig struct {
	Name          string `json:"name"`
	Model         string `json:"model"`
	WindowMinutes int    `json:"window_minutes"`
	MaxRequests   int    `json:"max_requests"`
}

type RateLimitRules struct {
	TpmConfigs     []TPMConfig `json:"tpm"`
	RpmConfigs     []RPMConfig `json:"rpm"`
	MaxConcurrency *int        `json:"max_concurrency"`
}

type RateLimitPolicyParam struct {
	Enabled *bool           `json:"enabled"`
	Rules   *RateLimitRules `json:"rules"`
}

type AiRouteTargetParam struct {
	ClusterName *string `json:"cluster_name"`
	Model       *string `json:"model"`
	Weight      *int    `json:"weight"`
}

// UnmarshalJSON accepts both the new snake_case keys and the legacy camel-case
// keys for backward compatibility with existing database records and old
// clients. Serialization continues to emit only the new keys.
func (t *AiRouteTargetParam) UnmarshalJSON(data []byte) error {
	type Alias AiRouteTargetParam
	aux := &struct {
		*Alias
		OldClusterName *string `json:"ClusterName"`
		OldModel       *string `json:"Model"`
		OldWeight      *int    `json:"Weight"`
	}{
		Alias: (*Alias)(t),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if t.ClusterName == nil && aux.OldClusterName != nil {
		t.ClusterName = aux.OldClusterName
	}
	if t.Model == nil && aux.OldModel != nil {
		t.Model = aux.OldModel
	}
	if t.Weight == nil && aux.OldWeight != nil {
		t.Weight = aux.OldWeight
	}
	return nil
}

type AiRouteFallbackParam struct {
	ClusterName *string `json:"cluster_name"`
	Model       *string `json:"model"`
}

// UnmarshalJSON accepts both the new snake_case keys and the legacy camel-case
// keys for backward compatibility.
func (f *AiRouteFallbackParam) UnmarshalJSON(data []byte) error {
	type Alias AiRouteFallbackParam
	aux := &struct {
		*Alias
		OldClusterName *string `json:"ClusterName"`
		OldModel       *string `json:"Model"`
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if f.ClusterName == nil && aux.OldClusterName != nil {
		f.ClusterName = aux.OldClusterName
	}
	if f.Model == nil && aux.OldModel != nil {
		f.Model = aux.OldModel
	}
	return nil
}

type AiRouteRuleParam struct {
	Name      *string                 `json:"name"`
	Cond      *string                 `json:"cond"`
	Targets   []*AiRouteTargetParam   `json:"targets"`
	Fallbacks []*AiRouteFallbackParam `json:"fallbacks"`
}

// UnmarshalJSON accepts both the new snake_case keys and the legacy camel-case
// keys for backward compatibility.
func (r *AiRouteRuleParam) UnmarshalJSON(data []byte) error {
	type Alias AiRouteRuleParam
	aux := &struct {
		*Alias
		OldCond *string `json:"Cond"`
	}{
		Alias: (*Alias)(r),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if r.Cond == nil && aux.OldCond != nil {
		r.Cond = aux.OldCond
	}
	return nil
}

type RouteRulesParam struct {
	ID      *int64              `json:"-"`
	Enabled *bool               `json:"enabled"`
	Rules   []*AiRouteRuleParam `json:"rules"`
}

type RouteTableParam struct {
	ID      *int64 `json:"id,omitempty"`
	Type    string `json:"type"`
	Owner   string `json:"owner"`
	Enabled bool   `json:"enabled"`
}

const (
	// RouteRulesTypeAPIKey is the internal storage/BFE export value for
	// API-Key route rules. The OpenAPI /route-tables endpoint exposes it
	// as RouteTableTypeAPIKey ("api_key") for consistency with the API doc.
	RouteRulesTypeAPIKey = "apikey"
	RouteRulesTypeEntity = "entity"
	RouteRulesTypeGlobal = "global"
)

// OpenAPI-facing route table type values, as defined in the API document.
const (
	RouteTableTypeAPIKey = "api_key"
	RouteTableTypeEntity = "entity"
	RouteTableTypeGlobal = "global"
)

// ToRouteTableType maps an internal route rules type to the value exposed by
// the OpenAPI /route-tables endpoint.
func ToRouteTableType(internalType string) string {
	if internalType == RouteRulesTypeAPIKey {
		return RouteTableTypeAPIKey
	}
	return internalType
}

// FromRouteTableType maps an OpenAPI /route-tables type query parameter to the
// internal route rules type used in storage and BFE exports.
func FromRouteTableType(apiType string) string {
	if apiType == RouteTableTypeAPIKey {
		return RouteRulesTypeAPIKey
	}
	return apiType
}

// RouteRulesFilter defines filters for querying route rules
type RouteRulesFilter struct {
	Type      *string `form:"type"`
	Owner     *string `form:"owner"`
	Enabled   *bool   `form:"enabled"`
	Page      *int    `form:"page"`
	PageSize  *int    `form:"page_size"`
	SortBy    *string `form:"sort_by"`
	SortOrder *string `form:"sort_order"`
}

// RouteRulesStorager defines storage operations for route rules
type RouteRulesStorager interface {
	CreateRouteRules(ctx context.Context, ruleType string, owner *string, param *RouteRulesParam) (int64, error)
	FetchRouteRules(ctx context.Context, ruleType string, owner *string) (*RouteRulesParam, error)
	FetchRouteRulesList(ctx context.Context, filter *RouteRulesFilter) ([]*RouteTableParam, int64, error)
	UpdateRouteRules(ctx context.Context, id int64, param *RouteRulesParam) (int64, error)
	DeleteRouteRules(ctx context.Context, id int64) error
	FetchRouteRulesByID(ctx context.Context, id int64) (*RouteRulesParam, error)
	// FetchAllRouteRules returns all route rules without pagination.
	// Used by reference checkers that must scan every rule table.
	FetchAllRouteRules(ctx context.Context) ([]*RouteRulesParam, error)
}

type QuotaPlanStorager interface {
	CreateQuotaPlan(ctx context.Context, param *QuotaPlanParam) (int64, error)
	UpdateQuotaPlan(ctx context.Context, id int64, param *QuotaPlanParam) (int64, error)
	DeleteQuotaPlan(ctx context.Context, id int64) error
	FetchQuotaPlan(ctx context.Context, id int64) (*QuotaPlanParam, error)
}

type RateLimitPolicyStorager interface {
	CreateRateLimitPolicy(ctx context.Context, param *RateLimitPolicyParam) (int64, error)
	UpdateRateLimitPolicy(ctx context.Context, id int64, param *RateLimitPolicyParam) (int64, error)
	DeleteRateLimitPolicy(ctx context.Context, id int64) error
	FetchRateLimitPolicy(ctx context.Context, id int64) (*RateLimitPolicyParam, error)
}
