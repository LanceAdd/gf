// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/util/guid"
	"github.com/gogf/gf/v2/util/gutil"
	"github.com/seata/seata-go/pkg/tm"
)

// SeataDB Seata 数据库包装对象
type SeataDB struct {
	*gdb.Core // 嵌入 Core，自动实现 gdb.DB 接口
	resource  *Resource
	config    *Config
}

// Begin 开始事务
func (db *SeataDB) Begin(ctx context.Context) (gdb.TX, error) {
	return db.BeginWithOptions(ctx, gdb.DefaultTxOptions())
}

// BeginWithOptions 使用选项开始事务
func (db *SeataDB) BeginWithOptions(ctx context.Context, opts gdb.TxOptions) (gdb.TX, error) {
	// 检查是否在全局事务中
	if !db.isInGlobalTransaction(ctx) {
		// 不在全局事务中，使用 GF 原生事务
		glog.Debug(ctx, "[Seata] Not in global transaction, using native GF transaction")
		return db.Core.BeginWithOptions(ctx, opts)
	}

	// 在全局事务中，创建 Seata 事务
	glog.Debugf(ctx, "[Seata] In global transaction, XID: %s", tm.GetXID(ctx))
	return db.beginSeataTransaction(ctx, opts)
}

// Transaction 执行事务
func (db *SeataDB) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
	// 检查是否在全局事务中
	if !db.isInGlobalTransaction(ctx) {
		// 不在全局事务中，使用 GF 原生事务
		glog.Debug(ctx, "[Seata] Not in global transaction, using native GF transaction")
		return db.Core.Transaction(ctx, f)
	}

	// 在全局事务中，执行 Seata 事务
	glog.Debugf(ctx, "[Seata] Executing Seata transaction, XID: %s", tm.GetXID(ctx))
	return db.executeSeataTransaction(ctx, f)
}

// beginSeataTransaction 开始 Seata 事务
func (db *SeataDB) beginSeataTransaction(ctx context.Context, opts gdb.TxOptions) (gdb.TX, error) {
	// 使用 GF 原生方式开始事务
	// 事务的 Seata 增强将在事务提交时处理
	coreTx, err := db.Core.BeginWithOptions(ctx, opts)
	if err != nil {
		return nil, err
	}

	// 包装为 Seata 事务
	seataTx := &SeataTX{
		TX:           coreTx,
		resource:     db.resource,
		ctx:          ctx,
		undoLogItems: nil, // 延迟初始化（在 AddUndoLog 时）
		localTxId:    guid.S(),
		branchId:     0, // 注册后获得
	}

	// 【关键修复】将 SeataTX 注册到 context 中，确保后续可以通过 TXFromCtx 获取
	// 这样在 DoUpdate/DoInsert/DoDelete 拦截器中才能正确获取到 SeataTX 对象
	ctxWithTx := gdb.WithTX(ctx, seataTx)
	seataTx.ctx = ctxWithTx // 更新 SeataTX 的 context

	return seataTx, nil
}

// executeSeataTransaction 执行 Seata 事务
func (db *SeataDB) executeSeataTransaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
	// 开始事务
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}

	// 执行业务逻辑
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

	// 提交事务
	return tx.Commit()
}

// isInGlobalTransaction 检查是否在全局事务中
func (db *SeataDB) isInGlobalTransaction(ctx context.Context) bool {
	// 检查 Seata 是否启用
	if !db.config.Enabled {
		return false
	}

	// 检查是否有全局事务 XID
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

// Open 创建和返回底层 sql.DB 对象
// 直接实现 MySQL 的 Open 逻辑
func (db *SeataDB) Open(config *gdb.ConfigNode) (*sql.DB, error) {
	var (
		source               = db.configNodeToSource(config)
		underlyingDriverName = "mysql"
	)
	sqlDB, err := sql.Open(underlyingDriverName, source)
	if err != nil {
		err = gerror.WrapCodef(
			gcode.CodeDbOperationError, err,
			`sql.Open failed for driver "%s" by source "%s"`, underlyingDriverName, source,
		)
		return nil, err
	}
	return sqlDB, nil
}

// configNodeToSource 将配置节点转换为 MySQL DSN
// [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
func (db *SeataDB) configNodeToSource(config *gdb.ConfigNode) string {
	var (
		source  string
		portStr string
	)
	if config.Port != "" {
		portStr = ":" + config.Port
	}
	source = fmt.Sprintf(
		"%s:%s@%s(%s%s)/%s?charset=%s",
		config.User, config.Pass, config.Protocol, config.Host, portStr, config.Name, config.Charset,
	)
	if config.Timezone != "" {
		if strings.Contains(config.Timezone, "/") {
			config.Timezone = url.QueryEscape(config.Timezone)
		}
		source = fmt.Sprintf("%s&loc=%s", source, config.Timezone)
	}
	if config.Extra != "" {
		source = fmt.Sprintf("%s&%s", source, config.Extra)
	}
	return source
}

// GetChars 获取当前数据库的字符集引用符号
func (db *SeataDB) GetChars() (charLeft string, charRight string) {
	// MySQL 使用反引号
	return "`", "`"
}

// TableFields 获取表结构
// 直接实现 MySQL 的 TableFields 逻辑，而不是委托
func (db *SeataDB) TableFields(ctx context.Context, table string, schema ...string) (fields map[string]*gdb.TableField, err error) {
	var (
		result     gdb.Result
		link       gdb.Link
		usedSchema = gutil.GetOrDefaultStr(db.GetSchema(), schema...)
	)
	if link, err = db.SlaveLink(usedSchema); err != nil {
		return nil, err
	}

	// MySQL 使用 SHOW FULL COLUMNS 查询表结构
	tableFieldsSql := fmt.Sprintf(`SHOW FULL COLUMNS FROM %s`, db.QuoteWord(table))

	result, err = db.DoSelect(ctx, link, tableFieldsSql)
	if err != nil {
		return nil, err
	}

	fields = make(map[string]*gdb.TableField)
	for i, m := range result {
		fields[m["Field"].String()] = &gdb.TableField{
			Index:   i,
			Name:    m["Field"].String(),
			Type:    m["Type"].String(),
			Null:    m["Null"].Bool(),
			Key:     m["Key"].String(),
			Default: m["Default"].Val(),
			Extra:   m["Extra"].String(),
			Comment: m["Comment"].String(),
		}
	}
	return fields, nil
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
		return db.Core.DoUpdate(ctx, link, table, data, condition, args...)
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
	result, err := db.Core.DoUpdate(ctx, link, table, data, condition, args...)
	if err != nil {
		return nil, err
	}

	// 3. 生成 afterImage
	pkValues := beforeImage.ExtractPrimaryKeyValues()
	afterImage, err := imageGen.GenerateAfterImage(ctx, link, table, pkValues)
	if err != nil {
		glog.Errorf(ctx, "[Seata] Failed to generate afterImage: %v", err)
		return nil, err
	}

	// 4. 保存 undo log 到事务对象
	if err := db.saveUndoLog(ctx, SQLTypeUpdate, table, beforeImage, afterImage); err != nil {
		glog.Errorf(ctx, "[Seata] Failed to save undo log: %v", err)
		return nil, err
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
		return db.Core.DoInsert(ctx, link, table, list, option)
	}

	glog.Debugf(ctx, "[Seata] Intercepting INSERT on table: %s", table)

	// 1. 执行 INSERT
	result, err := db.Core.DoInsert(ctx, link, table, list, option)
	if err != nil {
		return nil, err
	}

	// 2. 生成 afterImage
	imageGen := NewImageGenerator(db.resource)
	afterImage, err := imageGen.GenerateInsertImage(ctx, link, table, result)
	if err != nil {
		glog.Warningf(ctx, "[Seata] Failed to generate afterImage for INSERT: %v", err)
		// INSERT 的 afterImage 可能获取失败（如无自增主键），继续执行
	}

	// 3. 保存 undo log（beforeImage 为空）
	emptyBeforeImage := &TableRecords{
		TableName: table,
		Rows:      []Row{},
	}
	if err := db.saveUndoLog(ctx, SQLTypeInsert, table, emptyBeforeImage, afterImage); err != nil {
		glog.Errorf(ctx, "[Seata] Failed to save undo log: %v", err)
		return nil, err
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
		return db.Core.DoDelete(ctx, link, table, condition, args...)
	}

	glog.Debugf(ctx, "[Seata] Intercepting DELETE on table: %s", table)

	// 1. 生成 beforeImage
	imageGen := NewImageGenerator(db.resource)
	beforeImage, err := imageGen.GenerateBeforeImage(ctx, link, table, condition, args)
	if err != nil {
		glog.Errorf(ctx, "[Seata] Failed to generate beforeImage: %v", err)
		return nil, err
	}

	// 2. 执行 DELETE
	result, err := db.Core.DoDelete(ctx, link, table, condition, args...)
	if err != nil {
		return nil, err
	}

	// 3. afterImage 为空（数据已删除）
	emptyAfterImage := &TableRecords{
		TableName: table,
		Rows:      []Row{},
	}

	// 4. 保存 undo log
	if err := db.saveUndoLog(ctx, SQLTypeDelete, table, beforeImage, emptyAfterImage); err != nil {
		glog.Errorf(ctx, "[Seata] Failed to save undo log: %v", err)
		return nil, err
	}

	glog.Debugf(ctx, "[Seata] DELETE intercepted successfully, rows affected: %d", len(beforeImage.Rows))
	return result, nil
}

// shouldIntercept 判断是否应该拦截 SQL
func (db *SeataDB) shouldIntercept(ctx context.Context, link gdb.Link) bool {
	// 必须在全局事务中
	if !db.isInGlobalTransaction(ctx) {
		return false
	}

	// 必须在本地事务中（link 是事务对象）
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
	// 从 context 获取 Seata 事务对象
	tx := gdb.TXFromCtx(ctx, db.GetGroup())
	if tx == nil {
		return fmt.Errorf("no transaction found in context, group=%s", db.GetGroup())
	}

	// 转换为 SeataTX
	seataTx, ok := tx.(*SeataTX)
	if !ok {
		// 提供更详细的错误信息，帮助调试
		return fmt.Errorf("transaction is not a SeataTX, actual type: %T", tx)
	}

	// 添加 undo log
	seataTx.AddUndoLog(&SQLUndoLog{
		SQLType:     sqlType,
		TableName:   tableName,
		BeforeImage: beforeImage,
		AfterImage:  afterImage,
	})

	glog.Debugf(ctx, "[Seata] Undo log saved to transaction, type=%s, table=%s", sqlType, tableName)
	return nil
}
