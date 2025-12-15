#!/bin/bash

set -e

MODE=${1:-"check"}

case $MODE in
    "check")
        echo "🔍 检查测试环境..."
        docker ps | grep mysql || (echo "❌ MySQL 未运行" && exit 1)
        docker ps | grep seata || (echo "❌ Seata 未运行" && exit 1)
        echo "✅ 环境正常"
        ;;
    "unit")
        echo "🧪 运行单元测试..."
        go test -v ./unit/...
        ;;
    "integration")
        echo "🔗 运行集成测试..."
        go test -v ./integration/... -timeout 300s
        ;;
    "all")
        echo "🚀 运行完整测试套件..."
        go test -v ./... -timeout 600s
        ;;
    *)
        echo "用法: $0 {check|unit|integration|all}"
        echo "  check      - 检查环境"
        echo "  unit       - 单元测试"
        echo "  integration- 集成测试"
        echo "  all        - 完整测试"
        exit 1
        ;;
esac