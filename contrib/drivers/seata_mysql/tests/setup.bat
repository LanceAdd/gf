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
docker-compose -f setup/docker-compose.yml up -d

echo ⏳ 等待服务启动...
timeout /t 30 /nobreak >nul

echo 🗄️ 初始化数据库...
mysql -h127.0.0.1 -P3306 -uroot -p123456 < setup/init.sql

echo ✅ 环境设置完成！
echo 📊 MySQL: 127.0.0.1:3306
echo 🔧 Seata: 127.0.0.1:8091