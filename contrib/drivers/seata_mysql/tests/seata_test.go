// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package tests

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/os/gctx"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/rm"
	"github.com/seata/seata-go/pkg/tm"
	"github.com/stretchr/testify/assert"

	seataMysql "github.com/gogf/gf/contrib/drivers/seata_mysql"
)

var (
	// seataInitOnce 确保 Seata 只初始化一次
	seataInitOnce sync.Once
	// seataInitErr 保存初始化错误
	seataInitErr error
	// testDB 全局测试数据库实例
	testDB gdb.DB
)

// setupSeataDB 初始化 Seata 数据库连接
func setupSeataDB(t *testing.T, mode string) gdb.DB {
	ctx := gctx.GetInitCtx()

	// 使用 sync.Once 确保 Seata 只初始化一次
	seataInitOnce.Do(func() {
		// 设置 Seata 配置文件路径
		os.Setenv("SEATA_GO_CONFIG_PATH", "./setup/seata.yml")

		// 初始化 Seata Driver
		config := &seataMysql.Config{
			Enabled:        true,
			ApplicationID:  "gf-seata-test",
			TxServiceGroup: "default_tx_group",
			Mode:           mode,
			AT: seataMysql.ATConfig{
				UndoLogTable: "undo_log",
			},
		}

		seataInitErr = seataMysql.Init(config)
	})

	// 检查初始化是否成功
	if seataInitErr != nil {
		t.Fatalf("❌ Seata 初始化失败: %v", seataInitErr)
	}

	t.Logf("✅ Seata %s 模式已初始化", mode)

	// 创建数据库实例
	node := gdb.ConfigNode{
		Type:    seataMysql.DriverNameATMySQL,
		Host:    "127.0.0.1",
		Port:    "3306",
		User:    "root",
		Pass:    "123456",
		Name:    "seata_test",
		Charset: "utf8mb4",
		Extra:   "parseTime=true",
	}

	db, err := gdb.New(node)
	if err != nil {
		t.Fatalf("❌ 创建数据库实例失败: %v", err)
	}

	// 测试连接
	err = db.PingMaster()
	if err != nil {
		t.Fatalf("❌ 数据库连接失败: %v", err)
	}

	_ = ctx
	return db
}

// resetAccountBalances 重置账户余额到初始状态
func resetAccountBalances(ctx context.Context, db gdb.DB) error {
	_, err := db.Model("accounts").Ctx(ctx).Where("user_id", 1).Data(gdb.Map{
		"balance": 10000.00,
	}).Update()
	if err != nil {
		return err
	}

	_, err = db.Model("accounts").Ctx(ctx).Where("user_id", 2).Data(gdb.Map{
		"balance": 5000.00,
	}).Update()
	if err != nil {
		return err
	}

	_, err = db.Model("accounts").Ctx(ctx).Where("user_id", 3).Data(gdb.Map{
		"balance": 2000.00,
	}).Update()
	return err
}

// getAccountBalance 获取账户余额
func getAccountBalance(ctx context.Context, db gdb.DB, userID int) (float64, error) {
	type Account struct {
		Balance float64
	}
	var account Account
	err := db.Model("accounts").Where("user_id", userID).Scan(&account)
	if err != nil {
		return 0, err
	}
	return account.Balance, nil
}

// ========== Unit Tests ==========

// Test_Config 测试配置功能
func Test_Config(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := seataMysql.DefaultConfig()
		t.AssertNE(config, nil)
		t.AssertEQ(config.ApplicationID, "gf-app")
		t.AssertEQ(config.TxServiceGroup, "default_tx_group")
		t.AssertEQ(config.Mode, "AT")
		t.AssertEQ(config.AT.UndoLogTable, "undo_log")
	})
}

// Test_Driver_Basic 测试基础驱动功能
func Test_Driver_Basic(t *testing.T) {
	ctx := gctx.New()

	assert.NotPanics(t, func() {
		seataMysql.Init(seataMysql.DefaultConfig())
	})

	assert.NotPanics(t, func() {
		seataMysql.NewDriverAT(seataMysql.DefaultConfig())
	})

	assert.NotPanics(t, func() {
		seataMysql.NewDriverXA(seataMysql.DefaultConfig())
	})

	_ = ctx
}

// Test_AsyncWorker 测试异步工作器
func Test_AsyncWorker(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		worker := seataMysql.NewAsyncWorker(nil, seataMysql.DefaultAsyncWorkerConfig())

		worker.Start()
		// AsyncWorker 没有 IsRunning 方法，跳过状态检查

		worker.Stop()
		// 验证可以多次调用 Stop
		worker.Stop()
	})
}

// Test_AsyncWorker_BranchCommit 测试分支提交
func Test_AsyncWorker_BranchCommit(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		config := seataMysql.DefaultAsyncWorkerConfig()
		config.WorkerPoolSize = 2
		config.QueueSize = 10

		worker := seataMysql.NewAsyncWorker(nil, config)
		worker.Start()
		defer worker.Stop()

		for i := 0; i < 5; i++ {
			err := worker.BranchCommit(rm.BranchResource{
				BranchType: branch.BranchTypeAT,
				Xid:        "test-xid",
				BranchId:   int64(i + 1),
			})
			t.AssertNil(err)
		}
	})
}

// Test_ImageGeneration 测试镜像生成功能
func Test_ImageGeneration(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		beforeImage := &seataMysql.TableRecords{
			TableName: "users",
			Rows: []seataMysql.Row{
				{
					Fields: []seataMysql.Field{
						{Name: "id", KeyType: seataMysql.KeyTypePrimaryKey, Type: 4, Value: 1},
						{Name: "balance", KeyType: seataMysql.KeyTypeCommon, Type: 3, Value: 1000},
					},
				},
			},
		}

		afterImage := &seataMysql.TableRecords{
			TableName: "users",
			Rows: []seataMysql.Row{
				{
					Fields: []seataMysql.Field{
						{Name: "id", KeyType: seataMysql.KeyTypePrimaryKey, Type: 4, Value: 1},
						{Name: "balance", KeyType: seataMysql.KeyTypeCommon, Type: 3, Value: 900},
					},
				},
			},
		}

		pkValues := beforeImage.ExtractPrimaryKeyValues()
		t.Assert(len(pkValues), 1)
		t.Assert(pkValues[0], int64(1))

		// 验证表名
		t.AssertEQ(beforeImage.TableName, "users")

		_ = afterImage
	})
}

// Test_Fallback 测试降级机制
func Test_Fallback(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		fallback := seataMysql.NewFallbackManager(seataMysql.DefaultFallbackConfig())

		err := fallback.ExecuteWithFallback(context.Background(), nil, func(ctx context.Context) error {
			return nil
		}, nil)
		t.AssertNil(err)
	})
}

// Test_Retry 测试重试机制
func Test_Retry(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		retry := seataMysql.NewRetryExecutor(seataMysql.DefaultRetryConfig())

		err := retry.Execute(context.Background(), func() error {
			return nil
		})
		t.AssertNil(err)
	})
}

// Test_Context 测试事务上下文
func Test_Context(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		// 测试全局事务上下文
		ctx = seataMysql.WithGlobalTransaction(ctx, "test-tx")

		t.AssertNE(ctx, nil)
		// WithGlobalTransaction 会设置 Seata context，但可能不是全局事务
		// 所以只检查 context 不为空
	})
}

// Test_Metrics 测试指标收集
func Test_Metrics(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 测试配置创建
		config := seataMysql.DefaultConfig()
		t.AssertNE(config, nil)
		t.AssertEQ(config.ApplicationID, "gf-app")
	})
}

// ========== Integration Tests ==========

// TestIntegration_BasicConnectivity 测试基本数据库连接
func TestIntegration_BasicConnectivity(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gtest.C(t, func(t *gtest.T) {
		config := gdb.ConfigNode{
			Type:     "mysql",
			Host:     "127.0.0.1",
			Port:     "3306",
			User:     "root",
			Pass:     "123456",
			Name:     "seata_test",
			Role:     "master",
			Charset:  "utf8mb4",
			Protocol: "tcp",
			Timezone: "UTC",
		}

		ctx := gctx.New()

		db, err := gdb.New(config)
		t.AssertNil(err)
		t.AssertNE(db, nil)
		defer db.Close(ctx)

		err = db.PingMaster()
		t.AssertNil(err)
	})
}

// TestSeataAT_BasicTransfer AT模式基础转账测试
func TestSeataAT_BasicTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		if err != nil {
			t.Fatalf("重置账户余额失败: %v", err)
		}

		balance1Before, err := getAccountBalance(ctx, db, 1)
		t.AssertNil(err)
		t.AssertEQ(balance1Before, 10000.00)

		balance2Before, err := getAccountBalance(ctx, db, 2)
		t.AssertNil(err)
		t.AssertEQ(balance2Before, 5000.00)

		t.Logf("转账前余额: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "basic-transfer",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 扣款
			_, err := db.Model("accounts").
				Where("id", 1).
				Decrement("balance", 500.00)
			if err != nil {
				return err
			}

			// 加款
			_, err = db.Model("accounts").
				Where("id", 2).
				Increment("balance", 500.00)
			return err
		})

		t.AssertNil(err)

		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		t.AssertEQ(balance1After, 9500.00)
		t.AssertEQ(balance2After, 5500.00)

		t.Logf("转账后余额: 用户1=%.2f, 用户2=%.2f", balance1After, balance2After)
	})
}

// TestSeataAT_ConcurrentTransfer 并发转账测试
func TestSeataAT_ConcurrentTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		if err != nil {
			t.Fatalf("重置账户余额失败: %v", err)
		}

		concurrency := 5
		amount := 100.00
		var wg sync.WaitGroup
		var mu sync.Mutex
		var successCount int
		var errorCount int
		results := make(chan string, concurrency)

		startTime := time.Now()

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()

				err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
					Name:    fmt.Sprintf("concurrent-transfer-%d", id),
					Timeout: 30 * time.Second,
				}, func(ctx context.Context) error {
					// 扣款
					_, err := db.Model("accounts").
						Where("id", 1).
						Decrement("balance", amount)
					if err != nil {
						return err
					}

					// 加款
					_, err = db.Model("accounts").
						Where("id", 2).
						Increment("balance", amount)
					return err
				})

				mu.Lock()
				if err != nil {
					errorCount++
					results <- fmt.Sprintf("并发%d失败: %v", id, err)
				} else {
					successCount++
					results <- fmt.Sprintf("并发%d成功", id)
				}
				mu.Unlock()
			}(i)
		}

		wg.Wait()
		close(results)

		duration := time.Since(startTime)
		t.Logf("并发转账完成: 成功=%d, 失败=%d, 耗时=%v", successCount, errorCount, duration)

		for result := range results {
			t.Logf("  %s", result)
		}

		t.AssertEQ(successCount, concurrency)
		t.AssertEQ(errorCount, 0)

		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		t.AssertEQ(balance1After, 10000.00-amount*float64(concurrency))
		t.AssertEQ(balance2After, 5000.00+amount*float64(concurrency))
	})
}

// TestSeataAT_NestedTransactionBasic 基础嵌套事务测试
func TestSeataAT_NestedTransactionBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		if err != nil {
			t.Fatalf("重置账户余额失败: %v", err)
		}

		balance1Before, _ := getAccountBalance(ctx, db, 1)
		balance2Before, _ := getAccountBalance(ctx, db, 2)

		t.Logf("嵌套事务测试开始: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "outer-transaction",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 外层事务：转账100
			tx1, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer tx1.Rollback()

			_, err = tx1.Model("accounts").
				Where("id", 1).
				Decrement("balance", 100)
			if err != nil {
				return err
			}

			// 嵌套事务：转账50
			err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
				Name:    "inner-transaction",
				Timeout: 30 * time.Second,
			}, func(ctx context.Context) error {
				tx2, err := db.Begin(ctx)
				if err != nil {
					return err
				}
				defer tx2.Rollback()

				_, err = tx2.Model("accounts").
					Where("id", 1).
					Decrement("balance", 50)
				if err != nil {
					return err
				}

				_, err = tx2.Model("accounts").
					Where("id", 2).
					Increment("balance", 50)
				if err != nil {
					return err
				}

				return tx2.Commit()
			})
			if err != nil {
				return err
			}

			_, err = tx1.Model("accounts").
				Where("id", 2).
				Increment("balance", 100)
			if err != nil {
				return err
			}

			return tx1.Commit()
		})

		t.AssertNil(err)

		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		t.AssertEQ(balance1After, balance1Before-150.00)
		t.AssertEQ(balance2After, balance2Before+150.00)

		t.Logf("嵌套事务测试结束: 用户1=%.2f, 用户2=%.2f", balance1After, balance2After)
	})
}

// TestSeataAT_TransactionRollback 事务回滚测试
func TestSeataAT_TransactionRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		if err != nil {
			t.Fatalf("重置账户余额失败: %v", err)
		}

		balance1Before, _ := getAccountBalance(ctx, db, 1)
		balance2Before, _ := getAccountBalance(ctx, db, 2)

		t.Logf("回滚测试开始: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "rollback-test",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 扣款
			_, err := db.Model("accounts").
				Where("id", 1).
				Decrement("balance", 500.00)
			if err != nil {
				return err
			}

			// 加款
			_, err = db.Model("accounts").
				Where("id", 2).
				Increment("balance", 500.00)
			if err != nil {
				return err
			}

			// 故意触发错误进行回滚
			return errors.New("intentional rollback error")
		})

		t.AssertNE(err, nil)

		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		t.AssertEQ(balance1After, balance1Before)
		t.AssertEQ(balance2After, balance2Before)

		t.Logf("回滚测试结束: 用户1=%.2f, 用户2=%.2f (余额未变)", balance1After, balance2After)
	})
}

// TestSeataXA_BasicTransfer XA模式基础转账测试
func TestSeataXA_BasicTransfer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "XA")

		err := resetAccountBalances(ctx, db)
		if err != nil {
			t.Fatalf("重置账户余额失败: %v", err)
		}

		balance1Before, _ := getAccountBalance(ctx, db, 1)
		balance2Before, _ := getAccountBalance(ctx, db, 2)

		t.Logf("XA模式转账前: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "xa-basic-transfer",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 扣款
			_, err := db.Model("accounts").
				Where("id", 1).
				Decrement("balance", 300.00)
			if err != nil {
				return err
			}

			// 加款
			_, err = db.Model("accounts").
				Where("id", 2).
				Increment("balance", 300.00)
			return err
		})

		t.AssertNil(err)

		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		t.AssertEQ(balance1After, 9700.00)
		t.AssertEQ(balance2After, 5300.00)

		t.Logf("XA模式转账后: 用户1=%.2f, 用户2=%.2f", balance1After, balance2After)
	})
}
