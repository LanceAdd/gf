# 架构说明

## 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                      应用层 (Application)                         │
│                                                                   │
│    seata.GlobalTransaction(ctx, db, "tx-name", timeout, func)    │
└────────────────────────┬──────────────────────────────────────── ┘
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
│   GF Core.TX     │          │   SeataDB          │
└──────────────────┘          └─────────┬──────────┘
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

## 核心组件

### 1. 驱动层 (Driver Layer)

```
┌─────────────────────────────────────────┐
│           DriverAT                      │
│  ┌────────────────────────────────┐    │
│  │ New(core, node) -> SeataDB     │    │
│  │                                │    │
│  │ 1. openUnderlyingDB()          │    │
│  │ 2. BuildResourceID()           │    │
│  │ 3. NewResource()               │    │
│  │ 4. registerResource()          │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
                   │
                   ▼
         implements gdb.Driver
```

### 2. 数据库层 (Database Layer)

```
┌─────────────────────────────────────────┐
│           SeataDB                       │
│  ┌────────────────────────────────┐    │
│  │ *gdb.Core (嵌入)                │    │
│  │ resource *Resource             │    │
│  │ config *Config                 │    │
│  └────────────────────────────────┘    │
│                                         │
│  Methods:                               │
│  ┌────────────────────────────────┐    │
│  │ Begin(ctx) -> TX               │    │
│  │ Transaction(ctx, f) -> error   │    │
│  │ isInGlobalTransaction(ctx)     │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

### 3. 事务层 (Transaction Layer)

```
┌─────────────────────────────────────────┐
│           SeataTX                       │
│  ┌────────────────────────────────┐    │
│  │ gdb.TX (嵌入)                   │    │
│  │ resource *Resource             │    │
│  │ ctx context.Context            │    │
│  └────────────────────────────────┘    │
│                                         │
│  Methods:                               │
│  ┌────────────────────────────────┐    │
│  │ Commit() -> error              │    │
│  │   ├─ 注册分支事务 (TODO)        │    │
│  │   ├─ 保存 Undo Log (TODO)      │    │
│  │   ├─ 提交本地事务               │    │
│  │   └─ 上报分支状态 (TODO)        │    │
│  │                                │    │
│  │ Rollback() -> error            │    │
│  │   ├─ 回滚本地事务               │    │
│  │   └─ 上报失败状态 (TODO)        │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

### 4. 资源层 (Resource Layer)

```
┌─────────────────────────────────────────┐
│           Resource                      │
│  ┌────────────────────────────────┐    │
│  │ resourceID string              │    │
│  │ branchType branch.BranchType   │    │
│  │ db *sql.DB                     │    │
│  │ gfDB gdb.DB                    │    │
│  │ config *Config                 │    │
│  └────────────────────────────────┘    │
│                                         │
│  Implements:                            │
│  ┌────────────────────────────────┐    │
│  │ rm.Resource interface          │    │
│  │   ├─ GetResourceGroupId()      │    │
│  │   ├─ GetResourceId()           │    │
│  │   └─ GetBranchType()           │    │
│  └────────────────────────────────┘    │
└─────────────────────────────────────────┘
```

## 调用流程

### 场景 1: 非全局事务

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
gdb.Core.Transaction()
    │
    └─ 标准 GF 本地事务流程
```

### 场景 2: 全局事务

```
Application
    │
    ├─ seata.GlobalTransaction(ctx, db, "tx", timeout, f)
    │
    ▼
WithGlobalTransaction()
    │
    ├─ tm.InitSeataContext(ctx)
    ├─ tm.SetTxName(ctx, "tx")
    ├─ tm.SetTxRole(ctx, Launcher)
    │
    ▼
GlobalTransactionManager.Begin(ctx)
    │
    ├─ 向 TC 申请 XID
    ├─ tm.SetXID(ctx, xid)
    │
    ▼
SeataDB.Transaction(ctx, f)
    │
    ├─ isInGlobalTransaction(ctx) = true
    │
    ▼
SeataDB.beginSeataTransaction()
    │
    ├─ gdb.Core.BeginWithOptions()  → 原生 GF 事务
    │
    ▼
SeataTX (包装原生事务)
    │
    ├─ 执行业务逻辑 f(ctx, tx)
    │
    ▼
SeataTX.Commit()
    │
    ├─ [TODO] 注册分支事务
    ├─ [TODO] 保存 Undo Log
    ├─ tx.Commit() → 提交本地事务
    ├─ [TODO] 上报分支状态
    │
    ▼
GlobalTransactionManager.Commit(ctx)
    │
    └─ 提交全局事务
```

## Context 传递机制

```
┌──────────────────────────────────────────────────┐
│              context.Context                      │
│                                                   │
│  Seata Context Variables:                        │
│  ┌─────────────────────────────────────────┐    │
│  │ seataContextVariable (ContextVariable)  │    │
│  │   ├─ Xid: string                        │    │
│  │   ├─ TxName: string                     │    │
│  │   ├─ TxRole: GlobalTransactionRole      │    │
│  │   ├─ TxStatus: GlobalStatus             │    │
│  │   └─ BusinessActionContext              │    │
│  └─────────────────────────────────────────┘    │
│                                                   │
│  GF Context Variables:                            │
│  ┌─────────────────────────────────────────┐    │
│  │ CtxKeyForDB (DB)                        │    │
│  │ transactionKeyForContext (TX)           │    │
│  └─────────────────────────────────────────┘    │
└──────────────────────────────────────────────────┘
```

## 配置流程

```
┌────────────────────────────────────────┐
│  1. 应用初始化                          │
│     seata.Init(config)                 │
└────────────┬───────────────────────────┘
             │
             ▼
┌────────────────────────────────────────┐
│  2. 初始化 Seata 客户端                 │
│     client.Init(seataConf)             │
└────────────┬───────────────────────────┘
             │
             ▼
┌────────────────────────────────────────┐
│  3. 初始化 AT 模式                      │
│     sql.InitAT(undoConfig, asyncConfig)│
└────────────┬───────────────────────────┘
             │
             ▼
┌────────────────────────────────────────┐
│  4. 注册驱动                            │
│     gdb.Register("seata-at-mysql", ...) │
└────────────┬───────────────────────────┘
             │
             ▼
┌────────────────────────────────────────┐
│  5. 配置数据库                          │
│     gdb.AddConfigNode(...)             │
└────────────┬───────────────────────────┘
             │
             ▼
┌────────────────────────────────────────┐
│  6. 获取数据库实例                      │
│     db := g.DB()                       │
│     → SeataDB 实例                     │
└────────────────────────────────────────┘
```

## 文件依赖关系

```
seata.go (入口)
    │
    ├──> config.go (配置定义)
    │
    ├──> driver_at.go (驱动实现)
    │        │
    │        ├──> resource.go (资源管理)
    │        │
    │        └──> db.go (DB 包装)
    │                 │
    │                 └──> transaction.go (TX 包装)
    │
    └──> context.go (Context 工具)
```

## 数据流向

### 写操作流程

```
Application INSERT/UPDATE/DELETE
    │
    ├─ tx.Model("table").Insert(data)
    │
    ▼
SeataTX (代理)
    │
    ├─ [TODO] 拦截 SQL
    ├─ [TODO] 生成前镜像
    │
    ▼
gdb.TX.Insert()
    │
    ├─ 执行实际 SQL
    │
    ▼
MySQL Database
    │
    ├─ INSERT 数据
    │
    ▼
SeataTX.Commit()
    │
    ├─ [TODO] 生成后镜像
    ├─ [TODO] 保存 Undo Log
    ├─ 提交事务
    │
    ▼
Seata TC
    │
    └─ 记录分支状态
```

## 关键设计点

### 1. 最小侵入原则
- ✅ 不修改 GF 核心代码
- ✅ 通过 Driver 扩展机制集成
- ✅ 保持 GF API 兼容性

### 2. 透明降级
- ✅ 检测全局事务上下文
- ✅ 非全局事务自动降级
- ✅ 无感知切换

### 3. 灵活配置
- ✅ 支持启用/禁用
- ✅ 支持多种注册中心
- ✅ 支持 AT/XA 模式

---

**文档版本**: v1.0 (阶段一)
**更新日期**: 2025-12-13
