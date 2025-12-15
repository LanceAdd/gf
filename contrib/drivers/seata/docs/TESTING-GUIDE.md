# Seata-Go GF Driver 测试指南

> **版本**: v2.0  
> **测试覆盖率**: 40.9%  
> **最后更新**: 2025-12-13

---

## 目录

- [1. 测试概述](#1-测试概述)
- [2. 运行测试](#2-运行测试)
- [3. 测试类型](#3-测试类型)
- [4. 编写测试](#4-编写测试)
- [5. 测试覆盖率](#5-测试覆盖率)

---

## 1. 测试概述

### 1.1 测试统计

- **测试文件**: 15个
- **测试代码**: 3,740行
- **测试用例**: 172个
- **覆盖率**: **40.9%**

### 1.2 测试分类

| 类型 | 数量 | 说明 |
|-----|------|------|
| 单元测试 | 157 | 测试独立函数和组件 |
| 集成测试 | 5 | 测试数据库操作 |
| 并发测试 | 8 | 测试线程安全 |
| 边界测试 | 12 | 测试边界情况 |

---

## 2. 运行测试

### 2.1 运行所有测试

```bash
# 标准测试
go test -v -timeout 30s

# 带覆盖率
go test -v -coverprofile=coverage.out -timeout 30s

# 查看覆盖率
go tool cover -html=coverage.out
```

### 2.2 运行特定测试

```bash
# 运行指定测试
go test -v -run TestSeataTX_AddUndoLog

# 运行匹配模式的测试
go test -v -run "TestSeata.*"

# 跳过集成测试
go test -v -short -timeout 30s
```

### 2.3 集成测试

**前提条件**：
- MySQL 数据库（127.0.0.1:3306）
- 用户名/密码: root/root
- 数据库: test

**运行集成测试**：
```bash
go test -v -run TestIntegration -timeout 30s
```

---

## 3. 测试类型

### 3.1 单元测试示例

```go
func TestSeataTX_AddUndoLog(t *testing.T) {
    tx := &SeataTX{
        ctx: context.Background(),
    }
    
    // 添加 undo log
    item := &SQLUndoLog{
        SQLType:   SQLTypeUpdate,
        TableName: "users",
    }
    tx.AddUndoLog(item)
    
    // 验证
    if tx.undoLogItems.Len() != 1 {
        t.Errorf("Expected 1 item, got %d", tx.undoLogItems.Len())
    }
}
```

### 3.2 表驱动测试

```go
func TestDriverAT_BuildDSN(t *testing.T) {
    tests := []struct {
        name     string
        node     *gdb.ConfigNode
        expected string
    }{
        {
            name: "basic config",
            node: &gdb.ConfigNode{
                User: "root",
                Pass: "root",
                Host: "127.0.0.1",
                Port: "3306",
                Name: "test_db",
            },
            expected: "root:root@tcp(127.0.0.1:3306)/test_db",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := buildDSN(tt.node)
            if result != tt.expected {
                t.Errorf("Expected %s, got %s", tt.expected, result)
            }
        })
    }
}
```

### 3.3 并发测试

```go
func TestSeataTX_ConcurrentAddUndoLog(t *testing.T) {
    tx := &SeataTX{
        ctx: context.Background(),
    }
    
    done := make(chan bool)
    
    // 10个协程并发添加
    for i := 0; i < 10; i++ {
        go func() {
            for j := 0; j < 5; j++ {
                tx.AddUndoLog(&SQLUndoLog{
                    SQLType: SQLTypeUpdate,
                })
            }
            done <- true
        }()
    }
    
    // 等待完成
    for i := 0; i < 10; i++ {
        <-done
    }
    
    // 验证
    if tx.undoLogItems.Len() != 50 {
        t.Errorf("Expected 50, got %d", tx.undoLogItems.Len())
    }
}
```

### 3.4 集成测试

```go
func TestIntegration_BasicConnection(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // 配置数据库
    gdb.SetConfig(gdb.Config{
        "default": gdb.ConfigGroup{
            gdb.ConfigNode{
                Host: "127.0.0.1",
                Port: "3306",
                User: "root",
                Pass: "root",
                Name: "test",
            },
        },
    })
    
    db := g.DB()
    
    // 测试连接
    err := db.PingMaster()
    if err != nil {
        t.Skipf("Cannot connect: %v", err)
    }
    
    // 执行查询
    result, err := db.Query(ctx, "SELECT 1")
    if err != nil {
        t.Fatalf("Query failed: %v", err)
    }
}
```

---

## 4. 编写测试

### 4.1 测试命名规范

```go
// 格式: Test<FunctionName>
func TestAddUndoLog(t *testing.T) {}

// 格式: Test<StructName>_<MethodName>
func TestSeataTX_Commit(t *testing.T) {}

// 格式: Test<Feature>_<Scenario>
func TestTransaction_Rollback(t *testing.T) {}
```

### 4.2 子测试组织

```go
func TestConfig(t *testing.T) {
    t.Run("default config", func(t *testing.T) {
        config := DefaultConfig()
        // 测试默认配置
    })
    
    t.Run("custom config", func(t *testing.T) {
        config := &Config{Enabled: true}
        // 测试自定义配置
    })
}
```

### 4.3 测试辅助函数

```go
// 创建测试数据库配置
func setupTestDB(t *testing.T) gdb.DB {
    gdb.SetConfig(gdb.Config{
        "test": gdb.ConfigGroup{
            gdb.ConfigNode{
                Host: "127.0.0.1",
                Port: "3306",
                User: "root",
                Pass: "root",
                Name: "test",
            },
        },
    })
    
    db := g.DB("test")
    if db == nil {
        t.Skip("Database not available")
    }
    
    return db
}

// 清理测试数据
func cleanupTestData(t *testing.T, db gdb.DB, tables ...string) {
    for _, table := range tables {
        _, err := db.Exec(context.Background(), 
            fmt.Sprintf("DROP TABLE IF EXISTS %s", table))
        if err != nil {
            t.Logf("Failed to drop table %s: %v", table, err)
        }
    }
}
```

### 4.4 Mock 对象

```go
type mockDB struct {
    gdb.DB
    queryCalled bool
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...interface{}) (gdb.Result, error) {
    m.queryCalled = true
    return nil, nil
}

func TestWithMock(t *testing.T) {
    mock := &mockDB{}
    
    // 使用 mock 对象
    _, _ = mock.Query(context.Background(), "SELECT 1")
    
    if !mock.queryCalled {
        t.Error("Query should be called")
    }
}
```

---

## 5. 测试覆盖率

### 5.1 当前覆盖率

| 文件 | 覆盖率 | 状态 |
|-----|--------|------|
| image_generator.go | 100% | 🟢 优秀 |
| sql_parser.go | 85% | 🟢 良好 |
| retry.go | 85% | 🟢 良好 |
| async_worker.go | 80% | 🟡 中等 |
| fallback.go | 65%+ | 🟡 中等 |
| config.go | 55%+ | 🟡 中等 |
| resource.go | 50%+ | 🟡 中等 |
| transaction.go | 45%+ | 🟠 基础 |
| driver_at.go | 40%+ | 🟠 基础 |
| db.go | 35%+ | 🔵 基础 |

### 5.2 提升覆盖率

**1. 识别未覆盖代码**
```bash
go tool cover -func=coverage.out | grep "0.0%"
```

**2. 补充测试用例**
- 测试边界情况
- 测试错误处理
- 测试并发场景

**3. 验证覆盖率提升**
```bash
go test -coverprofile=coverage.out
go tool cover -func=coverage.out | grep "total"
```

### 5.3 覆盖率目标

- ✅ **已达成**: 40.9%
- 🎯 **短期目标**: 50%
- 🎯 **长期目标**: 60%

---

## 附录

### A. 测试最佳实践

1. **每个功能都要有测试**
2. **测试要独立，不依赖执行顺序**
3. **使用表驱动测试覆盖多种场景**
4. **边界情况和错误处理要测试**
5. **并发代码要有并发测试**
6. **集成测试要能自动跳过**

### B. 常用命令

```bash
# 运行测试
go test -v

# 运行带覆盖率的测试
go test -v -coverprofile=coverage.out

# 查看覆盖率报告
go tool cover -html=coverage.out

# 运行指定测试
go test -v -run TestName

# 跳过长时间测试
go test -v -short

# 并行运行测试
go test -v -parallel 4

# 竞态检测
go test -v -race
```

### C. 持续集成

示例 GitHub Actions 配置：

```yaml
name: Tests

on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Set up Go
        uses: actions/setup-go@v2
        with:
          go-version: 1.20
      
      - name: Run tests
        run: go test -v -coverprofile=coverage.out ./...
      
      - name: Upload coverage
        uses: codecov/codecov-action@v2
        with:
          file: ./coverage.out
```
