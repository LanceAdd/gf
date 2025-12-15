# Seata MySQL 驱动测试说明

## 测试文件结构

所有测试已合并到单一测试文件：`seata_test.go`

## 测试分类

### 1. 单元测试（Unit Tests）

这些测试不需要数据库连接，可以快速运行：

- `Test_Config` - 测试配置功能
- `Test_Driver_Basic` - 测试基础驱动功能
- `Test_AsyncWorker` - 测试异步工作器
- `Test_AsyncWorker_BranchCommit` - 测试分支提交
- `Test_ImageGeneration` - 测试镜像生成功能
- `Test_Fallback` - 测试降级机制
- `Test_Retry` - 测试重试机制
- `Test_Context` - 测试事务上下文
- `Test_Metrics` - 测试指标收集

### 2. 集成测试（Integration Tests）

这些测试需要数据库和 Seata Server 连接：

- `TestIntegration_BasicConnectivity` - 测试基本数据库连接
- `TestSeataAT_BasicTransfer` - AT模式基础转账测试
- `TestSeataAT_ConcurrentTransfer` - 并发转账测试
- `TestSeataAT_NestedTransactionBasic` - 基础嵌套事务测试
- `TestSeataAT_TransactionRollback` - 事务回滚测试
- `TestSeataXA_BasicTransfer` - XA模式基础转账测试

## 运行测试

### 运行单元测试（不需要数据库）

```bash
cd gf/contrib/drivers/seata_mysql/tests
go test -v -short
```

### 运行特定测试

```bash
# 运行所有单元测试
go test -v -run "Test_Config|Test_Driver_Basic|Test_AsyncWorker|Test_ImageGeneration|Test_Fallback|Test_Retry|Test_Context|Test_Metrics" -short

# 运行所有集成测试（需要数据库和 Seata Server）
go test -v -run "TestIntegration_|TestSeataAT_|TestSeataXA_"
```

### 运行所有测试

```bash
go test -v
```

## 测试前准备

### 集成测试环境准备

1. 启动 MySQL 数据库
2. 启动 Seata Server
3. 初始化测试数据库

```bash
# 使用 Docker Compose 启动环境
cd setup
docker-compose up -d

# 或使用脚本
./setup.sh  # Linux/Mac
./setup.bat # Windows
```

## 测试结果

最新测试结果：

```
=== 单元测试 ===
✅ Test_Config
✅ Test_Driver_Basic
✅ Test_AsyncWorker
✅ Test_AsyncWorker_BranchCommit
✅ Test_ImageGeneration
✅ Test_Fallback
✅ Test_Retry
✅ Test_Context
✅ Test_Metrics

所有单元测试通过！
```

## 清理测试环境

```bash
# 清理测试数据
./cleanup.sh  # Linux/Mac
./cleanup.bat # Windows

# 停止 Docker 容器
cd setup
docker-compose down
```

## 注意事项

1. 单元测试可以在没有数据库的情况下运行
2. 集成测试需要完整的测试环境（MySQL + Seata Server）
3. 使用 `-short` 标志可以跳过集成测试
4. 确保测试环境配置正确（参考 `setup/seata.yml`）

## 故障排除

如果测试失败，请检查：

1. 数据库是否正常运行
2. Seata Server 是否正常运行
3. 配置文件路径是否正确
4. 测试数据库是否已初始化
5. 网络连接是否正常
