// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package seata_mysql implements Seata distributed transaction support for MySQL.
// This is an improved version that embeds mysql.Driver instead of gdb.Core.
package seata_mysql

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/seata/seata-go/pkg/tm"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/util/guid"

	"github.com/gogf/gf/contrib/drivers/mysql/v2"
)

// SeataDB Seata 数据库包装对象
// 关键改进：嵌入 mysql.Driver 而不是 gdb.Core
// 这样可以自动继承所有 MySQL 驱动的方法（TableFields, Open, GetChars 等）
type SeataDB struct {
	*mysql.Driver // 嵌入 MySQL Driver，自动继承所有方法
	resource      *Resource
	config        *Config
}

// Begin 开始事务
func (db *SeataDB) Begin(ctx context.Context) (gdb.TX, error) {
	return db.BeginWithOptions(ctx, gdb.DefaultTxOptions())
}

// BeginWithOptions 使用选项开始事务
func (db *SeataDB) BeginWithOptions(ctx context.Context, opts gdb.TxOptions) (gdb.TX, error) {
	// 检查是否在全局事务中
	if !db.isInGlobalTransaction(ctx) {
		// 不在全局事务中，使用原生 MySQL 事务
		glog.Debug(ctx, "[Seata] Not in global transaction, using native MySQL transaction")
		return db.Driver.BeginWithOptions(ctx, opts)
	}

	// 在全局事务中，创建 Seata 事务
	glog.Debugf(ctx, "[Seata] In global transaction, XID: %s", tm.GetXID(ctx))
	return db.beginSeataTransaction(ctx, opts)
}

// Transaction 执行事务
func (db *SeataDB) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
	// 检查是否在全局事务中
	if !db.isInGlobalTransaction(ctx) {
		// 不在全局事务中，使用原生 MySQL 事务
		glog.Debug(ctx, "[Seata] Not in global transaction, using native MySQL transaction")
		return db.Driver.Transaction(ctx, f)
	}

	// 在全局事务中，执行 Seata 事务
	glog.Debugf(ctx, "[Seata] Executing Seata transaction, XID: %s", tm.GetXID(ctx))
	return db.executeSeataTransaction(ctx, f)
}

// beginSeataTransaction 开始 Seata 事务
func (db *SeataDB) beginSeataTransaction(ctx context.Context, opts gdb.TxOptions) (gdb.TX, error) {
	// 使用 MySQL Driver 的方式开始事务
	coreTx, err := db.Driver.BeginWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 包装为 Seata 事务
	seataTx := &SeataTX{
		TX:           coreTx,
		resource:     db.resource,
		ctx:          ctx,
		undoLogItems: nil,
		localTxId:    generateLocalTxId(),
		branchId:     0,
	}

	// 将 SeataTX 注册到 context 中
	ctxWithTx := gdb.WithTX(ctx, seataTx)
	seataTx.ctx = ctxWithTx

	return seataTx, nil
}

// executeSeataTransaction 执行 Seata 事务
func (db *SeataDB) executeSeataTransaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			_ = tx.Rollback()
			panic(r)
		}
	}()

	if err = f(ctx, tx); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

// isInGlobalTransaction 检查是否在全局事务中
func (db *SeataDB) isInGlobalTransaction(ctx context.Context) bool {
	if !db.config.Enabled {
		return false
	}
	return tm.IsGlobalTx(ctx)
}

// GetSeataResource 获取资源对象
func (db *SeataDB) GetSeataResource() *Resource {
	return db.resource
}

// GetSeataConfig 获取配置
func (db *SeataDB) GetSeataConfig() *Config {
	return db.config
}

// DoUpdate 拦截 UPDATE 操作，生成 undo log
func (db *SeataDB) DoUpdate(
	ctx context.Context,
	link gdb.Link,
	table string,
	data interface{},
	condition string,
	args ...interface{},
) (sql.Result, error) {
	// 检查是否需要 Seata 处理
	if !db.shouldIntercept(ctx, link) {
		return db.GetCore().DoUpdate(ctx, link, table, data, condition, args...)
	}

	glog.Debugf(ctx, "[Seata] Intercepting UPDATE on table: %s", table)

	// 1. 生成 beforeImage
	imageGen := NewImageGenerator(db.resource)
	beforeImage, err := imageGen.GenerateBeforeImage(ctx, link, table, condition, args)
	if err != nil {
		glog.Errorf(ctx, "[Seata] Failed to generate beforeImage: %v", err)
		return nil, err
	}

	// 2. 执行 UPDATE
	result, err := db.GetCore().DoUpdate(ctx, link, table, data, condition, args...)
	if err != nil {
		return nil, err
	}

	// 3. 生成 afterImage
	pkValues := beforeImage.ExtractPrimaryKeyValues()
	afterImage, err := imageGen.GenerateAfterImage(ctx, link, table, pkValues)
	if err != nil {
		return nil, fmt.Errorf(ErrGenerateAfterImage, table, err)
	}

	// 4. 保存 undo log
	if err := db.saveUndoLog(ctx, SQLTypeUpdate, table, beforeImage, afterImage); err != nil {
		return nil, fmt.Errorf(ErrSaveUndoLog, table, err)
	}

	glog.Debugf(ctx, "[Seata] UPDATE intercepted successfully, rows affected: %d", len(beforeImage.Rows))
	return result, nil
}

// DoInsert 拦截 INSERT 操作，生成 undo log
func (db *SeataDB) DoInsert(
	ctx context.Context,
	link gdb.Link,
	table string,
	list gdb.List,
	option gdb.DoInsertOption,
) (sql.Result, error) {
	// 检查是否需要 Seata 处理
	if !db.shouldIntercept(ctx, link) {
		return db.GetCore().DoInsert(ctx, link, table, list, option)
	}

	glog.Debugf(ctx, "[Seata] Intercepting INSERT on table: %s", table)

	// 1. 执行 INSERT
	result, err := db.GetCore().DoInsert(ctx, link, table, list, option)
	if err != nil {
		return nil, err
	}

	// 2. 生成 afterImage
	imageGen := NewImageGenerator(db.resource)
	afterImage, err := imageGen.GenerateInsertImage(ctx, link, table, result)
	if err != nil {
		glog.Warningf(ctx, "[Seata] Failed to generate afterImage for INSERT: %v", err)
	}

	// 3. 保存 undo log
	emptyBeforeImage := &TableRecords{
		TableName: table,
		Rows:      []Row{},
	}
	if err := db.saveUndoLog(ctx, SQLTypeInsert, table, emptyBeforeImage, afterImage); err != nil {
		return nil, fmt.Errorf(ErrSaveUndoLog, table, err)
	}

	glog.Debugf(ctx, "[Seata] INSERT intercepted successfully")
	return result, nil
}

// DoDelete 拦截 DELETE 操作，生成 undo log
func (db *SeataDB) DoDelete(
	ctx context.Context,
	link gdb.Link,
	table string,
	condition string,
	args ...interface{},
) (sql.Result, error) {
	// 检查是否需要 Seata 处理
	if !db.shouldIntercept(ctx, link) {
		return db.GetCore().DoDelete(ctx, link, table, condition, args...)
	}

	glog.Debugf(ctx, "[Seata] Intercepting DELETE on table: %s", table)

	// 1. 生成 beforeImage
	imageGen := NewImageGenerator(db.resource)
	beforeImage, err := imageGen.GenerateBeforeImage(ctx, link, table, condition, args)
	if err != nil {
		return nil, fmt.Errorf(ErrGenerateBeforeImage, table, err)
	}

	// 2. 执行 DELETE
	result, err := db.GetCore().DoDelete(ctx, link, table, condition, args...)
	if err != nil {
		return nil, err
	}

	// 3. afterImage 为空
	emptyAfterImage := &TableRecords{
		TableName: table,
		Rows:      []Row{},
	}

	// 4. 保存 undo log
	if err := db.saveUndoLog(ctx, SQLTypeDelete, table, beforeImage, emptyAfterImage); err != nil {
		return nil, fmt.Errorf(ErrSaveUndoLog, table, err)
	}

	glog.Debugf(ctx, "[Seata] DELETE intercepted successfully, rows affected: %d", len(beforeImage.Rows))
	return result, nil
}

// shouldIntercept 判断是否应该拦截 SQL
func (db *SeataDB) shouldIntercept(ctx context.Context, link gdb.Link) bool {
	if !db.isInGlobalTransaction(ctx) {
		return false
	}
	if !link.IsTransaction() {
		return false
	}
	return true
}

// saveUndoLog 保存 undo log 到事务对象
func (db *SeataDB) saveUndoLog(
	ctx context.Context,
	sqlType SQLType,
	tableName string,
	beforeImage *TableRecords,
	afterImage *TableRecords,
) error {
	tx := gdb.TXFromCtx(ctx, db.GetGroup())
	if tx == nil {
		return gerror.Newf("no transaction found in context, group=%s", db.GetGroup())
	}

	seataTx, ok := tx.(*SeataTX)
	if !ok {
		return gerror.Newf("transaction is not a SeataTX, actual type: %T", tx)
	}

	seataTx.AddUndoLog(&SQLUndoLog{
		SQLType:     sqlType,
		TableName:   tableName,
		BeforeImage: beforeImage,
		AfterImage:  afterImage,
	})

	glog.Debugf(ctx, "[Seata] Undo log saved to transaction, type=%s, table=%s", sqlType, tableName)
	return nil
}

// generateLocalTxId 生成本地事务 ID
func generateLocalTxId() string {
	return guid.S()
}
