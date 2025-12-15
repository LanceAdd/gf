// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata

// SQLUndoLog SQL undo log 项
type SQLUndoLog struct {
	SQLType     SQLType       `json:"sqlType"`
	TableName   string        `json:"tableName"`
	BeforeImage *TableRecords `json:"beforeImage"`
	AfterImage  *TableRecords `json:"afterImage"`
}

// BranchUndoLog 分支 undo log（包含多个 SQL undo log）
type BranchUndoLog struct {
	XID         string        `json:"xid"`
	BranchID    int64         `json:"branchId"`
	SQLUndoLogs []*SQLUndoLog `json:"sqlUndoLogs"`
}

// UndoLogStatus undo log 状态
type UndoLogStatus int

const (
	// UndoLogStatusNormal 正常状态
	UndoLogStatusNormal UndoLogStatus = 0
	// UndoLogStatusGlobalFinished 全局已完成
	UndoLogStatusGlobalFinished UndoLogStatus = 1
)
