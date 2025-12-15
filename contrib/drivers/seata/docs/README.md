# Seata-Go GF Driver 文档索引

> **项目**: GoFrame Seata 分布式事务驱动  
> **最后更新**: 2025-12-13  
> **文档版本**: v2.0

---

## 📚 文档导航

### 1. 快速开始

- [README.md](../README.md) - 项目主文档
- [生产就绪评估](./PRODUCTION-READINESS.md) - **是否可以投入生产？** ⭐
- [快速入门指南](#快速入门) - 5分钟快速上手
- [安装配置](#安装配置) - 安装和配置说明

### 2. 开发文档

- [设计文档](./DESIGN-DOCUMENT.md) - **完整设计文档（推荐）**
- [开发指南](./DEVELOPMENT-GUIDE.md) - 完整开发文档
- [API 参考](./API-REFERENCE.md) - API 接口文档
- [测试指南](./TESTING-GUIDE.md) - 测试开发指南

### 3. 实现细节

- [设计文档](./DESIGN-DOCUMENT.md) - 包含架构设计、开发步骤、详细流程

---

## 🚀 快速入门

### 安装

```bash
go get -u github.com/gogf/gf/contrib/drivers/seata/v2
```

### 配置

```yaml
database:
  default:
    type: "seata-at-mysql"
    host: "127.0.0.1"
    port: "3306"
    user: "root"
    pass: "root"
    name: "test"

seata:
  enabled: true
  application-id: "my-app"
  tx-service-group: "default_tx_group"
  mode: "AT"
```

### 使用示例

```go
import (
    "github.com/gogf/gf/v2/frame/g"
    _ "github.com/gogf/gf/contrib/drivers/seata/v2"
)

func main() {
    // 初始化 Seata
    seata.Init(seata.DefaultConfig())
    
    // 使用全局事务
    err := g.DB().Transaction(ctx, func(ctx context.Context, tx gdb.TX) error {
        // 业务逻辑
        return nil
    })
}
```

---

## 📖 详细文档

### [开发指南](./DEVELOPMENT-GUIDE.md)

包含完整的开发流程、架构设计、核心实现等内容。

### [API 参考](./API-REFERENCE.md)

详细的 API 接口文档，包括所有公开函数、配置项等。

### [测试指南](./TESTING-GUIDE.md)

测试开发规范、测试用例编写、覆盖率提升等。

---

## 🎯 项目统计

- **代码**: 12个文件，2,758行
- **测试**: 15个文件，3,740行，172个测试用例，覆盖率 40.9%
- **文档**: 6篇核心文档
- **总计**: 23,685行

---

## 📝 更新日志

### v2.0 (2025-12-13)

- ✅ 完成核心功能开发
- ✅ 测试覆盖率达到 40.9%
- ✅ 文档结构优化
- ✅ 代码优化（Context 复用等）

### v1.0 (历史版本)

- ✅ AT 模式实现
- ✅ XA 模式基础实现
- ✅ 异步工作器
- ✅ 降级策略
