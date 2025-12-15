@echo off
chcp 65001 >nul

echo ==========================================
echo   Seata Example 自动化测试
echo ==========================================
echo.

REM 检查环境
echo 🔍 检查环境...
docker ps | findstr seata-example-mysql >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ MySQL 未运行，请先执行: setup.bat
    pause
    exit /b 1
)

docker ps | findstr seata-example-server >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Seata Server 未运行，请先执行: setup.bat
    pause
    exit /b 1
)

echo ✅ 环境检查通过
echo.

REM 运行测试
echo 🧪 运行测试...
echo.

go test -v -count=1 ./...

echo.
echo ==========================================
echo   测试完成！ ✅
echo ==========================================
echo.
pause
