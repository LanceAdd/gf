# Docker 环境配置

本目录包含运行 Seata 示例所需的 Docker 环境配置。

## 文件说明

- `docker-compose.yml` - Docker Compose 配置文件，定义 MySQL 和 Seata Server 服务
- `init.sql` - MySQL 数据库初始化脚本，创建测试所需的表和数据

## 服务说明

### MySQL 数据库

- **镜像**: mysql:8.0
- **容器名**: seata-example-mysql
- **端口**: 3306
- **数据库**: seata_demo
- **用户**: root / root
- **字符集**: utf8mb4

### Seata Server

- **镜像**: seataio/seata-server:1.7.1
- **容器名**: seata-example-server
- **端口**: 8091 (服务端口), 7091 (管理端口)
- **存储模式**: file（文件模式，适合测试）

## 使用方法

### 启动服务

```bash
# 在 example 目录下执行
docker-compose -f docker/docker-compose.yml up -d
```

### 停止服务

```bash
docker-compose -f docker/docker-compose.yml down
```

### 清理数据（包括数据卷）

```bash
docker-compose -f docker/docker-compose.yml down -v
```

### 查看日志

```bash
# MySQL 日志
docker logs -f seata-example-mysql

# Seata Server 日志
docker logs -f seata-example-server
```

## 数据初始化

`init.sql` 脚本会在 MySQL 首次启动时自动执行，创建以下表：

1. **accounts** - 账户表（用于转账测试）
2. **products** - 产品表（用于下单测试）
3. **orders** - 订单表（用于下单测试）
4. **undo_log** - Seata AT 模式回滚日志表

同时会插入测试数据：
- 2个测试账户（余额分别为 10000 和 5000）
- 3个测试产品

## 健康检查

MySQL 容器配置了健康检查，确保数据库完全就绪后才会启动 Seata Server。

```yaml
healthcheck:
  test: ["CMD", "mysqladmin", "ping", "-h", "localhost", "-uroot", "-proot"]
  interval: 5s
  timeout: 3s
  retries: 10
```

## 故障排查

### 端口已被占用

如果 3306 或 8091 端口已被占用，可以修改 `docker-compose.yml` 中的端口映射：

```yaml
ports:
  - "13306:3306"  # 使用本地的 13306 端口
```

### MySQL 启动失败

查看日志：
```bash
docker logs seata-example-mysql
```

常见原因：
- 端口被占用
- 权限问题
- 磁盘空间不足

### Seata Server 启动失败

查看日志：
```bash
docker logs seata-example-server
```

常见原因：
- MySQL 未就绪
- 端口被占用
- 配置文件错误

## 数据持久化

MySQL 数据存储在 Docker volume 中：

```bash
# 查看 volume
docker volume ls | grep seata

# 删除 volume（清空所有数据）
docker volume rm seata_mysql_data
```

---

**注意**: 这是测试环境配置，生产环境请参考 Seata 官方文档进行配置。
