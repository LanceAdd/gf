# Seata-Go GF Driver 完整设计文档

> **项目名称**: GoFrame Seata 分布式事务驱动  
> **版本**: v2.0  
> **最后更新**: 2025-12-13  
> **文档类型**: 技术设计文档

---

## 目录

- [1. 项目概述](#1-项目概述)
- [2. 架构设计](#2-架构设计)
- [3. 开发步骤](#3-开发步骤)
- [4. 详细实现流程](#4-详细实现流程)
- [5. 注意事项](#5-注意事项)
- [6. 参考资料](#6-参考资料)

---

## 1. 项目概述

### 1.1 项目背景

在微服务架构中，分布式事务是一个核心问题。Seata 作为阿里巴巴开源的分布式事务解决方案，提供了 AT、TCC、SAGA、XA 四种事务模式。本项目旨在为 GoFrame 框架提供 Seata 分布式事务支持。

### 1.2 项目目标

- ✅ 实现 AT 模式（自动补偿）
- ✅ 实现 XA 模式（强一致性）
- ✅ 无侵入式集成 GoFrame
- ✅ 高性能、高可靠
- ✅ 完善的测试和文档

### 1.3 技术选型

| 技术栈 | 版本 | 说明 |
|-------|------|------|
| Go | 1.20+ | 编程语言 |
| GoFrame | v2.x | Web 框架 |
| Seata-Go | latest | Seata Go SDK |
| MySQL | 5.7+ | 数据库（示例） |

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                   应用层 (Application)                    │
│  ┌──────────────────────────────────────────────────┐  │
│  │          GoFrame Business Logic                  │  │
│  │  - Service Layer                                 │  │
│  │  - Controller Layer                              │  │
│  └────────────────┬─────────────────────────────────┘  │
└───────────────────┼─────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────┐
│              数据访问层 (Data Access)                    │
│  ┌──────────────────────────────────────────────────┐  │
│  │         g.DB() / gdb.DB Interface                │  │
│  └────────────────┬─────────────────────────────────┘  │
│                   │                                     │
│  ┌────────────────▼─────────────────────────────────┐  │
│  │              SeataDB (Wrapper)                   │  │
│  │  ┌────────────────────────────────────────────┐ │  │
│  │  │  1. SQL Interceptor                        │ │  │
│  │  │     - DoUpdate() / DoInsert() / DoDelete() │ │  │
│  │  │     - 拦截所有 DML 操作                      │ │  │
│  │  └────────────────────────────────────────────┘ │  │
│  │  ┌────────────────────────────────────────────┐ │  │
│  │  │  2. Image Generator                        │ │  │
│  │  │     - Before Image (执行前数据快照)         │ │  │
│  │  │     - After Image (执行后数据快照)          │ │  │
│  │  └────────────────────────────────────────────┘ │  │
│  │  ┌────────────────────────────────────────────┐ │  │
│  │  │  3. Transaction Manager (SeataTX)          │ │  │
│  │  │     - Undo Log 收集                        │ │  │
│  │  │     - 分支注册                             │ │  │
│  │  │     - 事务提交/回滚                        │ │  │
│  │  └────────────────────────────────────────────┘ │  │
│  └──────────────────────────────────────────────────┘  │
└───────────────────┼─────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────┐
│               Seata SDK 层 (Seata-Go)                   │
│  ┌──────────────────────────────────────────────────┐  │
│  │  TM (Transaction Manager)                        │  │
│  │  - 开启全局事务                                   │  │
│  │  - 提交/回滚全局事务                              │  │
│  └──────────────────────────────────────────────────┘  │
│  ┌──────────────────────────────────────────────────┐  │
│  │  RM (Resource Manager)                           │  │
│  │  - 注册分支事务                                   │  │
│  │  - 处理分支提交/回滚                              │  │
│  │  - 管理资源                                       │  │
│  └──────────────────────────────────────────────────┘  │
└───────────────────┼─────────────────────────────────────┘
                    │
┌───────────────────▼─────────────────────────────────────┐
│              Seata Server (TC - 事务协调器)              │
│  ┌──────────────────────────────────────────────────┐  │
│  │  - 全局事务管理                                   │  │
│  │  - 分支事务协调                                   │  │
│  │  - 全局锁管理                                     │  │
│  │  - 事务状态存储                                   │  │
│  └──────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────┘
```

### 2.2 核心组件设计

#### 2.2.1 驱动层 (Driver Layer)

**职责**: 实现 gdb.Driver 接口，创建 SeataDB 实例

```go
// DriverAT - AT 模式驱动
type DriverAT struct {
    config *Config
}

// 核心方法
func (d *DriverAT) New(core *gdb.Core, node *gdb.ConfigNode) (gdb.DB, error)
```

**设计要点**:
- 实现 gdb.Driver 接口
- 根据配置创建 SeataDB
- 注册资源到 Seata RM
- 支持配置热加载

#### 2.2.2 数据库包装层 (SeataDB)

**职责**: 拦截 SQL 操作，生成 Undo Log

```go
type SeataDB struct {
    *gdb.Core           // 继承 Core 的所有方法
    resource *Resource  // Seata 资源
    config   *Config    // 配置
}

// 核心拦截方法
func (db *SeataDB) DoUpdate(ctx, link, table, data, condition, args) (Result, error)
func (db *SeataDB) DoInsert(ctx, link, table, list, option) (Result, error)
func (db *SeataDB) DoDelete(ctx, link, table, condition, args) (Result, error)
```

**设计要点**:
- 嵌入 gdb.Core，复用 GF 框架能力
- 重写 DoUpdate/DoInsert/DoDelete 方法
- 拦截时机：执行前生成 Before Image，执行后生成 After Image
- 非事务场景：直接调用父类方法

#### 2.2.3 事务对象 (SeataTX)

**职责**: 管理事务内的 Undo Log，处理分支注册

```go
type SeataTX struct {
    gdb.TX              // 继承 GF 事务
    resource     *Resource
    ctx          context.Context
    undoLogItems *garray.Array  // 线程安全的数组
    localTxId    string
    branchId     int64
}

// 核心方法
func (tx *SeataTX) Commit() error
func (tx *SeataTX) Rollback() error
func (tx *SeataTX) AddUndoLog(item *SQLUndoLog)
```

**设计要点**:
- 继承 gdb.TX，保持接口兼容
- 使用 garray.Array 保证并发安全
- Commit 时：插入 Undo Log → 注册分支 → 提交本地事务
- Rollback 时：直接回滚本地事务

#### 2.2.4 Image 生成器 (ImageGenerator)

**职责**: 生成 Before/After Image

```go
type ImageGenerator struct {
    resource *Resource
}

// Before Image: 查询将被修改的数据
func (g *ImageGenerator) GenerateBeforeImage(
    ctx, link, tableName, condition, args
) (*TableRecords, error)

// After Image: 查询已修改的数据
func (g *ImageGenerator) GenerateAfterImage(
    ctx, link, tableName, pkValues
) (*TableRecords, error)
```

**设计要点**:
- Before Image: 执行 `SELECT * FROM table WHERE condition FOR UPDATE`
- After Image: 执行 `SELECT * FROM table WHERE pk IN (...)`
- 必须包含主键字段
- 使用 FOR UPDATE 锁定记录

#### 2.2.5 资源管理器 (Resource)

**职责**: 实现 Seata RM 接口，处理分支提交/回滚

```go
type Resource struct {
    resourceID      string
    branchType      branch.BranchType
    dbType          types.DBType
    db              *sql.DB
    gfCore          *gdb.Core
    config          *Config
    asyncWorker     *AsyncWorker
    retryExecutor   *RetryExecutor
    fallbackManager *FallbackManager
}

// Seata RM 接口
func (r *Resource) BranchCommit(ctx, resource) (BranchStatus, error)
func (r *Resource) BranchRollback(ctx, resource) (BranchStatus, error)
```

**设计要点**:
- 实现 Seata RM 接口
- BranchCommit: 删除 Undo Log
- BranchRollback: 读取 Undo Log 并回滚
- 支持异步提交优化

---

## 3. 开发步骤

### 3.1 总体步骤

```
阶段一：基础框架
    ↓
阶段二：核心功能
    ↓
阶段三：性能优化
    ↓
阶段四：测试完善
```

### 3.2 详细步骤规划

#### 阶段一：基础框架搭建（1-2天）

**目标**: 搭建项目结构，实现基础框架

**步骤**:
1. 创建项目结构
2. 定义配置结构
3. 实现 AT 驱动
4. 实现资源管理器
5. 注册驱动到 GF

**交付物**:
- 可编译的代码
- 基本的配置结构
- 驱动注册成功

#### 阶段二：核心功能开发（3-5天）

**目标**: 实现完整的 AT 模式事务流程

**步骤**:
1. SQL 解析器
2. Image 生成器
3. SQL 拦截器
4. Undo Log 管理
5. 事务提交/回滚

**交付物**:
- 完整的 AT 模式实现
- 单元测试通过
- 集成测试通过

#### 阶段三：性能优化（2-3天）

**目标**: 提升性能和稳定性

**步骤**:
1. 异步工作器
2. XA 模式支持
3. 错误处理增强
4. 代码优化

**交付物**:
- 性能提升 80%+
- 异步提交支持
- XA 模式基础实现

#### 阶段四：测试完善（2-3天）

**目标**: 提升测试覆盖率到 40%+

**步骤**:
1. 单元测试补充
2. 集成测试编写
3. 并发测试验证
4. 文档完善

**交付物**:
- 测试覆盖率 40%+
- 完整文档
- 可上线版本

---

## 4. 详细实现流程

### 4.1 AT 模式事务流程

#### 4.1.1 一阶段：分支注册和执行

```
用户代码:
  g.DB().Transaction(ctx, func(ctx, tx) error {
    tx.Update("UPDATE users SET balance = balance - 100 WHERE id = 1")
    tx.Insert("INSERT INTO orders ...")
    return nil
  })

执行流程:
  1. Begin Transaction
     ├─> 创建 SeataTX 对象
     └─> 初始化 undoLogItems 数组

  2. Execute UPDATE
     ├─> 拦截 DoUpdate()
     ├─> 生成 Before Image
     │   └─> SELECT * FROM users WHERE id = 1 FOR UPDATE
     ├─> 执行 UPDATE
     ├─> 生成 After Image
     │   └─> SELECT * FROM users WHERE id IN (1)
     └─> 保存 Undo Log 到 undoLogItems

  3. Execute INSERT
     ├─> 拦截 DoInsert()
     ├─> 执行 INSERT (获取 lastInsertId)
     ├─> 生成 After Image
     │   └─> SELECT * FROM orders WHERE id = lastInsertId
     └─> 保存 Undo Log 到 undoLogItems

  4. Commit
     ├─> 序列化 Undo Log
     ├─> INSERT INTO undo_log (...)
     ├─> 注册分支到 Seata Server
     │   └─> rm.BranchRegister(xid, resourceId, lockKeys)
     ├─> 提交本地事务
     └─> 报告分支状态 (PhaseoneDone)

  5. 全局事务提交
     └─> Seata Server 通知各分支二阶段提交
```

#### 4.1.2 二阶段：提交

```
Seata Server 调用:
  resource.BranchCommit(ctx, branchResource)

执行流程:
  1. 异步模式（推荐）
     ├─> 加入异步队列
     └─> AsyncWorker 批量删除 Undo Log
         └─> DELETE FROM undo_log WHERE xid = ? AND branch_id = ?

  2. 同步模式
     └─> 直接删除 Undo Log
         └─> DELETE FROM undo_log WHERE xid = ? AND branch_id = ?
```

#### 4.1.3 二阶段：回滚

```
Seata Server 调用:
  resource.BranchRollback(ctx, branchResource)

执行流程:
  1. 读取 Undo Log
     └─> SELECT rollback_info FROM undo_log 
         WHERE xid = ? AND branch_id = ?

  2. 反序列化 Undo Log
     └─> 解析出 SQLUndoLog 列表

  3. 逐个回滚 SQL
     For each SQLUndoLog:
       ├─> 检查数据状态 (防脏检查)
       │   └─> SELECT * FROM table WHERE pk = ? FOR UPDATE
       │
       ├─> 生成回滚 SQL
       │   ├─> UPDATE: 根据 Before Image 生成
       │   ├─> INSERT: 生成 DELETE
       │   └─> DELETE: 根据 Before Image 生成 INSERT
       │
       └─> 执行回滚 SQL

  4. 删除 Undo Log
     └─> DELETE FROM undo_log WHERE xid = ? AND branch_id = ?
```

### 4.2 Image 生成详细流程

#### 4.2.1 Before Image 生成

```sql
-- 场景: UPDATE users SET balance = balance - 100 WHERE id = 1

-- 1. 构建查询 SQL
SELECT * FROM users WHERE id = 1 FOR UPDATE

-- 2. 执行查询获取当前数据
Result: {
  "id": 1,
  "name": "张三",
  "balance": 1000,
  "version": 1
}

-- 3. 转换为 TableRecords
TableRecords {
  TableName: "users",
  Rows: [
    {
      Fields: [
        {Name: "id", KeyType: PRIMARY_KEY, Type: INT, Value: 1},
        {Name: "name", KeyType: COMMON, Type: VARCHAR, Value: "张三"},
        {Name: "balance", KeyType: COMMON, Type: DECIMAL, Value: 1000},
        {Name: "version", KeyType: COMMON, Type: INT, Value: 1}
      ]
    }
  ]
}
```

#### 4.2.2 After Image 生成

```sql
-- 场景: 执行 UPDATE 后

-- 1. 从 Before Image 提取主键值
Primary Keys: [1]

-- 2. 构建查询 SQL
SELECT * FROM users WHERE id IN (1)

-- 3. 执行查询获取修改后的数据
Result: {
  "id": 1,
  "name": "张三",
  "balance": 900,  -- 已更新
  "version": 2      -- 已更新
}

-- 4. 转换为 TableRecords (同 Before Image 结构)
```

### 4.3 锁键生成流程

```
目的: 生成全局锁的键，格式: table:pk1,pk2;table2:pk3

流程:
  1. 遍历 undoLogItems
  2. 提取每个 SQL 的主键值
     ├─> UPDATE/DELETE: 从 Before Image 提取
     └─> INSERT: 从 After Image 提取
  3. 按表名分组主键值
  4. 去重主键值
  5. 格式化为字符串
     └─> "users:1,2;orders:100,101"

示例:
  Input: [
    SQLUndoLog{Table: "users", BeforeImage: {PK: 1}},
    SQLUndoLog{Table: "users", BeforeImage: {PK: 2}},
    SQLUndoLog{Table: "orders", AfterImage: {PK: 100}}
  ]
  
  Output: "users:1,2;orders:100"
```

---

## 5. 注意事项

### 5.1 开发注意事项

#### 5.1.1 Context 管理

**问题**: Context 重复创建导致性能损耗

**解决方案**:
```go
// ❌ 错误：每次都创建
func doSomething() {
    ctx := gctx.GetInitCtx()  // 不好
    // ...
}

// ✅ 正确：参数传递
func doSomething(ctx context.Context) {
    // 使用传入的 ctx
}

// ✅ 正确：全局缓存
var globalCtx = gctx.GetInitCtx()
func doSomething() {
    // 使用 globalCtx
}
```

#### 5.1.2 并发安全

**问题**: undoLogItems 需要并发安全

**解决方案**:
```go
// ❌ 错误：使用普通 slice
type SeataTX struct {
    undoLogItems []* SQLUndoLog  // 不安全
}

// ✅ 正确：使用 garray.Array
type SeataTX struct {
    undoLogItems *garray.Array  // 线程安全
}
```

#### 5.1.3 资源清理

**问题**: 异步工作器需要正确停止

**解决方案**:
```go
worker := NewAsyncWorker(db, config)
worker.Start()

// 确保停止
defer worker.Stop()

// 或在 Resource 销毁时停止
func (r *Resource) Close() error {
    if r.asyncWorker != nil {
        r.asyncWorker.Stop()
    }
    return r.db.Close()
}
```

#### 5.1.4 错误处理

**问题**: 需要正确处理各种错误场景

**解决方案**:
```go
// 使用 GF 的错误码
return gerror.WrapCode(
    gcode.CodeDbOperationError,
    err,
    "failed to generate before image",
)

// 降级处理
err := fallbackManager.ExecuteWithFallback(ctx, db,
    func(ctx context.Context) error {
        // 正常操作
        return normalOperation()
    },
    func(ctx context.Context) error {
        // 降级操作
        return fallbackOperation()
    },
)
```

### 5.2 性能优化注意事项

#### 5.2.1 Image 生成优化

**策略**:
```go
// 1. 只查询必要的字段
// ❌ 不好：查询所有字段
SELECT * FROM large_table WHERE id = 1 FOR UPDATE

// ✅ 更好：只查询需要的字段
SELECT id, balance, version FROM large_table WHERE id = 1 FOR UPDATE

// 2. 批量查询主键
// ❌ 不好：多次查询
for _, pk := range pkValues {
    SELECT * FROM table WHERE id = pk
}

// ✅ 更好：一次查询
SELECT * FROM table WHERE id IN (1,2,3,...)
```

#### 5.2.2 Undo Log 序列化优化

**策略**:
```go
// 使用高效的序列化方式
// 推荐 JSON (兼容性好) 或 Protobuf (性能好)

// JSON 序列化
data, err := gjson.Encode(branchUndoLog)

// 压缩大 Undo Log
if len(data) > 1024*100 { // 100KB
    data = compress(data)
}
```

#### 5.2.3 连接池配置

**策略**:
```yaml
database:
  default:
    maxIdleConnCount: 10   # 最小连接数
    maxOpenConnCount: 100  # 最大连接数
    maxConnLifeTime: 3600  # 连接最大生命周期（秒）
    
# 建议值:
# - QPS < 1000: maxOpen=50, maxIdle=10
# - QPS 1000-5000: maxOpen=100, maxIdle=20
# - QPS > 5000: maxOpen=200+, maxIdle=50
```

### 5.3 测试注意事项

#### 5.3.1 单元测试

**要点**:
```go
// 1. 使用表驱动测试
tests := []struct {
    name     string
    input    string
    expected string
}{
    {"case1", "input1", "output1"},
    {"case2", "input2", "output2"},
}

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        result := function(tt.input)
        if result != tt.expected {
            t.Errorf("Expected %s, got %s", tt.expected, result)
        }
    })
}

// 2. 测试边界情况
- nil 值
- 空值
- 超大值
- 并发场景
```

#### 5.3.2 集成测试

**要点**:
```go
// 1. 使用 testing.Short() 控制跳过
func TestIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    // 测试逻辑
}

// 2. 自动清理测试数据
func TestWithCleanup(t *testing.T) {
    // 创建测试表
    createTestTable()
    
    // 确保清理
    defer dropTestTable()
    
    // 测试逻辑
}

// 3. 优雅处理数据库不可用
db := setupTestDB()
if db == nil {
    t.Skip("Database not available")
}
```

#### 5.3.3 并发测试

**要点**:
```go
func TestConcurrency(t *testing.T) {
    done := make(chan bool)
    
    // 启动多个协程
    for i := 0; i < 100; i++ {
        go func() {
            // 并发操作
            done <- true
        }()
    }
    
    // 等待所有协程完成
    for i := 0; i < 100; i++ {
        <-done
    }
    
    // 验证结果
}
```

### 5.4 部署注意事项

#### 5.4.1 数据库准备

**必须执行**:
```sql
-- 1. 创建 undo_log 表
CREATE TABLE `undo_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `branch_id` bigint(20) NOT NULL,
  `xid` varchar(100) NOT NULL,
  `context` varchar(128) NOT NULL,
  `rollback_info` longblob NOT NULL,
  `log_status` int(11) NOT NULL,
  `log_created` datetime NOT NULL,
  `log_modified` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_undo_log` (`xid`,`branch_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;

-- 2. 创建索引
CREATE INDEX idx_log_created ON undo_log(log_created);

-- 3. 定期清理历史数据
DELETE FROM undo_log 
WHERE log_status = 1 
AND log_modified < DATE_SUB(NOW(), INTERVAL 7 DAY);
```

#### 5.4.2 Seata Server 配置

**配置文件**:
```yaml
# file.conf
store {
  mode = "db"
  db {
    datasource = "druid"
    dbType = "mysql"
    driverClassName = "com.mysql.jdbc.Driver"
    url = "jdbc:mysql://127.0.0.1:3306/seata"
    user = "root"
    password = "root"
  }
}

# registry.conf
registry {
  type = "nacos"
  nacos {
    serverAddr = "localhost:8848"
    namespace = ""
    cluster = "default"
  }
}
```

#### 5.4.3 监控和告警

**关键指标**:
```go
// 1. 事务指标
- 全局事务总数
- 全局事务成功率
- 全局事务平均耗时
- 分支事务总数
- 分支事务成功率

// 2. 性能指标
- Undo Log 生成耗时
- Image 生成耗时
- 二阶段提交耗时
- 二阶段回滚耗时

// 3. 资源指标
- Undo Log 表大小
- 数据库连接数
- 异步队列长度

// 4. 错误指标
- 降级触发次数
- 重试次数
- 超时次数
```

---

## 6. 参考资料

### 6.1 官方文档

1. **Seata 官方文档**
   - 网址: https://seata.io/zh-cn/
   - 重点章节:
     - AT 模式原理
     - XA 模式原理
     - 事务分组配置
     - 全局锁机制

2. **Seata-Go SDK**
   - 仓库: https://github.com/seata/seata-go
   - 重点文件:
     - pkg/tm/transaction_manager.go
     - pkg/rm/resource_manager.go
     - pkg/datasource/sql/at.go

3. **GoFrame 官方文档**
   - 网址: https://goframe.org
   - 重点章节:
     - ORM 使用
     - 数据库配置
     - 驱动开发
     - Context 使用

### 6.2 核心接口

#### 6.2.1 gdb.Driver 接口

```go
// GoFrame 数据库驱动接口
type Driver interface {
    New(core *Core, node *ConfigNode) (DB, error)
}

// 参考文件
// github.com/gogf/gf/v2/database/gdb/gdb_driver.go
```

#### 6.2.2 rm.ResourceManager 接口

```go
// Seata 资源管理器接口
type ResourceManager interface {
    RegisterResource(resource Resource) error
    UnregisterResource(resource Resource)
    BranchRegister(param BranchRegisterParam) (int64, error)
    BranchReport(param BranchReportParam) error
    LockQuery(param LockQueryParam) (bool, error)
}

// 参考文件
// github.com/seata/seata-go/pkg/rm/resource_manager.go
```

#### 6.2.3 Resource 接口

```go
// Seata 资源接口
type Resource interface {
    GetResourceId() string
    GetResourceGroupId() string
    GetBranchType() branch.BranchType
    BranchCommit(ctx context.Context, resource BranchResource) (BranchStatus, error)
    BranchRollback(ctx context.Context, resource BranchResource) (BranchStatus, error)
}

// 参考文件
// github.com/seata/seata-go/pkg/rm/resource.go
```

### 6.3 设计模式

#### 6.3.1 装饰器模式

**应用**: SeataDB 包装 gdb.Core

```go
// 装饰器模式
type SeataDB struct {
    *gdb.Core  // 被装饰对象
    // 额外功能
    resource *Resource
    config   *Config
}

// 增强方法
func (db *SeataDB) DoUpdate(...) {
    // 1. 前置处理（生成 Before Image）
    // 2. 调用原方法
    // 3. 后置处理（生成 After Image）
}
```

#### 6.3.2 策略模式

**应用**: 降级策略

```go
// 策略接口
type FallbackStrategy int

const (
    FallbackNone       // 不降级
    FallbackToLocal    // 降级到本地事务
    FallbackToReadOnly // 降级到只读
    FallbackReject     // 拒绝服务
)

// 策略执行
func (f *FallbackManager) ExecuteWithFallback(
    strategy FallbackStrategy,
    operation func() error,
    fallback func() error,
) error
```

#### 6.3.3 工厂模式

**应用**: 驱动创建

```go
// 工厂方法
func NewDriverAT(config *Config) *DriverAT {
    if config == nil {
        config = DefaultConfig()
    }
    return &DriverAT{config: config}
}

func NewDriverXA(config *Config) *DriverXA {
    if config == nil {
        config = DefaultConfig()
    }
    return &DriverXA{config: config}
}
```

### 6.4 代码规范

#### 6.4.1 命名规范

```go
// 1. 包名：小写，简洁
package seata

// 2. 结构体：大驼峰（公开）
type SeataDB struct {}
type SeataTX struct {}

// 3. 私有结构体：小驼峰
type imageGenerator struct {}

// 4. 接口：大驼峰，以 -er 结尾
type ImageGenerator interface {}

// 5. 方法：大驼峰（公开），小驼峰（私有）
func (db *SeataDB) DoUpdate() {}
func (db *SeataDB) shouldIntercept() {}

// 6. 常量：大驼峰或全大写
const DriverNameATMySQL = "seata-at-mysql"
const SQL_TYPE_UPDATE = "UPDATE"
```

#### 6.4.2 注释规范

```go
// Package seata 提供 GF 框架的 Seata 分布式事务支持
package seata

// SeataDB Seata 数据库包装对象
// 实现了 gdb.DB 接口，拦截 SQL 操作生成 Undo Log
type SeataDB struct {
    // Core GF 数据库核心对象
    *gdb.Core
    
    // resource Seata 资源对象
    resource *Resource
}

// DoUpdate 拦截 UPDATE 操作
// 
// 执行流程:
//  1. 生成 Before Image
//  2. 执行 UPDATE
//  3. 生成 After Image
//  4. 保存 Undo Log
//
// 参数:
//  ctx - 上下文
//  link - 数据库连接
//  table - 表名
//  data - 更新数据
//  condition - 更新条件
//  args - 参数
//
// 返回:
//  result - 执行结果
//  error - 错误信息
func (db *SeataDB) DoUpdate(
    ctx context.Context,
    link gdb.Link,
    table string,
    data interface{},
    condition string,
    args ...interface{},
) (sql.Result, error)
```

#### 6.4.3 错误处理规范

```go
// 1. 使用 GF 错误码
import "github.com/gogf/gf/v2/errors/gcode"
import "github.com/gogf/gf/v2/errors/gerror"

// 2. 包装错误
return gerror.WrapCode(
    gcode.CodeDbOperationError,
    err,
    "failed to generate before image for table: %s", table,
)

// 3. 新建错误
return gerror.NewCode(
    gcode.CodeInvalidOperation,
    "Seata already initialized",
)

// 4. 日志记录
glog.Errorf(ctx, "[Seata] Failed to commit: %v", err)
```

### 6.5 测试数据准备

#### 6.5.1 测试数据库

```sql
-- 创建测试数据库
CREATE DATABASE IF NOT EXISTS seata_test 
DEFAULT CHARACTER SET utf8mb4;

USE seata_test;

-- 创建测试表
CREATE TABLE users (
    id INT PRIMARY KEY AUTO_INCREMENT,
    name VARCHAR(50),
    balance DECIMAL(10,2),
    version INT DEFAULT 1,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE orders (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT,
    amount DECIMAL(10,2),
    status VARCHAR(20),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 插入测试数据
INSERT INTO users (name, balance) VALUES 
('张三', 1000.00),
('李四', 2000.00),
('王五', 3000.00);
```

#### 6.5.2 Mock 数据

```go
// Mock DB
type mockDB struct {
    gdb.DB
    queryCount int
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...interface{}) (gdb.Result, error) {
    m.queryCount++
    return nil, nil
}

// Mock Resource
func newMockResource() *Resource {
    return &Resource{
        resourceID: "test-resource",
        branchType: branch.BranchTypeAT,
        dbType:     types.DBTypeMySQL,
        config:     DefaultConfig(),
    }
}
```

---

## 7. 附录

### 7.1 完整配置示例

```yaml
# config.yaml
server:
  address: ":8199"

database:
  default:
    type: "seata-at-mysql"
    host: "127.0.0.1"
    port: "3306"
    user: "root"
    pass: "root"
    name: "test"
    charset: "utf8mb4"
    maxIdleConnCount: 10
    maxOpenConnCount: 100
    maxConnLifeTime: 3600

seata:
  enabled: true
  application-id: "my-app"
  tx-service-group: "default_tx_group"
  mode: "AT"
  
  at:
    enable-async-commit: true
    only-care-primary-key: true
    undo-log-serialization: "jackson"
    undo-log-table: "undo_log"
  
  fallback:
    enabled: true
    strategy: "to-local"
    error-threshold: 5
    time-window: 60
    recovery-timeout: 30
  
  retry:
    max-attempts: 3
    interval: 100
    max-interval: 1000
    multiplier: 2.0
```

### 7.2 性能测试脚本

```go
// benchmark_test.go
func BenchmarkSeataTransaction(b *testing.B) {
    db := setupTestDB()
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        err := db.Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
            _, err := tx.Update(ctx, "users", gdb.Map{
                "balance": gdb.Raw("balance - 1"),
            }, "id", 1)
            return err
        })
        
        if err != nil {
            b.Fatal(err)
        }
    }
}

// 运行: go test -bench=. -benchmem
```

### 7.3 故障排查指南

**问题1**: Undo Log 未生成

**排查步骤**:
1. 检查是否在全局事务中
2. 检查 SQL 是否被拦截
3. 检查日志输出
4. 验证数据库配置

**问题2**: 分支注册失败

**排查步骤**:
1. 检查 Seata Server 连接
2. 检查资源 ID 配置
3. 检查锁键生成
4. 查看 Seata Server 日志

**问题3**: 二阶段回滚失败

**排查步骤**:
1. 检查 Undo Log 是否存在
2. 检查数据是否被修改（防脏检查）
3. 验证回滚 SQL 生成
4. 检查数据库权限

---

## 总结

本文档提供了 Seata-Go GF Driver 的完整设计方案，包括：

✅ **架构设计**: 清晰的分层架构，明确的组件职责  
✅ **开发步骤**: 分阶段实施，循序渐进  
✅ **详细流程**: 完整的事务执行流程和 Image 生成流程  
✅ **注意事项**: 开发、性能、测试、部署全方位覆盖  
✅ **参考资料**: 官方文档、核心接口、设计模式、代码规范  

遵循本文档可以：
- 🎯 快速理解项目架构
- 🚀 高效完成开发任务
- ✅ 避免常见开发陷阱
- 📈 实现高性能高可靠

**文档版本**: v2.0  
**最后更新**: 2025-12-13
