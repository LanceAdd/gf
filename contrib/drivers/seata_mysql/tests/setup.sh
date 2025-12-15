#!/bin/bash

set -e

echo "🚀 设置 Seata MySQL 测试环境..."

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ Docker 未安装，请先安装 Docker"
    exit 1
fi

# 检查 Docker Compose
if ! command -v docker-compose &> /dev/null; then
    echo "❌ Docker Compose 未安装，请先安装 Docker Compose"
    exit 1
fi

# 启动环境
echo "📦 启动 MySQL 和 Seata 服务..."
cd "$(dirname "$0")"  # 切换到脚本所在目录
docker-compose -f setup/docker-compose.yml up -d

# 等待服务启动
echo "⏳ 等待 MySQL 服务启动..."
sleep 10

# 等待 MySQL 完全就绪
echo "⏳ 检查 MySQL 连接..."
for i in {1..30}; do
    if docker exec seata_mysql_test mysqladmin ping -h localhost -uroot -p123456 --silent > /dev/null 2>&1; then
        echo "✅ MySQL 已就绪"
        break
    fi
    echo "等待 MySQL 启动... ($i/30)"
    sleep 2
done

# 初始化数据库
echo "🗄️ 初始化数据库..."
docker exec -i seata_mysql_test mysql -uroot -p123456 < setup/init.sql

# 等待 Seata 服务启动
echo "⏳ 等待 Seata 服务启动..."
sleep 10

for i in {1..30}; do
    if docker logs seata_server_test 2>&1 | grep -q "Server started"; then
        echo "✅ Seata 已就绪"
        break
    fi
    echo "等待 Seata 启动... ($i/30)"
    sleep 2
done

echo "✅ 环境设置完成！"
echo "📊 MySQL: 127.0.0.1:3306"
echo "🔧 Seata: 127.0.0.1:8091"