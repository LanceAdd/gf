// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/seata/seata-go/pkg/datasource/sql/types"
	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"
)

// Resource Seata 数据库资源
type Resource struct {
	// resourceID 资源 ID
	resourceID string

	// branchType 分支类型
	branchType branch.BranchType

	// dbType 数据库类型
	dbType types.DBType

	// db 原始数据库连接
	db *sql.DB

	// gfCore GF Core 对象
	gfCore *gdb.Core

	// config Seata 配置
	config *Config

	// asyncWorker 异步工作器（用于异步删除 undo log）
	asyncWorker *AsyncWorker

	// retryExecutor 重试执行器
	retryExecutor *RetryExecutor

	// fallbackManager 降级管理器
	fallbackManager *FallbackManager

	// mu 互斥锁
	mu sync.RWMutex

	// resourceCache 资源缓存（用于实现 ResourceManager 接口）
	resourceCache sync.Map

	// 用于 XA 模式连接保持
	keeper       sync.Map
	shouldBeHeld bool
}

// NewResource 创建新的资源
func NewResource(resourceID string, branchType branch.BranchType, dbType types.DBType, db *sql.DB, gfCore *gdb.Core, config *Config) *Resource {
	return NewResourceWithDB(resourceID, branchType, dbType, db, gfCore, nil, config)
}

// NewResourceWithDB 创建新的资源
func NewResourceWithDB(resourceID string, branchType branch.BranchType, dbType types.DBType, db *sql.DB, gfCore *gdb.Core, _ gdb.DB, config *Config) *Resource {
	resource := &Resource{
		resourceID: resourceID,
		branchType: branchType,
		dbType:     dbType,
		db:         db,
		gfCore:     gfCore,
		// underlyingDB 参数已废弃，不再使用
		config: config,
	}

	// 创建并启动异步工作器
	if config.AT.EnableAsyncCommit {
		resource.asyncWorker = NewAsyncWorker(db, DefaultAsyncWorkerConfig())
		resource.asyncWorker.Start()
	}

	// 创建重试执行器
	resource.retryExecutor = NewRetryExecutor(DefaultRetryConfig())

	// 创建降级管理器
	resource.fallbackManager = NewFallbackManager(DefaultFallbackConfig())

	return resource
}

// GetResourceGroupId 获取资源组 ID
func (r *Resource) GetResourceGroupId() string {
	return r.config.TxServiceGroup
}

// GetResourceId 获取资源 ID
func (r *Resource) GetResourceId() string {
	return r.resourceID
}

// GetBranchType 获取分支类型
func (r *Resource) GetBranchType() branch.BranchType {
	return r.branchType
}

// GetDB 获取原始数据库连接
func (r *Resource) GetDB() *sql.DB {
	return r.db
}

// GetGFCore 获取 GF Core 对象
func (r *Resource) GetGFCore() *gdb.Core {
	return r.gfCore
}

// GetDBType 获取数据库类型
func (r *Resource) GetDBType() types.DBType {
	return r.dbType
}

// GetConfig 获取配置
func (r *Resource) GetConfig() *Config {
	return r.config
}

// IsShouldBeHeld 是否需要保持连接（XA 模式专用）
func (r *Resource) IsShouldBeHeld() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.shouldBeHeld
}

// SetShouldBeHeld 设置是否需要保持连接
func (r *Resource) SetShouldBeHeld(shouldBeHeld bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shouldBeHeld = shouldBeHeld
}

// Hold 保持连接（XA 模式专用）
func (r *Resource) Hold(xaBranchID string, conn interface{}) error {
	_, exist := r.keeper.Load(xaBranchID)
	if !exist {
		r.keeper.Store(xaBranchID, conn)
		return nil
	}
	return nil
}

// Release 释放连接（XA 模式专用）
func (r *Resource) Release(xaBranchID string) {
	r.keeper.Delete(xaBranchID)
}

// Lookup 查找连接（XA 模式专用）
func (r *Resource) Lookup(xaBranchID string) (interface{}, bool) {
	return r.keeper.Load(xaBranchID)
}

// GetKeeper 获取连接保持器
func (r *Resource) GetKeeper() *sync.Map {
	return &r.keeper
}

// BuildResourceID 构建资源 ID
func BuildResourceID(node *gdb.ConfigNode) string {
	return fmt.Sprintf("%s:%s@%s:%s/%s",
		node.Type,
		node.User,
		node.Host,
		node.Port,
		node.Name,
	)
}

// BranchCommit 二阶段提交 - 删除 undo log
func (r *Resource) BranchCommit(ctx context.Context, resource rm.BranchResource) (branch.BranchStatus, error) {
	glog.Infof(ctx, "[Seata] Branch commit, XID=%s, BranchID=%d", resource.Xid, resource.BranchId)

	// 如果启用了异步提交，使用异步工作器
	if r.asyncWorker != nil {
		err := r.asyncWorker.BranchCommit(resource)
		if err != nil {
			glog.Errorf(ctx, "[Seata] Failed to async commit: %v", err)
			return branch.BranchStatusPhaseoneFailed, err
		}
		glog.Infof(ctx, "[Seata] Branch commit queued (async)")
		return branch.BranchStatusPhasetwoCommitted, nil
	}

	// 同步删除 undo log
	deleteSql := "DELETE FROM undo_log WHERE xid = ? AND branch_id = ?"
	_, err := r.db.ExecContext(ctx, deleteSql, resource.Xid, resource.BranchId)
	if err != nil {
		glog.Errorf(ctx, "[Seata] Failed to delete undo log: %v", err)
		return branch.BranchStatusPhaseoneFailed, err
	}

	glog.Infof(ctx, "[Seata] Branch commit success, undo log deleted")
	return branch.BranchStatusPhasetwoCommitted, nil
}

// BranchRollback 二阶段回滚 - 执行 undo log
func (r *Resource) BranchRollback(ctx context.Context, resource rm.BranchResource) (branch.BranchStatus, error) {
	glog.Infof(ctx, "[Seata] Branch rollback, XID=%s, BranchID=%d", resource.Xid, resource.BranchId)

	// 1. 查询 undo log
	var rollbackInfo string
	querySql := "SELECT rollback_info FROM undo_log WHERE xid = ? AND branch_id = ? FOR UPDATE"
	err := r.db.QueryRowContext(ctx, querySql, resource.Xid, resource.BranchId).Scan(&rollbackInfo)
	if err != nil {
		if err == sql.ErrNoRows {
			// 没有 undo log，可能已经回滚过了
			glog.Warningf(ctx, "[Seata] No undo log found, may already rollback")
			return branch.BranchStatusPhasetwoRollbacked, nil
		}
		glog.Errorf(ctx, "[Seata] Failed to query undo log: %v", err)
		return branch.BranchStatusPhasetwoRollbackFailedRetryable, err
	}

	// 2. 反序列化 undo log
	var branchUndoLog BranchUndoLog
	if err := gjson.Unmarshal([]byte(rollbackInfo), &branchUndoLog); err != nil {
		glog.Errorf(ctx, "[Seata] Failed to unmarshal undo log: %v", err)
		return branch.BranchStatusPhasetwoRollbackFailedRetryable, err
	}

	// 3. 开启事务执行回滚
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		glog.Errorf(ctx, "[Seata] Failed to begin transaction: %v", err)
		return branch.BranchStatusPhasetwoRollbackFailedRetryable, err
	}
	defer tx.Rollback()

	// 4. 执行每个 SQL 的回滚
	for _, sqlUndoLog := range branchUndoLog.SQLUndoLogs {
		if err := r.executeUndoLog(ctx, tx, sqlUndoLog); err != nil {
			glog.Errorf(ctx, "[Seata] Failed to execute undo log: %v", err)
			return branch.BranchStatusPhasetwoRollbackFailedRetryable, err
		}
	}

	// 5. 删除 undo log
	deleteSql := "DELETE FROM undo_log WHERE xid = ? AND branch_id = ?"
	if _, err := tx.ExecContext(ctx, deleteSql, resource.Xid, resource.BranchId); err != nil {
		glog.Errorf(ctx, "[Seata] Failed to delete undo log: %v", err)
		return branch.BranchStatusPhasetwoRollbackFailedRetryable, err
	}

	// 6. 提交回滚事务
	if err := tx.Commit(); err != nil {
		glog.Errorf(ctx, "[Seata] Failed to commit rollback transaction: %v", err)
		return branch.BranchStatusPhasetwoRollbackFailedRetryable, err
	}

	glog.Infof(ctx, "[Seata] Branch rollback success")
	return branch.BranchStatusPhasetwoRollbacked, nil
}

// executeUndoLog 执行单个 SQL 的回滚
func (r *Resource) executeUndoLog(ctx context.Context, tx *sql.Tx, undoLog *SQLUndoLog) error {
	switch undoLog.SQLType {
	case SQLTypeInsert:
		// INSERT 回滚：删除插入的数据
		return r.rollbackInsert(ctx, tx, undoLog)
	case SQLTypeUpdate:
		// UPDATE 回滚：恢复原始数据
		return r.rollbackUpdate(ctx, tx, undoLog)
	case SQLTypeDelete:
		// DELETE 回滚：重新插入删除的数据
		return r.rollbackDelete(ctx, tx, undoLog)
	default:
		return fmt.Errorf("unknown SQL type: %s", undoLog.SQLType)
	}
}

// rollbackInsert 回滚 INSERT 操作
func (r *Resource) rollbackInsert(ctx context.Context, tx *sql.Tx, undoLog *SQLUndoLog) error {
	if undoLog.AfterImage == nil || len(undoLog.AfterImage.Rows) == 0 {
		return nil
	}

	// 提取主键值
	pkValues := undoLog.AfterImage.ExtractPrimaryKeyValues()
	if len(pkValues) == 0 {
		return fmt.Errorf("no primary key found in afterImage")
	}

	// 获取主键名
	pkName := r.getPrimaryKeyName(undoLog.AfterImage)
	if pkName == "" {
		return fmt.Errorf("no primary key field found")
	}

	// 构建 DELETE SQL
	placeholders := r.buildPlaceholders(len(pkValues))
	deleteSql := fmt.Sprintf("DELETE FROM %s WHERE %s IN (%s)",
		r.quoteIdentifier(undoLog.TableName),
		r.quoteIdentifier(pkName),
		placeholders)

	_, err := tx.ExecContext(ctx, deleteSql, pkValues...)
	if err != nil {
		return fmt.Errorf("failed to rollback INSERT: %w", err)
	}

	glog.Debugf(ctx, "[Seata] Rollback INSERT: %s", deleteSql)
	return nil
}

// rollbackUpdate 回滚 UPDATE 操作
func (r *Resource) rollbackUpdate(ctx context.Context, tx *sql.Tx, undoLog *SQLUndoLog) error {
	if undoLog.BeforeImage == nil || len(undoLog.BeforeImage.Rows) == 0 {
		return nil
	}

	// 获取主键名
	pkName := r.getPrimaryKeyName(undoLog.BeforeImage)
	if pkName == "" {
		return fmt.Errorf("no primary key field found")
	}

	// 对每一行执行 UPDATE
	for _, row := range undoLog.BeforeImage.Rows {
		// 构建 UPDATE SQL
		var setClauses []string
		var setValues []interface{}
		var pkValue interface{}

		for _, field := range row.Fields {
			if field.KeyType == KeyTypePrimaryKey {
				pkValue = field.Value
			} else {
				setClauses = append(setClauses, fmt.Sprintf("%s = ?", r.quoteIdentifier(field.Name)))
				setValues = append(setValues, field.Value)
			}
		}

		if pkValue == nil {
			return fmt.Errorf("no primary key value found")
		}

		// 添加主键值到参数列表
		setValues = append(setValues, pkValue)

		// 构建 SET 子句
		setClause := ""
		for i, clause := range setClauses {
			if i > 0 {
				setClause += ", "
			}
			setClause += clause
		}

		updateSql := fmt.Sprintf("UPDATE %s SET %s WHERE %s = ?",
			r.quoteIdentifier(undoLog.TableName),
			setClause,
			r.quoteIdentifier(pkName))

		_, err := tx.ExecContext(ctx, updateSql, setValues...)
		if err != nil {
			return fmt.Errorf("failed to rollback UPDATE: %w", err)
		}

		glog.Debugf(ctx, "[Seata] Rollback UPDATE: %s", updateSql)
	}

	return nil
}

// rollbackDelete 回滚 DELETE 操作
func (r *Resource) rollbackDelete(ctx context.Context, tx *sql.Tx, undoLog *SQLUndoLog) error {
	if undoLog.BeforeImage == nil || len(undoLog.BeforeImage.Rows) == 0 {
		return nil
	}

	// 对每一行执行 INSERT
	for _, row := range undoLog.BeforeImage.Rows {
		// 构建 INSERT SQL
		var columns []string
		var placeholders []string
		var values []interface{}

		for _, field := range row.Fields {
			columns = append(columns, r.quoteIdentifier(field.Name))
			placeholders = append(placeholders, "?")
			values = append(values, field.Value)
		}

		// 拼接字符串
		columnStr := ""
		for i, col := range columns {
			if i > 0 {
				columnStr += ", "
			}
			columnStr += col
		}

		placeholderStr := ""
		for i, ph := range placeholders {
			if i > 0 {
				placeholderStr += ", "
			}
			placeholderStr += ph
		}

		insertSql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
			r.quoteIdentifier(undoLog.TableName),
			columnStr,
			placeholderStr)

		_, err := tx.ExecContext(ctx, insertSql, values...)
		if err != nil {
			return fmt.Errorf("failed to rollback DELETE: %w", err)
		}

		glog.Debugf(ctx, "[Seata] Rollback DELETE: %s", insertSql)
	}

	return nil
}

// getPrimaryKeyName 获取主键字段名
func (r *Resource) getPrimaryKeyName(records *TableRecords) string {
	if records == nil || len(records.Rows) == 0 {
		return ""
	}

	for _, field := range records.Rows[0].Fields {
		if field.KeyType == KeyTypePrimaryKey {
			return field.Name
		}
	}

	return ""
}

// buildPlaceholders 构建占位符
func (r *Resource) buildPlaceholders(count int) string {
	if count == 0 {
		return ""
	}
	placeholders := "?"
	for i := 1; i < count; i++ {
		placeholders += ",?"
	}
	return placeholders
}

// quoteIdentifier 引用标识符（表名、列名）
func (r *Resource) quoteIdentifier(name string) string {
	// MySQL 使用反引号
	return "`" + name + "`"
}

// RegisterResource 注册资源
func (r *Resource) RegisterResource(resource rm.Resource) error {
	r.resourceCache.Store(resource.GetResourceId(), resource)
	return nil
}

// UnregisterResource 注销资源
func (r *Resource) UnregisterResource(resource rm.Resource) error {
	r.resourceCache.Delete(resource.GetResourceId())
	return nil
}

// GetCachedResources 获取所有管理的资源
func (r *Resource) GetCachedResources() *sync.Map {
	return &r.resourceCache
}

// BranchRegister 注册分支事务
func (r *Resource) BranchRegister(ctx context.Context, param rm.BranchRegisterParam) (int64, error) {
	return rm.GetRMRemotingInstance().BranchRegister(param)
}

// BranchReport 报告分支事务状态
func (r *Resource) BranchReport(ctx context.Context, param rm.BranchReportParam) error {
	return rm.GetRMRemotingInstance().BranchReport(param)
}

// LockQuery 查询全局锁
func (r *Resource) LockQuery(ctx context.Context, param rm.LockQueryParam) (bool, error) {
	return rm.GetRMRemotingInstance().LockQuery(param)
}
