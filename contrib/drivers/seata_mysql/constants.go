// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

const (
	// Driver name
	DriverName = "seata-at-mysql"

	// Resource ID format
	ResourceIDFormat = "seata-at-mysql:%s@%s:%s/%s"

	// SAVEPOINT format for nested transactions
	SavepointPrefix = "seata_sp_"
	SavepointFormat = "seata_sp_%d" // Format: seata_sp_0, seata_sp_1, ...

	// Undo log table and columns
	UndoLogTableName      = "undo_log"
	UndoLogColumnBranchID = "branch_id"
	UndoLogColumnXID      = "xid"
	UndoLogColumnContext  = "context"
	UndoLogColumnRollback = "rollback_info"
	UndoLogColumnStatus   = "log_status"

	// Default values
	DefaultUndoLogContext  = "{}"  // Empty JSON object
	DefaultGlobalTxTimeout = 60000 // 60 seconds in milliseconds

	// Error messages - 提供更详细的错误信息模板
	ErrNoTransactionInContext = "no transaction found in context, group=%s"
	ErrNotSeataTX             = "transaction is not a SeataTX, actual type: %T"
	ErrGenerateBeforeImage    = "failed to generate before image for table '%s': %w"
	ErrGenerateAfterImage     = "failed to generate after image for table '%s': %w"
	ErrSaveUndoLog            = "failed to save undo log for table '%s': %w"
	ErrInsertUndoLog          = "failed to insert undo log: %w"
	ErrEncodeUndoLog          = "failed to encode undo log to JSON: %w"
	ErrRegisterBranch         = "failed to register branch for XID '%s': %w"
	ErrReportBranchStatus     = "failed to report branch status (BranchID=%d, Status=%v): %w"
	ErrCommitTransaction      = "failed to commit transaction: %w"
	ErrRollbackTransaction    = "failed to rollback transaction: %w"
	ErrCreateSavepoint        = "failed to create savepoint '%s': %w"
	ErrRollbackToSavepoint    = "failed to rollback to savepoint '%s': %w, original error: %v"
	ErrReleaseSavepoint       = "failed to release savepoint '%s': %w"

	// Log messages
	LogDriverInit           = "[Seata] Initializing Seata driver..."
	LogDriverInitSuccess    = "[Seata] Seata driver initialized successfully"
	LogATModeInit           = "[Seata] Initializing AT mode..."
	LogATModeInitSuccess    = "[Seata] AT mode initialized successfully"
	LogATModeAlreadyInit    = "[Seata] AT mode already initialized (metrics already registered), skipping..."
	LogDBInit               = "[Seata] Initializing AT mode driver for: %s"
	LogDBInitSuccess        = "[Seata] AT mode driver initialized successfully for: %s"
	LogInGlobalTx           = "[Seata] In global transaction, XID: %s"
	LogNotInGlobalTx        = "[Seata] Not in global transaction, using native MySQL transaction"
	LogInterceptingSQL      = "[Seata] Intercepting %s on table: %s"
	LogInterceptedSuccess   = "[Seata] %s intercepted successfully"
	LogUndoLogAdded         = "[Seata] Added undo log item, type=%s, table=%s, total=%d"
	LogUndoLogSaved         = "[Seata] Undo log saved to transaction, type=%s, table=%s"
	LogUndoLogInserted      = "[Seata] Undo log inserted, items=%d"
	LogBranchRegistered     = "[Seata] Branch registered: BranchID=%d"
	LogBranchStatusReported = "[Seata] Branch status reported: BranchID=%d, Status=%s"
	LogCommitting           = "[Seata] Committing transaction, XID: %s"
	LogCommitSuccess        = "[Seata] Transaction committed successfully"
	LogRollingBack          = "[Seata] Rolling back transaction, XID: %s"
	LogRollbackSuccess      = "[Seata] Transaction rolled back successfully"
	LogUndoLogRollback      = "[Seata] Rolled back %d undo log items to savepoint, remaining: %d"
)
