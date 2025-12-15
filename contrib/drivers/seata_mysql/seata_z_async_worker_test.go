// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql

import (
	"testing"
	"time"

	"github.com/gogf/gf/v2/test/gtest"
	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"
)

func TestAsyncWorker_StartStop(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		worker := NewAsyncWorker(nil, DefaultAsyncWorkerConfig())

		// 测试启动
		worker.Start()
		t.Assert(worker.running, true)

		// 测试停止
		worker.Stop()
		t.Assert(worker.running, false)

		// 重复停止不应该panic
		worker.Stop()
	})
}

func TestAsyncWorker_BranchCommit(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := DefaultAsyncWorkerConfig()
		config.WorkerPoolSize = 2
		config.QueueSize = 10

		worker := NewAsyncWorker(nil, config)
		worker.Start()
		defer worker.Stop()

		// 发送任务
		for i := 0; i < 5; i++ {
			err := worker.BranchCommit(rm.BranchResource{
				BranchType: branch.BranchTypeAT,
				Xid:        "test-xid",
				BranchId:   int64(i),
			})
			t.AssertNil(err)
		}

		// 等待处理
		time.Sleep(100 * time.Millisecond)

		// 检查统计
		stats := worker.GetStats()
		t.AssertGT(stats["total_tasks"], int64(0))
	})
}

func TestAsyncWorker_GetStats(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		worker := NewAsyncWorker(nil, DefaultAsyncWorkerConfig())
		worker.Start()
		defer worker.Stop()

		stats := worker.GetStats()
		t.AssertNE(stats, nil)
		t.Assert(stats["total_tasks"], int64(0))
		t.Assert(stats["success_tasks"], int64(0))
		t.Assert(stats["failed_tasks"], int64(0))
	})
}

func TestDefaultAsyncWorkerConfig(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := DefaultAsyncWorkerConfig()

		t.Assert(config.WorkerPoolSize, 10)
		t.Assert(config.QueueSize, 1000)
		t.Assert(config.BatchSize, 100)
		t.Assert(config.BatchInterval, 1000)
		t.Assert(config.RetryTimes, 3)
		t.Assert(config.RetryInterval, 100)
	})
}
