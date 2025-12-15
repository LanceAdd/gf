#!/bin/bash

set -e

echo "🧹 清理 Seata MySQL 测试环境..."

# 停止并删除容器
echo "🛑 停止服务..."
docker-compose -f setup/docker-compose.yml down -v

# 删除镜像（可选）
read -p "是否删除相关镜像？(y/N): " -n 1 -r
echo
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🗑️ 删除镜像..."
    docker rmi seata-mysql-mysql 2>/dev/null || true
    docker rmi seata/seata-server:1.7.1 2>/dev/null || true
fi

echo "✅ 清理完成！"