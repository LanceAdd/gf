// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"fmt"
	"strings"

	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"
	"github.com/seata/seata-go/pkg/tm"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/glog"
)

// SeataTX Seata 事务对象
// 嵌入 gdb.TX 以继承 GF 原生事务的所有方法，同时添加 Seata 分布式事务的增强功能：
// - Undo Log 收集：自动记录 INSERT/UPDATE/DELETE 操作的数据快照
// - 分支注册：向 Seata Server 注册分支事务
// - 状态上报：报告分支事务执行状态
// - 嵌套事务：使用 SAVEPOINT 机制支持嵌套事务
type SeataTX struct {
	gdb.TX                           // 嵌入 GF 原生事务接口
	resource         *Resource       // Seata 资源对象，包含数据库连接信息
	ctx              context.Context // 事务上下文，包含 XID 等全局事务信息
	undoLogItems     *garray.Array   // 存储所有的 undo log 项，并发安全
	localTxId        string          // 本地事务ID，用于跟踪和调试
	branchId         int64           // 分支事务ID（注册后获得）
	nestingLevel     int             // 嵌套层级计数器，用于 SAVEPOINT 命名
	savepointMarkers []int           // 每个嵌套层级的 undo log 数量标记，用于嵌套回滚
}

// GetCtx 返回当前事务的 context
// 该 context 包含了 Seata 全局事务的 XID 等信息
func (tx *SeataTX) GetCtx() context.Context {
	return tx.ctx
}

// Ctx 设置并返回事务的 context
// 注意：同时会更新底层 GF 事务的 context，保证上下文一致性
func (tx *SeataTX) Ctx(ctx context.Context) gdb.TX {
	if ctx == nil {
		return tx
	}
	tx.ctx = ctx
	// 同时更新底层事务的 context
	tx.TX = tx.TX.Ctx(ctx)
	return tx
}

// Model 返回 Model 对象，确保使用 SeataTX 的 context
// 这样在 SQL 拦截器中能正确获取到 SeataTX 对象，从而正确收集 Undo Log
func (tx *SeataTX) Model(tableNameQueryOrStruct ...interface{}) *gdb.Model {
	// 使用底层TX的Model方法，但要确保context正确
	model := tx.TX.Model(tableNameQueryOrStruct...)
	// 使用SeataTX的context，这样在拦截器中能正确获取到SeataTX对象
	return model.Ctx(tx.ctx)
}

// Transaction 嵌套事务处理
//
// 功能说明：
// 1. 重写此方法确保嵌套事务继续使用当前 SeataTX 对象，而不是创建新的事务对象
// 2. 使用 SAVEPOINT 机制实现嵌套事务：
//   - 每个嵌套层级创建一个 SAVEPOINT
//   - 嵌套事务失败时回滚到 SAVEPOINT，不影响外层事务
//   - 嵌套事务成功时释放 SAVEPOINT
//
// 3. 关键特性：嵌套事务回滚时，同时回滚 Undo Log 到保存点
//
// 参数：
//   - ctx: 上下文对象
//   - f: 嵌套事务的回调函数
//
// 返回：
//   - error: 嵌套事务执行错误，如果有
func (tx *SeataTX) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
	// 更新 context
	if ctx != nil {
		tx.ctx = ctx
	}

	// 确保 context 中包含当前 SeataTX
	if gdb.TXFromCtx(tx.ctx, tx.GetDB().GetGroup()) != tx {
		tx.ctx = gdb.WithTX(tx.ctx, tx)
	}

	// 记录当前的 undo log 数量（作为保存点）
	undoLogCount := 0
	if tx.undoLogItems != nil {
		undoLogCount = tx.undoLogItems.Len()
	}

	// 开始 SAVEPOINT
	savepointName := fmt.Sprintf(SavepointFormat, tx.nestingLevel)
	_, err := tx.Exec("SAVEPOINT " + savepointName)
	if err != nil {
		return fmt.Errorf(ErrCreateSavepoint, savepointName, err)
	}
	tx.nestingLevel++

	// 执行嵌套事务逻辑
	err = f(tx.ctx, tx)

	// 根据结果决定提交或回滚 SAVEPOINT
	if err != nil {
		tx.nestingLevel--

		// 回滚数据库到 SAVEPOINT
		_, rollbackErr := tx.Exec("ROLLBACK TO SAVEPOINT " + savepointName)
		if rollbackErr != nil {
			return fmt.Errorf(ErrRollbackToSavepoint, savepointName, rollbackErr, err)
		}

		// ⭐ 关键：回滚 Undo Log 到保存点
		if tx.undoLogItems != nil && tx.undoLogItems.Len() > undoLogCount {
			// 记录回滚的数量
			rolledBackCount := tx.undoLogItems.Len() - undoLogCount

			// 删除在这个嵌套事务中添加的 undo log
			newArray := garray.New(true)
			for i := 0; i < undoLogCount; i++ {
				newArray.Append(tx.undoLogItems.Get(i))
			}
			tx.undoLogItems = newArray
			glog.Debugf(tx.ctx, "[Seata] Rolled back %d undo log items to savepoint, remaining: %d",
				rolledBackCount, undoLogCount)
		}

		return err
	}

	// 成功则释放 SAVEPOINT
	tx.nestingLevel--
	_, err = tx.Exec("RELEASE SAVEPOINT " + savepointName)
	if err != nil {
		return fmt.Errorf(ErrReleaseSavepoint, savepointName, err)
	}
	return nil
}

// Commit 提交事务
//
// 执行流程：
// 1. 插入 Undo Log 到数据库（如果有）
// 2. 注册分支事务到 Seata Server（如果有 Undo Log）
// 3. 提交本地数据库事务
// 4. 报告分支状态为一阶段完成
//
// 注意：这是 Seata AT 模式的一阶段提交，二阶段由 Seata Server 协调执行
func (tx *SeataTX) Commit() error {
	glog.Debugf(tx.ctx, "[Seata] Committing transaction, XID: %s", tm.GetXID(tx.ctx))

	// 1. 插入 undo log（如果有）
	if err := tx.insertUndoLog(); err != nil {
		glog.Errorf(tx.ctx, "[Seata] Failed to insert undo log: %v", err)
		return err
	}

	// 2. 注册分支事务（如果有 undo log）
	if tx.undoLogItems != nil && tx.undoLogItems.Len() > 0 {
		branchId, err := tx.registerBranch()
		if err != nil {
			glog.Errorf(tx.ctx, "[Seata] Failed to register branch: %v", err)
			return err
		}
		tx.branchId = branchId
		glog.Infof(tx.ctx, "[Seata] Branch registered: BranchID=%d", branchId)
	}

	// 3. 提交本地事务
	err := tx.TX.Commit()
	if err != nil {
		glog.Errorf(tx.ctx, "[Seata] Failed to commit transaction: %v", err)
		// 报告分支失败
		if tx.branchId > 0 {
			_ = tx.reportBranchStatus(branch.BranchStatusPhaseoneFailed)
		}
		return err
	}

	// 4. 报告分支状态为一阶段完成
	if tx.branchId > 0 {
		if err := tx.reportBranchStatus(branch.BranchStatusPhaseoneDone); err != nil {
			glog.Warningf(tx.ctx, "[Seata] Failed to report branch status: %v", err)
			// 报告失败不影响本地提交
		}
	}

	glog.Debugf(tx.ctx, "[Seata] Transaction committed successfully")
	return nil
}

// Rollback 回滚事务
//
// 功能：回滚本地数据库事务，放弃所有未提交的修改
// 注意：回滚后 Undo Log 也会被放弃，不会插入数据库
func (tx *SeataTX) Rollback() error {
	glog.Debugf(tx.ctx, "[Seata] Rolling back transaction, XID: %s", tm.GetXID(tx.ctx))

	// 回滚本地事务
	err := tx.TX.Rollback()
	if err != nil {
		glog.Errorf(tx.ctx, "[Seata] Failed to rollback transaction: %v", err)
		return err
	}

	glog.Debugf(tx.ctx, "[Seata] Transaction rolled back successfully")
	return nil
}

// AddUndoLog 添加 undo log 项
//
// 功能：将一条 SQL 操作的 Undo Log 添加到事务的 Undo Log 列表中
// 该方法由 SQL 拦截器调用，在 INSERT/UPDATE/DELETE 操作时自动收集
//
// 参数：
//   - item: Undo Log 项，包含 SQL 类型、表名、Before/After Image
func (tx *SeataTX) AddUndoLog(item *SQLUndoLog) {
	if tx.undoLogItems == nil {
		tx.undoLogItems = garray.New(true) // 并发安全
	}
	tx.undoLogItems.Append(item)
	glog.Debugf(tx.ctx, "[Seata] Added undo log item, type=%s, table=%s, total=%d",
		item.SQLType, item.TableName, tx.undoLogItems.Len())
}

// GetUndoLogItems 获取所有 undo log 项
func (tx *SeataTX) GetUndoLogItems() []*SQLUndoLog {
	if tx.undoLogItems == nil {
		return nil
	}

	items := make([]*SQLUndoLog, 0, tx.undoLogItems.Len())
	tx.undoLogItems.Iterator(func(k int, v interface{}) bool {
		if item, ok := v.(*SQLUndoLog); ok {
			items = append(items, item)
		}
		return true
	})
	return items
}

// insertUndoLog 插入 undo log 到数据库
func (tx *SeataTX) insertUndoLog() error {
	// 如果没有 undo log，直接返回
	if tx.undoLogItems == nil || tx.undoLogItems.Len() == 0 {
		glog.Debug(tx.ctx, "[Seata] No undo log to insert")
		return nil
	}

	// 构建 BranchUndoLog
	branchUndoLog := &BranchUndoLog{
		XID:         tm.GetXID(tx.ctx),
		BranchID:    tx.branchId,
		SQLUndoLogs: tx.GetUndoLogItems(),
	}

	// 序列化为 JSON
	rollbackInfo, err := gjson.Encode(branchUndoLog)
	if err != nil {
		return fmt.Errorf(ErrEncodeUndoLog, err)
	}

	// 插入 undo_log 表
	sql := fmt.Sprintf(`INSERT INTO %s (
		%s, %s, %s, %s,
		%s, log_created, log_modified
	) VALUES (?, ?, ?, ?, ?, NOW(), NOW())`,
		UndoLogTableName,
		UndoLogColumnBranchID,
		UndoLogColumnXID,
		UndoLogColumnContext,
		UndoLogColumnRollback,
		UndoLogColumnStatus)

	_, err = tx.TX.Exec(sql,
		tx.branchId,
		tm.GetXID(tx.ctx),
		DefaultUndoLogContext,
		string(rollbackInfo),
		UndoLogStatusNormal,
	)

	if err != nil {
		return fmt.Errorf(ErrInsertUndoLog, err)
	}

	glog.Infof(tx.ctx, "[Seata] Undo log inserted, items=%d", tx.undoLogItems.Len())
	return nil
}

// deleteUndoLog 删除 undo log（提交后执行）
func (tx *SeataTX) deleteUndoLog() error {
	if tx.branchId == 0 {
		return nil
	}

	sql := "DELETE FROM undo_log WHERE branch_id = ? AND xid = ?"
	_, err := tx.TX.Exec(sql, tx.branchId, tm.GetXID(tx.ctx))
	if err != nil {
		glog.Warningf(tx.ctx, "[Seata] Failed to delete undo log: %v", err)
		// 删除失败不影响事务提交
	}
	return nil
}

// registerBranch 注册分支事务
func (tx *SeataTX) registerBranch() (int64, error) {
	// 生成锁键
	lockKeys := tx.generateLockKeys()

	// 调用 Seata RM API 注册分支
	branchId, err := rm.GetRMRemotingInstance().BranchRegister(rm.BranchRegisterParam{
		BranchType:      branch.BranchTypeAT,
		ResourceId:      tx.resource.GetResourceId(),
		Xid:             tm.GetXID(tx.ctx),
		ApplicationData: "",
		LockKeys:        lockKeys,
	})

	if err != nil {
		return 0, fmt.Errorf(ErrRegisterBranch, tm.GetXID(tx.ctx), err)
	}

	return branchId, nil
}

// reportBranchStatus 报告分支状态
func (tx *SeataTX) reportBranchStatus(status branch.BranchStatus) error {
	if tx.branchId == 0 {
		return nil
	}

	err := rm.GetRMRemotingInstance().BranchReport(rm.BranchReportParam{
		Xid:      tm.GetXID(tx.ctx),
		BranchId: tx.branchId,
		Status:   status,
	})

	if err != nil {
		return fmt.Errorf(ErrReportBranchStatus, tx.branchId, status, err)
	}

	glog.Debugf(tx.ctx, "[Seata] Branch status reported: BranchID=%d, Status=%v", tx.branchId, status)
	return nil
}

// generateLockKeys 生成锁键
// 格式: table:pk1,pk2;table2:pk3
func (tx *SeataTX) generateLockKeys() string {
	if tx.undoLogItems == nil || tx.undoLogItems.Len() == 0 {
		return ""
	}

	// 按表名分组收集主键值
	tablePKs := make(map[string][]interface{})

	tx.undoLogItems.Iterator(func(k int, v interface{}) bool {
		if item, ok := v.(*SQLUndoLog); ok {
			// 从 beforeImage 提取主键值
			var pkValues []interface{}
			if item.BeforeImage != nil && len(item.BeforeImage.Rows) > 0 {
				pkValues = item.BeforeImage.ExtractPrimaryKeyValues()
			} else if item.AfterImage != nil && len(item.AfterImage.Rows) > 0 {
				// INSERT 场景，beforeImage 为空，使用 afterImage
				pkValues = item.AfterImage.ExtractPrimaryKeyValues()
			}

			// 添加到 map 中
			if len(pkValues) > 0 {
				if existing, ok := tablePKs[item.TableName]; ok {
					tablePKs[item.TableName] = append(existing, pkValues...)
				} else {
					tablePKs[item.TableName] = pkValues
				}
			}
		}
		return true
	})

	// 格式化为字符串
	var lockKeyParts []string
	for tableName, pkValues := range tablePKs {
		if len(pkValues) == 0 {
			continue
		}

		// 去重主键值
		uniqueKeys := make(map[string]bool)
		for _, pk := range pkValues {
			keyStr := fmt.Sprintf("%v", pk)
			uniqueKeys[keyStr] = true
		}

		// 生成 table:pk1,pk2 格式
		var keys []string
		for key := range uniqueKeys {
			keys = append(keys, key)
		}
		lockKeyParts = append(lockKeyParts, fmt.Sprintf("%s:%s", tableName, strings.Join(keys, ",")))
	}

	return strings.Join(lockKeyParts, ";")
}
