# Seata MySQL Driver 架构说明

## 🎯 架构改进概述

新方案相比旧方案的核心改进：
- **嵌入对象**：从 `*gdb.Core` 改为 `*mysql.Driver`
- **代码减少**：自动继承MySQL方法，减少约77行代码
- **更符合GF规范**：遵循GoFrame推荐的驱动扩展模式

---

## 📊 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      应用层 (Application)                         │
│                                                                   │
│    tm.WithGlobalTx(ctx, config, func)  或  db.Transaction(ctx)   │
└────────────────────────┬────────────────────────────────────────┘
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                     GF Framework Layer                            │
│                                                                   │
│    db.Transaction() / db.Begin() / tx.Commit() / tx.Rollback()   │
└────────────────────────┬────────────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         │                               │
┌────────▼─────────┐          ┌─────────▼──────────┐
│   Non-Global TX  │          │   Global TX        │
│   (本地事务)      │          │   (全局事务)        │
│                  │          │                    │
│  MySQL Driver    │          │   SeataDB          │
│  (自动降级)       │          │   ↓                │
└──────────────────┘          │   mysql.Driver     │
                              │   (嵌入，自动继承) │
                              └─────────┬──────────┘
                                        │
                              ┌─────────▼──────────┐
                              │   SeataTX          │
                              │   (事务代理)        │
                              └─────────┬──────────┘
                                        │
                              ┌─────────▼──────────┐
                              │   Resource         │
                              │   (资源管理)        │
                              └─────────┬──────────┘
                                        │
                              ┌─────────▼──────────┐
                              │   Seata RM         │
                              │   (资源管理器)      │
                              └─────────┬──────────┘
                                        │
                              ┌─────────▼──────────┐
                              │   Seata TC         │
                              │   (事务协调器)      │
                              └────────────────────┘
```

---

## 🔧 核心组件

### 1. 驱动层 (Driver Layer)

#### DriverAT (AT模式驱动)

```go
┌─────────────────────────────────────────┐
│           DriverAT                      │
│  ┌────────────────────────────────┐    │
│  │ config *Config                 │    │
│  └────────────────────────────────┘    │
│                                         │
│  New(core, node) -> gdb.DB              │
│  ┌────────────────────────────────┐    │
│  │ 1. mysql.New()                 │    │ 
│  │ 2. mysqlDB.New(core, node)     │    │
│  │ 3. mysqlDB.Open(node) -> sqlDB │    │
│  │ 4. BuildResourceID()           │    │
│  │ 5. NewResource()               │    │
│  │ 6. registerResource()          │    │
│  │ 7. Return SeataDB{             │    │
│  │      Driver: mysqlDriver ✅    │    │
│  │    }                           │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
         │
         ▼
  implements gdb.Driver
```

**关键点**：
- ✅ 创建 mysql.Driver 实例
- ✅ 临时获取 `*sql.DB` 用于 Seata RM 注册
- ✅ 返回嵌入了 `*mysql.Driver` 的 `SeataDB`

#### DriverXA (XA模式驱动)

结构与 DriverAT 类似，但：
- 使用 `branch.BranchTypeXA`
- 注册到 XA 模式资源管理器

---

### 2. 数据库层 (Database Layer)

#### SeataDB（新架构⭐）

```go
┌─────────────────────────────────────────┐
│           SeataDB                       │
│  ┌────────────────────────────────┐    │
│  │ *mysql.Driver (嵌入) ✅        │    │  ← 关键改进！
│  │ resource *Resource             │    │
│  │ config *Config                 │    │
│  └────────────────────────────────┘    │
│                                         │
│  自动继承的方法 (无需手动实现)：         │
│  ┌────────────────────────────────┐    │
│  │ TableFields() ✅               │    │
│  │ Open() ✅                      │    │
│  │ GetChars() ✅                  │    │
│  │ ... 所有 MySQL 方法             │    │
│  └────────────────────────────────┘    │
│                                         │
│  重写的方法 (Seata拦截逻辑)：           │
│  ┌────────────────────────────────┐    │
│  │ Begin(ctx) -> TX               │    │
│  │ BeginWithOptions(ctx) -> TX    │    │
│  │ Transaction(ctx, f) -> error   │    │
│  │ DoUpdate() -> 拦截UPDATE        │    │
│  │ DoInsert() -> 拦截INSERT        │    │
│  │ DoDelete() -> 拦截DELETE        │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

**架构优势**：
1. ✅ **自动继承**：无需手动实现 `TableFields()`、`Open()` 等方法
2. ✅ **代码减少**：减少约 77 行重复代码
3. ✅ **符合规范**：遵循 GF 推荐的驱动嵌入模式
4. ✅ **职责分离**：MySQL 功能由 mysql.Driver 负责，SeataDB 只负责事务拦截

---

### 3. 事务层 (Transaction Layer)

#### SeataTX

```go
┌─────────────────────────────────────────┐
│           SeataTX                       │
│  ┌────────────────────────────────┐    │
│  │ gdb.TX (嵌入原生事务)           │    │
│  │ resource *Resource             │    │
│  │ ctx context.Context            │    │
│  │ undoLogItems *garray.Array     │    │
│  │ localTxId string               │    │
│  │ branchId int64                 │    │
│  └────────────────────────────────┘    │
│                                         │
│  Commit() 流程：                        │
│  ┌────────────────────────────────┐    │
│  │ 1. 注册分支事务 ✅              │    │
│  │ 2. 保存 Undo Log ✅            │    │
│  │ 3. 提交本地事务 ✅              │    │
│  │ 4. 上报分支状态 ✅              │    │
│  └────────────────────────────────┘    │
│                                         │
│  Rollback() 流程：                      │
│  ┌────────────────────────────────┐    │
│  │ 1. 回滚本地事务 ✅              │    │
│  │ 2. 上报失败状态 ✅              │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

---

### 4. 资源层 (Resource Layer)

```go
┌─────────────────────────────────────────┐
│           Resource                      │
│  ┌────────────────────────────────┐    │
│  │ resourceID string              │    │
│  │ branchType branch.BranchType   │    │
│  │ dbType types.DBType            │    │
│  │ db *sql.DB                     │    │
│  │ gfCore *gdb.Core               │    │
│  │ config *Config                 │    │
│  └────────────────────────────────┘    │
│                                         │
│  Implements rm.Resource:                │
│  ┌────────────────────────────────┐    │
│  │ GetResourceGroupId()           │    │
│  │ GetResourceId()                │    │
│  │ GetBranchType()                │    │
│  │ BranchCommit()                 │    │
│  │ BranchRollback()               │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

---

## 🔄 调用流程

### 场景 1: 非全局事务（自动降级）

```
Application
    │
    ├─ db.Transaction(ctx, f)
    │
    ▼
SeataDB.Transaction()
    │
    ├─ isInGlobalTransaction(ctx) = false
    │
    ▼
mysql.Driver.Transaction()  ← 自动降级到 MySQL
    │
    └─ 标准 MySQL 本地事务流程
```

**特点**：
- ✅ 检测到非全局事务时自动降级
- ✅ 直接使用嵌入的 `mysql.Driver`
- ✅ 无性能损失

---

### 场景 2: 全局事务（AT模式）

```
Application
    │
    ├─ tm.WithGlobalTx(ctx, config, f)
    │
    ▼
Seata TM (事务管理器)
    │
    ├─ 向 TC 申请 XID
    ├─ tm.SetXID(ctx, xid)
    │
    ▼
db.Transaction(ctx, f)
    │
    ▼
SeataDB.Transaction()
    │
    ├─ isInGlobalTransaction(ctx) = true ✅
    │
    ▼
SeataDB.beginSeataTransaction()
    │
    ├─ mysql.Driver.BeginWithOptions()  → 原生 MySQL 事务
    │
    ▼
SeataTX (包装原生事务)
    │
    ├─ 执行业务 SQL
    │   ├─ UPDATE → DoUpdate() 拦截
    │   │   ├─ 生成 beforeImage
    │   │   ├─ 执行 UPDATE
    │   │   ├─ 生成 afterImage
    │   │   └─ 保存 undoLog
    │   │
    │   ├─ INSERT → DoInsert() 拦截
    │   └─ DELETE → DoDelete() 拦截
    │
    ▼
SeataTX.Commit()
    │
    ├─ 1. 注册分支事务到 TC
    │      ├─ branchId = rm.BranchRegister()
    │      └─ lockKeys = generateLockKeys()
    │
    ├─ 2. 保存 Undo Log 到数据库
    │      └─ INSERT INTO undo_log (...)
    │
    ├─ 3. 提交本地事务
    │      └─ tx.Commit()
    │
    ├─ 4. 上报分支状态到 TC
    │      └─ rm.BranchReport()
    │
    ▼
Seata TC (事务协调器)
    │
    ├─ 全局事务提交
    │   └─ 异步删除 undo_log
    │
    └─ 或全局事务回滚
        └─ 调用 BranchRollback()
            └─ 读取 undo_log 执行回滚
```

---

## 🆚 新旧方案对比

### 架构对比

| 组件 | 旧方案 | 新方案 | 改进 |
|------|-------|--------|------|
| **SeataDB嵌入** | `*gdb.Core` | `*mysql.Driver` | ✅ 更符合GF规范 |
| **TableFields** | 手动实现(33行) | 自动继承 | ✅ 减少重复代码 |
| **Open** | 手动实现(21行) | 自动继承 | ✅ 减少重复代码 |
| **GetChars** | 未实现 | 自动继承 | ✅ 功能增强 |
| **代码总量** | 更多 | 减少77行 | ✅ 更简洁 |

### 初始化流程对比

#### 旧方案

```go
func (d *DriverAT) New(core *gdb.Core, node *gdb.ConfigNode) (gdb.DB, error) {
    // 1. 创建 MySQL DB（临时）
    mysqlDB := mysql.New().New(core, node)
    
    // 2. 获取 sql.DB
    sqlDB := mysqlDB.Open(node)
    
    // 3. 注册资源
    resource := NewResource(...)
    
    // 4. 返回 SeataDB（嵌入 Core）❌
    return &SeataDB{
        Core: core,  // 直接嵌入 Core
        resource: resource,
    }, nil
}
```

**问题**：
- ❌ 需要手动实现 `TableFields()`
- ❌ 需要手动实现 `Open()`
- ❌ 代码重复

#### 新方案

```go
func (d *DriverAT) New(core *gdb.Core, node *gdb.ConfigNode) (gdb.DB, error) {
    // 1. 创建 MySQL Driver
    mysqlDriver := mysql.New()
    mysqlDB := mysqlDriver.New(core, node)
    
    // 2. 获取 sql.DB
    sqlDB := mysqlDB.Open(node)
    
    // 3. 注册资源
    resource := NewResource(...)
    
    // 4. 类型断言获取 mysql.Driver
    driver := mysqlDB.(*mysql.Driver)
    
    // 5. 返回 SeataDB（嵌入 mysql.Driver）✅
    return &SeataDB{
        Driver: driver,  // 嵌入 mysql.Driver
        resource: resource,
    }, nil
}
```

**优势**：
- ✅ 自动继承所有 MySQL 方法
- ✅ 无需手动实现
- ✅ 代码更简洁

---

## 📈 性能考量

### 临时对象开销

**问题**：为什么在 `Driver.New()` 中创建临时 MySQL DB？

**回答**：
1. Seata RM 注册**必须**要 `*sql.DB` 对象
2. 在 `Driver.New()` 执行时，`core.db` 还未设置
3. 无法使用 `core.Master()` 获取连接
4. **必须**临时创建 MySQL DB 调用 `Open()` 获取

**开销分析**：
- ✅ 临时对象很小（只是一个 Driver 实例）
- ✅ 获取 `*sql.DB` 后不再使用（GC 回收）
- ✅ 真正使用的是嵌入到 SeataDB 中的 `mysql.Driver`
- ✅ 换来 77 行代码简化，**非常值得**

### 降级性能

非全局事务时自动降级到 `mysql.Driver`：
- ✅ 零性能损失
- ✅ 与原生 MySQL 驱动性能一致

---

## 🎯 设计原则

### 1. 职责分离

```
mysql.Driver    →  负责所有 MySQL 特定功能
SeataDB         →  只负责 Seata 事务拦截逻辑
```

### 2. 最小侵入

- 非全局事务时完全透明
- 全局事务时只拦截必要的操作

### 3. 符合规范

- 遵循 GoFrame 推荐的驱动扩展模式
- 代码风格与 GF 官方驱动一致

---

## 🔧 扩展性

### 支持其他数据库

新架构更容易扩展到其他数据库：

```go
// PostgreSQL 支持
type SeataPostgresDB struct {
    *pgsql.Driver  // 嵌入 PostgreSQL Driver
    resource *Resource
    config *Config
}

// Oracle 支持
type SeataOracleDB struct {
    *oracle.Driver  // 嵌入 Oracle Driver
    resource *Resource
    config *Config
}
```

**优势**：
- ✅ 自动继承对应数据库的所有方法
- ✅ 无需为每个数据库实现重复逻辑
- ✅ 维护成本低

---

## 📚 参考资料

- [GoFrame 数据库驱动开发](https://goframe.org/pages/viewpage.action?pageId=1114119)
- [Seata AT 模式原理](https://seata.io/zh-cn/docs/dev/mode/at-mode.html)
- [Seata-Go SDK](https://github.com/seata/seata-go)

---

## ✅ 总结

新架构的核心改进：

1. **嵌入 mysql.Driver**：符合 GF 规范，自动继承所有方法
2. **代码减少**：删除 77 行重复代码
3. **职责清晰**：MySQL 功能与 Seata 逻辑分离
4. **易于扩展**：支持其他数据库更容易
5. **性能优化**：非全局事务零损失

**结论**：新架构在保持功能完整的同时，代码更简洁、更优雅、更易维护！✨
