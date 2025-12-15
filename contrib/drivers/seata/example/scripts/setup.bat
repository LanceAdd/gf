@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

echo ==========================================
echo   Seata Example 环境搭建脚本
echo ==========================================
echo.

REM 检查 Docker
where docker >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ 错误: 未找到 Docker，请先安装 Docker Desktop
    pause
    exit /b 1
)

where docker-compose >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ 错误: 未找到 Docker Compose
    pause
    exit /b 1
)

echo ✅ Docker 环境检查通过
echo.

REM 停止旧容器
echo 🔄 清理旧环境...
cd ..
docker-compose -f docker/docker-compose.yml down -v 2>nul
cd scripts

REM 启动服务
echo.
echo 🚀 启动服务（MySQL + Seata Server）...
cd ..
docker-compose -f docker/docker-compose.yml up -d
cd scripts

REM 等待 MySQL 就绪
echo.
echo ⏳ 等待 MySQL 启动...
timeout /t 5 /nobreak >nul

set /a count=0
:wait_mysql
docker exec seata-example-mysql mysqladmin ping -h localhost -uroot -proot --silent >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ MySQL 已就绪
    goto mysql_ready
)
set /a count+=1
if %count% geq 30 (
    echo ❌ MySQL 启动超时
    pause
    exit /b 1
)
timeout /t 1 /nobreak >nul
goto wait_mysql

:mysql_ready
REM 等待 Seata Server 就绪
echo.
echo ⏳ 等待 Seata Server 启动...
timeout /t 10 /nobreak >nul

docker ps | findstr seata-example-server | findstr Up >nul 2>&1
if %errorlevel% equ 0 (
    echo ✅ Seata Server 已就绪
) else (
    echo ❌ Seata Server 启动失败
    docker logs seata-example-server
    pause
    exit /b 1
)

REM 显示状态
echo.
echo ==========================================
echo   环境启动成功！ ✅
echo ==========================================
echo.
echo 📊 服务信息:
echo   MySQL:
echo     - 地址: 127.0.0.1:3306
echo     - 用户: root
echo     - 密码: root
echo     - 数据库: seata_demo
echo.
echo   Seata Server:
echo     - 地址: 127.0.0.1:8091
echo     - 管理端口: 127.0.0.1:7091
echo.
echo 🎯 下一步: 运行测试
echo    go test -v
echo.
pause
