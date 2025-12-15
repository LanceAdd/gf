// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"context"
	"database/sql"
	"sync"
	"time"

	"github.com/gogf/gf/v2/os/glog"
	"github.com/seata/seata-go/pkg/rm"
)

// AsyncWorkerConfig 异步工作器配置
type AsyncWorkerConfig struct {
	// WorkerPoolSize 工作协程池大小
	WorkerPoolSize int

	// QueueSize 任务队列大小
	QueueSize int

	// BatchSize 批量处理大小
	BatchSize int

	// BatchInterval 批量处理间隔（毫秒）
	BatchInterval int

	// RetryTimes 重试次数
	RetryTimes int

	// RetryInterval 重试间隔（毫秒）
	RetryInterval int
}

// DefaultAsyncWorkerConfig 默认配置
func DefaultAsyncWorkerConfig() *AsyncWorkerConfig {
	return &AsyncWorkerConfig{
		WorkerPoolSize: 10,
		QueueSize:      1000,
		BatchSize:      100,
		BatchInterval:  1000, // 1秒
		RetryTimes:     3,
		RetryInterval:  100, // 100毫秒
	}
}

// CommitTask 提交任务
type CommitTask struct {
	Resource rm.BranchResource
	CreateAt time.Time
}

// AsyncWorker 异步工作器（用于异步删除 undo log）
type AsyncWorker struct {
	config *AsyncWorkerConfig
	db     *sql.DB

	// 任务队列
	taskQueue chan *CommitTask

	// 工作协程池
	workers []*Worker

	// 统计信息
	stats *WorkerStats

	// 控制
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// 状态
	running bool
	mu      sync.RWMutex
}

// Worker 工作协程
type Worker struct {
	id     int
	worker *AsyncWorker
}

// WorkerStats 工作统计
type WorkerStats struct {
	TotalTasks      int64 // 总任务数
	SuccessTasks    int64 // 成功任务数
	FailedTasks     int64 // 失败任务数
	RetryTasks      int64 // 重试任务数
	QueueLength     int64 // 当前队列长度
	ProcessDuration int64 // 总处理时间（毫秒）

	mu sync.RWMutex
}

// NewAsyncWorker 创建异步工作器
func NewAsyncWorker(db *sql.DB, config *AsyncWorkerConfig) *AsyncWorker {
	if config == nil {
		config = DefaultAsyncWorkerConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	worker := &AsyncWorker{
		config:    config,
		db:        db,
		taskQueue: make(chan *CommitTask, config.QueueSize),
		workers:   make([]*Worker, config.WorkerPoolSize),
		stats:     &WorkerStats{},
		ctx:       ctx,
		cancel:    cancel,
		running:   false,
	}

	return worker
}

// Start 启动异步工作器
func (aw *AsyncWorker) Start() {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if aw.running {
		return
	}

	glog.Info(aw.ctx, "[AsyncWorker] Starting async worker...")

	// 启动工作协程池
	for i := 0; i < aw.config.WorkerPoolSize; i++ {
		w := &Worker{
			id:     i,
			worker: aw,
		}
		aw.workers[i] = w

		aw.wg.Add(1)
		go w.run()
	}

	// 启动批量处理协程
	aw.wg.Add(1)
	go aw.batchProcessor()

	aw.running = true
	glog.Infof(aw.ctx, "[AsyncWorker] Async worker started, pool size: %d", aw.config.WorkerPoolSize)
}

// Stop 停止异步工作器
func (aw *AsyncWorker) Stop() {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if !aw.running {
		return
	}

	glog.Info(aw.ctx, "[AsyncWorker] Stopping async worker...")

	// 关闭任务队列
	close(aw.taskQueue)

	// 取消上下文
	aw.cancel()

	// 等待所有工作协程退出
	aw.wg.Wait()

	aw.running = false
	glog.Info(aw.ctx, "[AsyncWorker] Async worker stopped")
}

// BranchCommit 异步提交分支（删除 undo log）
func (aw *AsyncWorker) BranchCommit(resource rm.BranchResource) error {
	aw.mu.RLock()
	defer aw.mu.RUnlock()

	if !aw.running {
		// 如果未启动，直接同步删除
		return aw.deleteUndoLogSync(resource)
	}

	// 创建任务
	task := &CommitTask{
		Resource: resource,
		CreateAt: time.Now(),
	}

	// 发送到队列（非阻塞）
	select {
	case aw.taskQueue <- task:
		aw.stats.incTotalTasks()
		aw.stats.setQueueLength(int64(len(aw.taskQueue)))
		return nil
	default:
		// 队列满，降级为同步删除
		glog.Warningf(aw.ctx, "[AsyncWorker] Task queue is full, fallback to sync delete")
		return aw.deleteUndoLogSync(resource)
	}
}

// Worker.run 工作协程运行
func (w *Worker) run() {
	defer w.worker.wg.Done()

	glog.Debugf(w.worker.ctx, "[AsyncWorker] Worker #%d started", w.id)

	for {
		select {
		case task, ok := <-w.worker.taskQueue:
			if !ok {
				// 队列已关闭
				glog.Debugf(w.worker.ctx, "[AsyncWorker] Worker #%d stopped (queue closed)", w.id)
				return
			}

			// 处理任务
			w.processTask(task)

		case <-w.worker.ctx.Done():
			// 上下文取消
			glog.Debugf(w.worker.ctx, "[AsyncWorker] Worker #%d stopped (context done)", w.id)
			return
		}
	}
}

// Worker.processTask 处理任务
func (w *Worker) processTask(task *CommitTask) {
	start := time.Now()
	defer func() {
		duration := time.Since(start).Milliseconds()
		w.worker.stats.addProcessDuration(duration)
	}()

	// 更新队列长度
	w.worker.stats.setQueueLength(int64(len(w.worker.taskQueue)))

	// 尝试删除 undo log
	var lastErr error
	for i := 0; i <= w.worker.config.RetryTimes; i++ {
		err := w.worker.deleteUndoLogSync(task.Resource)
		if err == nil {
			// 成功
			w.worker.stats.incSuccessTasks()
			glog.Debugf(w.worker.ctx, "[AsyncWorker] Worker #%d deleted undo log: XID=%s, BranchID=%d",
				w.id, task.Resource.Xid, task.Resource.BranchId)
			return
		}

		lastErr = err

		if i < w.worker.config.RetryTimes {
			// 重试
			w.worker.stats.incRetryTasks()
			glog.Warningf(w.worker.ctx, "[AsyncWorker] Worker #%d retry %d/%d: %v",
				w.id, i+1, w.worker.config.RetryTimes, err)
			time.Sleep(time.Duration(w.worker.config.RetryInterval) * time.Millisecond)
		}
	}

	// 重试失败
	w.worker.stats.incFailedTasks()
	glog.Errorf(w.worker.ctx, "[AsyncWorker] Worker #%d failed to delete undo log after %d retries: %v",
		w.id, w.worker.config.RetryTimes, lastErr)
}

// batchProcessor 批量处理器（可选实现，暂不启用）
func (aw *AsyncWorker) batchProcessor() {
	defer aw.wg.Done()

	ticker := time.NewTicker(time.Duration(aw.config.BatchInterval) * time.Millisecond)
	defer ticker.Stop()

	batch := make([]*CommitTask, 0, aw.config.BatchSize)

	for {
		select {
		case <-ticker.C:
			// 定时批量处理
			if len(batch) > 0 {
				aw.processBatch(batch)
				batch = batch[:0] // 清空
			}

		case <-aw.ctx.Done():
			// 上下文取消，处理剩余批次
			if len(batch) > 0 {
				aw.processBatch(batch)
			}
			return
		}
	}
}

// processBatch 批量处理（批量删除 undo log）
func (aw *AsyncWorker) processBatch(batch []*CommitTask) {
	if len(batch) == 0 {
		return
	}

	glog.Debugf(aw.ctx, "[AsyncWorker] Processing batch of %d tasks", len(batch))

	// 构建批量删除 SQL
	// DELETE FROM undo_log WHERE (xid = ? AND branch_id = ?) OR (xid = ? AND branch_id = ?) ...
	// 暂不实现，保持简单
}

// deleteUndoLogSync 同步删除 undo log
func (aw *AsyncWorker) deleteUndoLogSync(resource rm.BranchResource) error {
	// 如果 db 为 nil（测试环境），直接返回
	if aw.db == nil {
		return nil
	}

	ctx := context.Background()

	deleteSql := "DELETE FROM undo_log WHERE xid = ? AND branch_id = ?"
	_, err := aw.db.ExecContext(ctx, deleteSql, resource.Xid, resource.BranchId)
	return err
}

// GetStats 获取统计信息
func (aw *AsyncWorker) GetStats() map[string]interface{} {
	aw.stats.mu.RLock()
	defer aw.stats.mu.RUnlock()

	return map[string]interface{}{
		"total_tasks":      aw.stats.TotalTasks,
		"success_tasks":    aw.stats.SuccessTasks,
		"failed_tasks":     aw.stats.FailedTasks,
		"retry_tasks":      aw.stats.RetryTasks,
		"queue_length":     aw.stats.QueueLength,
		"process_duration": aw.stats.ProcessDuration,
		"avg_duration":     aw.getAvgDuration(),
		"success_rate":     aw.getSuccessRate(),
	}
}

// getAvgDuration 获取平均处理时间
func (aw *AsyncWorker) getAvgDuration() int64 {
	if aw.stats.SuccessTasks == 0 {
		return 0
	}
	return aw.stats.ProcessDuration / aw.stats.SuccessTasks
}

// getSuccessRate 获取成功率
func (aw *AsyncWorker) getSuccessRate() float64 {
	if aw.stats.TotalTasks == 0 {
		return 0
	}
	return float64(aw.stats.SuccessTasks) / float64(aw.stats.TotalTasks) * 100
}

// WorkerStats 方法

func (s *WorkerStats) incTotalTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TotalTasks++
}

func (s *WorkerStats) incSuccessTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SuccessTasks++
}

func (s *WorkerStats) incFailedTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FailedTasks++
}

func (s *WorkerStats) incRetryTasks() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RetryTasks++
}

func (s *WorkerStats) setQueueLength(length int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.QueueLength = length
}

func (s *WorkerStats) addProcessDuration(duration int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ProcessDuration += duration
}
