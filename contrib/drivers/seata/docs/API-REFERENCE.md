# Seata-Go GF Driver API 参考

> **版本**: v2.0  
> **最后更新**: 2025-12-13

---

## 目录

- [1. 初始化](#1-初始化)
- [2. 配置](#2-配置)
- [3. 上下文管理](#3-上下文管理)
- [4. 驱动](#4-驱动)
- [5. 工具类](#5-工具类)

---

## 1. 初始化

### Init

初始化 Seata 驱动。

```go
func Init(config *Config) error
```

**参数**：
- `config` - Seata 配置，如果为 nil 则使用默认配置

**返回**：
- `error` - 错误信息

**示例**：
```go
err := seata.Init(seata.DefaultConfig())
if err != nil {
    panic(err)
}
```

---

## 2. 配置

### Config

Seata 配置结构。

```go
type Config struct {
    Enabled         bool            // 是否启用
    ApplicationID   string          // 应用 ID
    TxServiceGroup  string          // 事务分组
    Mode            string          // 模式: AT/XA/TCC/SAGA
    AT              ATConfig        // AT 模式配置
    XA              XAConfig        // XA 模式配置
    Fallback        FallbackConfig  // 降级配置
}
```

### DefaultConfig

获取默认配置。

```go
func DefaultConfig() *Config
```

**返回**：
- `*Config` - 默认配置

**示例**：
```go
config := seata.DefaultConfig()
config.Enabled = true
config.ApplicationID = "my-app"
```

### GetBranchType

获取分支类型。

```go
func (c *Config) GetBranchType() branch.BranchType
```

**返回**：
- `branch.BranchType` - 分支类型（AT/XA/TCC/SAGA）

---

## 3. 上下文管理

### WithGlobalTransaction

创建带全局事务的上下文。

```go
func WithGlobalTransaction(ctx context.Context, transactionName string) context.Context
```

**参数**：
- `ctx` - 原始上下文
- `transactionName` - 事务名称

**返回**：
- `context.Context` - 新上下文

**示例**：
```go
ctx = seata.WithGlobalTransaction(ctx, "transfer_money")

err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
    // 业务逻辑
    return nil
})
```

### IsInGlobalTransaction

检查是否在全局事务中。

```go
func IsInGlobalTransaction(ctx context.Context) bool
```

**参数**：
- `ctx` - 上下文

**返回**：
- `bool` - 是否在全局事务中

### GetGlobalTransactionXID

获取全局事务 ID。

```go
func GetGlobalTransactionXID(ctx context.Context) string
```

**参数**：
- `ctx` - 上下文

**返回**：
- `string` - 全局事务 XID

---

## 4. 驱动

### NewDriverAT

创建 AT 模式驱动。

```go
func NewDriverAT(config *Config) *DriverAT
```

**参数**：
- `config` - Seata 配置

**返回**：
- `*DriverAT` - AT 驱动实例

### NewDriverXA

创建 XA 模式驱动。

```go
func NewDriverXA(config *Config) *DriverXA
```

**参数**：
- `config` - Seata 配置

**返回**：
- `*DriverXA` - XA 驱动实例

---

## 5. 工具类

### AsyncWorker

异步工作器，用于异步删除 Undo Log。

```go
type AsyncWorker struct {
    // 私有字段
}

func NewAsyncWorker(db *sql.DB, config *AsyncWorkerConfig) *AsyncWorker

func (w *AsyncWorker) Start()
func (w *AsyncWorker) Stop()
func (w *AsyncWorker) BranchCommit(resource rm.BranchResource) error
```

**示例**：
```go
worker := seata.NewAsyncWorker(db, &seata.AsyncWorkerConfig{
    WorkerCount:      3,
    QueueSize:        1000,
    BatchSize:        100,
    FlushInterval:    time.Second,
})
worker.Start()
defer worker.Stop()
```

### RetryExecutor

重试执行器。

```go
type RetryExecutor struct {
    // 私有字段
}

func NewRetryExecutor(config *RetryConfig) *RetryExecutor

func (r *RetryExecutor) Execute(fn func() error) error
```

**示例**：
```go
executor := seata.NewRetryExecutor(&seata.RetryConfig{
    MaxAttempts:  3,
    Interval:     100 * time.Millisecond,
    MaxInterval:  time.Second,
    Multiplier:   2.0,
})

err := executor.Execute(func() error {
    // 可能失败的操作
    return doSomething()
})
```

### FallbackManager

降级管理器。

```go
type FallbackManager struct {
    // 私有字段
}

func NewFallbackManager(config *FallbackConfig) *FallbackManager

func (f *FallbackManager) IsFallback() bool
func (f *FallbackManager) RecordError(ctx context.Context)
func (f *FallbackManager) RecordSuccess()
func (f *FallbackManager) ExecuteWithFallback(
    ctx context.Context,
    db gdb.DB,
    operation func(context.Context) error,
    fallback func(context.Context) error,
) error
```

**示例**：
```go
manager := seata.NewFallbackManager(&seata.FallbackConfig{
    Enabled:         true,
    Strategy:        seata.FallbackToLocal,
    ErrorThreshold:  5,
    TimeWindow:      60,
    RecoveryTimeout: 30,
})

err := manager.ExecuteWithFallback(ctx, db, 
    func(ctx context.Context) error {
        // 正常操作
        return normalOp(ctx)
    },
    func(ctx context.Context) error {
        // 降级操作
        return fallbackOp(ctx)
    },
)
```

### SQLParser

SQL 解析器。

```go
type SQLParser struct {
    // 私有字段
}

func NewSQLParser(sql string) *SQLParser

func (p *SQLParser) GetSQLType() SQLType
func (p *SQLParser) GetTableName() string
func (p *SQLParser) NeedUndoLog() bool
```

**示例**：
```go
parser := seata.NewSQLParser("UPDATE users SET name = ? WHERE id = ?")

sqlType := parser.GetSQLType()      // SQLTypeUpdate
tableName := parser.GetTableName()  // "users"
needUndo := parser.NeedUndoLog()   // true
```

---

## 附录

### A. 常量

```go
// 驱动名称
const (
    DriverNameATMySQL = "seata-at-mysql"
    DriverNameXAMySQL = "seata-xa-mysql"
)

// SQL 类型
const (
    SQLTypeSelect = "SELECT"
    SQLTypeInsert = "INSERT"
    SQLTypeUpdate = "UPDATE"
    SQLTypeDelete = "DELETE"
)

// 降级策略
const (
    FallbackNone       FallbackStrategy = iota
    FallbackToLocal
    FallbackToReadOnly
    FallbackReject
)
```

### B. 类型定义

```go
type SQLType string
type FallbackStrategy int

type TableRecords struct {
    TableName string
    Rows      []Row
}

type Row struct {
    Fields []Field
}

type Field struct {
    Name    string
    KeyType int32
    Type    int32
    Value   interface{}
}
```

### C. 错误码

| 错误码 | 说明 |
|-------|------|
| CodeInvalidOperation | 无效操作（如重复初始化） |
| CodeDbOperationError | 数据库操作错误 |
| CodeInternalError | 内部错误 |
