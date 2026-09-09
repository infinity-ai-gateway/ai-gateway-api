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
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/glebarez/go-sqlite"
	_ "github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

// InitTestDB 初始化 SQLite 测试数据库
// 创建数据库文件并执行 DDL
func InitTestDB(dbPath, ddlPath string) error {
	// 确保数据目录存在
	dataDir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// 删除旧数据库文件（包括 WAL 和 SHM）
	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")

	// 打开数据库连接（使用自定义驱动剥离 FOR UPDATE）
	db, err := sql.Open("sqlite-strip", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite db: %w", err)
	}
	defer db.Close()

	// 读取并执行 DDL 文件
	ddlContent, err := os.ReadFile(ddlPath)
	if err != nil {
		return fmt.Errorf("read ddl file: %w", err)
	}

	// 规范化换行符（Windows \r\n -> \n），然后按 ;\n 分割
	// 不使用 ; 单独分割，避免触发器中 BEGIN ... END; 内的分号被错误拆分
	normalized := strings.ReplaceAll(string(ddlContent), "\r\n", "\n")
	statements := strings.Split(normalized, ";\n")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		// 移除注释行（以 -- 开头的行），保留非注释内容
		lines := strings.Split(stmt, "\n")
		var cleanLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" && !strings.HasPrefix(trimmed, "--") {
				cleanLines = append(cleanLines, line)
			}
		}
		stmt = strings.TrimSpace(strings.Join(cleanLines, "\n"))
		if stmt == "" {
			continue
		}
		// 补充末尾的分号（被 split 消费掉了）
		execSQL := stmt + ";"
		if _, err := db.Exec(execSQL); err != nil {
			return fmt.Errorf("exec ddl: %s: %w", truncateSQL(stmt, 80), err)
		}
	}

	return nil
}

// SeedTestData 插入默认测试数据
// DDL 已插入默认 product/pool/bfe_cluster，这里补充 clusters / sub_clusters / lb_matrices 等默认集群关联数据
func SeedTestData(dbPath string) error {
	db, err := sql.Open("sqlite-strip", dbPath)
	if err != nil {
		return fmt.Errorf("open sqlite db: %w", err)
	}
	defer db.Close()

	// 查询 AI_product 的 id
	var productID int64
	err = db.QueryRow("SELECT id FROM products WHERE name = 'AI_product'").Scan(&productID)
	if err != nil {
		return fmt.Errorf("find AI_product: %w", err)
	}

	// 插入默认 provider，供 seed cluster 引用
	_, err = db.Exec(`
		INSERT OR IGNORE INTO providers (
			id, name, description, model_endpoint, models, api_keys,
			instance_pool, model_protocols, created_at, updated_at
		) VALUES (
			1, 'deepseek', 'Default AI provider for integration tests',
			'{"schema":"https","uri":"/v1/models"}',
			'["deepseek-chat"]',
			'[]',
			'[{"addr":"10.0.0.1","weight":100,"port":8080}]',
			'["openai"]',
			datetime('now'), datetime('now')
		);
	`)
	if err != nil {
		return fmt.Errorf("seed provider: %w", err)
	}

	// 插入默认 cluster（AI_route 规则引用）
	// clusters 表在 DDL 中未初始化，但 AI_route 设置规则时会校验该表
	llmConfig := `{"model_endpoint":{"schema":"https","uri":"/v1/models","headers":null},"models":["deepseek-chat"],"model_mappings":null,"keys":[],"key_policy":null,"provider":"deepseek","match_prefix":null,"strip_prefix":null}`
	_, err = db.Exec(`
		INSERT OR IGNORE INTO clusters (
			id, name, description, product_id, protocol,
			healthcheck_host, healthcheck_uri, llm_config, created_at, updated_at
		) VALUES (
			1, 'BFE-AI_product.szyf', 'Default AI cluster for integration tests', ?, 'http',
			'localhost', '/health', ?, datetime('now'), datetime('now')
		);
	`, productID, llmConfig)
	if err != nil {
		return fmt.Errorf("seed cluster: %w", err)
	}

	// 为默认 cluster 绑定子集群与实例池，使其 instance_pool 非空
	_, err = db.Exec(`
		INSERT OR IGNORE INTO sub_clusters (
			id, name, cluster_id, product_id, description, bns_name_id, enabled, role, created_at, updated_at
		) VALUES (
			1, 'BFE-AI_product.szyf', 1, ?, 'Default sub cluster for integration tests', 1, 1, 'COMMON',
			datetime('now'), datetime('now')
		);
	`, productID)
	if err != nil {
		return fmt.Errorf("seed sub_cluster: %w", err)
	}

	// 为默认 cluster 写入负载均衡矩阵
	lbMatrix := `{"BFE-AI_product.szyf":{"BFE-AI_product.szyf":100,"GSLB_BLACKHOLE":0}}`
	_, err = db.Exec(`
		INSERT OR IGNORE INTO lb_matrices (
			cluster_id, lb_matrix, product_id, created_at, updated_at
		) VALUES (
			1, ?, ?, datetime('now'), datetime('now')
		);
	`, lbMatrix, productID)
	if err != nil {
		return fmt.Errorf("seed lb_matrix: %w", err)
	}

	// 预置 Auth 模块各测试用例依赖的产品线（原 /open-api/v1/products 创建接口已废弃）
	productNames := []string{
		"product_demo",
		"product_search",
		"product_empty",
		"product_unbind",
		"product_token",
		"product_token_del",
		"product_token_search",
		"product_token_list",
		"product_token_list2",
		"product_token_detail",
		"product_token_empty",
	}
	for _, name := range productNames {
		_, err = db.Exec(`
			INSERT OR IGNORE INTO products (
				name, mail_list, contact_person, sms_list, description, created_at, updated_at
			) VALUES (?, 'test@example.com', 'test', 'no sms', 'Seeded product for integration tests', datetime('now'), datetime('now'));
		`, name)
		if err != nil {
			return fmt.Errorf("seed product %s: %w", name, err)
		}
	}

	return nil
}

// CleanupTestDB 清理测试数据库文件
func CleanupTestDB(dbPath string) {
	os.Remove(dbPath)
	os.Remove(dbPath + "-wal")
	os.Remove(dbPath + "-shm")
}

func truncateSQL(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}
