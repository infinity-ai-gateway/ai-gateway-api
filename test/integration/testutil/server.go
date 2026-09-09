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
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/alicebob/miniredis/v2"
	golibquota "github.com/bfenetworks/go-lib/quota"
	_ "github.com/rainway-ai-gateway/ai-gateway-api/stateful"
)

// ServerManager 测试服务器管理器
type ServerManager struct {
	cmd         *exec.Cmd
	cancel      context.CancelFunc
	ServerURL   string
	DBPath      string
	DataDir     string
	ConfDir     string
	binPath     string
	tmpConfDir  string
	Redis       *miniredis.Miniredis
	sharedRedis bool // Redis 是否由外部共享，Shutdown 时不关闭
	sharedDB    bool // DB 是否由外部共享，Shutdown 时不删除
}

// StartServer 使用项目编译的 ai-gateway-api.exe 作为子进程启动测试服务器
func StartServer() (*ServerManager, error) {
	return StartServerWithSharedInfra(nil, "")
}

// StartServerWithSharedInfra 启动一个测试服务器，可复用外部传入的 miniredis 与 SQLite 数据库文件。
// 当 sharedRedis == nil 时创建新的 miniredis；当 sharedDBPath == "" 时创建新的 SQLite 数据库。
// 该函数用于多实例部署场景，让多个 ai-gateway-api 实例共享同一 Redis（分布式锁）与同一 DB。
func StartServerWithSharedInfra(sharedRedis *miniredis.Miniredis, sharedDBPath string) (*ServerManager, error) {
	sm := &ServerManager{}

	// 1. 获取 integration 目录和项目根目录
	testRoot, err := findIntegrationRoot()
	if err != nil {
		return nil, fmt.Errorf("find integration root: %w", err)
	}

	projectRoot, err := findProjectRoot()
	if err != nil {
		return nil, fmt.Errorf("find project root: %w", err)
	}

	confDir := filepath.Join(testRoot, "conf")
	dataDir := filepath.Join(testRoot, "data")

	sm.ConfDir = confDir
	sm.DataDir = dataDir

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	// 2. 数据库文件
	var dbPath string
	if sharedDBPath != "" {
		dbPath = sharedDBPath
		sm.sharedDB = true
	} else {
		dbPath = filepath.Join(dataDir, fmt.Sprintf("test_ai_gateway_%d.db", os.Getpid()))

		// 3. 初始化 SQLite 数据库（执行 DDL）
		ddlPath := filepath.Join(projectRoot, "db_ddl_sqlite.sql")
		if _, err := os.Stat(ddlPath); os.IsNotExist(err) {
			ddlPath = filepath.Join(testRoot, "..", "..", "db_ddl_sqlite.sql")
		}
		if err := InitTestDB(dbPath, ddlPath); err != nil {
			return nil, fmt.Errorf("init test db: %w", err)
		}

		// 3.5 插入默认测试数据
		if err := SeedTestData(dbPath); err != nil {
			return nil, fmt.Errorf("seed test data: %w", err)
		}
	}
	sm.DBPath = dbPath

	// 4. 查找真实 ai-gateway-api.exe 二进制
	binSrc := filepath.Join(projectRoot, "ai-gateway-api.exe")
	if _, err := os.Stat(binSrc); os.IsNotExist(err) {
		return nil, fmt.Errorf("ai-gateway-api.exe not found at %s, run 'make build' first", binSrc)
	}

	// 复制到 data 目录（避免文件锁冲突）
	sm.binPath = filepath.Join(dataDir, fmt.Sprintf("ai-gateway-api-%d-%d.exe", os.Getpid(), time.Now().UnixNano()))
	if err := copyFile(binSrc, sm.binPath); err != nil {
		return nil, fmt.Errorf("copy binary: %w", err)
	}

	// 5. Redis
	var redisServer *miniredis.Miniredis
	if sharedRedis != nil {
		redisServer = sharedRedis
		sm.sharedRedis = true
	} else {
		r, err := miniredis.Run()
		if err != nil {
			return nil, fmt.Errorf("start miniredis: %w", err)
		}
		redisServer = r
	}
	sm.Redis = redisServer

	// 6. 生成随机端口并创建临时配置文件
	port, err := getRandomPort()
	if err != nil {
		return nil, fmt.Errorf("get random port: %w", err)
	}

	tmpConfDir, err := createTempConfig(confDir, sm.binPath, dbPath, port, redisServer.Addr())
	if err != nil {
		return nil, fmt.Errorf("create temp config: %w", err)
	}
	sm.tmpConfDir = tmpConfDir

	// 7. 启动 ai-gateway-api 子进程
	ctx, cancel := context.WithCancel(context.Background())
	sm.cancel = cancel

	serverCmd := exec.CommandContext(ctx, sm.binPath,
		"-c", tmpConfDir+string(os.PathSeparator),
		"-sc", "ai_gateway_api.toml",
		"-l", dataDir,
	)
	serverCmd.Dir = testRoot

	// 捕获 stdout 和 stderr
	stdout, err := serverCmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf bytes.Buffer
	serverCmd.Stderr = &stderrBuf

	if err := serverCmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start ai-gateway-api: %w", err)
	}
	sm.cmd = serverCmd

	// 8. 等待服务器就绪（TCP 拨号检测，带超时）
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	sm.ServerURL = fmt.Sprintf("http://%s", addr)

	// 启动 stdout 消费 goroutine，防止子进程阻塞
	go func() {
		io.Copy(io.Discard, stdout)
	}()

	if err := sm.waitForReady(addr, 10*time.Second); err != nil {
		// 收集 stderr 输出用于调试
		errMsg := err.Error()
		if stderrBuf.Len() > 0 {
			errMsg = fmt.Sprintf("%s\nstderr: %s", errMsg, stderrBuf.String())
		}
		// 检查进程是否已退出
		if sm.cmd.ProcessState != nil {
			errMsg = fmt.Sprintf("%s\nexit code: %d", errMsg, sm.cmd.ProcessState.ExitCode())
		}
		sm.Shutdown()
		return nil, fmt.Errorf("server ready: %s", errMsg)
	}

	// 设置全局客户端（仅当非共享模式时，避免覆盖其他实例的 URL）
	if sharedRedis == nil && sharedDBPath == "" {
		SetServerURL(sm.ServerURL)
	}

	return sm, nil
}

// Shutdown 停止测试服务器并清理资源
func (sm *ServerManager) Shutdown() {
	// 取消 context，终止子进程
	if sm.cancel != nil {
		sm.cancel()
	}

	// 等待子进程退出
	if sm.cmd != nil && sm.cmd.Process != nil {
		sm.cmd.Wait()
	}

	// 清理复制的二进制文件
	if sm.binPath != "" {
		os.Remove(sm.binPath)
	}

	// 清理临时配置文件
	if sm.tmpConfDir != "" {
		os.RemoveAll(sm.tmpConfDir)
	}

	// 清理数据库文件（仅当 DB 非外部共享时）
	if sm.DBPath != "" && !sm.sharedDB {
		CleanupTestDB(sm.DBPath)
	}

	// 关闭 miniredis（仅当 Redis 非外部共享时）
	if sm.Redis != nil && !sm.sharedRedis {
		sm.Redis.Close()
	}
}

// waitForReady 等待服务器就绪
func (sm *ServerManager) waitForReady(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server not ready at %s in %v", addr, timeout)
}

// getRandomPort 获取一个随机可用端口
func getRandomPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

// createTempConfig 创建临时配置文件（覆盖端口、数据库路径和 Redis 配置）
func createTempConfig(srcConfDir, binPath, dbPath string, port int, redisAddr string) (string, error) {
	// 创建临时配置目录
	tmpDir, err := os.MkdirTemp("", "ai-gateway-test-conf-")
	if err != nil {
		return "", err
	}

	// 复制所有配置文件
	entries, err := os.ReadDir(srcConfDir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		src := filepath.Join(srcConfDir, entry.Name())
		dst := filepath.Join(tmpDir, entry.Name())
		if entry.IsDir() {
			if err := copyDir(src, dst); err != nil {
				os.RemoveAll(tmpDir)
				return "", err
			}
		} else {
			if err := copyFile(src, dst); err != nil {
				os.RemoveAll(tmpDir)
				return "", err
			}
		}
	}

	// 修改主配置文件：端口、数据库路径
	confFile := filepath.Join(tmpDir, "ai_gateway_api.toml")
	content, err := os.ReadFile(confFile)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	confStr := string(content)
	// 替换端口
	confStr = strings.Replace(confStr, "ServerPort = 8199", fmt.Sprintf("ServerPort = %d", port), 1)
	// 替换数据库路径（使用正斜杠避免 TOML 转义问题）
	dbPathForTOML := strings.ReplaceAll(dbPath, "\\", "/")
	confStr = strings.Replace(confStr, `DBName  = "./data/test_ai_gateway.db"`, fmt.Sprintf(`DBName  = "%s"`, dbPathForTOML), 1)
	// 替换 Redis 配置为指向 miniredis
	confStr = strings.Replace(confStr, `Bns = "mock"`, `Bns = "test.redis.miniredis"`, 1)
	confStr = strings.Replace(confStr, `ClusterMode = "mock"`, `ClusterMode = "proxy"`, 1)

	if err := os.WriteFile(confFile, []byte(confStr), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	// 写入 name_conf.data，让 redis_client 能解析到 miniredis 地址
	host, redisPort, err := net.SplitHostPort(redisAddr)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("split redis addr %s: %w", redisAddr, err)
	}
	redisPortInt, err := strconv.Atoi(redisPort)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("parse redis port %s: %w", redisPort, err)
	}
	nameConf := map[string]interface{}{
		"Version": "init version",
		"Config": map[string]interface{}{
			"test.redis.miniredis": []map[string]interface{}{
				{
					"Host":   host,
					"Port":   redisPortInt,
					"Weight": 10,
				},
			},
		},
	}
	nameConfData, err := json.Marshal(nameConf)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("marshal name conf: %w", err)
	}
	nameConfPath := filepath.Join(tmpDir, "name_conf.data")
	if err := os.WriteFile(nameConfPath, nameConfData, 0644); err != nil {
		os.RemoveAll(tmpDir)
		return "", fmt.Errorf("write name conf: %w", err)
	}

	return tmpDir, nil
}

// copyFile 复制文件
func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return err
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	return err
}

// copyDir 复制目录
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// SetQuotaRemaining 直接设置 miniredis 中某 owner 的配额剩余量。
// ownerKey 为 API-Key ID 或 Entity ID；unit 为 total_token/RMB 等配额单位。
func (sm *ServerManager) SetQuotaRemaining(ownerKey string, remaining float64, unit string) {
	if sm.Redis == nil {
		return
	}
	redisKey := fmt.Sprintf("QUOTA_%s", ownerKey)
	value := golibquota.ToRedisValue(remaining, unit)
	sm.Redis.Set(redisKey, fmt.Sprintf("%d", value))
}

// GetQuotaRemaining 读取 miniredis 中某 owner 的配额剩余量。
func (sm *ServerManager) GetQuotaRemaining(ownerKey string, unit string) float64 {
	if sm.Redis == nil {
		return 0
	}
	redisKey := fmt.Sprintf("QUOTA_%s", ownerKey)
	v, err := sm.Redis.Get(redisKey)
	if err != nil {
		return 0
	}
	var value int64
	_, err = fmt.Sscanf(v, "%d", &value)
	if err != nil {
		return 0
	}
	return golibquota.FromRedisValue(value, unit)
}

// UpdateQuotaPlanLastResetAt 直接更新 SQLite 中某 API-Key / Entity 对应 quota_plans.last_reset_at。
// ownerType 取 "api_key" 或 "entity"。
func (sm *ServerManager) UpdateQuotaPlanLastResetAt(ownerID string, ownerType string, lastResetAt time.Time) error {
	db, err := sql.Open("sqlite-strip", sm.DBPath)
	if err != nil {
		return fmt.Errorf("open sqlite db: %w", err)
	}
	defer db.Close()

	var quotaPlanID int64
	switch ownerType {
	case "api_key":
		err = db.QueryRow("SELECT quota_plan_id FROM api_keys WHERE id = ?", ownerID).Scan(&quotaPlanID)
	case "entity":
		err = db.QueryRow("SELECT quota_plan_id FROM entities WHERE entity_id = ?", ownerID).Scan(&quotaPlanID)
	default:
		return fmt.Errorf("unsupported ownerType: %s", ownerType)
	}
	if err != nil {
		return fmt.Errorf("find quota_plan_id for %s %s: %w", ownerType, ownerID, err)
	}

	_, err = db.Exec("UPDATE quota_plans SET last_reset_at = ? WHERE id = ?", lastResetAt, quotaPlanID)
	if err != nil {
		return fmt.Errorf("update quota_plans.last_reset_at: %w", err)
	}
	return nil
}

// GetQuotaPlanLastResetAt 读取某 API-Key / Entity 对应 quota_plans.last_reset_at。
func (sm *ServerManager) GetQuotaPlanLastResetAt(ownerID string, ownerType string) (*time.Time, error) {
	db, err := sql.Open("sqlite-strip", sm.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite db: %w", err)
	}
	defer db.Close()

	var quotaPlanID int64
	switch ownerType {
	case "api_key":
		err = db.QueryRow("SELECT quota_plan_id FROM api_keys WHERE id = ?", ownerID).Scan(&quotaPlanID)
	case "entity":
		err = db.QueryRow("SELECT quota_plan_id FROM entities WHERE entity_id = ?", ownerID).Scan(&quotaPlanID)
	default:
		return nil, fmt.Errorf("unsupported ownerType: %s", ownerType)
	}
	if err != nil {
		return nil, fmt.Errorf("find quota_plan_id for %s %s: %w", ownerType, ownerID, err)
	}

	var lastResetAt time.Time
	err = db.QueryRow("SELECT last_reset_at FROM quota_plans WHERE id = ?", quotaPlanID).Scan(&lastResetAt)
	if err != nil {
		return nil, fmt.Errorf("select quota_plans.last_reset_at: %w", err)
	}
	return &lastResetAt, nil
}

// findIntegrationRoot 查找 integration 目录
func findIntegrationRoot() (string, error) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("failed to get caller info")
	}
	dir := filepath.Dir(filepath.Dir(filename))
	return dir, nil
}

// findProjectRoot 查找 ai-gateway-api 项目根目录
func findProjectRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		goModPath := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			content, err := os.ReadFile(goModPath)
			if err == nil && strings.Contains(string(content), "module github.com/rainway-ai-gateway/ai-gateway-api") && !strings.Contains(string(content), "integration") {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("project root not found from %s", cwd)
}
