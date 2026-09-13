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

package instance_name_default_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/rainway-ai-gateway/ai-gateway-api/integration/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var sm *testutil.ServerManager

func TestMain(m *testing.M) {
	var err error
	sm, err = testutil.StartServer()
	if err != nil {
		panic("failed to start server: " + err.Error())
	}
	code := m.Run()
	sm.Shutdown()
	os.Exit(code)
}

// TestProvider_InstanceNameUsesAddrPort 验证 Provider instance_pool 移除 name 字段后，
// 导出的 cluster_table 中 backend Name 使用 addr_port 格式生成。
func TestProvider_InstanceNameUsesAddrPort(t *testing.T) {
	providerName := testutil.UniqueProviderName()
	clusterName := testutil.UniqueClusterName()
	addr := "10.0.0.100"

	// 1. 创建 Provider，instance_pool 不包含 name
	createResp, err := testutil.GetClient().Post("/open-api/v1/providers", map[string]interface{}{
		"name": providerName,
		"instance_pool": []interface{}{
			map[string]interface{}{
				"addr":   addr,
				"weight": 100,
				"port":   8080,
			},
		},
		"models":          []string{"deepseek-chat"},
		"model_protocols": []string{"openai"},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, createResp)

	var providerData map[string]interface{}
	require.NoError(t, json.Unmarshal(createResp.Data, &providerData))
	insts, ok := providerData["instance_pool"].([]interface{})
	require.True(t, ok, "instance_pool should be a list")
	require.Len(t, insts, 1)
	inst, ok := insts[0].(map[string]interface{})
	require.True(t, ok, "instance should be an object")
	_, hasName := inst["name"]
	assert.False(t, hasName, "instance should not contain name field")
	assert.Equal(t, addr, inst["addr"])

	// 2. 创建引用该 Provider 的 Cluster，触发 pool/sub-cluster 自动生成
	clusterResp, err := testutil.GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": clusterName,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	require.NoError(t, err)
	testutil.AssertSuccess(t, clusterResp)

	// 3. 导出 cluster_table，校验 backend Name 使用 addr_port 格式
	exportResp, err := testutil.GetClient().Get("/inner-api/v1/configs/gslb_data/cluster_table")
	require.NoError(t, err)
	testutil.AssertSuccess(t, exportResp)

	var exportData map[string]interface{}
	require.NoError(t, json.Unmarshal(exportResp.Data, &exportData))
	config, ok := exportData["Config"].(map[string]interface{})
	require.True(t, ok, "Config should be an object")
	cluster, ok := config[clusterName].(map[string]interface{})
	require.True(t, ok, "cluster should exist in Config")
	backends, ok := cluster[clusterName].([]interface{})
	require.True(t, ok, "sub-cluster backends should be a list")
	require.Len(t, backends, 1)
	backend, ok := backends[0].(map[string]interface{})
	require.True(t, ok, "backend should be an object")
	assert.Equal(t, "10.0.0.100_8080", backend["Name"], "exported backend name should be addr_port")
	assert.Equal(t, addr, backend["Addr"])

	t.Cleanup(func() {
		testutil.DeleteCluster(clusterName)
		testutil.DeleteProvider(providerName)
	})
}
