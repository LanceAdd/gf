// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package seata_mysql_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/seata/seata-go/pkg/protocol/branch"
	"github.com/seata/seata-go/pkg/tm"

	"github.com/gogf/gf/contrib/drivers/seata_mysql/v2"
	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/test/gtest"
)

var (
	// testDB 测试数据库连接
	testDB gdb.DB

	// ctx 测试上下文
	ctx = context.Background()
)

// init 初始化测试环境
func init() {
	// 1. 设置 Seata 配置文件路径（必须在 Init 之前）
	// 获取当前工作目录
	cwd, err := os.Getwd()
	if err != nil {
		panic(fmt.Sprintf("获取工作目录失败: %v", err))
	}

	// 设置 Seata 配置文件路径
	configPath := filepath.Join(cwd, "tests", "config", "config.yaml")
	if err := os.Setenv("SEATA_GO_CONFIG_PATH", configPath); err != nil {
		panic(fmt.Sprintf("设置 SEATA_GO_CONFIG_PATH 失败: %v", err))
	}

	fmt.Printf("Seata 配置文件路径: %s\n", configPath)

	// 2. 初始化 Seata AT 模式（注册驱动并连接 Seata Server）
	config := &seata_mysql.Config{
		Enabled: true,
	}

	if err = seata_mysql.Init(config); err != nil {
		panic(fmt.Sprintf("初始化 Seata 失败: %v", err))
	}

	// 3. 配置并创建数据库连接
	node := gdb.ConfigNode{
		Type:    "seata-at-mysql",
		Host:    "127.0.0.1",
		Port:    "3306",
		User:    "root",
		Pass:    "123456",
		Name:    "seata_test",
		Charset: "utf8mb4",
	}

	// 使用 AddConfigNode + NewByGroup 模式
	if err := gdb.AddConfigNode(gdb.DefaultGroupName, node); err != nil {
		panic(fmt.Sprintf("添加配置节点失败: %v", err))
	}

	if r, err := gdb.NewByGroup(); err != nil {
		panic(fmt.Sprintf("创建数据库实例失败: %v", err))
	} else {
		testDB = r
	}
}

// ============================================================================
// 单元测试 (不需要数据库连接)
// ============================================================================

// Test_Config 测试配置功能
func Test_Config(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 测试默认配置
		config := seata_mysql.DefaultConfig()
		t.AssertNE(config, nil)
		t.AssertEQ(config.Enabled, false)

		// 测试 AT 配置
		t.AssertEQ(config.AT.OnlyCarePrimaryKey, false)
		t.AssertEQ(config.AT.EnableAsyncCommit, true)

		// 测试分支类型（仅支持 AT）
		branchType := config.GetBranchType()
		t.AssertEQ(branchType, branch.BranchTypeAT)
	})
}

// Test_Driver_Basic 测试驱动基础功能
func Test_Driver_Basic(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 测试驱动名称
		t.AssertEQ(seata_mysql.DriverNameATMySQL, "seata-at-mysql")

		// 测试驱动创建
		config := seata_mysql.DefaultConfig()
		driver := seata_mysql.NewDriverAT(config)
		t.AssertNE(driver, nil)
	})
}

// Test_BranchType 测试分支类型（仅支持 AT）
func Test_BranchType(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		// 测试 GetBranchType 始终返回 AT
		branchType := seata_mysql.GetBranchType()
		t.AssertEQ(branchType, branch.BranchTypeAT)

		// 即使未初始化也应该返回 AT
		t.AssertEQ(seata_mysql.GetBranchType(), branch.BranchTypeAT)
	})
}

// cleanupTestData 清理测试数据
func cleanupTestData() {
	if testDB == nil {
		return
	}

	// 清理测试表数据
	testDB.Model("accounts").Ctx(ctx).Where("user_id >= 100").Delete()
	testDB.Model("orders").Ctx(ctx).Where("user_id >= 100").Delete()
	testDB.Exec(ctx, "TRUNCATE TABLE order_items")

	// 重置 products 表的库存为初始值 100
	testDB.Exec(ctx, "UPDATE products SET stock = 100 WHERE id = 1")

	// 清理 undo_log 表
	testDB.Exec(ctx, "DELETE FROM undo_log WHERE 1=1")
}

// ============================================================================
// 完整使用示例测试
// ============================================================================

// TestSeataAT_CompleteUsageExample 测试完整的 Seata AT 模式使用示例
// 这个测试展示了在 GF 项目中如何完整使用 Seata AT 模式
func TestSeataAT_CompleteUsageExample(t *testing.T) {
	// 测试前先清理
	cleanupTestData()
	defer cleanupTestData()

	gtest.C(t, func(t *gtest.T) {
		//========================================
		// 步骤 1: 准备测试数据
		// ========================================
		_, err := testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 100,
			"balance": 10000.00,
		})
		t.AssertNil(err)

		// ========================================
		// 步骤 2: 使用 Seata 全局事务
		// ========================================
		// 方式 1: 使用 tm.WithGlobalTx() 包装全局事务
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "test-order-transaction",
			Timeout: 60000, // 60秒超时
		}, func(ctx context.Context) error {
			// 在全局事务中执行本地事务
			return testDB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 业务逻辑 1: 扣款
				_, err := tx.Model("accounts").Ctx(ctx).
					Where("user_id", 100).
					Decrement("balance", 7999.00)
				if err != nil {
					return err
				}

				// 业务逻辑 2: 减库存
				_, err = tx.Model("products").Ctx(ctx).
					Where("id", 1).
					Decrement("stock", 1)
				if err != nil {
					return err
				}

				// 业务逻辑 3: 创建订单
				_, err = tx.Model("orders").Ctx(ctx).Insert(gdb.Map{
					"user_id":    100,
					"product_id": 1,
					"amount":     7999.00,
					"status":     "PAID",
				})
				return err
			})
		})
		t.AssertNil(err)

		// ========================================
		// 步骤 3: 验证结果
		// ========================================
		// 验证账户余额
		balance, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 100).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance.Float64(), 2001.00) // 10000 - 7999

		// 验证库存
		stock, err := testDB.Model("products").Ctx(ctx).
			Where("id", 1).
			Value("stock")
		t.AssertNil(err)
		t.AssertEQ(stock.Int(), 99) // 100 - 1

		// 验证订单
		count, err := testDB.Model("orders").Ctx(ctx).
			Where("user_id", 100).
			Count()
		t.AssertNil(err)
		t.AssertEQ(count, 1)
	})
}

// TestSeataAT_LocalTransactionWithoutGlobalTx 测试不在全局事务中使用本地事务
// 展示如何在不需要分布式事务时使用普通事务
func TestSeataAT_LocalTransactionWithoutGlobalTx(t *testing.T) {
	// 测试前先清理
	cleanupTestData()
	defer cleanupTestData()

	gtest.C(t, func(t *gtest.T) {
		// 准备测试数据
		_, err := testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 101,
			"balance": 5000.00,
		})
		t.AssertNil(err)

		// ========================================
		// 不使用全局事务，直接使用本地事务
		// 这时 Seata 驱动会自动降级为普通 MySQL 事务
		// ========================================
		err = testDB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			_, err := tx.Model("accounts").Ctx(ctx).
				Where("user_id", 101).
				Decrement("balance", 1000.00)
			return err
		})
		t.AssertNil(err)

		// 验证结果
		balance, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 101).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance.Float64(), 4000.00)
	})
}

// TestSeataAT_NestedTransaction 测试嵌套事务
// 展示如何在 Seata 事务中使用嵌套事务（SAVEPOINT 机制）
func TestSeataAT_NestedTransaction(t *testing.T) {
	// 测试前先清理
	cleanupTestData()
	defer cleanupTestData()

	gtest.C(t, func(t *gtest.T) {
		// 准备测试账户
		_, err := testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 105,
			"balance": 1000.00,
		})
		t.AssertNil(err)

		_, err = testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 106,
			"balance": 500.00,
		})
		t.AssertNil(err)

		// ========================================
		// 测试嵌套事务：外层事务中包含内层事务
		// 使用 tx.Transaction() 实现嵌套（SAVEPOINT）
		// ========================================
		err = testDB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			// 外层事务：扣款
			_, err := tx.Model("accounts").Ctx(ctx).
				Where("user_id", 105).
				Decrement("balance", 300.00)
			if err != nil {
				return err
			}

			// ⭐ 内层嵌套事务：使用 tx.Transaction() 而不是 testDB.Transaction()
			err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
				_, err := tx2.Model("accounts").Ctx(ctx).
					Where("user_id", 106).
					Increment("balance", 300.00)
				return err
			})

			return err
		})
		t.AssertNil(err)

		// 验证嵌套事务结果
		balance105, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 105).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance105.Float64(), 700.00)

		balance106, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 106).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance106.Float64(), 800.00)
	})
}

// TestSeataAT_NestedRollback 测试嵌套事务回滚
// 展示内层事务失败时如何回滚
func TestSeataAT_NestedRollback(t *testing.T) {
	// 测试前先清理
	cleanupTestData()
	defer cleanupTestData()

	gtest.C(t, func(t *gtest.T) {
		// 准备测试账户
		_, err := testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 107,
			"balance": 1000.00,
		})
		t.AssertNil(err)

		_, err = testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 108,
			"balance": 500.00,
		})
		t.AssertNil(err)

		originalBalance107 := 1000.00
		originalBalance108 := 500.00

		// ========================================
		// 测试嵌套事务回滚：内层事务失败导致外层回滚
		// ========================================
		err = testDB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			// 外层事务：扣款
			_, err := tx.Model("accounts").Ctx(ctx).
				Where("user_id", 107).
				Decrement("balance", 300.00)
			if err != nil {
				return err
			}

			// 内层嵌套事务：模拟失败
			err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
				_, err := tx2.Model("accounts").Ctx(ctx).
					Where("user_id", 108).
					Increment("balance", 300.00)
				if err != nil {
					return err
				}

				// 模拟内层事务失败
				return fmt.Errorf("内层事务失败")
			})

			// 内层失败，外层也应该回滚
			return err
		})
		t.AssertNE(err, nil)

		// 验证所有操作都已回滚
		balance107, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 107).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance107.Float64(), originalBalance107)

		balance108, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 108).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance108.Float64(), originalBalance108)
	})
}

// TestSeataAT_GlobalTransactionRollback 测试全局事务回滚
// 展示分布式事务失败时的回滚机制
func TestSeataAT_GlobalTransactionRollback(t *testing.T) {
	// 测试前先清理
	cleanupTestData()
	defer cleanupTestData()

	gtest.C(t, func(t *gtest.T) {
		// 准备测试数据
		_, err := testDB.Model("accounts").Ctx(ctx).Insert(gdb.Map{
			"user_id": 109,
			"balance": 10000.00,
		})
		t.AssertNil(err)

		originalBalance := 10000.00
		originalStockVal, err := testDB.Model("products").Ctx(ctx).
			Where("id", 1).
			Value("stock")
		t.AssertNil(err)
		originalStock := originalStockVal.Int()

		// ========================================
		// 测试全局事务回滚：模拟业务异常
		// ========================================
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "test-rollback-transaction",
			Timeout: 60000,
		}, func(ctx context.Context) error {
			return testDB.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 扣款
				_, err := tx.Model("accounts").Ctx(ctx).
					Where("user_id", 109).
					Decrement("balance", 7999.00)
				if err != nil {
					return err
				}

				// 减库存
				_, err = tx.Model("products").Ctx(ctx).
					Where("id", 1).
					Decrement("stock", 1)
				if err != nil {
					return err
				}

				// 模拟业务异常，触发回滚
				return fmt.Errorf("业务处理失败，触发回滚")
			})
		})
		t.AssertNE(err, nil) // 应该有错误

		// ========================================
		// 验证数据已回滚
		// ========================================
		balance, err := testDB.Model("accounts").Ctx(ctx).
			Where("user_id", 109).
			Value("balance")
		t.AssertNil(err)
		t.AssertEQ(balance.Float64(), originalBalance) // 余额未变

		stock, err := testDB.Model("products").Ctx(ctx).
			Where("id", 1).
			Value("stock")
		t.AssertNil(err)
		t.AssertEQ(stock.Int(), originalStock) // 库存未变
	})
}

// ============================================================================
// 环境检查测试
// ============================================================================

// TestIntegration_EnvironmentCheck 测试环境检查
// 验证 MySQL 和 Seata Server 是否正常运行
func TestIntegration_EnvironmentCheck(t *testing.T) {

	gtest.C(t, func(t *gtest.T) {
		// 测试数据库连接
		count, err := testDB.Model("accounts").Ctx(ctx).Count()
		t.AssertNil(err)
		t.AssertGT(count, 0)

		// 测试查询
		var result gdb.Record
		err = testDB.Model("accounts").Ctx(ctx).Where("user_id", 1).Scan(&result)
		if err == nil {
			t.AssertGT(len(result), 0)
		}

		// 验证 undo_log 表存在
		count, err = testDB.Model("undo_log").Ctx(ctx).Count()
		t.AssertNil(err)
	})
}
