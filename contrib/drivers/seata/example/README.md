# Seata Driver 示例和测试

本目录包含 GoFrame Seata Driver 的示例和自动化测试。

## 📁 目录结构

```
example/
├── README.md              # 本文件
├── docker/                # Docker 环境配置
│   ├── docker-compose.yml # Docker Compose 配置
│   └── init.sql           # 数据库初始化脚本
├── scripts/               # 自动化脚本
│   ├── setup.sh           # Linux/Mac 环境搭建脚本
│   ├── setup.bat          # Windows 环境搭建脚本
│   ├── run_tests.sh       # Linux/Mac 测试运行脚本
│   └── run_tests.bat      # Windows 测试运行脚本
├── config/                # Seata 配置文件
│   ├── file.conf          # Seata 客户端配置
│   └── registry.conf      # Seata 注册中心配置
├── common_test.go         # 公共测试辅助函数
└── seata_at_test.go       # AT 模式测试（使用 Seata Driver）
```

## 🚀 快速开始（3步）

### 步骤1: 启动环境

```bash
# Windows 用户
cd scripts
setup.bat

# Linux/Mac 用户
cd scripts
chmod +x setup.sh
./setup.sh
```

这将自动启动：
- ✅ MySQL 数据库（端口 3306）
- ✅ Seata Server（端口 8091）
- ✅ 初始化数据表和测试数据

### 步骤2: 运行测试

```bash
# 返回 example 目录
cd ..

# 运行所有测试
go test -v

# 或者运行指定测试
go test -v -run TestSeataAT_BasicTransfer
```

### 步骤3: 查看结果

测试会输出详细的日志，包括：
- Seata 初始化过程
- 事务执行过程
- 数据变化情况
- 测试结果验证

## 📋 测试列表

| 测试名称 | 说明 | 验证点 |
|---------|------|--------|
| `TestSeataAT_BasicTransfer` | AT模式基础转账 | ✅ Seata Driver 使用<br>✅ 事务提交<br>✅ 数据一致性 |
| `TestSeataAT_TransferRollback` | AT模式转账回滚 | ✅ 事务回滚<br>✅ 数据恢复 |
| `TestSeataAT_CreateOrder` | AT模式下单场景 | ✅ 多表事务<br>✅ 库存扣减<br>✅ 订单创建 |
| `TestSeataAT_InsufficientStock` | AT模式库存不足 | ✅ 业务验证<br>✅ 错误处理 |

## 🎯 测试特点

### 1. 真正使用 Seata Driver ✅

所有测试都真正使用了 Seata Driver，而不是普通的 MySQL 驱动：

```go
// 配置数据库使用 Seata AT 驱动
gdb.SetConfig(gdb.Config{
    "default": gdb.ConfigGroup{
        gdb.ConfigNode{
            Type: "seata-at-mysql",  // ← 使用 Seata Driver
            Link: "root:root@tcp(127.0.0.1:3306)/seata_demo",
        },
    },
})
```

### 2. 完整的事务场景

- **单表事务**: 账户转账
- **多表事务**: 下单（订单+库存+账户）
- **事务回滚**: 模拟失败场景
- **业务验证**: 库存检查、余额检查

### 3. 数据一致性验证

每个测试都会验证：
- 事务前数据状态
- 事务执行过程
- 事务后数据状态
- 数据变化的正确性

## 🔧 环境要求

### 必需
- **Docker** & **Docker Compose** - 用于运行 MySQL 和 Seata Server
- **Go 1.18+** - 用于运行测试

### 可选
- **GoLand** 或其他 IDE - 更好的开发体验

## 📊 数据库信息

测试使用的数据库结构：

### 表: accounts（账户表）
```sql
CREATE TABLE accounts (
    id INT PRIMARY KEY,
    user_id INT,
    balance DECIMAL(10, 2),
    INDEX idx_user_id (user_id)
);
```

### 表: products（产品表）
```sql
CREATE TABLE products (
    id INT PRIMARY KEY,
    name VARCHAR(255),
    price DECIMAL(10, 2),
    stock INT
);
```

### 表: orders（订单表）
```sql
CREATE TABLE orders (
    id INT PRIMARY KEY AUTO_INCREMENT,
    user_id INT,
    product_id INT,
    quantity INT,
    total_price DECIMAL(10, 2),
    status VARCHAR(20)
);
```

### 表: undo_log（Seata AT 模式回滚日志表）
```sql
CREATE TABLE undo_log (
    id BIGINT(20) NOT NULL AUTO_INCREMENT,
    branch_id BIGINT(20) NOT NULL,
    xid VARCHAR(100) NOT NULL,
    context VARCHAR(128) NOT NULL,
    rollback_info LONGBLOB NOT NULL,
    log_status INT(11) NOT NULL,
    log_created DATETIME NOT NULL,
    log_modified DATETIME NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY ux_undo_log (xid, branch_id)
);
```

## 🐛 故障排查

### 问题1: 容器启动失败

```bash
# 查看容器状态
docker ps -a

# 查看日志
docker logs seata-example-mysql
docker logs seata-example-server

# 重新启动
cd scripts
./setup.sh  # 或 setup.bat
```

### 问题2: 数据库连接失败

```bash
# 检查 MySQL 是否就绪
docker exec seata-example-mysql mysql -uroot -proot -e "SHOW DATABASES;"

# 检查网络
netstat -an | grep 3306
```

### 问题3: Seata Server 未就绪

```bash
# 检查 Seata Server 日志
docker logs -f seata-example-server

# 检查端口
netstat -an | grep 8091
```

### 问题4: 测试失败

```bash
# 清理环境重新开始
cd scripts
./setup.sh  # 或 setup.bat

cd ..
go test -v -count=1  # -count=1 禁用测试缓存
```

## 📝 注意事项

1. **首次运行** - 需要下载 Docker 镜像，可能需要几分钟
2. **端口占用** - 确保 3306 和 8091 端口未被占用
3. **网络环境** - 需要能够访问 Docker Hub
4. **测试顺序** - 测试之间会重置数据，可以任意顺序运行

## 🎓 学习建议

1. **先看测试代码** - `seata_at_test.go` 包含了完整的使用示例
2. **运行单个测试** - 理解每个测试的目的
3. **查看日志** - 了解 Seata 的工作流程
4. **修改数据** - 尝试不同的业务场景
5. **查看数据库** - 观察 undo_log 表的变化

## 🔗 相关文档

- [GoFrame 官方文档](https://goframe.org)
- [Seata 官方文档](https://seata.io)
- [Seata-Go SDK](https://github.com/seata/seata-go)

## 💡 提示

如果你想了解更多 Seata AT 模式的工作原理，可以：

1. 在测试中添加断点，观察事务执行过程
2. 查看 `undo_log` 表，了解回滚日志的结构
3. 阅读 Seata Driver 的源码（`../seata_driver_at.go`）

---

**祝您使用愉快！** 🎉
