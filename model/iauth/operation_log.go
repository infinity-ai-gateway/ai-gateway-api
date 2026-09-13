// Copyright (c) 2021 The BFE Authors.
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

package iauth

import (
	"context"
	"strconv"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ibasic"
	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (m *AuthenticateManager) recordUserOperation(ctx context.Context, action string, userID int64, userName, parentID string, before, after map[string]interface{}, err error) {
	if m.operationLogManager == nil {
		return
	}

	recordAuthOperation(m.operationLogManager, ctx, action, ioperlog.ResourceTypeUser, userID, userName, parentID, before, after, err)
}

func (m *AuthenticateManager) recordTokenOperation(ctx context.Context, action string, tokenID int64, tokenName, parentID string, before, after map[string]interface{}, err error) {
	if m.operationLogManager == nil {
		return
	}

	recordAuthOperation(m.operationLogManager, ctx, action, ioperlog.ResourceTypeToken, tokenID, tokenName, parentID, before, after, err)
}

func (m *AuthorizeManager) recordUserOperation(ctx context.Context, action string, userID int64, userName, parentID string, before, after map[string]interface{}, err error) {
	if m.operationLogManager == nil {
		return
	}

	recordAuthOperation(m.operationLogManager, ctx, action, ioperlog.ResourceTypeUser, userID, userName, parentID, before, after, err)
}

func (m *AuthorizeManager) recordUserProductBinding(ctx context.Context, action string, user *User, product *ibasic.Product, err error) {
	if m.operationLogManager == nil || user == nil {
		return
	}

	parentID := ""
	if product != nil {
		parentID = product.Name
	}

	recordAuthOperation(m.operationLogManager, ctx, action, ioperlog.ResourceTypeUser, user.ID, user.Name, parentID,
		map[string]interface{}{"user": userToMap(user)},
		map[string]interface{}{"product": productToMap(product)},
		err,
	)
}

func recordAuthOperation(recorder ioperlog.OperationLogRecorder, ctx context.Context, action string, resourceType ioperlog.ResourceType, resourceID int64, resourceName, parentID string, before, after map[string]interface{}, err error) {
	idStr := ""
	if resourceID != 0 {
		idStr = strconv.FormatInt(resourceID, 10)
	}

	status := ioperlog.StatusSuccess
	errorMsg := ""
	if err != nil {
		status = ioperlog.StatusFailed
		errorMsg = ioperlog.TruncateErrorMessageDefault(err)
	}

	entry := &ioperlog.OperationLogEntry{
		Action:           action,
		ResourceType:     string(resourceType),
		ResourceID:       idStr,
		ResourceName:     resourceName,
		ResourceParentID: parentID,
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	recorder.Record(ctx, entry)
}

func userParamName(param *UserParam) string {
	if param == nil || param.Name == nil {
		return ""
	}
	return *param.Name
}

func userParamToMap(param *UserParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.Name != nil {
		m["name"] = *param.Name
	}
	if param.Password != nil {
		m["password"] = *param.Password
	}
	if len(param.Scopes) > 0 {
		m["scopes"] = param.Scopes
	}
	if param.SessionKey != nil {
		m["session_key"] = *param.SessionKey
	}
	if param.SessionKeyCreateAt != nil {
		m["session_key_create_at"] = *param.SessionKeyCreateAt
	}

	return m
}

func userToMap(user *User) map[string]interface{} {
	if user == nil {
		return nil
	}

	m := map[string]interface{}{
		"id":    user.ID,
		"name":  user.Name,
		"type":  user.Type,
		"admin": user.Admin,
	}
	if user.Password != "" {
		m["password"] = user.Password
	}
	if user.SessionKey != "" {
		m["session_key"] = user.SessionKey
	}

	return m
}

func tokenToMap(token *Token) map[string]interface{} {
	if token == nil {
		return nil
	}

	m := map[string]interface{}{
		"id":    token.ID,
		"name":  token.Name,
		"scope": token.Scope,
	}
	if token.Token != "" {
		m["token"] = token.Token
	}
	if token.Product != nil {
		m["product"] = token.Product.Name
	}

	return m
}

func tokenParamToMap(param *TokenParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.Name != nil {
		m["name"] = *param.Name
	}
	if param.Token != nil {
		m["token"] = *param.Token
	}
	if param.Scope != nil {
		m["scope"] = *param.Scope
	}

	return m
}

func productToMap(product *ibasic.Product) map[string]interface{} {
	if product == nil {
		return nil
	}

	m := map[string]interface{}{
		"name": product.Name,
	}
	if product.ID != 0 {
		m["id"] = product.ID
	}

	return m
}
