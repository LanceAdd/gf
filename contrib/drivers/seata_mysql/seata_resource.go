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

	"github.com/seata/seata-go/pkg/datasource/sql/types"
	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/glog"
)

// Resource Seata AT 模式数据库资源
type Resource struct {
	// resourceID 资源 ID
	resourceID string

	// db 原始数据库连接
	db *sql.DB

	// gfCore GF Core 对象
	gfCore *gdb.Core

	// config Seata 配置
	config *Config

	// mu 互斥锁
	mu sync.RWMutex

	// resourceCache 资源缓存（用于实现 ResourceManager 接口）
	resourceCache sync.Map
}

// NewResource 创建新的 AT 模式资源
func NewResource(resourceID string, db *sql.DB, gfCore *gdb.Core, config *Config) *Resource {
	return NewResourceWithDB(resourceID, db, gfCore, nil, config)
}

// NewResourceWithDB 创建新的 AT 模式资源
func NewResourceWithDB(resourceID string, db *sql.DB, gfCore *gdb.Core, _ gdb.DB, config *Config) *Resource {
	resource := &Resource{
		resourceID: resourceID,
		db:         db,
		gfCore:     gfCore,
		config:     config,
	}

	return resource
}

// GetResourceGroupId 获取资源组 ID
// 注意：这个值在完整模式下会从 seatago.yml 读取
// 精简模式下返回默认值
func (r *Resource) GetResourceGroupId() string {
	// 返回默认的资源组 ID
	// 完整模式下，Seata-Go 会使用配置文件中的 tx_service_group
	return "default_tx_group"
}

// GetResourceId 获取资源 ID
func (r *Resource) GetResourceId() string {
	return r.resourceID
}

// GetBranchType 获取分支类型（始终返回 AT）
func (r *Resource) GetBranchType() branch.BranchType {
	return branch.BranchTypeAT
}

// GetDB 获取原始数据库连接
func (r *Resource) GetDB() *sql.DB {
	return r.db
}

// GetGFCore 获取 GF Core 对象
func (r *Resource) GetGFCore() *gdb.Core {
	return r.gfCore
}

// GetDBType 获取数据库类型（始终返回 MySQL）
func (r *Resource) GetDBType() types.DBType {
	return types.DBTypeMySQL
}

// GetConfig 获取配置
func (r *Resource) GetConfig() *Config {
	return r.config
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

	// 删除 undo log
	// 注意：如果需要异步删除，应该使用 Seata-Go SDK 的 AsyncWorker
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
