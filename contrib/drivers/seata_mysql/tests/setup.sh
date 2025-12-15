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
docker-compose -f setup/docker-compose.yml up -d

# 等待服务启动
echo "⏳ 等待服务启动..."
sleep 30

# 初始化数据库
echo "🗄️ 初始化数据库..."
mysql -h127.0.0.1 -P3306 -uroot -p123456 < setup/init.sql

echo "✅ 环境设置完成！"
echo "📊 MySQL: 127.0.0.1:3306"
echo "🔧 Seata: 127.0.0.1:8091"