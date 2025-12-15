# Seata 嵌套事务完整指南

## 📋 目录

1. [功能概述](#功能概述)
2. [实现原理](#实现原理)
3. [使用示例](#使用示例)
4. [问题修复记录](#问题修复记录)
5. [测试验证](#测试验证)
6. [最佳实践](#最佳实践)

---

## 功能概述

### ✅ 核心结论

**Seata 事务完全支持嵌套事务**，功能包括：

1. ✅ **基础嵌套事务** - Seata 全局事务内支持 GF 嵌套事务
2. ✅ **多层嵌套** - 支持多达 3-4 层的嵌套事务
3. ✅ **部分回滚** - 嵌套事务回滚不影响外层事务
4. ✅ **手动保存点** - 支持 `SavePoint()` 和 `RollbackTo()`
5. ✅ **混合使用** - 可以在 Seata 事务和原生事务间自由嵌套
6. ✅ **并发安全** - 嵌套事务在并发场景下工作正常

---

## 实现原理

### 1. GF 原生事务嵌套机制

GF 使用 **SAVEPOINT (保存点)** 机制实现嵌套事务：

```go
// gdb_core_txcore.go

// Begin 开始嵌套事务
func (tx *TXCore) Begin() error {
    _, err := tx.Exec("SAVEPOINT " + tx.transactionKeyForNestedPoint())
    if err != nil {
        return err
    }
    tx.transactionCount++  // 嵌套计数器 +1
    return nil
}

// Commit 提交嵌套事务
func (tx *TXCore) Commit() error {
    if tx.transactionCount > 0 {
        tx.transactionCount--
        // 嵌套事务提交 → 释放保存点
        _, err := tx.Exec("RELEASE SAVEPOINT " + tx.transactionKeyForNestedPoint())
        return err
    }
    // 最外层事务提交 → 真正的 COMMIT
    return tx.db.Commit()
}

// Rollback 回滚嵌套事务
func (tx *TXCore) Rollback() error {
    if tx.transactionCount > 0 {
        tx.transactionCount--
        // 嵌套事务回滚 → 回滚到保存点
        _, err := tx.Exec("ROLLBACK TO SAVEPOINT " + tx.transactionKeyForNestedPoint())
        return err
    }
    // 最外层事务回滚 → 真正的 ROLLBACK
    return tx.db.Rollback()
}
```

### 2. Seata 事务嵌套实现

**核心机制**：`SeataTX` 嵌入 `gdb.TX`，自动继承嵌套事务能力

```go
type SeataTX struct {
    gdb.TX               // ⭐ 嵌入 GF 原生事务接口
    resource         *Resource
    ctx              context.Context
    undoLogItems     *garray.Array   // 所有嵌套层级共享
    localTxId        string
    branchId         int64
    nestingLevel     int             // 嵌套层级计数器
    savepointMarkers []int           // Undo Log 保存点标记
}
```

**重写 Transaction() 方法**：

```go
func (tx *SeataTX) Transaction(ctx context.Context, f func(ctx context.Context, tx gdb.TX) error) error {
    // 1. 确保 context 中包含当前 SeataTX
    if gdb.TXFromCtx(tx.ctx, tx.GetDB().GetGroup()) != tx {
        tx.ctx = gdb.WithTX(tx.ctx, tx)
    }
    
    // 2. 记录当前的 undo log 数量（作为保存点）
    undoLogCount := 0
    if tx.undoLogItems != nil {
        undoLogCount = tx.undoLogItems.Len()
    }
    
    // 3. 开始 SAVEPOINT
    savepointName := fmt.Sprintf(SavepointFormat, tx.nestingLevel)
    _, err := tx.Exec("SAVEPOINT " + savepointName)
    if err != nil {
        return fmt.Errorf(ErrCreateSavepoint, savepointName, err)
    }
    tx.nestingLevel++
    
    // 4. 执行嵌套事务逻辑
    err = f(tx.ctx, tx)
    
    // 5. 根据结果决定提交或回滚 SAVEPOINT
    if err != nil {
        tx.nestingLevel--
        
        // 回滚数据库到 SAVEPOINT
        _, rollbackErr := tx.Exec("ROLLBACK TO SAVEPOINT " + savepointName)
        if rollbackErr != nil {
            return fmt.Errorf(ErrRollbackToSavepoint, savepointName, rollbackErr, err)
        }
        
        // ⭐ 关键：回滚 Undo Log 到保存点
        if tx.undoLogItems != nil && tx.undoLogItems.Len() > undoLogCount {
            rolledBackCount := tx.undoLogItems.Len() - undoLogCount
            
            // 删除在这个嵌套事务中添加的 undo log
            newArray := garray.New(true)
            for i := 0; i < undoLogCount; i++ {
                newArray.Append(tx.undoLogItems.Get(i))
            }
            tx.undoLogItems = newArray
        }
        
        return err
    }
    
    // 成功则释放 SAVEPOINT
    tx.nestingLevel--
    _, err = tx.Exec("RELEASE SAVEPOINT " + savepointName)
    return err
}
```

### 3. 关键设计要点

| 设计要点 | 说明 |
|---------|------|
| **共享 Undo Log 数组** | 所有嵌套层级的 Undo Log 收集到同一个 `undoLogItems` 数组 |
| **保存点标记** | 在嵌套事务开始前记录当前 Undo Log 数量 |
| **双重回滚** | 嵌套事务失败时：① 回滚数据库到 SAVEPOINT ② 回滚 Undo Log 到保存点 |
| **Context 传递** | 确保嵌套事务中的 SQL 操作能正确获取到 `SeataTX` 对象 |

---

## 使用示例

### 示例 1：基础嵌套事务

```go
func TestBasicNested(t *testing.T) {
    ctx := context.Background()
    db := setupSeataDB(t, "AT")
    
    // Seata 全局事务
    err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
        Name:    "basic-nested",
        Timeout: 30 * time.Second,
    }, func(ctx context.Context) error {
        return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
            // 外层事务：扣款
            _, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 100)
            if err != nil {
                return err
            }
            
            // 嵌套事务：创建订单
            err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
                _, err := tx2.Model("orders").Data(gdb.Map{
                    "user_id": 1,
                    "amount":  100,
                    "status":  "PAID",
                }).Insert()
                return err
            })
            if err != nil {
                return err
            }
            
            // 外层事务：加款
            _, err = tx.Model("accounts").Where("id", 2).Increment("balance", 100)
            return err
        })
    })
    
    // 验证结果
    if err != nil {
        t.Errorf("Transaction failed: %v", err)
    }
}
```

### 示例 2：嵌套事务部分回滚

```go
func TestPartialRollback(t *testing.T) {
    err := tm.WithGlobalTx(ctx, &tm.GtxConfig{
        Name: "partial-rollback",
    }, func(ctx context.Context) error {
        return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
            // 外层：扣款（成功）
            _, err := tx.Model("accounts").Where("id", 1).Decrement("balance", 50)
            if err != nil {
                return err
            }
            
            // 嵌套：创建订单（失败）
            err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
                _, err := tx2.Model("orders").Data(gdb.Map{
                    "user_id": 1,
                    "amount":  50,
                }).Insert()
                if err != nil {
                    return err
                }
                // 人为返回错误
                return errors.New("order creation failed")
            })
            
            // ⭐ 忽略嵌套事务错误，继续执行
            if err != nil {
                log.Println("Nested transaction failed:", err)
            }
            
            // 外层：加款（成功）
            _, err = tx.Model("accounts").Where("id", 2).Increment("balance", 50)
            return err
        })
    })
    
    // 结果：账户余额变化了，但订单没有创建（嵌套事务已回滚）
}
```

### 示例 3：多层嵌套事务

```go
func TestMultiLevelNested(t *testing.T) {
    err := db.Transaction(ctx, func(ctx context.Context, tx1 gdb.TX) error {
        // 第 1 层：扣款
        _, err := tx1.Model("accounts").Where("id", 1).Decrement("balance", 100)
        if err != nil {
            return err
        }
        
        // 第 2 层：创建订单
        err = tx1.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
            _, err := tx2.Model("orders").Data(...).Insert()
            if err != nil {
                return err
            }
            
            // 第 3 层：创建订单详情
            return tx2.Transaction(ctx, func(ctx context.Context, tx3 gdb.TX) error {
                _, err := tx3.Model("order_items").Data(...).Insert()
                return err
            })
        })
        
        return err
    })
}
```

---

## 问题修复记录

### 问题 1：嵌套事务获取不到 SeataTX

**错误信息**：
```
transaction is not a SeataTX, actual type: *gdb.TXCore
```

**原因**：
- GF 的 `TXCore.Transaction()` 内部会调用 `gdb.WithTX()` 把自己（`TXCore`）放入 context
- 这会覆盖我们之前设置的 `SeataTX`

**修复方案**：
重写 `SeataTX.Transaction()` 方法，在回调函数中重新把 `SeataTX` 放回 context：

```go
// 确保 context 中包含当前 SeataTX
if gdb.TXFromCtx(tx.ctx, tx.GetDB().GetGroup()) != tx {
    tx.ctx = gdb.WithTX(tx.ctx, tx)
}
```

### 问题 2：嵌套事务回滚时 Undo Log 未回滚

**现象**：
虽然嵌套事务执行了 `ROLLBACK TO SAVEPOINT`，但 Undo Log 仍然被收集并最终提交。

**原因**：
`ROLLBACK TO SAVEPOINT` 只撤销数据库的修改，不会影响内存中的 `undoLogItems` 数组。

**修复方案**：
在嵌套事务回滚时，同时回滚 Undo Log：

```go
// ⭐ 关键：回滚 Undo Log 到保存点
if tx.undoLogItems != nil && tx.undoLogItems.Len() > undoLogCount {
    // 记录回滚的数量
    rolledBackCount := tx.undoLogItems.Len() - undoLogCount
    
    // 删除在这个嵌套事务中添加的 undo log
    newArray := garray.New(true)
    for i := 0; i < undoLogCount; i++ {
        newArray.Append(tx.undoLogItems.Get(i))
    }
    tx.undoLogItems = newArray
}
```

---

## 测试验证

### 测试用例列表

| # | 测试用例 | 状态 | 说明 |
|---|---------|------|------|
| 1 | `TestSeataAT_BasicNested` | ✅ | 基础嵌套事务 |
| 2 | `TestSeataAT_NestedPartialRollback` | ✅ | 部分回滚 |
| 3 | `TestSeataAT_MultiLevelNested` | ✅ | 多层嵌套（3层） |
| 4 | `TestSeataAT_NestedBusiness` | ✅ | 嵌套业务流程 |
| 5 | `TestSeataAT_NestedWithConcurrency` | ✅ | 并发嵌套 |
| 6 | `TestSeataAT_NestedComplexBusiness` | ✅ | 复杂业务场景 |
| 7 | `TestSeataAT_NestedRollbackAll` | ✅ | 全部回滚 |

**通过率**: 100% (7/7)

### 测试结果

```bash
=== RUN   TestSeataAT_BasicNested
    seata_nested_transaction_test.go:76: ✅ 基础嵌套事务测试通过
--- PASS: TestSeataAT_BasicNested (0.04s)

=== RUN   TestSeataAT_NestedPartialRollback
    seata_nested_transaction_test.go:143: ✅ 嵌套事务部分回滚测试通过
--- PASS: TestSeataAT_NestedPartialRollback (0.05s)

=== RUN   TestSeataAT_MultiLevelNested
    seata_nested_transaction_test.go:234: ✅ 多层嵌套事务测试通过
--- PASS: TestSeataAT_MultiLevelNested (0.06s)

... (其他测试)

PASS
ok      github.com/gogf/gf/contrib/drivers/seata_mysql/v2/example    0.350s
```

---

## 最佳实践

### 1. 何时使用嵌套事务

✅ **适合使用的场景**：
- 需要独立回滚某个操作，但不影响外层事务
- 复杂业务流程，需要分步处理
- 可选操作（失败不影响主流程）

❌ **不适合使用的场景**：
- 简单的顺序操作（直接用单层事务即可）
- 性能敏感的高并发场景（SAVEPOINT 有开销）

### 2. 错误处理

**正确做法**：
```go
// 外层事务
err := tx.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
    // 关键操作
    _, err := tx.Model("accounts").Decrement("balance", 100)
    if err != nil {
        return err
    }
    
    // 可选操作（嵌套事务）
    err = tx.Transaction(ctx, func(ctx context.Context, tx2 gdb.TX) error {
        _, err := tx2.Model("logs").Insert(...)
        return err
    })
    if err != nil {
        // ⭐ 记录日志，但不中断外层事务
        log.Println("Failed to insert log:", err)
    }
    
    return nil
})
```

### 3. 性能考虑

- 嵌套层级不要超过 3-4 层
- 嵌套事务尽量短小精悍
- 避免在嵌套事务中执行大量 SQL

### 4. Undo Log 管理

- 所有嵌套层级的 Undo Log 会合并到一起
- 嵌套事务回滚时，Undo Log 会自动回滚到保存点
- 最终提交时，所有 Undo Log 会一起插入数据库

---

## 总结

Seata MySQL Driver 通过以下机制实现了完整的嵌套事务支持：

1. **继承 GF 原生能力** - 嵌入 `gdb.TX` 自动获得 SAVEPOINT 支持
2. **重写 Transaction() 方法** - 确保嵌套事务继续使用 `SeataTX` 对象
3. **双重回滚机制** - 数据库回滚 + Undo Log 回滚
4. **Context 正确传递** - 保证 SQL 拦截器能获取到 `SeataTX`

通过 7 个测试用例的全面验证，嵌套事务功能稳定可靠，可以放心使用。
