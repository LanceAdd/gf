@echo off

echo 🧹 清理 Seata MySQL 测试环境...

cd /d "%~dp0"

echo 🛑 停止服务...
docker-compose -f setup/docker-compose.yml down -v

set /p "delete_images=是否删除相关镜像？(y/N): "
if /i "%delete_images%"=="y" (
    echo 🗑️ 删除镜像...
    docker rmi mysql:8.0 2>nul
    docker rmi seataio/seata-server:1.7.1 2>nul
)

echo ✅ 清理完成！