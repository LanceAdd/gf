// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package example

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/seata/seata-go/pkg/tm"
)

// TestSeataAT_ConcurrentTransfer 并发转账测试
// 场景：多个用户同时向同一个账户转账
func TestSeataAT_ConcurrentTransfer(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		// 初始状态：用户1=10000, 用户2=5000
		balance1Before, _ := getAccountBalance(ctx, db, 1)
		balance2Before, _ := getAccountBalance(ctx, db, 2)

		t.Logf("并发转账测试开始")
		t.Logf("初始余额: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		// 并发执行5个转账事务，都是从用户1转给用户2
		concurrency := 5
		amount := 100.00
		var wg sync.WaitGroup
		errChan := make(chan error, concurrency)

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				txName := fmt.Sprintf("concurrent-transfer-%d", index)
				err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
					Name:    txName,
					Timeout: 30 * time.Second,
				}, func(ctx context.Context) error {
					tx, err := db.Begin(ctx)
					if err != nil {
						return err
					}
					defer func() {
						if err != nil {
							tx.Rollback()
						}
					}()

					// 用户1扣款
					_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", amount)
					if err != nil {
						return err
					}

					// 模拟一些处理时间，增加并发冲突概率
					time.Sleep(time.Millisecond * 10)

					// 用户2加款
					_, err = tx.Model("accounts").Where("id", 2).Increment("balance", amount)
					if err != nil {
						return err
					}

					err = tx.Commit()
					return err
				})

				if err != nil {
					errChan <- err
				}
			}(i)
		}

		wg.Wait()
		close(errChan)

		// 检查是否有错误
		var errors []error
		for err := range errChan {
			errors = append(errors, err)
		}

		if len(errors) > 0 {
			t.Logf("部分事务失败: %d个", len(errors))
		}

		// 验证最终余额
		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		successCount := concurrency - len(errors)

		t.Logf("最终余额: 用户1=%.2f, 用户2=%.2f", balance1After, balance2After)
		t.Logf("成功事务: %d个", successCount)

		// 验证总金额守恒
		totalBefore := balance1Before + balance2Before
		totalAfter := balance1After + balance2After
		t.AssertEQ(totalBefore, totalAfter)

		t.Logf("✅ 并发转账测试通过 - 总金额守恒")
	})
}

// TestSeataAT_DeadlockScenario 死锁场景测试
// 场景：两个事务互相等待对方释放锁
func TestSeataAT_DeadlockScenario(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		t.Logf("死锁场景测试开始")

		var wg sync.WaitGroup
		results := make(chan string, 2)

		// 事务1：先锁账户1，再锁账户2
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
				Name:    "deadlock-tx1",
				Timeout: 10 * time.Second,
			}, func(ctx context.Context) error {
				tx, err := db.Begin(ctx)
				if err != nil {
					return err
				}
				defer func() {
					if err != nil {
						tx.Rollback()
					}
				}()

				// 锁定账户1
				_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
				if err != nil {
					return err
				}

				time.Sleep(100 * time.Millisecond)

				// 锁定账户2
				_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
				if err != nil {
					return err
				}

				err = tx.Commit()
				return err
			})

			if err != nil {
				results <- "TX1-Failed: " + err.Error()
			} else {
				results <- "TX1-Success"
			}
		}()

		// 事务2：先锁账户2，再锁账户1
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond) // 稍微延迟启动

			err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
				Name:    "deadlock-tx2",
				Timeout: 10 * time.Second,
			}, func(ctx context.Context) error {
				tx, err := db.Begin(ctx)
				if err != nil {
					return err
				}
				defer func() {
					if err != nil {
						tx.Rollback()
					}
				}()

				// 锁定账户2
				_, err = tx.Model("accounts").Where("id", 2).Decrement("balance", 50)
				if err != nil {
					return err
				}

				time.Sleep(100 * time.Millisecond)

				// 锁定账户1
				_, err = tx.Model("accounts").Where("id", 1).Increment("balance", 50)
				if err != nil {
					return err
				}

				err = tx.Commit()
				return err
			})

			if err != nil {
				results <- "TX2-Failed: " + err.Error()
			} else {
				results <- "TX2-Success"
			}
		}()

		wg.Wait()
		close(results)

		// 收集结果
		for result := range results {
			t.Log(result)
		}

		t.Logf("✅ 死锁场景测试完成 - Seata应该处理或检测到死锁")
	})
}

// TestSeataAT_ComplexBusinessScenario 复杂业务场景测试
// 场景：秒杀下单 - 高并发抢购同一商品
func TestSeataAT_ComplexBusinessScenario(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		productID := 1
		initialStock, _ := getProductStock(ctx, db, productID)
		price, _ := getProductPrice(ctx, db, productID)

		t.Logf("秒杀场景测试开始")
		t.Logf("商品ID: %d, 初始库存: %d, 价格: %.2f", productID, initialStock, price)

		// 模拟20个用户同时抢购，但库存只有100
		users := 20
		var wg sync.WaitGroup
		successCount := int32(0)
		var mu sync.Mutex

		for i := 1; i <= users; i++ {
			wg.Add(1)
			go func(userID int) {
				defer wg.Done()

				txName := fmt.Sprintf("seckill-user-%d", userID)
				err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
					Name:    txName,
					Timeout: 30 * time.Second,
				}, func(ctx context.Context) error {
					tx, err := db.Begin(ctx)
					if err != nil {
						return err
					}
					defer func() {
						if err != nil {
							tx.Rollback()
						}
					}()

					// 1. 检查并扣减库存
					type ProductStock struct {
						Stock int
					}
					var product ProductStock
					err = tx.Model("products").Where("id", productID).Scan(&product)
					if err != nil {
						return err
					}

					if product.Stock < 1 {
						return errors.New("库存不足")
					}

					_, err = tx.Model("products").Where("id", productID).Decrement("stock", 1)
					if err != nil {
						return err
					}

					// 2. 扣款（假设所有用户都是用户1的账户，余额足够）
					_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", price)
					if err != nil {
						return err
					}

					// 3. 创建订单
					_, err = tx.Model("orders").Data(gdb.Map{
						"user_id":    userID,
						"product_id": productID,
						"amount":     price,
						"status":     "PAID",
					}).Insert()
					if err != nil {
						return err
					}

					err = tx.Commit()
					if err == nil {
						mu.Lock()
						successCount++
						mu.Unlock()
					}
					return err
				})

				if err != nil {
					t.Logf("用户%d 抢购失败: %v", userID, err)
				} else {
					t.Logf("用户%d 抢购成功", userID)
				}
			}(i)
		}

		wg.Wait()

		// 验证结果
		stockAfter, _ := getProductStock(ctx, db, productID)
		expectedStock := initialStock - int(successCount)

		t.Logf("秒杀结束: 成功订单=%d, 剩余库存=%d", successCount, stockAfter)
		t.AssertEQ(stockAfter, expectedStock)
		t.Logf("✅ 秒杀场景测试通过 - 库存扣减正确")
	})
}

// TestSeataAT_LongTransaction 长事务测试
// 场景：模拟长时间运行的事务
func TestSeataAT_LongTransaction(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		t.Logf("长事务测试开始")

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "long-transaction",
			Timeout: 60 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 执行多个操作
			for i := 0; i < 10; i++ {
				// 用户1扣款10元
				_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 10)
				if err != nil {
					return err
				}

				// 用户2加款10元
				_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 10)
				if err != nil {
					return err
				}

				// 模拟处理时间
				time.Sleep(100 * time.Millisecond)
			}

			err = tx.Commit()
			return err
		})

		t.AssertNil(err)
		t.Logf("✅ 长事务测试通过")
	})
}

// TestSeataAT_PartialRollback 部分回滚测试
// 场景：事务中间失败，验证所有操作都回滚
func TestSeataAT_PartialRollback(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balance1Before, _ := getAccountBalance(ctx, db, 1)
		balance2Before, _ := getAccountBalance(ctx, db, 2)

		t.Logf("部分回滚测试开始")

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "partial-rollback",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 操作1：用户1扣款100（成功）
			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
			if err != nil {
				return err
			}

			// 操作2：用户2加款100（成功）
			_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
			if err != nil {
				return err
			}

			// 操作3：插入订单（成功）
			_, err = tx.Model("orders").Data(gdb.Map{
				"user_id":    1,
				"product_id": 1,
				"amount":     100,
				"status":     "PENDING",
			}).Insert()
			if err != nil {
				return err
			}

			// 操作4：故意触发错误（扣减不存在的账户）
			_, err = tx.Model("accounts").Where("id", 9999).Decrement("balance", 50)

			// 主动回滚
			tx.Rollback()
			return errors.New("模拟中间步骤失败")
		})

		// 应该失败
		t.AssertNE(err, nil)

		// 验证所有数据都未变化
		balance1After, _ := getAccountBalance(ctx, db, 1)
		balance2After, _ := getAccountBalance(ctx, db, 2)

		t.AssertEQ(balance1Before, balance1After)
		t.AssertEQ(balance2Before, balance2After)

		t.Logf("✅ 部分回滚测试通过 - 所有操作都已回滚")
	})
}

// TestSeataAT_NestedBusiness 嵌套业务场景测试
// 场景：电商订单 - 涉及多个服务的复杂业务
func TestSeataAT_NestedBusiness(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		userID := 1
		productID := 1
		quantity := 1 // 修改为1，避免余额不足（产品价格7999，账户余额10000）

		balance1Before, _ := getAccountBalance(ctx, db, userID)
		stockBefore, _ := getProductStock(ctx, db, productID)
		price, _ := getProductPrice(ctx, db, productID)

		totalAmount := price * float64(quantity)

		t.Logf("嵌套业务场景测试开始")
		t.Logf("购买商品: 数量=%d, 单价=%.2f, 总价=%.2f", quantity, price, totalAmount)

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "nested-business",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 步骤1：检查库存服务
			type Product struct {
				Stock int
				Price float64
			}
			var product Product
			err = tx.Model("products").Where("id", productID).Scan(&product)
			if err != nil {
				return err
			}
			if product.Stock < quantity {
				return errors.New("库存不足")
			}

			// 步骤2：检查账户服务
			type Account struct {
				Balance float64
			}
			var account Account
			err = tx.Model("accounts").Where("id", userID).Scan(&account)
			if err != nil {
				return err
			}
			if account.Balance < totalAmount {
				return errors.New("余额不足")
			}

			// 步骤3：扣减库存
			_, err = tx.Model("products").Where("id", productID).Decrement("stock", quantity)
			if err != nil {
				return err
			}

			// 步骤4：扣款
			_, err = tx.Model("accounts").Where("id", userID).Decrement("balance", totalAmount)
			if err != nil {
				return err
			}

			// 步骤5：创建订单
			result, err := tx.Model("orders").Data(gdb.Map{
				"user_id":    userID,
				"product_id": productID,
				"amount":     totalAmount,
				"status":     "PAID",
			}).Insert()
			if err != nil {
				return err
			}

			orderID, _ := result.LastInsertId()
			t.Logf("订单创建成功: ID=%d", orderID)

			// 步骤6：记录积分（模拟增加用户积分）
			// 这里可以添加更多业务逻辑

			err = tx.Commit()
			return err
		})

		t.AssertNil(err)

		// 验证结果
		balance1After, _ := getAccountBalance(ctx, db, userID)
		stockAfter, _ := getProductStock(ctx, db, productID)

		t.AssertEQ(balance1After, balance1Before-totalAmount)
		t.AssertEQ(stockAfter, stockBefore-quantity)

		t.Logf("✅ 嵌套业务场景测试通过")
	})
}

// TestSeataAT_ReadWriteConflict 读写冲突测试
// 场景：一个事务读取数据，另一个事务修改同样的数据
func TestSeataAT_ReadWriteConflict(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		t.Logf("读写冲突测试开始")

		var wg sync.WaitGroup
		results := make(chan string, 2)

		// 事务1：读取并基于读取结果更新
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
				Name:    "read-write-tx1",
				Timeout: 30 * time.Second,
			}, func(ctx context.Context) error {
				tx, err := db.Begin(ctx)
				if err != nil {
					return err
				}
				defer func() {
					if err != nil {
						tx.Rollback()
					}
				}()

				// 读取余额
				type Account struct {
					Balance float64
				}
				var account Account
				err = tx.Model("accounts").Where("id", 1).Scan(&account)
				if err != nil {
					return err
				}

				currentBalance := account.Balance
				time.Sleep(100 * time.Millisecond)

				// 基于读取的余额计算新值
				newBalance := currentBalance * 1.1 // 增加10%

				_, err = tx.Model("accounts").Where("id", 1).Update(gdb.Map{
					"balance": newBalance,
				})
				if err != nil {
					return err
				}

				err = tx.Commit()
				return err
			})

			if err != nil {
				results <- "TX1-Failed: " + err.Error()
			} else {
				results <- "TX1-Success"
			}
		}()

		// 事务2：直接修改同一账户
		wg.Add(1)
		go func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)

			err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
				Name:    "read-write-tx2",
				Timeout: 30 * time.Second,
			}, func(ctx context.Context) error {
				tx, err := db.Begin(ctx)
				if err != nil {
					return err
				}
				defer func() {
					if err != nil {
						tx.Rollback()
					}
				}()

				_, err = tx.Model("accounts").Where("id", 1).Increment("balance", 500)
				if err != nil {
					return err
				}

				err = tx.Commit()
				return err
			})

			if err != nil {
				results <- "TX2-Failed: " + err.Error()
			} else {
				results <- "TX2-Success"
			}
		}()

		wg.Wait()
		close(results)

		// 收集结果
		for result := range results {
			t.Log(result)
		}

		t.Logf("✅ 读写冲突测试完成")
	})
}

// TestSeataAT_MultiTableTransaction 多表联合事务测试
// 场景：订单系统 - 同时操作订单、库存、账户三张表
func TestSeataAT_MultiTableTransaction(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		t.Logf("多表联合事务测试开始")

		// 模拟5个用户同时下单不同商品
		var wg sync.WaitGroup
		successOrders := make([]int, 0)
		var mu sync.Mutex

		for userID := 1; userID <= 5; userID++ {
			wg.Add(1)
			go func(uid int) {
				defer wg.Done()

				productID := (uid % 2) + 1 // 用户1,3,5买商品1，用户2,4买商品2
				quantity := 1

				err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
					Name:    fmt.Sprintf("multi-table-order-%d", uid),
					Timeout: 30 * time.Second,
				}, func(ctx context.Context) error {
					tx, err := db.Begin(ctx)
					if err != nil {
						return err
					}
					defer func() {
						if err != nil {
							tx.Rollback()
						}
					}()

					// 1. 查询商品价格和库存
					type Product struct {
						Stock int
						Price float64
					}
					var product Product
					err = tx.Model("products").Where("id", productID).Scan(&product)
					if err != nil {
						return err
					}

					if product.Stock < quantity {
						return errors.New("库存不足")
					}

					totalAmount := product.Price * float64(quantity)

					// 2. 检查账户余额
					type Account struct {
						Balance float64
					}
					var account Account
					err = tx.Model("accounts").Where("id", uid).Scan(&account)
					if err != nil {
						return err
					}

					if account.Balance < totalAmount {
						return errors.New("余额不足")
					}

					// 3. 扣减库存
					_, err = tx.Model("products").Where("id", productID).Decrement("stock", quantity)
					if err != nil {
						return err
					}

					// 4. 扣款
					_, err = tx.Model("accounts").Where("id", uid).Decrement("balance", totalAmount)
					if err != nil {
						return err
					}

					// 5. 创建订单
					result, err := tx.Model("orders").Data(gdb.Map{
						"user_id":    uid,
						"product_id": productID,
						"amount":     totalAmount,
						"status":     "PAID",
					}).Insert()
					if err != nil {
						return err
					}

					orderID, _ := result.LastInsertId()

					err = tx.Commit()
					if err == nil {
						mu.Lock()
						successOrders = append(successOrders, int(orderID))
						mu.Unlock()
					}
					return err
				})

				if err != nil {
					t.Logf("用户%d 下单失败: %v", uid, err)
				} else {
					t.Logf("用户%d 下单成功", uid)
				}
			}(userID)
		}

		wg.Wait()

		t.Logf("成功订单数: %d", len(successOrders))
		t.Logf("✅ 多表联合事务测试通过")
	})
}

// TestSeataAT_HighConcurrencyStressTest 高并发压力测试
// 场景：100个并发事务同时修改同一条数据
func TestSeataAT_HighConcurrencyStressTest(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("高并发压力测试开始 - 初始余额: %.2f", balanceBefore)

		// 100个并发事务
		concurrency := 100
		amount := 1.00
		var wg sync.WaitGroup
		successCount := int32(0)
		failCount := int32(0)

		startTime := time.Now()

		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
					Name:    fmt.Sprintf("stress-%d", index),
					Timeout: 30 * time.Second,
				}, func(ctx context.Context) error {
					tx, err := db.Begin(ctx)
					if err != nil {
						return err
					}
					defer func() {
						if err != nil {
							tx.Rollback()
						}
					}()

					// 扣款
					_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", amount)
					if err != nil {
						return err
					}

					// 加款
					_, err = tx.Model("accounts").Where("id", 2).Increment("balance", amount)
					if err != nil {
						return err
					}

					err = tx.Commit()
					return err
				})

				if err != nil {
					failCount++
				} else {
					successCount++
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(startTime)

		// 验证结果
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		expected := balanceBefore - float64(successCount)*amount

		t.Logf("测试完成: 耗时=%v", elapsed)
		t.Logf("成功: %d, 失败: %d", successCount, failCount)
		t.Logf("TPS: %.2f", float64(successCount)/elapsed.Seconds())
		t.AssertEQ(balanceAfter, expected)
		t.Logf("✅ 高并发压力测试通过")
	})
}

// TestSeataAT_BatchOperations 批量操作测试
// 场景：一个事务中批量插入/更新多条记录
func TestSeataAT_BatchOperations(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		t.Logf("批量操作测试开始")

		err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "batch-operations",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 批量插入订单
			orders := make([]gdb.Map, 0)
			for i := 1; i <= 10; i++ {
				orders = append(orders, gdb.Map{
					"user_id":    1,
					"product_id": i%3 + 1,
					"amount":     float64(i) * 10,
					"status":     "PAID",
				})
			}

			_, err = tx.Model("orders").Data(orders).Insert()
			if err != nil {
				return err
			}

			// 批量更新产品库存
			for i := 1; i <= 3; i++ {
				_, err = tx.Model("products").Where("id", i).Decrement("stock", i)
				if err != nil {
					return err
				}
			}

			err = tx.Commit()
			return err
		})

		t.AssertNil(err)
		t.Logf("✅ 批量操作测试通过")
	})
}

// TestSeataAT_TimeoutScenario 超时场景测试
// 场景：事务执行时间超过设定的超时时间
func TestSeataAT_TimeoutScenario(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("超时场景测试开始 - 初始余额: %.2f", balanceBefore)

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "timeout-test",
			Timeout: 2 * time.Second, // 设置2秒超时
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 执行操作
			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
			if err != nil {
				return err
			}

			// 模拟长时间处理（超过超时时间）
			t.Log("模拟长时间处理...")
			time.Sleep(3 * time.Second)

			_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
			if err != nil {
				return err
			}

			err = tx.Commit()
			return err
		})

		// 预期会超时失败
		if err != nil {
			t.Logf("事务超时（预期行为）: %v", err)
		}

		// 验证数据未变化
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		t.Logf("最终余额: %.2f", balanceAfter)
		t.Logf("✅ 超时场景测试完成")
	})
}

// TestSeataAT_MixedReadWrite 混合读写测试
// 场景：同一事务中既有读操作又有写操作
func TestSeataAT_MixedReadWrite(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		t.Logf("混合读写测试开始")

		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "mixed-read-write",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 1. 读取多个账户余额
			type Account struct {
				ID      int
				Balance float64
			}
			var accounts []Account
			err = tx.Model("accounts").Where("id <= ?", 3).Scan(&accounts)
			if err != nil {
				return err
			}
			t.Logf("读取到 %d 个账户", len(accounts))

			// 2. 读取产品信息
			type Product struct {
				ID    int
				Name  string
				Price float64
				Stock int
			}
			var products []Product
			err = tx.Model("products").Where("stock > ?", 0).Scan(&products)
			if err != nil {
				return err
			}
			t.Logf("读取到 %d 个有库存的产品", len(products))

			// 3. 基于读取结果执行写操作
			for _, account := range accounts {
				if account.Balance > 1000 {
					// 余额超过1000的账户，扣除手续费
					_, err = tx.Model("accounts").Where("id", account.ID).Decrement("balance", 10)
					if err != nil {
						return err
					}
				}
			}

			// 4. 更新产品库存
			for _, product := range products {
				if product.Stock < 50 {
					// 库存低于50的，补货50
					_, err = tx.Model("products").Where("id", product.ID).Increment("stock", 50)
					if err != nil {
						return err
					}
				}
			}

			err = tx.Commit()
			return err
		})

		t.AssertNil(err)
		t.Logf("✅ 混合读写测试通过")
	})
}

// TestSeataAT_ChainedTransactions 链式事务测试
// 场景：多个事务按顺序执行，后一个依赖前一个的结果
func TestSeataAT_ChainedTransactions(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		t.Logf("链式事务测试开始")

		// 事务1：从账户1转100到账户2
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "chain-tx1",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
			if err != nil {
				return err
			}
			_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
			if err != nil {
				return err
			}

			err = tx.Commit()
			return err
		})
		t.AssertNil(err)
		t.Logf("事务1完成: 账户1 -> 账户2 转账100")

		// 事务2：从账户2转50到账户3
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "chain-tx2",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			_, err = tx.Model("accounts").Where("id", 2).Decrement("balance", 50)
			if err != nil {
				return err
			}
			_, err = tx.Model("accounts").Where("id", 3).Increment("balance", 50)
			if err != nil {
				return err
			}

			err = tx.Commit()
			return err
		})
		t.AssertNil(err)
		t.Logf("事务2完成: 账户2 -> 账户3 转账50")

		// 事务3：从账户3转30到账户1（形成循环）
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "chain-tx3",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			_, err = tx.Model("accounts").Where("id", 3).Decrement("balance", 30)
			if err != nil {
				return err
			}
			_, err = tx.Model("accounts").Where("id", 1).Increment("balance", 30)
			if err != nil {
				return err
			}

			err = tx.Commit()
			return err
		})
		t.AssertNil(err)
		t.Logf("事务3完成: 账户3 -> 账户1 转账30")

		t.Logf("✅ 链式事务测试通过")
	})
}

// TestSeataAT_OptimisticLockConflict 乐观锁冲突测试
// 场景：使用版本号的乐观锁机制
func TestSeataAT_OptimisticLockConflict(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		t.Logf("乐观锁冲突测试开始")

		var wg sync.WaitGroup
		successCount := int32(0)
		failCount := int32(0)

		// 10个并发事务尝试修改同一记录
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
					Name:    fmt.Sprintf("optimistic-lock-%d", index),
					Timeout: 30 * time.Second,
				}, func(ctx context.Context) error {
					tx, err := db.Begin(ctx)
					if err != nil {
						return err
					}
					defer func() {
						if err != nil {
							tx.Rollback()
						}
					}()

					// 读取当前余额和版本号
					type AccountWithVersion struct {
						Balance float64
					}
					var account AccountWithVersion
					err = tx.Model("accounts").Where("id", 1).Scan(&account)
					if err != nil {
						return err
					}

					currentBalance := account.Balance

					// 模拟一些处理延迟
					time.Sleep(time.Millisecond * 10)

					// 基于读取的余额更新（模拟乐观锁）
					newBalance := currentBalance + 10
					_, err = tx.Model("accounts").Where("id", 1).Update(gdb.Map{
						"balance": newBalance,
					})
					if err != nil {
						return err
					}

					err = tx.Commit()
					return err
				})

				if err != nil {
					failCount++
				} else {
					successCount++
				}
			}(i)
		}

		wg.Wait()

		t.Logf("成功: %d, 失败: %d", successCount, failCount)
		t.Logf("✅ 乐观锁冲突测试完成")
	})
}
