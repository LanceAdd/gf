// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/seata/seata-go/pkg/datasource/sql/datasource"
	"github.com/seata/seata-go/pkg/datasource/sql/types"
	"github.com/seata/seata-go/pkg/protocol/branch"
)

const (
	// DriverNameXAMySQL XA 模式 MySQL 驱动名
	DriverNameXAMySQL = "seata-xa-mysql"
)

// DriverXA XA 模式驱动
type DriverXA struct {
	config *Config
}

// NewDriverXA 创建 XA 模式驱动
func NewDriverXA(config *Config) *DriverXA {
	if config == nil {
		config = DefaultConfig()
	}
	return &DriverXA{
		config: config,
	}
}

// New 创建数据库对象
// 关键改进：返回嵌入了 mysql.Driver 的 SeataDB
func (d *DriverXA) New(core *gdb.Core, node *gdb.ConfigNode) (gdb.DB, error) {
	ctx := gctx.GetInitCtx()

	// 如果未启用 Seata，返回原生 MySQL Driver
	if !d.config.Enabled {
		glog.Debug(ctx, "[Seata] Seata is disabled, using native MySQL driver")
		mysqlDriver := mysql.New()
		return mysqlDriver.New(core, node)
	}

	glog.Infof(ctx, "[Seata] Initializing XA mode driver for: %s", node.Name)

	// 1. 创建 MySQL Driver 实例
	mysqlDriver := mysql.New()
	mysqlDB, err := mysqlDriver.New(core, node)
	if err != nil {
		return nil, gerror.WrapCodef(
			gcode.CodeDbOperationError,
			err,
			"failed to create MySQL driver",
		)
	}

	// 2. 获取 *sql.DB（用于 Seata RM 注册）
	// 注意：这里需要临时获取 sql.DB，因为 Seata 需要注册资源
	sqlDB, err := mysqlDB.Open(node)
	if err != nil {
		return nil, gerror.WrapCodef(
			gcode.CodeDbOperationError,
			err,
			"failed to open database connection",
		)
	}

	// 3. 构建资源 ID
	resourceID := BuildResourceID(node)
	glog.Debugf(ctx, "[Seata] Resource ID: %s", resourceID)

	// 4. 创建 Seata 资源
	resource := NewResource(
		resourceID,
		branch.BranchTypeXA,
		types.DBTypeMySQL,
		sqlDB,
		core,
		d.config,
	)

	// 5. 注册资源到 Seata RM
	if err = d.registerResource(resource); err != nil {
		return nil, gerror.WrapCodef(
			gcode.CodeDbOperationError,
			err,
			"failed to register resource to Seata RM",
		)
	}

	glog.Infof(ctx, "[Seata] XA mode driver initialized successfully for: %s", node.Name)

	// 6. 返回嵌入了 mysql.Driver 的 SeataDB
	// 关键：类型断言获取 mysql.Driver
	mysqlDriverInstance, ok := mysqlDB.(*mysql.Driver)
	if !ok {
		return nil, gerror.Newf("failed to convert to mysql.Driver, got type: %T", mysqlDB)
	}

	return &SeataDB{
		Driver:   mysqlDriverInstance,
		resource: resource,
		config:   d.config,
	}, nil
}

// registerResource 注册资源到 Seata RM
func (d *DriverXA) registerResource(resource *Resource) error {
	// 获取 XA 模式资源管理器
	rm := datasource.GetDataSourceManager(branch.BranchTypeXA)
	if rm == nil {
		return gerror.NewCode(
			gcode.CodeInternalError,
			"XA mode resource manager not initialized, please call seata.InitXA() first",
		)
	}

	// 注册资源
	return rm.RegisterResource(resource)
}
