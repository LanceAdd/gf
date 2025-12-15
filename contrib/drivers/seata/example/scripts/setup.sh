#!/bin/bash
set -e

echo "=========================================="
echo "  Seata Example 环境搭建脚本"
echo "=========================================="
echo ""

# 检查 Docker
if ! command -v docker &> /dev/null; then
    echo "❌ 错误: 未找到 Docker，请先安装 Docker"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo "❌ 错误: 未找到 Docker Compose"
    exit 1
fi

echo "✅ Docker 环境检查通过"
echo ""

# 停止旧容器
echo "🔄 清理旧环境..."
cd ..
docker-compose -f docker/docker-compose.yml down -v 2>/dev/null || true
cd scripts

# 启动服务
echo ""
echo "🚀 启动服务（MySQL + Seata Server）..."
cd ..
docker-compose -f docker/docker-compose.yml up -d
cd scripts

# 等待 MySQL 就绪
echo ""
echo "⏳ 等待 MySQL 启动..."
for i in {1..30}; do
    if docker exec seata-example-mysql mysqladmin ping -h localhost -uroot -proot --silent 2>/dev/null; then
        echo "✅ MySQL 已就绪"
        break
    fi
    if [ $i -eq 30 ]; then
        echo "❌ MySQL 启动超时"
        exit 1
    fi
    sleep 1
done

# 等待 Seata Server 就绪
echo ""
echo "⏳ 等待 Seata Server 启动..."
sleep 10

if docker ps | grep seata-example-server | grep -q Up; then
    echo "✅ Seata Server 已就绪"
else
    echo "❌ Seata Server 启动失败"
    docker logs seata-example-server
    exit 1
fi

# 显示状态
echo ""
echo "=========================================="
echo "  环境启动成功！ ✅"
echo "=========================================="
echo ""
echo "📊 服务信息:"
echo "  MySQL:"
echo "    - 地址: 127.0.0.1:3306"
echo "    - 用户: root"
echo "    - 密码: root"
echo "    - 数据库: seata_demo"
echo ""
echo "  Seata Server:"
echo "    - 地址: 127.0.0.1:8091"
echo "    - 管理端口: 127.0.0.1:7091"
echo ""
echo "🎯 下一步: 运行测试"
echo "   go test -v"
echo ""
