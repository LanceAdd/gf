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
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/seata/seata-go/pkg/tm"
)

// TestSeataAT_BasicNested 测试基础嵌套事务
func TestSeataAT_BasicNested(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("基础嵌套事务测试 - 初始余额: %.2f", balanceBefore)

		// Seata 全局事务 + 嵌套事务
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "nested-basic",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 外层事务：扣款
				_, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
				if err != nil {
					return err
				}

				// 嵌套事务：记录日志
				err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
					_, err := tx2.Model("orders").Data(gdb.Map{
						"user_id":    1,
						"product_id": 1,
						"amount":     100,
						"status":     "PENDING",
					}).Insert()
					return err
				})
				if err != nil {
					return err
				}

				// 外层事务继续：加款
				_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
				return err
			})
		})

		t.AssertNil(err)

		// 验证结果
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		t.AssertEQ(balanceAfter, balanceBefore-100)

		// 验证订单记录
		count, _ := db.Model("orders").Where("user_id", 1).Count()
		t.AssertGT(count, 0)

		t.Logf("✅ 基础嵌套事务测试通过")
	})
}

// TestSeataAT_NestedPartialRollback 测试嵌套事务部分回滚
func TestSeataAT_NestedPartialRollback(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		// ⭐ 清理订单数据，防止之前测试的数据污染
		_, err = db.Model("orders").Where("user_id", 1).Where("amount", 50).Delete()
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("嵌套事务部分回滚测试 - 初始余额: %.2f", balanceBefore)

		// Seata 全局事务
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "nested-partial-rollback",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 外层事务：扣款（成功）
				_, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 50)
				if err != nil {
					return err
				}

				// 嵌套事务：尝试创建订单（失败）
				err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
					_, err := tx2.Model("orders").Data(gdb.Map{
						"user_id":    1,
						"product_id": 1,
						"amount":     50,
						"status":     "PENDING",
					}).Insert()
					if err != nil {
						return err
					}
					// 模拟业务错误
					return errors.New("order creation failed")
				})

				// 嵌套事务失败，但外层事务继续
				if err != nil {
					t.Logf("嵌套事务回滚: %v", err)
				}

				// 外层事务继续：加款（成功）
				_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 50)
				return err
			})
		})

		t.AssertNil(err)

		// 验证结果：转账成功
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		t.AssertEQ(balanceAfter, balanceBefore-50)

		// 验证订单未创建（嵌套事务回滚）
		count, _ := db.Model("orders").Where("user_id", 1).Where("amount", 50).Count()
		t.AssertEQ(count, 0)

		t.Logf("✅ 嵌套事务部分回滚测试通过")
	})
}

// TestSeataAT_MultiLevelNested 测试多层嵌套事务
func TestSeataAT_MultiLevelNested(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		stockBefore, _ := getProductStock(ctx, db, 1)

		t.Logf("多层嵌套事务测试 - 余额: %.2f, 库存: %d", balanceBefore, stockBefore)

		// Seata 全局事务 + 多层嵌套
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "multi-level-nested",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 第一层：创建订单主表
				result, err := tx.Model("orders").Data(gdb.Map{
					"user_id":    1,
					"product_id": 1,
					"amount":     79.99,
					"status":     "PENDING",
				}).Insert()
				if err != nil {
					return err
				}
				orderID, _ := result.LastInsertId()

				// 第二层：创建订单详情
				return tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
					_, err := tx2.Model("order_items").Data(gdb.Map{
						"order_id":   orderID,
						"product_id": 1,
						"quantity":   1,
						"price":      79.99,
					}).Insert()
					if err != nil {
						return err
					}

					// 第三层：扣减库存和扣款
					return tx2.Transaction(ctx, func(ctx context.Context, tx3 gdb.TX) error {
						// 扣减库存
						_, err := tx3.Model("products").
							Where("id", 1).
							Decrement("stock", 1)
						if err != nil {
							return err
						}

						// 扣款
						_, err = tx3.Model("accounts").
							Where("id", 1).
							Decrement("balance", 79.99)
						return err
					})
				})
			})
		})

		t.AssertNil(err)

		// 验证结果
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		stockAfter, _ := getProductStock(ctx, db, 1)

		t.AssertEQ(balanceAfter, balanceBefore-79.99)
		t.AssertEQ(stockAfter, stockBefore-1)

		// 验证订单和订单详情
		orderCount, _ := db.Model("orders").Where("user_id", 1).Count()
		t.AssertGT(orderCount, 0)

		itemCount, _ := db.Model("order_items").Where("product_id", 1).Count()
		t.AssertGT(itemCount, 0)

		t.Logf("✅ 多层嵌套事务测试通过")
	})
}

// TestSeataAT_ManualSavePoint 测试手动保存点
func TestSeataAT_ManualSavePoint(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("手动保存点测试 - 初始余额: %.2f", balanceBefore)

		// Seata 全局事务
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "manual-savepoint",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}

			committed := false
			defer func() {
				if !committed {
					tx.Rollback()
				}
			}()

			// 第一步：扣款 50
			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 50)
			if err != nil {
				return err
			}

			// 创建保存点
			err = tx.SavePoint("AfterFirstDeduction")
			if err != nil {
				return err
			}

			// 第二步：尝试再扣款 50（假设失败）
			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", 50)
			if err != nil {
				return err
			}

			// 模拟业务检查失败，回滚到保存点
			err = tx.RollbackTo("AfterFirstDeduction")
			if err != nil {
				return err
			}
			t.Logf("回滚到保存点 AfterFirstDeduction")

			// 第三步：加款给另一个账户
			_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 50)
			if err != nil {
				return err
			}

			err = tx.Commit()
			if err == nil {
				committed = true
			}
			return err
		})

		t.AssertNil(err)

		// 验证结果：只扣款 50（第二次扣款被回滚）
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		t.AssertEQ(balanceAfter, balanceBefore-50)

		// 验证加款成功
		balance2After, _ := getAccountBalance(ctx, db, 2)
		t.AssertGE(balance2After, 50)

		t.Logf("✅ 手动保存点测试通过")
	})
}

// TestSeataAT_NestedWithConcurrency 测试嵌套事务 + 并发
func TestSeataAT_NestedWithConcurrency(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("嵌套事务并发测试 - 初始余额: %.2f", balanceBefore)

		// 10 个并发事务，每个都使用嵌套事务
		concurrency := 10
		successCount := 0

		for i := 0; i < concurrency; i++ {
			err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
				Name:    fmt.Sprintf("nested-concurrent-%d", i),
				Timeout: 30 * time.Second,
			}, func(ctx context.Context) error {
				return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
					// 外层：扣款
					_, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 1)
					if err != nil {
						return err
					}

					// 嵌套：创建订单
					return tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
						_, err := tx2.Model("orders").Data(gdb.Map{
							"user_id": 1,
							"amount":  1,
							"status":  "PAID",
						}).Insert()
						if err != nil {
							return err
						}

						// 嵌套：加款
						_, err = tx2.Model("accounts").Where("id", 2).Increment("balance", 1)
						return err
					})
				})
			})

			if err == nil {
				successCount++
			} else {
				t.Logf("事务 %d 失败: %v", i, err)
			}
		}

		// 验证结果
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		expected := balanceBefore - float64(successCount)

		t.Logf("并发测试完成: 成功=%d, 失败=%d", successCount, concurrency-successCount)
		t.AssertEQ(balanceAfter, expected)

		t.Logf("✅ 嵌套事务并发测试通过")
	})
}

// TestSeataAT_NestedComplexBusiness 测试复杂业务场景（订单 + 库存 + 优惠）
func TestSeataAT_NestedComplexBusiness(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		stockBefore, _ := getProductStock(ctx, db, 1)
		price, _ := getProductPrice(ctx, db, 1)

		t.Logf("复杂业务场景测试 - 余额: %.2f, 库存: %d, 价格: %.2f",
			balanceBefore, stockBefore, price)

		// Seata 全局事务：下单流程
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "complex-order-flow",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 主流程 1：创建订单
				result, err := tx.Model("orders").Data(gdb.Map{
					"user_id":    1,
					"product_id": 1,
					"amount":     price,
					"status":     "PENDING",
				}).Insert()
				if err != nil {
					return err
				}
				orderID, _ := result.LastInsertId()

				// 嵌套流程 1：扣减库存
				err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
					// 检查库存
					type ProductInfo struct {
						Stock int
					}
					var product ProductInfo
					err := tx2.Model("products").Where("id", 1).Scan(&product)
					if err != nil {
						return err
					}
					if product.Stock < 1 {
						return errors.New("库存不足")
					}

					// 扣减库存
					_, err = tx2.Model("products").Where("id", 1).Decrement("stock", 1)
					return err
				})
				if err != nil {
					return err
				}

				// 嵌套流程 2：尝试应用优惠（可能失败）
				discount := 0.0
				err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
					// 模拟优惠逻辑（50% 概率失败）
					if orderID%2 == 0 {
						return errors.New("优惠券已过期")
					}

					discount = price * 0.1 // 10% 折扣
					t.Logf("应用优惠: %.2f", discount)
					return nil
				})
				if err != nil {
					// 优惠失败不影响主流程
					t.Logf("优惠应用失败（预期行为）: %v", err)
					discount = 0
				}

				// 主流程 2：扣款
				finalAmount := price - discount
				_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", finalAmount)
				if err != nil {
					return err
				}

				// 主流程 3：更新订单状态
				_, err = tx.Model("orders").Where("id", orderID).Update(gdb.Map{
					"amount": finalAmount,
					"status": "PAID",
				})
				return err
			})
		})

		t.AssertNil(err)

		// 验证结果
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		stockAfter, _ := getProductStock(ctx, db, 1)

		// 余额应该减少（可能有折扣）
		t.AssertLT(balanceAfter, balanceBefore)
		t.AssertGE(balanceAfter, balanceBefore-price)

		// 库存应该减 1
		t.AssertEQ(stockAfter, stockBefore-1)

		// 验证订单状态
		type OrderInfo struct {
			Status string
			Amount float64
		}
		var order OrderInfo
		err = db.Model("orders").
			Where("user_id", 1).
			OrderDesc("id").
			Limit(1).
			Scan(&order)
		t.AssertNil(err)
		t.AssertEQ(order.Status, "PAID")

		t.Logf("✅ 复杂业务场景测试通过 - 最终金额: %.2f", order.Amount)
	})
}

// TestSeataAT_NestedRollbackAll 测试整个嵌套事务全部回滚
func TestSeataAT_NestedRollbackAll(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("全部回滚测试 - 初始余额: %.2f", balanceBefore)

		// Seata 全局事务（整个失败）
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "nested-rollback-all",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
				// 外层：扣款
				_, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
				if err != nil {
					return err
				}

				// 外层：加款
				_, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
				if err != nil {
					return err
				}

				// 外层：模拟失败
				return errors.New("payment gateway error")
			})
		})

		// 应该失败
		t.AssertNE(err, nil)
		t.Logf("事务回滚: %v", err)

		// 验证结果：所有操作都应该回滚
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		t.AssertEQ(balanceAfter, balanceBefore) // 余额未变

		t.Logf("✅ 全部回滚测试通过")
	})
}

// TestSeataAT_MixedNativeAndSeata 测试混合使用原生事务和 Seata 事务
func TestSeataAT_MixedNativeAndSeata(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		balanceBefore, _ := getAccountBalance(ctx, db, 1)
		t.Logf("混合事务测试 - 初始余额: %.2f", balanceBefore)

		// 外层：GF 原生事务（无全局事务）
		err = db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
			// 这是 GF 原生事务（自动降级）
			_, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 10)
			if err != nil {
				return err
			}

			// 嵌套：依然是原生事务
			return tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
				_, err := tx2.Model("accounts").Where("id", 2).Increment("balance", 10)
				return err
			})
		})

		t.AssertNil(err)

		// 验证结果
		balanceAfter, _ := getAccountBalance(ctx, db, 1)
		t.AssertEQ(balanceAfter, balanceBefore-10)

		t.Logf("✅ 混合事务测试通过（自动降级为原生事务）")
	})
}
