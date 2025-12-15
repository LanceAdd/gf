#!/bin/bash

set -e

cd "$(dirname "$0")/.."  # 切换到项目根目录

echo "🚀 运行 Seata MySQL 驱动测试套件..."
echo "================================================"
go test -v -timeout 600s .