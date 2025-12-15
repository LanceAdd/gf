// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogf/gf/v2/container/garray"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/encoding/gjson"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"
	"github.com/seata/seata-go/pkg/tm"
)

// SeataTX Seata 事务对象
type SeataTX struct {
	gdb.TX
	resource     *Resource
	ctx          context.Context
	undoLogItems *garray.Array // 存储所有的 undo log 项
	localTxId    string        // 本地事务ID
	branchId     int64         // 分支事务ID（注册后获得）
}

// Ctx 返回当前事务的 context
func (tx *SeataTX) GetCtx() context.Context {
	return tx.ctx
}

// Ctx 设置并返回事务的 context
func (tx *SeataTX) Ctx(ctx context.Context) gdb.TX {
	if ctx == nil {
		return tx
	}
	tx.ctx = ctx
	// 同时更新底层事务的 context
	tx.TX = tx.TX.Ctx(ctx)
	return tx
}

// Model 返回Model对象，确保使用SeataTX的context
func (tx *SeataTX) Model(tableNameQueryOrStruct ...interface{}) *gdb.Model {
	// 使用底层TX的Model方法，但要确保context正确
	model := tx.TX.Model(tableNameQueryOrStruct...)
	// 使用SeataTX的context，这样在拦截器中能正确获取到SeataTX对象
	return model.Ctx(tx.ctx)
}

// Commit 提交事务
func (tx *SeataTX) Commit() error {
	// 阶段一：这里先使用简化实现
	// 完整的分支注册和 Undo Log 处理将在阶段二和阶段三实现
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
		glog.Errorf(tx.ctx, "[Seata] Failed to encode undo log: %v", err)
		return err
	}

	// 插入 undo_log 表
	sql := `INSERT INTO undo_log (
		branch_id, xid, context, rollback_info,
		log_status, log_created, log_modified
	) VALUES (?, ?, ?, ?, ?, NOW(), NOW())`

	_, err = tx.TX.Exec(sql,
		tx.branchId,
		tm.GetXID(tx.ctx),
		"{}", // context 默认空
		string(rollbackInfo),
		0, // UndoLogStatusNormal
	)

	if err != nil {
		glog.Errorf(tx.ctx, "[Seata] Failed to insert undo log: %v", err)
		return err
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
		return 0, fmt.Errorf("failed to register branch: %w", err)
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
		return fmt.Errorf("failed to report branch status: %w", err)
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
