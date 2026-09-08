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
	"context"
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/itxn"
)

// PriceMap is a map of price keys to their numeric values (yuan per unit).
// Prices may use more than 8 decimal places; both decimal and scientific
// notation are accepted on input and may appear in JSON output.
type PriceMap map[string]float64

// TierPriceMap is a map of tier names to PriceMap values.
type TierPriceMap map[string]map[string]float64

// ModelPrice represents a single model pricing/capability record.
type ModelPrice struct {
	ID                  int64                  `json:"id" yaml:"id,omitempty"`
	Provider            string                 `json:"provider" yaml:"provider"`
	Model               string                 `json:"model" yaml:"model"`
	BaseModel           string                 `json:"base_model" yaml:"base_model"`
	Mode                string                 `json:"mode" yaml:"mode"`
	Capabilities        []string               `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	SupportedParameters []string               `json:"supported_parameters,omitempty" yaml:"supported_parameters,omitempty"`
	Limits              map[string]interface{} `json:"limits,omitempty" yaml:"limits,omitempty"`
	Prices              PriceMap               `json:"prices,omitempty" yaml:"prices,omitempty"`
	TierPrices          TierPriceMap           `json:"tier_prices,omitempty" yaml:"tier_prices,omitempty"`
	PriceCurrency       string                 `json:"price_currency,omitempty" yaml:"price_currency,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	CreateTime          *int64                 `json:"create_time,omitempty" yaml:"create_time,omitempty"`
	UpdateTime          *int64                 `json:"update_time,omitempty" yaml:"update_time,omitempty"`
}

// ModelPriceFilter is used to query model price records.
type ModelPriceFilter struct {
	ID       *int64
	Provider *string
	Model    *string
	Mode     *string
	Page     *int
	PageSize *int
}

// Key returns the unique (provider, model, mode) key of a record.
func (m *ModelPrice) Key() string {
	return fmt.Sprintf("%s|%s|%s", m.Provider, m.Model, m.Mode)
}

// ModelPriceStorager defines storage operations for model pricing records.
type ModelPriceStorager interface {
	CreateModelPrice(ctx context.Context, param *ModelPrice) (int64, error)
	UpdateModelPrice(ctx context.Context, filter *ModelPriceFilter, param *ModelPrice) (int64, error)
	DeleteModelPrice(ctx context.Context, filter *ModelPriceFilter) error
	DeleteAllModelPrices(ctx context.Context) error
	FetchModelPrice(ctx context.Context, filter *ModelPriceFilter) (*ModelPrice, error)
	FetchModelPriceList(ctx context.Context, filter *ModelPriceFilter) ([]*ModelPrice, int64, error)
	ListProviders(ctx context.Context) ([]string, error)
}

// Manager provides business-level operations for model pricing records.
type Manager struct {
	txn                 itxn.TxnStorager
	storager            ModelPriceStorager
	operationLogManager ioperlog.OperationLogRecorder
}

// NewManager creates a new Manager instance.
func NewManager(txn itxn.TxnStorager, storager ModelPriceStorager) *Manager {
	return &Manager{
		txn:      txn,
		storager: storager,
	}
}

// SetOperationLogManager injects the operation log recorder.
func (m *Manager) SetOperationLogManager(manager ioperlog.OperationLogRecorder) {
	m.operationLogManager = manager
}

// CreateModelPrice creates a single model price record.
func (m *Manager) CreateModelPrice(ctx context.Context, param *ModelPrice) (int64, error) {
	if err := ValidateModelPrice(param); err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionCreate), "", modelPriceName(param), nil, modelPriceToMap(param), err)
		return 0, err
	}
	id, err := m.storager.CreateModelPrice(ctx, param)
	if err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionCreate), "", modelPriceName(param), nil, modelPriceToMap(param), err)
		return 0, err
	}

	m.recordModelPriceOperation(ctx, string(ioperlog.ActionCreate), modelPriceIDString(id), modelPriceName(param), nil, modelPriceToMap(param), nil)
	return id, nil
}

// UpdateModelPrice updates a model price record matched by filter.
func (m *Manager) UpdateModelPrice(ctx context.Context, filter *ModelPriceFilter, param *ModelPrice) (int64, error) {
	resourceID := ""
	if filter != nil && filter.ID != nil {
		resourceID = modelPriceIDString(*filter.ID)
	}

	if err := ValidateModelPrice(param); err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionUpdate), resourceID, modelPriceName(param), nil, modelPriceToMap(param), err)
		return 0, err
	}

	oldPrice, err := m.storager.FetchModelPrice(ctx, filter)
	if err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionUpdate), resourceID, modelPriceName(param), nil, modelPriceToMap(param), err)
		return 0, err
	}

	affected, err := m.storager.UpdateModelPrice(ctx, filter, param)
	if err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionUpdate), resourceID, modelPriceName(param), modelPriceToMap(oldPrice), modelPriceToMap(param), err)
		return affected, err
	}

	m.recordModelPriceOperation(ctx, string(ioperlog.ActionUpdate), resourceID, modelPriceName(param), modelPriceToMap(oldPrice), modelPriceToMap(param), nil)
	return affected, nil
}

// ListProviders returns all distinct provider names in model_prices.
func (m *Manager) ListProviders(ctx context.Context) ([]string, error) {
	return m.storager.ListProviders(ctx)
}

// DeleteModelPrice deletes model price records matched by filter.
func (m *Manager) DeleteModelPrice(ctx context.Context, filter *ModelPriceFilter) error {
	resourceID := ""
	if filter != nil && filter.ID != nil {
		resourceID = modelPriceIDString(*filter.ID)
	}

	oldPrice, err := m.storager.FetchModelPrice(ctx, filter)
	if err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionDelete), resourceID, "", nil, nil, err)
		return err
	}

	if err := m.storager.DeleteModelPrice(ctx, filter); err != nil {
		m.recordModelPriceOperation(ctx, string(ioperlog.ActionDelete), resourceID, modelPriceName(oldPrice), modelPriceToMap(oldPrice), nil, err)
		return err
	}

	m.recordModelPriceOperation(ctx, string(ioperlog.ActionDelete), resourceID, modelPriceName(oldPrice), modelPriceToMap(oldPrice), nil, nil)
	return nil
}

// FetchModelPrice fetches a single model price record matched by filter.
func (m *Manager) FetchModelPrice(ctx context.Context, filter *ModelPriceFilter) (*ModelPrice, error) {
	return m.storager.FetchModelPrice(ctx, filter)
}

// FetchModelPriceList fetches a paginated list of model price records.
func (m *Manager) FetchModelPriceList(ctx context.Context, filter *ModelPriceFilter) ([]*ModelPrice, int64, error) {
	return m.storager.FetchModelPriceList(ctx, filter)
}

// ImportMode defines how model prices are imported.
type ImportMode string

const (
	ImportModeReplace ImportMode = "replace"
	ImportModeMerge   ImportMode = "merge"
)

// ImportModelPrices imports a batch of model price records.
// mode = "replace": delete all existing records then insert the new ones.
// mode = "merge":  update existing (provider, model, mode) records and insert new ones.
func (m *Manager) ImportModelPrices(ctx context.Context, entries []*ModelPrice, mode string) (imported int, skipped int, err error) {
	if mode != string(ImportModeReplace) && mode != string(ImportModeMerge) {
		err = xerror.WrapParamErrorWithMsg("import mode must be replace or merge")
		m.recordModelPriceImport(ctx, mode, 0, entries, err)
		return 0, 0, err
	}

	for _, entry := range entries {
		if err := ValidateModelPrice(entry); err != nil {
			m.recordModelPriceImport(ctx, mode, 0, entries, err)
			return 0, 0, err
		}
	}

	err = m.txn.AtomExecute(ctx, func(ctx context.Context) error {
		imported = 0
		skipped = 0

		if mode == string(ImportModeReplace) {
			if err := m.storager.DeleteAllModelPrices(ctx); err != nil {
				return err
			}
			for _, entry := range entries {
				if _, err := m.storager.CreateModelPrice(ctx, entry); err != nil {
					return err
				}
				imported++
			}
			return nil
		}

		// merge mode
		existing, _, err := m.storager.FetchModelPriceList(ctx, &ModelPriceFilter{})
		if err != nil {
			return err
		}
		existingMap := make(map[string]*ModelPrice, len(existing))
		for _, e := range existing {
			existingMap[e.Key()] = e
		}

		for _, entry := range entries {
			if old, ok := existingMap[entry.Key()]; ok {
				filter := &ModelPriceFilter{ID: &old.ID}
				if _, err := m.storager.UpdateModelPrice(ctx, filter, entry); err != nil {
					return err
				}
				imported++
			} else {
				if _, err := m.storager.CreateModelPrice(ctx, entry); err != nil {
					return err
				}
				imported++
			}
		}
		return nil
	})
	if err != nil {
		m.recordModelPriceImport(ctx, mode, imported, entries, err)
		return imported, skipped, err
	}

	m.recordModelPriceImport(ctx, mode, imported, entries, nil)
	return imported, skipped, nil
}
