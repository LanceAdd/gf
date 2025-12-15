// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package seata 提供 GF 框架的 Seata 分布式事务支持
package seata

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/seata/seata-go/pkg/client"
	"github.com/seata/seata-go/pkg/datasource/sql"
	"github.com/seata/seata-go/pkg/datasource/sql/undo"
	"github.com/seata/seata-go/pkg/protocol/branch"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
)

var (
	// globalConfig 全局配置
	globalConfig *Config

	// initialized 是否已初始化
	initialized bool

	// atInitOnce 确保 AT模式只初始化一次
	atInitOnce sync.Once
)

// Init 初始化 Seata
func Init(config *Config) error {
	if initialized {
		return gerror.NewCode(gcode.CodeInvalidOperation, "Seata already initialized")
	}

	if config == nil {
		config = DefaultConfig()
	}

	globalConfig = config
	ctx := gctx.GetInitCtx()

	glog.Info(ctx, "[Seata] Initializing Seata driver...")

	// 1. 初始化 Seata 客户端配置
	if err := initSeataClient(ctx, config); err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "failed to initialize Seata client")
	}

	// 2. 初始化 AT 模式
	if config.Mode == "AT" || config.Mode == "" {
		if err := initATMode(ctx, config); err != nil {
			return gerror.WrapCode(gcode.CodeInternalError, err, "failed to initialize AT mode")
		}
	}

	// 3. 初始化 XA 模式
	if config.Mode == "XA" {
		if err := initXAMode(ctx, config); err != nil {
			return gerror.WrapCode(gcode.CodeInternalError, err, "failed to initialize XA mode")
		}
	}

	// 4. 注册驱动到 GF
	if err := registerDrivers(ctx, config); err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "failed to register drivers")
	}

	initialized = true
	glog.Info(ctx, "[Seata] Seata driver initialized successfully")

	return nil
}

// initSeataClient 初始化 Seata 客户端
func initSeataClient(ctx context.Context, config *Config) error {
	glog.Info(ctx, "[Seata] Initializing Seata client...")

	// 检查是否设置了配置文件路径
	configPath := os.Getenv("SEATA_GO_CONFIG_PATH")
	if configPath != "" {
		// 使用配置文件初始化
		glog.Infof(ctx, "[Seata] Loading config from file: %s", configPath)
		client.InitPath("")
	} else {
		// 使用程序内配置，跳过配置文件加载
		glog.Info(ctx, "[Seata] Using in-memory configuration (no config file)")
		// 不调用 client.InitPath，避免配置文件检查
		// Seata-Go SDK 允许直接使用，只是不会连接到 Seata Server
	}

	glog.Info(ctx, "[Seata] Seata client initialized")
	return nil
}

// initATMode 初始化 AT 模式
func initATMode(ctx context.Context, config *Config) error {
	var initErr error

	// 使用 sync.Once 确保只初始化一次，避免 Prometheus metrics 重复注册
	atInitOnce.Do(func() {
		glog.Info(ctx, "[Seata] Initializing AT mode...")

		// 使用 defer recover 捕获 panic，避免 metrics 重复注册导致的崩溃
		defer func() {
			if r := recover(); r != nil {
				// 检查是否是 metrics 重复注册错误
				errMsg := fmt.Sprintf("%v", r)
				if stringContains(errMsg, "duplicate metrics") {
					glog.Warning(ctx, "[Seata] AT mode already initialized (metrics already registered), skipping...")
					initErr = nil
				} else {
					// 其他 panic，重新抛出
					initErr = gerror.Newf("AT mode initialization failed: %v", r)
				}
			}
		}()

		// 初始化 Undo Log 配置
		undoConfig := undo.Config{
			OnlyCareUpdateColumns: !config.AT.OnlyCarePrimaryKey,
		}

		// 初始化异步工作器配置
		asyncConfig := sql.AsyncWorkerConfig{
			// 设置默认值，避免 NewTicker panic
			BufferLimit:            10000,       // 缓冲区大小
			BufferCleanInterval:    time.Second, // 1秒清理一次
			ReceiveChanSize:        100,         // 接收通道大小
			CommitWorkerCount:      5,           // 提交工作线程数
			CommitWorkerBufferSize: 100,         // 提交缓冲区大小
		}

		// 初始化 AT 模式
		sql.InitAT(undoConfig, asyncConfig)

		glog.Info(ctx, "[Seata] AT mode initialized")
	})

	return initErr
}

// stringContains 检查字符串是否包含子字符串
func stringContains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// initXAMode 初始化 XA 模式
func initXAMode(ctx context.Context, config *Config) error {
	glog.Info(ctx, "[Seata] Initializing XA mode...")

	// 初始化 XA 模式
	xaConfig := sql.XAConfig{}
	sql.InitXA(xaConfig)

	glog.Info(ctx, "[Seata] XA mode initialized")
	return nil
}

// registerDrivers 注册驱动
func registerDrivers(ctx context.Context, config *Config) error {

	// 注册 AT 模式驱动
	if config.Mode == "AT" || config.Mode == "" {
		glog.Info(ctx, "[Seata] Registering AT mode driver...")
		driver := NewDriverAT(config)
		if err := gdb.Register(DriverNameATMySQL, driver); err != nil {
			return fmt.Errorf("failed to register AT driver: %w", err)
		}
		glog.Info(ctx, "[Seata] AT mode driver registered")
	}

	// 注册 XA 模式驱动
	if config.Mode == "XA" {
		glog.Info(ctx, "[Seata] Registering XA mode driver...")
		// XA 模式将在阶段四实现
		glog.Warning(ctx, "[Seata] XA mode driver not implemented yet")
	}

	return nil
}

// GetConfig 获取全局配置
func GetConfig() *Config {
	return globalConfig
}

// IsInitialized 检查是否已初始化
func IsInitialized() bool {
	return initialized
}

// GetBranchType 获取分支类型
func GetBranchType() branch.BranchType {
	if globalConfig == nil {
		return branch.BranchTypeAT
	}
	return globalConfig.GetBranchType()
}
