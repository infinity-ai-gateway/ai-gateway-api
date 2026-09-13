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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"strings"
	"time"
)

// GenerateTestCert 生成测试用的自签名证书
// 返回 (certPEM, keyPEM)
func GenerateTestCert(commonName string) (string, string, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", err
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	return string(certPEM), string(keyPEM), nil
}

// RandomString 生成随机字符串
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		randIndex, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[randIndex.Int64()]
	}
	return string(b)
}

// UniqueName 生成带随机后缀的唯一名称
func UniqueName(prefix string) string {
	return prefix + "-" + RandomString(6)
}

// UniqueProductName 生成唯一的产品线名称
func UniqueProductName() string {
	return UniqueName("product")
}

// UniqueUserName 生成唯一的用户名
func UniqueUserName() string {
	return UniqueName("testuser")
}

// UniqueAPIKeyDesc 生成唯一的 API-Key 描述
func UniqueAPIKeyDesc() string {
	return UniqueName("test-key")
}

// UniqueEntityTypeName 生成唯一的 Entity-Type 名称
func UniqueEntityTypeName() string {
	return UniqueName("utype")
}

// UniqueEntityName 生成唯一的 Entity 名称
func UniqueEntityName() string {
	return UniqueName("entity")
}

// UniqueClusterName 生成唯一的集群名称
func UniqueClusterName() string {
	return UniqueName("cluster")
}

// UniqueCertName 生成唯一的证书名称
func UniqueCertName() string {
	return UniqueName("cert")
}

// UniqueProviderName 生成唯一的 Provider 名称
func UniqueProviderName() string {
	return UniqueName("provider")
}

// UniqueTokenName 生成唯一的 Token 名称
func UniqueTokenName() string {
	return UniqueName("token")
}

// Int64Ptr 返回 int64 指针
func Int64Ptr(v int64) *int64 {
	return &v
}

// BoolPtr 返回 bool 指针
func BoolPtr(v bool) *bool {
	return &v
}

// StringPtr 返回 string 指针
func StringPtr(v string) *string {
	return &v
}

// ContainsString 检查字符串是否包含子串（忽略大小写）
func ContainsString(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}