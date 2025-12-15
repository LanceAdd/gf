# Seata 事务与 GF 原生事务关系说明

## 🎯 核心关系

**简单总结**：
- **Seata 事务 = GF 原生事务 + Seata 分布式事务增强**
- Seata 事务**内部使用**GF 原生事务来管理本地数据库连接
- Seata 在 GF 原生事务的基础上**增加**了分布式事务协调能力

---

## 📊 层次关系图

```
┌─────────────────────────────────────────────────────┐
│              应用层业务代码                           │
│                                                       │
│   // 方式1: 使用Seata全局事务                          │
│   tm.WithGlobalTx(ctx, config, func(ctx) error {    │
│       db.Transaction(ctx, func(ctx, tx) error {     │
│           tx.Model("users").Update(...)             │
│           return nil                                 │
│       })                                            │
│   })                                                │
│                                                       │
│   // 方式2: 使用GF原生事务                            │
│   db.Transaction(ctx, func(ctx, tx) error {         │
│       tx.Model("users").Update(...)                 │
│       return nil                                     │
│   })                                                │
└────────────────┬────────────────────────────────────┘
                 │
        ┌────────┴────────┐
        │                 │
        │ 检查：是否在全局事务中？
        │ tm.IsGlobalTx(ctx)
        │                 │
   ┌────▼─────┐      ┌───▼──────┐
   │   NO     │      │   YES    │
   │          │      │          │
   ▼          │      ▼          │
┌──────────┐  │  ┌──────────┐  │
│ GF 原生  │  │  │  Seata   │  │
│  事务    │  │  │  事务    │  │
└────┬─────┘  │  └────┬─────┘  │
     │        │       │        │
     ▼        │       ▼        │
  gdb.TX      │   SeataTX      │
  (普通事务)   │   (增强事务)    │
     │        │       │        │
     │        │       ├─ 包含 gdb.TX
     │        │       ├─ 增加 UndoLog收集
     │        │       ├─ 增加 分支注册
     │        │       └─ 增加 二阶段提交/回滚
     │        │       │
     └────────┴───────┘
              │
              ▼
        底层都是同一个
        数据库连接池
        (*sql.DB)
```

---

## 🔍 详细工作机制

### 1. 事务开始（Begin）

#### GF 原生事务
```go
// 应用代码
db.Begin(ctx)

// SeataDB 内部判断
func (db *SeataDB) Begin(ctx context.Context) (gdb.TX, error) {
    // ❌ 不在全局事务中
    if !tm.IsGlobalTx(ctx) {
        // 直接调用 MySQL Driver 的 Begin
        return db.Driver.BeginWithOptions(ctx, opts)  // 返回 gdb.TX
    }
    
    // ... Seata 逻辑
}
```

**流程**：
```
应用调用 db.Begin(ctx)
  ↓
SeataDB.Begin() 检查 context
  ↓
tm.IsGlobalTx(ctx) = false
  ↓
调用 mysql.Driver.Begin()
  ↓
返回 gdb.TX (普通事务对象)
  ↓
使用原生 MySQL 事务
```

#### Seata 全局事务
```go
// 应用代码
tm.WithGlobalTx(ctx, &tm.GtxConfig{
    Name: "my-transaction",
    Timeout: 30 * time.Second,
}, func(ctx context.Context) error {
    // 在这个函数内部，ctx 中包含了全局事务信息
    db.Begin(ctx)
    // ...
})

// SeataDB 内部判断
func (db *SeataDB) Begin(ctx context.Context) (gdb.TX, error) {
    // ✅ 在全局事务中
    if tm.IsGlobalTx(ctx) {
        // 调用 Seata 事务逻辑
        return db.beginSeataTransaction(ctx, opts)  // 返回 SeataTX
    }
    
    // ... 原生逻辑
}
```

**流程**：
```
应用调用 tm.WithGlobalTx()
  ↓
初始化 Seata Context
  ├─ tm.InitSeataContext(ctx)
  ├─ tm.SetTxName(ctx, "my-transaction")
  ├─ tm.SetTxRole(ctx, tm.Launcher)
  └─ 向 Seata Server 申请全局 XID
  ↓
应用调用 db.Begin(ctx)
  ↓
SeataDB.Begin() 检查 context
  ↓
tm.IsGlobalTx(ctx) = true ✅
  ↓
调用 db.beginSeataTransaction()
  ↓
内部调用 mysql.Driver.Begin() 获取原生事务
  ↓
包装为 SeataTX (包含原生事务 + Seata增强)
  ↓
返回 SeataTX
```

---

### 2. SQL 执行（Execute）

#### GF 原生事务
```go
tx.Model("users").Where("id", 1).Update(g.Map{"balance": 100})
```

**流程**：
```
tx (gdb.TX 类型)
  ↓
tx.Model("users").Update(...)
  ↓
调用 mysql.Driver.DoUpdate()
  ↓
直接执行 SQL (没有拦截)
  ↓
UPDATE users SET balance=100 WHERE id=1
```

#### Seata 全局事务
```go
tx.Model("users").Where("id", 1).Update(g.Map{"balance": 100})
```

**流程**：
```
tx (SeataTX 类型)
  ↓
tx.Model("users").Update(...)
  ↓
调用 SeataDB.DoUpdate() (拦截器)
  ↓
1. 生成 Before Image
   └─ SELECT * FROM users WHERE id=1 FOR UPDATE
  ↓
2. 执行 UPDATE
   └─ UPDATE users SET balance=100 WHERE id=1
  ↓
3. 生成 After Image
   └─ SELECT * FROM users WHERE id IN (1)
  ↓
4. 保存 Undo Log 到 SeataTX
   └─ tx.AddUndoLog(...)
```

**关键代码**：
```go
// seata_mysql_db.go
func (db *SeataDB) DoUpdate(...) (sql.Result, error) {
    // 判断是否需要拦截
    if !db.shouldIntercept(ctx, link) {
        // ❌ 不拦截：直接调用 MySQL Driver 的 DoUpdate
        return db.Driver.DoUpdate(ctx, link, table, data, condition, args...)
    }
    
    // ✅ 拦截：执行 Seata 增强逻辑
    // 1. Before Image
    beforeImage, _ := generator.GenerateBeforeImage(...)
    
    // 2. 执行 SQL
    result, _ := db.Driver.DoUpdate(ctx, link, table, data, condition, args...)
    
    // 3. After Image
    afterImage, _ := generator.GenerateAfterImage(...)
    
    // 4. 保存 Undo Log
    db.saveUndoLog(ctx, UPDATE, table, beforeImage, afterImage)
    
    return result, nil
}

// 判断是否需要拦截
func (db *SeataDB) shouldIntercept(ctx context.Context, link gdb.Link) bool {
    // 1. 必须在全局事务中
    if !tm.IsGlobalTx(ctx) {
        return false
    }
    
    // 2. 必须在本地事务中
    if !link.IsTransaction() {
        return false
    }
    
    return true
}
```

---

### 3. 事务提交（Commit）

#### GF 原生事务
```go
tx.Commit()
```

**流程**：
```
tx.Commit()
  ↓
gdb.TX.Commit()
  ↓
直接提交本地事务
  ↓
COMMIT
```

#### Seata 全局事务
```go
tx.Commit()
```

**流程**：
```
SeataTX.Commit()
  ↓
【一阶段：分支事务提交】
  ↓
1. 插入 Undo Log
   └─ INSERT INTO undo_log (xid, branch_id, rollback_info) VALUES (...)
  ↓
2. 注册分支到 Seata Server
   └─ rm.BranchRegister(xid, resourceId, lockKeys)
   └─ 返回 branchId
  ↓
3. 提交本地事务
   └─ gdb.TX.Commit()  (调用原生事务的提交)
   └─ COMMIT
  ↓
4. 报告分支状态
   └─ rm.BranchReport(branchId, PhaseoneDone)
  ↓
【等待全局事务决议】
  ↓
tm.WithGlobalTx() 执行完毕
  ↓
向 Seata Server 提交全局事务
  ↓
【二阶段：全局提交】
  ↓
Seata Server 通知所有分支
  ↓
Resource.BranchCommit() 被调用
  ↓
异步删除 Undo Log
  └─ DELETE FROM undo_log WHERE xid=? AND branch_id=?
```

---

### 4. 事务回滚（Rollback）

#### GF 原生事务
```go
tx.Rollback()
```

**流程**：
```
tx.Rollback()
  ↓
gdb.TX.Rollback()
  ↓
直接回滚本地事务
  ↓
ROLLBACK
```

#### Seata 全局事务
```go
tx.Rollback()
```

**流程**：
```
SeataTX.Rollback()
  ↓
【一阶段：分支事务回滚】
  ↓
1. 直接回滚本地事务
   └─ gdb.TX.Rollback()  (调用原生事务的回滚)
   └─ ROLLBACK
  ↓
【等待全局事务决议】
  ↓
tm.WithGlobalTx() 检测到错误
  ↓
向 Seata Server 回滚全局事务
  ↓
【二阶段：全局回滚】
  ↓
Seata Server 通知所有分支
  ↓
Resource.BranchRollback() 被调用
  ↓
1. 读取 Undo Log
   └─ SELECT rollback_info FROM undo_log WHERE xid=? AND branch_id=?
  ↓
2. 解析并执行回滚 SQL
   └─ 根据 Before Image 生成反向 SQL
   └─ 执行回滚 SQL
  ↓
3. 删除 Undo Log
   └─ DELETE FROM undo_log WHERE xid=? AND branch_id=?
```

---

## 🔑 关键设计要点

### 1. SeataTX 包装了 gdb.TX

```go
type SeataTX struct {
    gdb.TX           // ⭐ 嵌入 GF 原生事务
    resource     *Resource
    ctx          context.Context
    undoLogItems *garray.Array  // Seata 增强：Undo Log 收集
    localTxId    string          // Seata 增强：本地事务ID
    branchId     int64           // Seata 增强：分支ID
}
```

**这意味着**：
- `SeataTX` **是一个** `gdb.TX`（通过嵌入实现接口）
- `SeataTX` 的所有 GF 原生方法都委托给内部的 `gdb.TX`
- `SeataTX` 增加了 Seata 分布式事务的额外功能

### 2. 自动降级机制

```go
func (db *SeataDB) Begin(ctx context.Context) (gdb.TX, error) {
    // 自动判断：是否在全局事务中？
    if !tm.IsGlobalTx(ctx) {
        // ❌ 不在全局事务 → 自动降级为 GF 原生事务
        return db.Driver.Begin(ctx)  // 返回 gdb.TX
    }
    
    // ✅ 在全局事务 → 使用 Seata 增强事务
    return db.beginSeataTransaction(ctx, opts)  // 返回 SeataTX
}
```

**优点**：
- 无需修改业务代码
- 根据 context 自动选择事务模式
- 不影响非分布式场景的性能

### 3. SQL 拦截的条件

```go
func (db *SeataDB) shouldIntercept(ctx context.Context, link gdb.Link) bool {
    // 条件1：必须在全局事务中
    if !tm.IsGlobalTx(ctx) {
        return false  // ❌ 不拦截
    }
    
    // 条件2：必须在本地事务中
    if !link.IsTransaction() {
        return false  // ❌ 不拦截
    }
    
    return true  // ✅ 拦截
}
```

**这意味着**：
- 只有同时满足两个条件才会拦截
- 单独使用 `db.Model().Update()` 不会触发 Seata 拦截
- 必须在 `tm.WithGlobalTx()` + `db.Transaction()` 内才会拦截

---

## 📝 使用示例对比

### 示例 1: GF 原生事务（单机）

```go
func TransferMoney(ctx context.Context, db gdb.DB) error {
    // ❌ 不在全局事务中
    return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
        // 扣款
        _, err := tx.Model("accounts").
            Where("id", 1).
            Decrement("balance", 100)
        if err != nil {
            return err
        }
        
        // 加款
        _, err = tx.Model("accounts").
            Where("id", 2).
            Increment("balance", 100)
        
        return err
    })
}
```

**执行流程**：
```
db.Transaction(ctx, f)
  ↓
SeataDB.Transaction() 检查
  ├─ tm.IsGlobalTx(ctx) = false ❌
  └─ 调用 mysql.Driver.Transaction() (原生事务)
  ↓
开始事务: BEGIN
  ↓
执行 SQL (无拦截):
  ├─ UPDATE accounts SET balance=balance-100 WHERE id=1
  └─ UPDATE accounts SET balance=balance+100 WHERE id=2
  ↓
提交事务: COMMIT
```

### 示例 2: Seata 全局事务（分布式）

```go
func TransferMoney(ctx context.Context, db gdb.DB) error {
    // ✅ 在全局事务中
    return tm.WithGlobalTx(ctx, &tm.GtxConfig{
        Name:    "transfer-money",
        Timeout: 30 * time.Second,
    }, func(ctx context.Context) error {
        return db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
            // 扣款
            _, err := tx.Model("accounts").
                Where("id", 1).
                Decrement("balance", 100)
            if err != nil {
                return err
            }
            
            // 加款
            _, err = tx.Model("accounts").
                Where("id", 2).
                Increment("balance", 100)
            
            return err
        })
    })
}
```

**执行流程**：
```
tm.WithGlobalTx(ctx, config, f)
  ↓
初始化全局事务
  ├─ 向 Seata Server 申请 XID
  └─ tm.InitSeataContext(ctx)
  ↓
db.Transaction(ctx, f)
  ↓
SeataDB.Transaction() 检查
  ├─ tm.IsGlobalTx(ctx) = true ✅
  └─ 调用 db.executeSeataTransaction()
  ↓
开始 Seata 事务
  ├─ 调用 mysql.Driver.Begin() 获取原生事务
  └─ 包装为 SeataTX
  ↓
执行 SQL (有拦截):
  ├─ UPDATE accounts SET balance=balance-100 WHERE id=1
  │   ├─ Before Image: SELECT * FROM accounts WHERE id=1 FOR UPDATE
  │   ├─ 执行 UPDATE
  │   ├─ After Image: SELECT * FROM accounts WHERE id IN (1)
  │   └─ 保存 Undo Log
  │
  └─ UPDATE accounts SET balance=balance+100 WHERE id=2
      ├─ Before Image: SELECT * FROM accounts WHERE id=2 FOR UPDATE
      ├─ 执行 UPDATE
      ├─ After Image: SELECT * FROM accounts WHERE id IN (2)
      └─ 保存 Undo Log
  ↓
SeataTX.Commit()
  ├─ 插入 Undo Log 到数据库
  ├─ 注册分支到 Seata Server
  ├─ 提交本地事务: COMMIT
  └─ 报告分支状态
  ↓
全局事务提交
  └─ Seata Server 协调所有分支提交
```

---

## 🎯 总结

### Seata 事务与 GF 原生事务的关系

| 维度 | GF 原生事务 | Seata 事务 |
|------|-----------|-----------|
| **对象类型** | `gdb.TX` | `SeataTX` (嵌入 `gdb.TX`) |
| **底层连接** | 单个数据库连接 | 单个数据库连接（相同） |
| **事务管理** | MySQL 本地事务 | MySQL 本地事务 + Seata 全局事务 |
| **SQL 执行** | 直接执行 | 拦截 + Before/After Image |
| **提交流程** | 直接 COMMIT | Undo Log + 分支注册 + COMMIT |
| **回滚流程** | 直接 ROLLBACK | ROLLBACK + 二阶段回滚 |
| **使用场景** | 单机事务 | 分布式事务 |
| **性能开销** | 低 | 稍高（Image生成+网络通信） |
| **一致性** | 单机 ACID | 分布式 ACID |

### 关键结论

1. ✅ **Seata 事务复用了 GF 原生事务**
   - SeataTX 内部包含一个 gdb.TX
   - 所有数据库操作最终还是通过 GF 原生事务执行

2. ✅ **自动降级机制**
   - 没有全局事务时，自动使用 GF 原生事务
   - 无需修改业务代码

3. ✅ **拦截增强**
   - 在 GF 原生事务基础上增加 Undo Log 生成
   - 不影响 GF 原生事务的正常功能

4. ✅ **完全兼容**
   - SeataTX 实现了 gdb.TX 接口
   - 业务代码无需区分是哪种事务

5. ✅ **透明接入**
   - 只需在最外层使用 `tm.WithGlobalTx()`
   - 内部的 `db.Transaction()` 自动识别并升级为 Seata 事务
