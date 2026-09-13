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

package iprotocol

import (
	"context"
	"time"

	"github.com/rainway-ai-gateway/ai-gateway-api/model/ioperlog"
)

func (pm *CertificateManager) recordCertificateOperation(ctx context.Context, action, resourceID, resourceName, parentID string, before, after map[string]interface{}, err error) {
	if pm.operationLogManager == nil {
		return
	}

	status := ioperlog.StatusSuccess
	errorMsg := ""
	if err != nil {
		status = ioperlog.StatusFailed
		errorMsg = ioperlog.TruncateErrorMessageDefault(err)
	}

	entry := &ioperlog.OperationLogEntry{
		Action:           action,
		ResourceType:     string(ioperlog.ResourceTypeCertificate),
		ResourceID:       resourceID,
		ResourceName:     resourceName,
		ResourceParentID: parentID,
		Status:           status,
		ErrorMsg:         errorMsg,
		CreatedAt:        time.Now(),
	}

	entry.ChangeSummary = ioperlog.BuildChangeSummary(action, before, after)

	pm.operationLogManager.Record(ctx, entry)
}

func certificateParamToMap(param *CertificateParam) map[string]interface{} {
	if param == nil {
		return nil
	}

	m := map[string]interface{}{}
	if param.CertName != nil {
		m["cert_name"] = *param.CertName
	}
	if param.Description != nil {
		m["description"] = *param.Description
	}
	if param.IsDefault != nil {
		m["is_default"] = *param.IsDefault
	}
	if param.CertFilePath != nil {
		m["cert_file_path"] = *param.CertFilePath
	}
	if param.KeyFilePath != nil {
		m["key_file_path"] = *param.KeyFilePath
	}
	if param.ExpiredDate != nil {
		m["expired_date"] = *param.ExpiredDate
	}

	return m
}

func certificateToMap(cert *Certificate) map[string]interface{} {
	if cert == nil {
		return nil
	}

	return map[string]interface{}{
		"cert_name":    cert.CertName,
		"description":  cert.Description,
		"is_default":   cert.IsDefault,
		"expired_date": cert.ExpiredDate,
	}
}
