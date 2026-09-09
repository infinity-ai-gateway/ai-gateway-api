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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
)

// APIResponse 统一的 API 响应结构
type APIResponse struct {
	ErrNum   int             `json:"ErrNum"`
	ErrMsg   string          `json:"ErrMsg"`
	Data     json.RawMessage `json:"Data"`
	WorkMode string          `json:"WorkMode"`
}

// Client HTTP 测试客户端
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
}

var (
	globalClient *Client
	clientMu     sync.Mutex
)

// GetClient 获取全局测试客户端
func GetClient() *Client {
	clientMu.Lock()
	defer clientMu.Unlock()
	if globalClient == nil {
		globalClient = &Client{
			HTTPClient: &http.Client{Timeout: 0},
		}
	}
	return globalClient
}

// SetServerURL 设置全局测试服务器 URL
func SetServerURL(url string) {
	client := GetClient()
	client.BaseURL = strings.TrimRight(url, "/")
}

// SetToken 设置认证 Token
func SetToken(token string) {
	client := GetClient()
	client.Token = token
}

// Get 发送 GET 请求
func (c *Client) Get(path string, queryParams ...map[string]string) (*APIResponse, error) {
	url := c.BaseURL + path
	if len(queryParams) > 0 {
		params := []string{}
		for k, v := range queryParams[0] {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}
	return c.doRequest("GET", url, nil)
}

// Post 发送 POST 请求
func (c *Client) Post(path string, body interface{}) (*APIResponse, error) {
	return c.doRequest("POST", c.BaseURL+path, body)
}

// Put 发送 PUT 请求
func (c *Client) Put(path string, body interface{}) (*APIResponse, error) {
	return c.doRequest("PUT", c.BaseURL+path, body)
}

// PutWithQuery 发送带 Query 参数的 PUT 请求
func (c *Client) PutWithQuery(path string, queryParams map[string]string, body interface{}) (*APIResponse, error) {
	url := c.BaseURL + path
	if len(queryParams) > 0 {
		params := []string{}
		for k, v := range queryParams {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}
	return c.doRequest("PUT", url, body)
}

// Patch 发送 PATCH 请求
func (c *Client) Patch(path string, body interface{}) (*APIResponse, error) {
	return c.doRequest("PATCH", c.BaseURL+path, body)
}

// Delete 发送 DELETE 请求
func (c *Client) Delete(path string, body ...interface{}) (*APIResponse, error) {
	var reqBody interface{}
	if len(body) > 0 {
		reqBody = body[0]
	}
	return c.doRequest("DELETE", c.BaseURL+path, reqBody)
}

// DeleteWithQuery 发送带 Query 参数的 DELETE 请求
func (c *Client) DeleteWithQuery(path string, queryParams map[string]string, body ...interface{}) (*APIResponse, error) {
	url := c.BaseURL + path
	if len(queryParams) > 0 {
		params := []string{}
		for k, v := range queryParams {
			params = append(params, fmt.Sprintf("%s=%s", k, v))
		}
		if len(params) > 0 {
			url += "?" + strings.Join(params, "&")
		}
	}
	var reqBody interface{}
	if len(body) > 0 {
		reqBody = body[0]
	}
	return c.doRequest("DELETE", url, reqBody)
}

// PostMultipartFile 发送带有文件上传的 multipart/form-data POST 请求
func (c *Client) PostMultipartFile(path, fieldName, fileName string, fileContent []byte, extraFields map[string]string) (*APIResponse, error) {
	return c.doMultipartRequest(http.MethodPost, path, fieldName, fileName, fileContent, extraFields)
}

// PutMultipartFile 发送带有文件上传的 multipart/form-data PUT 请求
func (c *Client) PutMultipartFile(path, fieldName, fileName string, fileContent []byte, extraFields map[string]string) (*APIResponse, error) {
	return c.doMultipartRequest(http.MethodPut, path, fieldName, fileName, fileContent, extraFields)
}

func (c *Client) doMultipartRequest(method, path, fieldName, fileName string, fileContent []byte, extraFields map[string]string) (*APIResponse, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(fileContent); err != nil {
		return nil, fmt.Errorf("write file content: %w", err)
	}

	for k, v := range extraFields {
		if err := writer.WriteField(k, v); err != nil {
			return nil, fmt.Errorf("write field %s: %w", k, err)
		}
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequest(method, c.BaseURL+path, &body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &apiResp, nil
}

// RawBody 发送带有原始 Body 的请求（用于发送非法 JSON 等非标准内容）
func (c *Client) RawBody(method, path, body string, contentType string) (*APIResponse, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &apiResp, nil
}

// doRequest 执行 HTTP 请求
func (c *Client) doRequest(method, url string, body interface{}) (*APIResponse, error) {
	var bodyReader io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w (body: %s)", err, string(respBody))
	}

	return &apiResp, nil
}

// GetDataField 从 APIResponse.Data 中提取指定字段的值
func GetDataField(resp *APIResponse, field string) (interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal data: %w", err)
	}
	val, ok := data[field]
	if !ok {
		return nil, fmt.Errorf("field %s not found in Data", field)
	}
	return val, nil
}

// GetDataList 从 APIResponse.Data 中提取列表数据
func GetDataList(resp *APIResponse) ([]interface{}, error) {
	var list []interface{}
	if err := json.Unmarshal(resp.Data, &list); err != nil {
		return nil, fmt.Errorf("unmarshal data list: %w", err)
	}
	return list, nil
}

// UnmarshalData 将 APIResponse.Data 反序列化到目标结构
func UnmarshalData(resp *APIResponse, target interface{}) error {
	return json.Unmarshal(resp.Data, target)
}

// GetDataListField 从 APIResponse.Data 中提取列表字段（如 Data.list）
func GetDataListField(resp *APIResponse, field string) ([]interface{}, error) {
	var data map[string]interface{}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("unmarshal data: %w", err)
	}
	val, ok := data[field]
	if !ok {
		return nil, fmt.Errorf("field %s not found in Data", field)
	}
	list, ok := val.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field %s is not a list", field)
	}
	return list, nil
}
