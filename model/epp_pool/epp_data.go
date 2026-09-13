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

package epp_pool

import (
	"context"
	"fmt"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/iversion_control"
)

// ConfigTopicEppData is the version-controlled config topic of the merged
// epp_data export (epp_config + assignment, see design-changes.md §3).
const ConfigTopicEppData = "epp_data"

// AssignmentEntry is one cluster entry of the exported assignment section.
type AssignmentEntry struct {
	Primary string  `json:"primary"`
	Standby *string `json:"standby"`
}

// EppDataConfig is the Config body of the epp_data export.
type EppDataConfig struct {
	EppConfig  map[string]*EndpointPickerConfig `json:"epp_config"`
	Assignment map[string]*AssignmentEntry      `json:"assignment"`
}

// ExportEppDataConfig is the versioned epp_data export payload.
type ExportEppDataConfig struct {
	Version string         `json:"Version"`
	Config  *EppDataConfig `json:"Config"`
}

// UpdateVersion implements iversion_control.VersionValuable.
func (c *ExportEppDataConfig) UpdateVersion(version string) error {
	c.Version = version
	return nil
}

// EppDataGenerator generates the merged epp_data export (design-changes.md §3):
// the epp_config section compiles every EPP-mode cluster's simplified
// epp_config into an EndpointPickerConfig, and the assignment section is the
// full read-time-join view restricted to validly assigned EPP clusters.
func (m *EppPoolManager) EppDataGenerator(ctx context.Context) (*iversion_control.ExportData, error) {
	if m.clusterSource == nil {
		return nil, fmt.Errorf("epp pool: cluster source is not configured")
	}

	eppClusters, err := m.clusterSource.FetchEPPClusters(ctx)
	if err != nil {
		return nil, err
	}

	instances, err := m.storager.FetchInstanceList(ctx, &InstanceFilter{})
	if err != nil {
		return nil, err
	}
	pool := buildPool(m.poolName, instances)

	assignments, err := m.storager.FetchAssignmentList(ctx)
	if err != nil {
		return nil, err
	}

	conf := &EppDataConfig{
		EppConfig:  map[string]*EndpointPickerConfig{},
		Assignment: map[string]*AssignmentEntry{},
	}

	for _, cluster := range eppClusters {
		simplified, err := ParseEppConfig(cluster.EppConfigJSON)
		if err != nil {
			return nil, fmt.Errorf("epp pool: parse epp_config of cluster %q error: %s", cluster.Name, err.Error())
		}
		if simplified == nil {
			return nil, fmt.Errorf("epp pool: cluster %q is in EPP mode but has no epp_config", cluster.Name)
		}
		conf.EppConfig[cluster.Name] = CompileEppConfig(cluster.Name, simplified)
	}

	view := buildAssignmentsView(pool, eppClusters, assignments)
	for _, entry := range view.Clusters {
		if entry.Primary == nil {
			// Unassigned or degraded: no assignment entry (EPP side alarms locally).
			continue
		}

		assignmentEntry := &AssignmentEntry{Primary: entry.Primary.ID}
		if entry.Standby != nil {
			standbyID := entry.Standby.ID
			assignmentEntry.Standby = &standbyID
		}
		conf.Assignment[entry.Cluster] = assignmentEntry
	}

	rst := &ExportEppDataConfig{Config: conf}
	rst.UpdateVersion(iversion_control.ZeroVersion)

	return &iversion_control.ExportData{
		Topic:              ConfigTopicEppData,
		DataWithoutVersion: rst,
	}, nil
}

// ExportEppData exports the epp_data config via the version-control framework:
// a new version is published only when the content changed, otherwise the last
// version is returned with a nil payload marker (Data: null semantics).
func (m *EppPoolManager) ExportEppData(ctx context.Context, lastVersion string) (*ExportEppDataConfig, error) {
	if m.versionControlManager == nil {
		return nil, fmt.Errorf("epp pool: version control manager is not configured")
	}

	rst, err := m.versionControlManager.ExportConfig(ctx, ConfigTopicEppData, m.EppDataGenerator)
	if err != nil {
		return nil, err
	}

	if rst.DataWithoutVersion == nil {
		return nil, fmt.Errorf("epp pool: EppDataGenerator.DataWithoutVersion is nil")
	}

	conf, ok := rst.DataWithoutVersion.(*ExportEppDataConfig)
	if !ok {
		return nil, fmt.Errorf("epp pool: convert EppDataGenerator.DataWithoutVersion to ExportEppDataConfig is error")
	}

	if conf.Version == lastVersion {
		return nil, nil
	}

	return conf, nil
}

// triggerEppDataExport eagerly publishes a new epp_data version when the
// version-control manager is configured. Errors are logged, not propagated.
func (m *EppPoolManager) triggerEppDataExport(ctx context.Context) {
	if m.versionControlManager == nil {
		return
	}

	if _, err := m.ExportEppData(ctx, ""); err != nil {
		eppLogError("failed to export epp_data: %v", err)
	}
}
