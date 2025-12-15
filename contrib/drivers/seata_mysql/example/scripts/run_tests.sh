#!/bin/bash
set -e

echo "=========================================="
echo "  Seata Example 自动化测试"
echo "=========================================="
echo ""

# 检查环境
echo "🔍 检查环境..."
if ! docker ps | grep -q seata-example-mysql; then
    echo "❌ MySQL 未运行，请先执行: ./setup.sh"
    exit 1
fi

if ! docker ps | grep -q seata-example-server; then
    echo "❌ Seata Server 未运行，请先执行: ./setup.sh"
    exit 1
fi

echo "✅ 环境检查通过"
echo ""

# 运行测试
echo "🧪 运行测试..."
echo ""

go test -v -count=1 ./...

echo ""
echo "=========================================="
echo "  测试完成！ ✅"
echo "=========================================="
