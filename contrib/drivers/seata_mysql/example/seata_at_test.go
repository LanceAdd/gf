// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package example

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/test/gtest"
	"github.com/seata/seata-go/pkg/tm"
)

// TestSeataAT_BasicTransfer AT模式基础转账测试
func TestSeataAT_BasicTransfer(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		if err != nil {
			t.Fatalf("重置账户余额失败: %v", err)
		}

		// 查询转账前余额
		balance1Before, err := getAccountBalance(ctx, db, 1)
		t.AssertNil(err)
		t.AssertEQ(balance1Before, 10000.00)

		balance2Before, err := getAccountBalance(ctx, db, 2)
		t.AssertNil(err)
		t.AssertEQ(balance2Before, 5000.00)

		t.Logf("转账前余额: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		// 使用 Seata 全局事务
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "transfer-basic",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 开始本地事务
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 用户1扣款100元（使用更简洁的链式调用）
			amount := 100.00
			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", amount)
			if err != nil {
				return err
			}

			// 用户2加款100元（使用更简洁的链式调用）
			_, err = tx.Model("accounts").Where("id", 2).Increment("balance", amount)
			if err != nil {
				return err
			}

			// 提交本地事务
			err = tx.Commit()
			if err != nil {
				return err
			}

			t.Logf("✅ AT模式转账成功: 用户1 -> 用户2, 金额 %.0f", amount)
			return nil
		})
		t.AssertNil(err)

		// 验证转账后余额
		balance1After, err := getAccountBalance(ctx, db, 1)
		t.AssertNil(err)
		t.AssertEQ(balance1After, 9900.00)

		balance2After, err := getAccountBalance(ctx, db, 2)
		t.AssertNil(err)
		t.AssertEQ(balance2After, 5100.00)

		t.Logf("转账后余额: 用户1=%.2f, 用户2=%.2f", balance1After, balance2After)
		t.Logf("✅ AT模式基础转账测试通过")
	})
}

// TestSeataAT_TransferRollback AT模式转账回滚测试
func TestSeataAT_TransferRollback(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)

		// 查询初始余额
		balance1Before, err := getAccountBalance(ctx, db, 1)
		t.AssertNil(err)

		balance2Before, err := getAccountBalance(ctx, db, 2)
		t.AssertNil(err)

		t.Logf("回滚测试前余额: 用户1=%.2f, 用户2=%.2f", balance1Before, balance2Before)

		// 使用 Seata 全局事务（应该回滚）
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "transfer-rollback",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 开始本地事务
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 用户1扣款（使用更简洁的链式调用）
			amount := 200.00
			_, err = tx.Model("accounts").Where("id", 1).Decrement("balance", amount)
			if err != nil {
				return err
			}

			// 模拟转账到不存在的账户，应该失败（使用链式调用）
			_, err = tx.Model("accounts").Where("id", 999).Increment("balance", amount)

			// 预期会失败（没有匹配的行）
			t.Log("✅ 预期错误: 转账到不存在的账户")

			// 主动回滚本地事务
			tx.Rollback()

			// 返回错误使全局事务也回滚
			return errors.New("转账到不存在的账户")
		})

		// 预期失败
		if err == nil {
			t.Fatal("应该返回错误")
		}

		// 验证余额未变化
		balance1After, err := getAccountBalance(ctx, db, 1)
		t.AssertNil(err)
		if balance1After != balance1Before {
			t.Fatalf("用户1余额应该未变化: before=%.2f, after=%.2f", balance1Before, balance1After)
		}

		balance2After, err := getAccountBalance(ctx, db, 2)
		t.AssertNil(err)
		if balance2After != balance2Before {
			t.Fatalf("用户2余额应该未变化: before=%.2f, after=%.2f", balance2Before, balance2After)
		}

		t.Logf("✅ AT模式转账回滚测试通过")
	})
}

// TestSeataAT_CreateOrder AT模式下单场景测试
func TestSeataAT_CreateOrder(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetAccountBalances(ctx, db)
		t.AssertNil(err)
		err = resetProductStock(ctx, db)
		t.AssertNil(err)

		// 查询初始状态
		userID := 1
		productID := 1
		quantity := 1

		balanceBefore, err := getAccountBalance(ctx, db, userID)
		t.AssertNil(err)
		t.AssertEQ(balanceBefore, 10000.00)

		stockBefore, err := getProductStock(ctx, db, productID)
		t.AssertNil(err)
		t.AssertEQ(stockBefore, 100)

		price, err := getProductPrice(ctx, db, productID)
		t.AssertNil(err)

		totalAmount := price * float64(quantity)

		t.Logf("下单信息:")
		t.Logf("  产品ID: %d, 价格: %.2f", productID, price)
		t.Logf("  数量: %d, 总额: %.2f", quantity, totalAmount)

		// 使用 Seata 全局事务
		var orderID int64
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "create-order",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 开始本地事务
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 1. 检查库存
			type ProductStock struct {
				Stock int
			}
			var currentProduct ProductStock
			err = tx.Model("products").Where("id", productID).Scan(&currentProduct)
			if err != nil || currentProduct.Stock < quantity {
				return errors.New("库存不足")
			}

			// 2. 扣减库存（使用链式调用）
			_, err = tx.Model("products").Where("id", productID).Decrement("stock", quantity)
			if err != nil {
				return err
			}

			// 3. 扣款（使用链式调用）
			_, err = tx.Model("accounts").Where("id", userID).Decrement("balance", totalAmount)
			if err != nil {
				return err
			}

			// 4. 创建订单（使用链式调用）
			result, err := tx.Model("orders").Data(gdb.Map{
				"user_id":    userID,
				"product_id": productID,
				"amount":     totalAmount,
				"status":     "PENDING",
			}).Insert()
			if err != nil {
				return err
			}

			orderID, err = result.LastInsertId()
			if err != nil {
				return err
			}

			// 提交本地事务
			err = tx.Commit()
			if err != nil {
				return err
			}

			return nil
		})
		t.AssertNil(err)

		t.Logf("✅ AT模式下单成功, 订单ID: %d", orderID)

		// 验证结果
		balanceAfter, err := getAccountBalance(ctx, db, userID)
		t.AssertNil(err)

		stockAfter, err := getProductStock(ctx, db, productID)
		t.AssertNil(err)

		expectedBalance := balanceBefore - totalAmount
		expectedStock := stockBefore - quantity

		if balanceAfter != expectedBalance {
			t.Fatalf("余额扣减不正确: expected=%.2f, actual=%.2f", expectedBalance, balanceAfter)
		}
		if stockAfter != expectedStock {
			t.Fatalf("库存扣减不正确: expected=%d, actual=%d", expectedStock, stockAfter)
		}

		t.Logf("下单后状态:")
		t.Logf("  余额: %.2f -> %.2f", balanceBefore, balanceAfter)
		t.Logf("  库存: %d -> %d", stockBefore, stockAfter)
		t.Logf("✅ AT模式下单场景测试通过")
	})
}

// TestSeataAT_InsufficientStock AT模式库存不足测试
func TestSeataAT_InsufficientStock(t *testing.T) {
	gtest.C(t, func(t *gtest.T) {
		ctx := context.Background()
		db := setupSeataDB(t.T, "AT")

		// 重置数据
		err := resetProductStock(ctx, db)
		t.AssertNil(err)

		productID := 1
		quantity := 200 // 库存只有100，购买200应该失败

		stockBefore, err := getProductStock(ctx, db, productID)
		t.AssertNil(err)

		// 使用 Seata 全局事务（应该因库存不足而失败）
		err = tm.WithGlobalTx(ctx, &tm.GtxConfig{
			Name:    "check-stock",
			Timeout: 30 * time.Second,
		}, func(ctx context.Context) error {
			// 开始本地事务
			tx, err := db.Begin(ctx)
			if err != nil {
				return err
			}
			defer func() {
				if err != nil {
					tx.Rollback()
				}
			}()

			// 检查库存
			type Product struct {
				Stock int
			}
			var product Product
			err = tx.Model("products").Where("id", productID).Scan(&product)
			if err != nil {
				return err
			}

			// 库存不足
			if product.Stock < quantity {
				tx.Rollback()
				t.Log("✅ 预期错误: 库存不足")
				return errors.New("库存不足")
			}

			return nil
		})

		// 预期失败
		if err == nil {
			t.Fatal("应该检测到库存不足")
		}

		// 验证库存未变化
		stockAfter, err := getProductStock(ctx, db, productID)
		t.AssertNil(err)
		if stockAfter != stockBefore {
			t.Fatalf("库存应该未变化: before=%d, after=%d", stockBefore, stockAfter)
		}

		t.Logf("✅ AT模式库存不足测试通过")
	})
}
