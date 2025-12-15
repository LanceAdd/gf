// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"github.com/seata/seata-go/pkg/datasource/sql/datasource"
	"github.com/seata/seata-go/pkg/protocol/branch"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
)

const (
	// DriverNameATMySQL AT 模式 MySQL 驱动名
	DriverNameATMySQL = "seata-at-mysql"
)

// DriverAT AT 模式驱动
type DriverAT struct {
	config *Config
}

// NewDriverAT 创建 AT 模式驱动
func NewDriverAT(config *Config) *DriverAT {
	if config == nil {
		config = DefaultConfig()
	}
	return &DriverAT{
		config: config,
	}
}

// New 创建数据库对象
func (d *DriverAT) New(core *gdb.Core, node *gdb.ConfigNode) (gdb.DB, error) {
	ctx := gctx.GetInitCtx()

	// 如果未启用 Seata，使用原生驱动
	if !d.config.Enabled {
		glog.Debug(ctx, "[Seata] Seata is disabled, using native driver")
		// 返回包装的 DB，但不启用 Seata 功能
		return &SeataDB{
			Core:   core,
			config: d.config,
		}, nil
	}

	// 使用公共函数初始化 Seata DB
	_, resource, err := initializeSeataDB(
		core,
		node,
		branch.BranchTypeAT,
		"AT",
		d.config,
	)
	if err != nil {
		return nil, err
	}

	// 注册资源到 Seata RM
	if err = d.registerResource(resource); err != nil {
		return nil, gerror.WrapCodef(
			gcode.CodeDbOperationError,
			err,
			"failed to register resource to Seata RM",
		)
	}

	// 创建 Seata DB 包装对象
	return &SeataDB{
		Core:     core,
		resource: resource,
		config:   d.config,
	}, nil
}

// registerResource 注册资源到 Seata RM
func (d *DriverAT) registerResource(resource *Resource) error {
	// 获取 AT 模式资源管理器
	rm := datasource.GetDataSourceManager(branch.BranchTypeAT)
	if rm == nil {
		return gerror.NewCode(
			gcode.CodeInternalError,
			"AT mode resource manager not initialized, please call seata.InitAT() first",
		)
	}

	// 注册资源
	return rm.RegisterResource(resource)
}
