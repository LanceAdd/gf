# Seata-Go GF Driver 开发指南

> **版本**: v2.0  
> **最后更新**: 2025-12-13

---

## 目录

- [1. 项目概述](#1-项目概述)
- [2. 架构设计](#2-架构设计)
- [3. 核心功能](#3-核心功能)
- [4. 使用指南](#4-使用指南)
- [5. 配置说明](#5-配置说明)
- [6. 最佳实践](#6-最佳实践)

---

## 1. 项目概述

### 1.1 简介

Seata-Go GF Driver 是为 GoFrame 框架提供的 Seata 分布式事务驱动，支持 AT 和 XA 两种事务模式。

### 1.2 特性

- ✅ **AT 模式**: 自动补偿机制，业务无侵入
- ✅ **XA 模式**: 强一致性保障
- ✅ **异步提交**: 异步删除 Undo Log，提升性能
- ✅ **降级策略**: 故障降级保护
- ✅ **重试机制**: 自动重试失败操作
- ✅ **GF 集成**: 完美集成 GoFrame 框架

### 1.3 项目结构

```
seata/
├── config.go           # 配置管理
├── context.go          # 上下文管理
├── db.go              # 数据库包装
├── driver_at.go       # AT 模式驱动
├── driver_xa.go       # XA 模式驱动
├── resource.go        # 资源管理
├── transaction.go     # 事务管理
├── undo_log.go        # Undo Log 结构
├── image_generator.go # Before/After Image 生成
├── sql_parser.go      # SQL 解析
├── async_worker.go    # 异步工作器
├── retry.go           # 重试执行器
└── fallback.go        # 降级管理器
```

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────┐
│          GoFrame Application            │
│                                         │
│  ┌──────────────────────────────────┐  │
│  │      g.DB() / gdb.DB             │  │
│  └────────────┬─────────────────────┘  │
│               │                         │
│  ┌────────────▼─────────────────────┐  │
│  │         SeataDB                  │  │
│  │  ┌──────────────────────────┐   │  │
│  │  │  SQL Interceptor         │   │  │
│  │  │  - DoUpdate/Insert/Delete│   │  │
│  │  └──────────────────────────┘   │  │
│  │  ┌──────────────────────────┐   │  │
│  │  │  Image Generator         │   │  │
│  │  │  - Before/After Image    │   │  │
│  │  └──────────────────────────┘   │  │
│  │  ┌──────────────────────────┐   │  │
│  │  │  Transaction Manager     │   │  │
│  │  │  - Undo Log              │   │  │
│  │  └──────────────────────────┘   │  │
│  └──────────────────────────────────┘  │
│               │                         │
│  ┌────────────▼─────────────────────┐  │
│  │      Seata-Go SDK                │  │
│  │  - TM (Transaction Manager)      │  │
│  │  - RM (Resource Manager)         │  │
│  └──────────────────────────────────┘  │
│               │                         │
└───────────────┼─────────────────────────┘
                │
    ┌───────────▼──────────────┐
    │   Seata Server (TC)      │
    │   - Branch Registration  │
    │   - Global Coordination  │
    └──────────────────────────┘
```

### 2.2 核心组件

#### SeataDB
- 数据库包装对象
- SQL 拦截和增强
- 事务管理

#### Resource
- 资源注册和管理
- Branch 提交和回滚
- Undo Log 处理

#### SeataTX
- 事务对象
- Undo Log 收集
- 分支注册

---

## 3. 核心功能

### 3.1 AT 模式

**工作流程**：

1. **一阶段**：
   - 拦截 SQL
   - 生成 Before Image
   - 执行业务 SQL
   - 生成 After Image
   - 插入 Undo Log
   - 提交本地事务
   - 注册分支

2. **二阶段提交**：
   - 异步删除 Undo Log

3. **二阶段回滚**：
   - 读取 Undo Log
   - 生成回滚 SQL
   - 执行回滚
   - 删除 Undo Log

### 3.2 Image 生成

**Before Image**：
```sql
-- 对于 UPDATE/DELETE 操作
SELECT * FROM users WHERE id = 1 FOR UPDATE
```

**After Image**：
```sql
-- 对于 UPDATE/INSERT 操作
SELECT * FROM users WHERE id IN (1, 2, 3)
```

### 3.3 Undo Log 结构

```go
type SQLUndoLog struct {
    SQLType     SQLType       // SQL 类型
    TableName   string        // 表名
    BeforeImage *TableRecords // Before Image
    AfterImage  *TableRecords // After Image
}

type BranchUndoLog struct {
    XID         string        // 全局事务 ID
    BranchID    int64         // 分支事务 ID
    SQLUndoLogs []*SQLUndoLog // SQL Undo Log 列表
}
```

---

## 4. 使用指南

### 4.1 初始化

```go
import (
    "github.com/gogf/gf/contrib/drivers/seata/v2"
    "github.com/gogf/gf/v2/frame/g"
)

func main() {
    // 方式一：使用默认配置
    seata.Init(seata.DefaultConfig())
    
    // 方式二：自定义配置
    config := &seata.Config{
        Enabled:         true,
        ApplicationID:   "my-app",
        TxServiceGroup:  "default_tx_group",
        Mode:            "AT",
        AT: seata.ATConfig{
            EnableAsyncCommit: true,
        },
    }
    seata.Init(config)
}
```

### 4.2 配置数据库

**方式一：YAML 配置**

```yaml
database:
  default:
    type: "seata-at-mysql"  # 使用 Seata AT 驱动
    host: "127.0.0.1"
    port: "3306"
    user: "root"
    pass: "root"
    name: "test"
```

**方式二：代码配置**

```go
gdb.SetConfig(gdb.Config{
    "default": gdb.ConfigGroup{
        gdb.ConfigNode{
            Type: "seata-at-mysql",
            Host: "127.0.0.1",
            Port: "3306",
            User: "root",
            Pass: "root",
            Name: "test",
        },
    },
})
```

### 4.3 使用全局事务

```go
// 开启全局事务
ctx = seata.WithGlobalTransaction(ctx, "业务名称")

// 执行事务
err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
    // 更新操作
    _, err := tx.Update(ctx, "users", gdb.Map{
        "balance": gdb.Raw("balance - 100"),
    }, "id", 1)
    if err != nil {
        return err
    }
    
    // 插入操作
    _, err = tx.Insert(ctx, "orders", gdb.Map{
        "user_id": 1,
        "amount":  100,
    })
    
    return err
})
```

---

## 5. 配置说明

### 5.1 完整配置

```go
type Config struct {
    // 基础配置
    Enabled         bool   // 是否启用 Seata
    ApplicationID   string // 应用 ID
    TxServiceGroup  string // 事务分组
    Mode            string // 模式: AT/XA/TCC/SAGA
    
    // AT 模式配置
    AT ATConfig
    
    // XA 模式配置
    XA XAConfig
    
    // 降级配置
    Fallback FallbackConfig
}

type ATConfig struct {
    EnableAsyncCommit    bool   // 启用异步提交
    OnlyCarePrimaryKey   bool   // 只关注主键
    UndoLogSerialization string // Undo Log 序列化方式
}

type FallbackConfig struct {
    Enabled         bool              // 是否启用降级
    Strategy        FallbackStrategy  // 降级策略
    ErrorThreshold  int               // 错误阈值
    TimeWindow      int               // 时间窗口（秒）
    RecoveryTimeout int               // 恢复超时（秒）
}
```

### 5.2 默认配置

```go
DefaultConfig() *Config {
    return &Config{
        Enabled:        false,
        ApplicationID:  "gf-app",
        TxServiceGroup: "default_tx_group",
        Mode:           "AT",
        AT: ATConfig{
            EnableAsyncCommit:    true,
            OnlyCarePrimaryKey:   true,
            UndoLogSerialization: "jackson",
        },
        Fallback: FallbackConfig{
            Enabled:         true,
            Strategy:        FallbackToLocal,
            ErrorThreshold:  5,
            TimeWindow:      60,
            RecoveryTimeout: 30,
        },
    }
}
```

---

## 6. 最佳实践

### 6.1 性能优化

**1. 启用异步提交**
```go
config.AT.EnableAsyncCommit = true
```

**2. 只关注主键**
```go
config.AT.OnlyCarePrimaryKey = true
```

**3. 合理设置连接池**
```yaml
database:
  default:
    maxIdleConnCount: 10
    maxOpenConnCount: 100
    maxConnLifeTime: 60
```

### 6.2 错误处理

**1. 使用降级策略**
```go
config.Fallback.Enabled = true
config.Fallback.Strategy = FallbackToLocal
```

**2. 配置重试机制**
```go
retryExecutor := seata.NewRetryExecutor(&seata.RetryConfig{
    MaxAttempts: 3,
    Interval:    100 * time.Millisecond,
})
```

### 6.3 监控和日志

**1. 启用日志**
```go
glog.SetLevel(glog.LEVEL_ALL)
```

**2. 关键指标监控**
- 全局事务数量
- 分支事务数量
- Undo Log 数量
- 降级触发次数

### 6.4 常见问题

**Q: 如何处理分布式事务超时？**

A: 配置合理的超时时间
```go
config.Timeout = 60 // 秒
```

**Q: 如何处理 Undo Log 堆积？**

A: 启用异步提交，增加工作线程数
```go
config.AT.EnableAsyncCommit = true
asyncWorker := seata.NewAsyncWorker(db, &seata.AsyncWorkerConfig{
    WorkerCount: 5,
})
```

**Q: 如何进行性能测试？**

A: 参考 [测试指南](./TESTING-GUIDE.md)

---

## 附录

### A. 相关链接

- [Seata 官方文档](https://seata.io/)
- [GoFrame 官方文档](https://goframe.org/)
- [项目仓库](https://github.com/gogf/gf)

### B. 贡献指南

欢迎贡献代码和文档！请遵循：

1. Fork 项目
2. 创建特性分支
3. 提交代码
4. 创建 Pull Request

### C. 许可证

MIT License
