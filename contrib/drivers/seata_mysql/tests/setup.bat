@echo off

echo 🚀 设置 Seata MySQL 测试环境...

docker --version >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Docker 未安装，请先安装 Docker
    exit /b 1
)

docker-compose --version >nul 2>&1
if %errorlevel% neq 0 (
    echo ❌ Docker Compose 未安装，请先安装 Docker Compose
    exit /b 1
)

echo 📦 启动 MySQL 和 Seata 服务...
cd /d "%~dp0"
docker-compose -f setup/docker-compose.yml up -d

echo ⏳ 等待 MySQL 服务启动...
timeout /t 10 /nobreak >nul

echo ⏳ 检查 MySQL 连接...
for /L %%i in (1,1,30) do (
    docker exec seata_mysql_test mysqladmin ping -h localhost -uroot -p123456 --silent >nul 2>&1
    if !errorlevel! equ 0 (
        echo ✅ MySQL 已就绪
        goto :mysql_ready
    )
    echo 等待 MySQL 启动... (%%i/30)
    timeout /t 2 /nobreak >nul
)
:mysql_ready

echo 🗄️ 初始化数据库...
docker exec -i seata_mysql_test mysql -uroot -p123456 < setup\init.sql

echo ⏳ 等待 Seata 服务启动...
timeout /t 10 /nobreak >nul

for /L %%i in (1,1,30) do (
    docker logs seata_server_test 2>&1 | findstr "Server started" >nul 2>&1
    if !errorlevel! equ 0 (
        echo ✅ Seata 已就绪
        goto :seata_ready
    )
    echo 等待 Seata 启动... (%%i/30)
    timeout /t 2 /nobreak >nul
)
:seata_ready

echo ✅ 环境设置完成！
echo 📊 MySQL: 127.0.0.1:3306
echo 🔧 Seata: 127.0.0.1:8091