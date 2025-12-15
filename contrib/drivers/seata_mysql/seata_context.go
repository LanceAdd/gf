// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"time"

	"github.com/seata/seata-go/pkg/tm"

	"github.com/gogf/gf/v2/database/gdb"
)

// WithGlobalTransaction 在 context 中注入全局事务
func WithGlobalTransaction(ctx context.Context, txName string) context.Context {
	// 初始化 Seata Context
	ctx = tm.InitSeataContext(ctx)

	// 设置事务名称
	tm.SetTxName(ctx, txName)

	// 设置角色为发起者
	tm.SetTxRole(ctx, tm.Launcher)

	return ctx
}

// GlobalTransaction 在全局事务中执行函数
func GlobalTransaction(ctx context.Context, db gdb.DB, txName string, timeout time.Duration, f func(ctx context.Context, tx gdb.TX) error) error {
	// 1. 创建全局事务上下文
	ctx = WithGlobalTransaction(ctx, txName)

	// 2. 开启全局事务
	gtm := tm.GetGlobalTransactionManager()
	if err := gtm.Begin(ctx, timeout); err != nil {
		return err
	}

	// 3. 执行业务逻辑
	err := db.Transaction(ctx, f)

	// 4. 提交或回滚全局事务
	gtr := tm.GetTx(ctx)
	if err != nil {
		_ = gtm.Rollback(ctx, gtr)
		return err
	}

	return gtm.Commit(ctx, gtr)
}

// IsInGlobalTransaction 检查是否在全局事务中
func IsInGlobalTransaction(ctx context.Context) bool {
	return tm.IsGlobalTx(ctx)
}

// GetGlobalTransactionXID 获取全局事务 XID
func GetGlobalTransactionXID(ctx context.Context) string {
	return tm.GetXID(ctx)
}
