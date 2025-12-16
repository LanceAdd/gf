// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package seata_mysql 提供 GF 框架的 Seata AT 模式分布式事务支持
//
// 本驱动仅支持 Seata AT 模式，不支持 TCC、XA、SAGA 等其他事务模式。
// AT 模式是 Seata 最常用的模式，通过 undo_log 实现自动回滚。
package seata_mysql

import (
	"context"
	"fmt"
	"os"

	"github.com/seata/seata-go/pkg/client"
	"github.com/seata/seata-go/pkg/protocol/branch"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gctx"
)

// Init 初始化 Seata AT 模式驱动
//
// 本驱动仅支持 AT 模式，不支持 TCC/XA/SAGA 等其他模式。
//
// 两种初始化模式：
// 1. 完整模式：设置环境变量 SEATA_GO_CONFIG_PATH，提供完整的分布式事务协调能力
//    - 需要配置 seatago.yml 文件
//    - 会初始化 RM、TM、远程通信等完整组件
//    - 适用于生产环境
//
// 2. 精简模式：不设置环境变量，仅提供驱动包装
//    - 不初始化任何 Seata 组件
//    - 仅提供基础的驱动注册
//    - 适用于开发测试或不需要分布式事务的场景
func Init(config *Config) error {
	if config == nil {
		config = DefaultConfig()
	}

	ctx := gctx.GetInitCtx()

	// 1. 初始化 Seata 客户端（如果配置了 SEATA_GO_CONFIG_PATH）
	if err := initSeataClient(ctx, config); err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "failed to initialize Seata client")
	}

	// 2. 注册 AT 驱动到 GF
	if err := registerDriver(ctx, config); err != nil {
		return gerror.WrapCode(gcode.CodeInternalError, err, "failed to register driver")
	}

	return nil
}

// initSeataClient 初始化 Seata 客户端
//
// 完整模式：检测到 SEATA_GO_CONFIG_PATH 环境变量时，调用 client.InitPath
// 精简模式：未设置环境变量时，仅记录日志，不初始化任何组件
func initSeataClient(ctx context.Context, config *Config) error {
	// 检查是否设置了配置文件路径
	configPath := os.Getenv("SEATA_GO_CONFIG_PATH")
	if configPath == "" {
		// 精简模式：不初始化 Seata 组件
		return nil
	}

	// 完整模式：使用配置文件初始化完整的 Seata 客户端
	// 这会初始化 RM、TM、远程通信、处理器等所有组件
	// 注意：虽然 client.InitPath 会初始化 TCC/XA，但我们只使用 AT 模式
	client.InitPath("")

	return nil
}

// registerDriver 注册 AT 模式驱动（本驱动仅支持 AT 模式）
func registerDriver(ctx context.Context, config *Config) error {
	driver := NewDriverAT(config)
	if err := gdb.Register(DriverNameATMySQL, driver); err != nil {
		return fmt.Errorf("failed to register AT driver: %w", err)
	}
	return nil
}

// GetBranchType 获取分支类型（始终返回 AT，因为本驱动只支持 AT 模式）
func GetBranchType() branch.BranchType {
	// 本驱动只支持 AT 模式，不支持 TCC/XA/SAGA
	return branch.BranchTypeAT
}
