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

package testutil

import (
	"encoding/json"
	"fmt"
	"time"
)

// CreateEntityType 创建 Entity-Type，返回 type_name
func CreateEntityType(typeName string, level int) (string, error) {
	resp, err := GetClient().Post("/open-api/v1/entity-types", map[string]interface{}{
		"type_name":   typeName,
		"description": "auto-created by test helper",
		"level":       level,
	})
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("create entity-type failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	name, err := GetDataField(resp, "type_name")
	if err != nil {
		return "", err
	}
	return name.(string), nil
}

// CreateEntity 创建 Entity，返回 id
func CreateEntity(name, typeName, parentID string) (string, error) {
	body := map[string]interface{}{
		"name": name,
		"type": typeName,
	}
	if parentID != "" {
		body["parent_id"] = parentID
	}
	resp, err := GetClient().Post("/open-api/v1/entities", body)
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("create entity failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	id, err := GetDataField(resp, "id")
	if err != nil {
		return "", err
	}
	return id.(string), nil
}

// CreateAPIKey 创建 API-Key，返回 id
func CreateAPIKey(description string, entityID string) (string, error) {
	id, _, err := CreateAPIKeyWithKey(description, entityID)
	return id, err
}

// CreateAPIKeyWithKey 创建 API-Key，返回 id 与 key 值
func CreateAPIKeyWithKey(description string, entityID string) (string, string, error) {
	body := map[string]interface{}{
		"description": description,
	}
	if entityID != "" {
		body["entity_id"] = entityID
	}
	resp, err := GetClient().Post("/open-api/v1/api-keys", body)
	if err != nil {
		return "", "", err
	}
	if resp.ErrNum != 200 {
		return "", "", fmt.Errorf("create api-key failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	id, err := GetDataField(resp, "id")
	if err != nil {
		return "", "", err
	}
	key, err := GetDataField(resp, "key")
	if err != nil {
		return "", "", err
	}
	return id.(string), key.(string), nil
}

// CreateProvider 创建 Provider，返回 name
func CreateProvider(name string, opts ...map[string]interface{}) (string, error) {
	body := map[string]interface{}{
		"name": name,
		"instance_pool": []interface{}{
			map[string]interface{}{
				"addr":   "10.0.0.1",
				"weight": 100,
				"port":   8080,
			},
		},
		"model_protocols": []string{"openai"},
		"models":          []string{"deepseek-chat"},
		"keys": []interface{}{
			map[string]interface{}{
				"name": "key-primary",
				"key":  "sk-aaaaaaaaaaaa",
			},
			map[string]interface{}{
				"name": "key-secondary",
				"key":  "sk-bbbbbbbbbbbb",
			},
		},
	}
	for _, opt := range opts {
		for k, v := range opt {
			body[k] = v
		}
	}
	resp, err := GetClient().Post("/open-api/v1/providers", body)
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("create provider failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return name, nil
}

// DeleteProvider 删除 Provider
func DeleteProvider(name string) error {
	resp, err := GetClient().Delete("/open-api/v1/providers/" + name)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete provider failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// UpdatePricingTiers 通过 JSON 设置 Provider 的高峰/闲时模板
func UpdatePricingTiers(providerName string, body map[string]interface{}) (*APIResponse, error) {
	return GetClient().Put("/open-api/v1/providers/"+providerName+"/pricing-tiers", body)
}

// UpdatePricingTiersYAML 通过 text/yaml 设置 Provider 的高峰/闲时模板
func UpdatePricingTiersYAML(providerName string, yamlContent []byte) (*APIResponse, error) {
	return GetClient().RawBody("PUT", "/open-api/v1/providers/"+providerName+"/pricing-tiers", string(yamlContent), "text/yaml")
}

// UpdatePricingTiersMultipartYAML 通过 multipart/form-data 上传 YAML 文件设置 Provider 的高峰/闲时模板
func UpdatePricingTiersMultipartYAML(providerName string, yamlContent []byte) (*APIResponse, error) {
	return GetClient().PutMultipartFile("/open-api/v1/providers/"+providerName+"/pricing-tiers", "file", "pricing-tiers.yaml", yamlContent, nil)
}

// CreateCluster 创建 Cluster，返回 name
func CreateCluster(name string) (string, error) {
	providerName := UniqueProviderName()
	if _, err := CreateProvider(providerName); err != nil {
		return "", fmt.Errorf("create provider for cluster: %w", err)
	}

	resp, err := GetClient().Post("/open-api/v1/clusters", map[string]interface{}{
		"name": name,
		"llm_config": map[string]interface{}{
			"models":   []string{"deepseek-chat"},
			"provider": providerName,
		},
	})
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("create cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return name, nil
}

// ImportModelPrices 通过 /model-prices/import 导入模型定价 YAML
func ImportModelPrices(yamlContent []byte, mode string) error {
	_, _, err := ImportModelPricesWithResult(yamlContent, mode)
	return err
}

// ImportModelPricesWithResult 导入模型定价 YAML，并返回导入结果计数
type ImportModelPricesResult struct {
	ImportedCount int `json:"imported_count"`
	SkippedCount  int `json:"skipped_count"`
}

func ImportModelPricesWithResult(yamlContent []byte, mode string) (ImportModelPricesResult, *APIResponse, error) {
	resp, err := GetClient().PostMultipartFile("/open-api/v1/model-prices/import", "file", "model-list.yaml", yamlContent, map[string]string{
		"mode": mode,
	})
	if err != nil {
		return ImportModelPricesResult{}, nil, err
	}
	if resp.ErrNum != 200 {
		return ImportModelPricesResult{}, resp, fmt.Errorf("import model prices failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	var result ImportModelPricesResult
	if err := UnmarshalData(resp, &result); err != nil {
		return ImportModelPricesResult{}, resp, err
	}
	return result, resp, nil
}

// CreateModelPrice 创建模型定价记录，返回记录 id
// 若 body 中包含 provider 且该 provider 不存在，则自动创建（已存在则跳过）。
func CreateModelPrice(body map[string]interface{}) (int64, error) {
	if providerName, ok := body["provider"].(string); ok && providerName != "" {
		resp, err := GetClient().Get("/open-api/v1/providers/" + providerName)
		if err != nil {
			return 0, fmt.Errorf("check provider existence: %w", err)
		}
		if resp.ErrNum == 404 {
			if _, err := CreateProvider(providerName); err != nil {
				return 0, fmt.Errorf("auto-create provider for model price: %w", err)
			}
		} else if resp.ErrNum != 200 {
			return 0, fmt.Errorf("check provider existence failed: %d %s", resp.ErrNum, resp.ErrMsg)
		}
	}
	resp, err := GetClient().Post("/open-api/v1/model-prices", body)
	if err != nil {
		return 0, err
	}
	if resp.ErrNum != 200 {
		return 0, fmt.Errorf("create model price failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	idVal, err := GetDataField(resp, "id")
	if err != nil {
		return 0, err
	}
	id, ok := idVal.(float64)
	if !ok {
		return 0, fmt.Errorf("id is not a number: %v", idVal)
	}
	return int64(id), nil
}

// DeleteModelPrice 按 id 删除模型定价记录
func DeleteModelPrice(id int64) error {
	resp, err := GetClient().Delete(fmt.Sprintf("/open-api/v1/model-prices/%d", id))
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete model price failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteModelPriceByQuery 按组合键删除模型定价记录
func DeleteModelPriceByQuery(provider, model, mode string) error {
	resp, err := GetClient().DeleteWithQuery("/open-api/v1/model-prices", map[string]string{
		"provider": provider,
		"model":    model,
		"mode":     mode,
	})
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete model price by query failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// CreateCertificate 创建证书，返回 cert_name
func CreateCertificate(certName string, isDefault bool) (string, error) {
	certPEM, keyPEM, err := GenerateTestCert(certName)
	if err != nil {
		return "", err
	}
	resp, err := GetClient().Post("/open-api/v1/certificates", map[string]interface{}{
		"cert_name":         certName,
		"description":       "auto-created by test helper",
		"is_default":        isDefault,
		"cert_file_content": certPEM,
		"key_file_content":  keyPEM,
	})
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("create certificate failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return certName, nil
}

// CreateUser 创建用户
func CreateUser(userName, password string) error {
	resp, err := GetClient().Post("/open-api/v1/auth/users", map[string]interface{}{
		"user_name": userName,
		"password":  password,
		"is_admin":  true,
	})
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 {
		return fmt.Errorf("create user failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// CreateToken 创建 Token，返回 token 值
func CreateToken(name, scope string) (string, error) {
	resp, err := GetClient().Post("/open-api/v1/auth/tokens", map[string]interface{}{
		"name":  name,
		"scope": scope,
	})
	if err != nil {
		return "", err
	}
	if resp.ErrNum != 200 {
		return "", fmt.Errorf("create token failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	token, err := GetDataField(resp, "token")
	if err != nil {
		return "", err
	}
	return token.(string), nil
}

// DeleteAPIKey 删除 API-Key
func DeleteAPIKey(id string) error {
	resp, err := GetClient().Delete("/open-api/v1/api-keys/" + id)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete api-key failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteEntity 删除 Entity
func DeleteEntity(id string) error {
	resp, err := GetClient().Delete("/open-api/v1/entities/" + id)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete entity failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteEntityType 删除 Entity-Type
func DeleteEntityType(typeName string) error {
	resp, err := GetClient().Delete("/open-api/v1/entity-types/" + typeName)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete entity-type failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteCluster 删除 Cluster
func DeleteCluster(name string) error {
	resp, err := GetClient().Delete("/open-api/v1/clusters/" + name)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete cluster failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteCertificate 删除证书
func DeleteCertificate(certName string) error {
	resp, err := GetClient().Delete("/open-api/v1/certificates/" + certName)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete certificate failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteUser 删除用户
func DeleteUser(userName string) error {
	resp, err := GetClient().Delete("/open-api/v1/auth/users/" + userName)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete user failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// DeleteToken 删除 Token
func DeleteToken(name string) error {
	resp, err := GetClient().Delete("/open-api/v1/auth/tokens/" + name)
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 && resp.ErrNum != 404 {
		return fmt.Errorf("delete token failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// FieldNotExists 检查 Data 中是否不包含指定字段
func FieldNotExists(resp *APIResponse, field string) (bool, error) {
	if resp == nil || len(resp.Data) == 0 {
		return true, nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return false, err
	}
	_, exists := data[field]
	return !exists, nil
}

// FieldExists 检查 Data 中是否包含指定字段
func FieldExists(resp *APIResponse, field string) (bool, error) {
	ok, err := FieldNotExists(resp, field)
	if err != nil {
		return false, err
	}
	return !ok, nil
}

// OperationLogEntry 是查询操作日志返回的单个条目结构（仅包含测试常用字段）。
type OperationLogEntry struct {
	ID            float64                `json:"id"`
	Action        string                 `json:"action"`
	ResourceType  string                 `json:"resource_type"`
	ResourceID    string                 `json:"resource_id"`
	ResourceName  string                 `json:"resource_name"`
	Status        float64                `json:"status"`
	ErrorMsg      string                 `json:"error_msg"`
	ClientIP      string                 `json:"client_ip"`
	UserAgent     string                 `json:"user_agent"`
	RequestPath   string                 `json:"request_path"`
	RequestMethod string                 `json:"request_method"`
	CreatedAt     float64                `json:"created_at"`
	ChangeSummary map[string]interface{} `json:"change_summary"`
}

// OperationLogListResult 是 GET /operation-logs 的分页结果。
type OperationLogListResult struct {
	List       []OperationLogEntry `json:"list"`
	Pagination struct {
		Page     int   `json:"page"`
		PageSize int   `json:"page_size"`
		Total    int64 `json:"total"`
	} `json:"pagination"`
}

// QueryOperationLogs 查询操作日志列表。
func QueryOperationLogs(query map[string]string) (*OperationLogListResult, *APIResponse, error) {
	resp, err := GetClient().Get("/open-api/v1/operation-logs", query)
	if err != nil {
		return nil, nil, err
	}
	if resp.ErrNum != 200 {
		return nil, resp, fmt.Errorf("query operation logs failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	var result OperationLogListResult
	if err := UnmarshalData(resp, &result); err != nil {
		return nil, resp, fmt.Errorf("unmarshal operation logs: %w", err)
	}
	return &result, resp, nil
}

// WaitForOperationLog 轮询等待匹配 filter 的操作日志出现，默认最长等待 10 秒。
func WaitForOperationLog(filter map[string]string, timeout time.Duration) (*OperationLogEntry, error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		result, _, err := QueryOperationLogs(filter)
		if err != nil {
			return nil, err
		}
		if len(result.List) > 0 {
			return &result.List[0], nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return nil, fmt.Errorf("operation log not found after %v, filter=%v", timeout, filter)
}

// ResetGlobalRouteRules 清空全局路由规则。
func ResetGlobalRouteRules() error {
	resp, err := GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
		"rules": []interface{}{},
	})
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 {
		return fmt.Errorf("reset global route rules failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// SetGlobalRouteRules 设置全局路由规则。
func SetGlobalRouteRules(rules []interface{}) error {
	resp, err := GetClient().Put("/open-api/v1/global-route-rules", map[string]interface{}{
		"rules": rules,
	})
	if err != nil {
		return err
	}
	if resp.ErrNum != 200 {
		return fmt.Errorf("set global route rules failed: %d %s", resp.ErrNum, resp.ErrMsg)
	}
	return nil
}

// SimpleRouteRule 构造一条最简单的 global route rule，用于集成测试。
func SimpleRouteRule(name, clusterName string) map[string]interface{} {
	return map[string]interface{}{
		"name":      name,
		"cond":      "default_t()",
		"targets":   []interface{}{map[string]interface{}{"cluster_name": clusterName, "model": "", "weight": 100}},
		"fallbacks": []interface{}{},
	}
}
