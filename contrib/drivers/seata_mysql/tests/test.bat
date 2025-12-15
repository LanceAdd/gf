@echo off

set MODE=%1
if "%MODE%"=="" set MODE=check

if "%MODE%"=="check" (
    echo 🔍 检查测试环境...
    docker ps | findstr mysql >nul || (echo ❌ MySQL 未运行 && exit /b 1)
    docker ps | findstr seata >nul || (echo ❌ Seata 未运行 && exit /b 1)
    echo ✅ 环境正常
) else if "%MODE%"=="unit" (
    echo 🧪 运行单元测试...
    go test -v ./unit/...
) else if "%MODE%"=="integration" (
    echo 🔗 运行集成测试...
    go test -v ./integration/... -timeout 300s
) else if "%MODE%"=="all" (
    echo 🚀 运行完整测试套件...
    go test -v ./... -timeout 600s
) else (
    echo 用法: %0 {check^|unit^|integration^|all}
    echo   check      - 检查环境
    echo   unit       - 单元测试
    echo   integration- 集成测试
    echo   all        - 完整测试
    exit /b 1
)