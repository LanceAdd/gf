# Seata MySQL Driver 测试套件

## 🚀 快速开始

### 1. 环境搭建
```bash
# Linux/macOS
./setup.sh

# Windows
setup.bat
```

### 2. 运行测试
```bash
# 检查环境
./test.sh check   # 或 test.bat check

# 单元测试
./test.sh unit    # 或 test.bat unit

# 集成测试
./test.sh integration # 或 test.bat integration

# 完整测试
./test.sh all     # 或 test.bat all
```

### 3. 清理环境
```bash
# Linux/macOS
./cleanup.sh

# Windows
cleanup.bat
```

## 📁 目录结构

```
tests/
├── setup.sh / setup.bat           # 环境搭建脚本
├── test.sh / test.bat             # 测试执行脚本
├── cleanup.sh / cleanup.bat      # 环境清理脚本
├── config/                        # 配置文件
├── setup/                         # Docker 环境配置
├── testdata/                      # 测试数据
├── unit/                          # 单元测试
├── integration/                   # 集成测试（包含所有功能测试）
└── common/                        # 公共工具
```

## 🧪 测试类型

- **单元测试**: 配置、驱动、异步器、镜像、降级、重试等模块功能
- **集成测试**: 基础连接、AT/XA模式转账、并发事务、嵌套事务、回滚场景

## 🔧 环境要求

- Docker & Docker Compose
- Go 1.19+
- MySQL 8.0+ (通过 Docker)
- Seata Server 1.7.1 (通过 Docker)