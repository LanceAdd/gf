@echo off
setlocal enabledelayedexpansion

cd /d "%~dp0.."

echo 🚀 运行 Seata MySQL 驱动测试套件...
echo ================================================
go test -v -timeout 600s .