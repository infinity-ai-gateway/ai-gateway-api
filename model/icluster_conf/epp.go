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

package icluster_conf

import (
	"context"
	"strings"

	"github.com/rainway-ai-gateway/ai-gateway-api/lib/xerror"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/epp_pool"
	"github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

const (
	// BalanceModeWRR is the default cluster balance mode (BFE local WRR).
	BalanceModeWRR = "WRR"
	// BalanceModeEPP hands backend selection over to the EPP scheduler.
	BalanceModeEPP = "EPP"
)

// EPPAssignmentResolver resolves the ordered EPP endpoint addresses of a
// cluster's assignment ([0]=primary, [1]=standby). It is implemented by
// epp_pool.EppPoolManager and injected at wiring time; the export path only
// depends on this interface (dependency injection, no reverse reference).
type EPPAssignmentResolver interface {
	// GetAssignmentEndpoints returns [primary, standby] addresses joined by
	// net.JoinHostPort (standby omitted for single instance groups).
	// found is false when the cluster has no valid assignment.
	GetAssignmentEndpoints(ctx context.Context, clusterName string) (endpoints []string, found bool, err error)
}

// validateClusterBalanceConfig validates the balance_mode/epp_config pair of
// a cluster write (design-changes.md §2.3):
//   - balance_mode is one of WRR/EPP (absent keeps oldMode, default WRR);
//   - epp_config, whenever non-empty, is parsed strictly and field-validated,
//     regardless of balance_mode (WRR hibernation keeps it validated);
//   - balance_mode=EPP requires a non-empty, non-empty-object epp_config.
//
// It returns the effective balance mode and the effective raw epp_config JSON
// (the carried value, or the stored one when not carried; hibernation
// retention: fields not explicitly carried are not persisted).
func validateClusterBalanceConfig(balanceMode *string, oldMode string, eppConfig *string, oldEppConfig string) (string, string, error) {
	mode := oldMode
	if mode == "" {
		mode = BalanceModeWRR
	}
	if balanceMode != nil && *balanceMode != "" {
		mode = *balanceMode
	}
	switch mode {
	case BalanceModeWRR, BalanceModeEPP:
	default:
		return "", "", xerror.WrapParamErrorWithMsg("balance_mode must be one of %q/%q, got %q", BalanceModeWRR, BalanceModeEPP, mode)
	}

	raw := oldEppConfig
	if eppConfig != nil {
		raw = strings.TrimSpace(*eppConfig)
	}

	conf, err := epp_pool.ParseEppConfig(raw)
	if err != nil {
		return "", "", err
	}
	if conf != nil {
		if err := conf.Validate(); err != nil {
			return "", "", err
		}
	}

	if mode == BalanceModeEPP && conf.IsEmpty() {
		return "", "", xerror.WrapParamErrorWithMsg("epp_config is required when balance_mode is %q", BalanceModeEPP)
	}

	return mode, raw, nil
}

// assignClusterToEPP triggers the EPP assignment for the cluster. Assignment
// failure never blocks the cluster write: it is logged at error level and the
// periodic reconciler converges later (design-changes.md §2.3/§4.3).
func (cm *ClusterManager) assignClusterToEPP(ctx context.Context, clusterName string) {
	if cm.eppPoolManager == nil {
		stateful.AccessLogger.Error("cluster %s is in EPP mode but epp pool manager is not configured, assignment deferred to reconciler", clusterName)
		return
	}

	if err := cm.eppPoolManager.AssignCluster(ctx, clusterName); err != nil {
		stateful.AccessLogger.Error("assign EPP cluster %s failed: %v (cluster write succeeds, reconciler will converge)", clusterName, err)
	}
}

// FetchEPPClusters returns all clusters with balance_mode=EPP together with
// their raw epp_config JSON. It implements epp_pool.EPPClusterSource and is
// injected into EppPoolManager at wiring time.
//
// The cluster list is read straight from the storager without wrapping
// AtomExecute: this is a pure read (no transaction needed) and one caller
// runs inside the version-control export transaction (EppDataGenerator);
// a nested AtomExecute there would reuse the enclosing DBContext and commit
// the export transaction prematurely (errTxDone on the outer commit).
func (cm *ClusterManager) FetchEPPClusters(ctx context.Context) ([]*epp_pool.EPPClusterInfo, error) {
	clusters, err := cm.storager.FetchClusterList(ctx, &ClusterFilter{})
	if err != nil {
		return nil, err
	}

	rst := []*epp_pool.EPPClusterInfo{}
	for _, cluster := range clusters {
		if cluster.getBalanceMode() != BalanceModeEPP {
			continue
		}
		rst = append(rst, &epp_pool.EPPClusterInfo{
			Name:          cluster.Name,
			EppConfigJSON: cluster.EppConfig,
		})
	}

	return rst, nil
}

// fetchEPPAssignmentAddrs resolves the ordered EPP addresses of one cluster
// for export. Degradation (design-changes.md §4.3/§5.1): when the cluster has
// no valid assignment (or the lookup fails), the cluster is exported with
// BalanceMode=WRR and without EPPAddr, and an error-level log carrying the
// cluster name and the reason is emitted; the rest of the export proceeds.
func fetchEPPAssignmentAddrs(ctx context.Context, resolver EPPAssignmentResolver, clusterName string) ([]string, bool) {
	if resolver == nil {
		stateful.AccessLogger.Error("export degrade: cluster %s balance_mode=EPP but EPP assignment resolver is not configured, export as WRR without EPPAddr", clusterName)
		return nil, false
	}

	addrs, found, err := resolver.GetAssignmentEndpoints(ctx, clusterName)
	if err != nil {
		stateful.AccessLogger.Error("export degrade: cluster %s balance_mode=EPP but assignment lookup failed: %v, export as WRR without EPPAddr", clusterName, err)
		return nil, false
	}
	if !found {
		stateful.AccessLogger.Error("export degrade: cluster %s balance_mode=EPP but has no valid assignment, export as WRR without EPPAddr", clusterName)
		return nil, false
	}

	return addrs, true
}
