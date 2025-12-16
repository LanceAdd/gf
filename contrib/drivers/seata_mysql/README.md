# Seata MySQL Driver - GoFrame 分布式事务支持 (仅 AT 模式)

## 🎯 概述

这是一个为 GoFrame 框架提供 Seata **AT 模式**分布式事务支持的数据库驱动。它集成了 Seata-Go SDK，使 GoFrame 应用能够参与分布式事务。

**重要说明**：
- ✅ **仅支持 AT 模式**：本驱动专注于 AT 模式，这是 Seata 最常用且最成熟的模式
- ❌ **不支持其他模式**：不支持 TCC、XA、SAGA 等其他事务模式
- 📝 **AT 模式优势**：自动补偿、无侵入、易于使用，通过 undo_log 实现自动回滚

## 📦 安装

```bash
go get -u github.com/gogf/gf/contrib/drivers/seata_mysql/v2
```

## 🚀 快速开始

### 两种使用模式

#### 模式 1：完整模式（推荐生产环境）

需要配置 Seata Server 连接，提供完整的分布式事务协调功能。

**步骤 1：设置环境变量**

```bash
export SEATA_GO_CONFIG_PATH=/path/to/seatago.yml
```

**步骤 2：创建 Seata 配置文件** (`seatago.yml`)

```yaml
seata:
  enabled: true
  # 应用 ID
  application-id: my-app
  # 事务服务组
  tx-service-group: default_tx_group
  
  # 客户端配置
  client:
    rm:
      async-commit-buffer-limit: 10000
      report-retry-count: 5
      lock:
        retry-interval: 30s
        retry-times: 10
    tm:
      commit-retry-count: 5
      rollback-retry-count: 5
      default-global-transaction-timeout: 60s
    undo:
      log-serialization: jackson
      log-table: undo_log
      only-care-update-columns: true
      
  # Seata Server 地址配置
  service:
    vgroup-mapping:
      default_tx_group: default
    grouplist:
      default: 127.0.0.1:8091
      
  # 传输配置
  transport:
    type: TCP
    server: NIO
    heartbeat: true
    serialization: seata
    
  # Getty 网络配置
  getty:
    reconnect-interval: 0
    connection-num: 1
    session:
      tcp-keep-alive: true
      keep-alive-period: 120s
```

**步骤 3：初始化驱动**

```go
import (
    _ "github.com/gogf/gf/contrib/drivers/seata_mysql/v2"
    "github.com/gogf/gf/v2/database/gdb"
)

func main() {
    // 初始化 Seata
    config := &seata_mysql.Config{
        Enabled:         true,
        ApplicationID:   "my-app",
        TxServiceGroup:  "default_tx_group",
        EnableAutoProxy: true,
    }
    
    err := seata_mysql.Init(config)
    if err != nil {
        panic(err)
    }
    
    // 配置数据库
    gdb.SetConfig(gdb.Config{
        "default": gdb.ConfigGroup{
            gdb.ConfigNode{
                Type: "seata-at-mysql",
                Link: "mysql:root:password@tcp(127.0.0.1:3306)/mydb",
            },
        },
    })
}
```

#### 模式 2：精简模式（开发/测试环境）

不需要 Seata Server，只提供本地 AT 模式基础功能（undo log 记录等）。

**特点**：
- ✅ 不需要配置文件
- ✅ 不需要 Seata Server
- ❌ 无法进行分布式事务协调
- ⚠️ 仅用于开发测试

```go
import (
    _ "github.com/gogf/gf/contrib/drivers/seata_mysql/v2"
    "github.com/gogf/gf/v2/database/gdb"
)

func main() {
    // 不设置 SEATA_GO_CONFIG_PATH 环境变量
    
    // 初始化 Seata（精简模式）
    config := &seata_mysql.Config{
        Enabled: true,
    }
    
    err := seata_mysql.Init(config)
    if err != nil {
        panic(err)
    }
    
    // 配置数据库
    gdb.SetConfig(gdb.Config{
        "default": gdb.ConfigGroup{
            gdb.ConfigNode{
                Type: "seata-at-mysql",
                Link: "mysql:root:password@tcp(127.0.0.1:3306)/mydb",
            },
        },
    })
}
```

## 🏗️ 架构设计

### Seata AT 模式客户端初始化流程

根据 Seata-Go SDK 的标准初始化流程（本驱动只使用 AT 模式部分）：

```go
// 完整模式 (client.InitPath)
1. LoadPath(configPath)        // 加载配置文件
2. initRmClient(cfg)            // 初始化 RM 客户端
   ├── log.Init()               // 初始化日志
   ├── initRemoting(cfg)        // 初始化远程通信 (Getty)
   ├── rm.InitRm()              // 初始化资源管理器
   ├── config.Init()            // 初始化锁配置
   ├── client.RegisterProcessor() // 注册消息处理器
   ├── integration.Init()       // 初始化集成模块
   ├── tcc.InitTCC()            // ❌ 初始化 TCC (不使用)
   ├── at.InitAT()              // ✅ 初始化 AT 模式 (我们使用这个)
   └── at.InitXA()              // ❌ 初始化 XA 模式 (不使用)
3. initTmClient(cfg)            // 初始化 TM 客户端
4. initDatasource()             // 初始化数据源管理
```

**本驱动的实现（仅 AT 模式）**：

```go
// 完整模式：环境变量 SEATA_GO_CONFIG_PATH 已设置
initSeataClient():
  ├── client.InitPath("")       // 调用 Seata-Go 标准初始化
  │                              // (虽然会初始化 TCC/XA，但我们只使用 AT)
  └── [RM, TM, Remoting, AT 全部就绪]

initATMode():
  └── [跳过，已在 client.InitPath 中初始化]

// 精简模式：环境变量 SEATA_GO_CONFIG_PATH 未设置  
initSeataClient():
  ├── initSeataLog()            // 初始化日志
  └── initDatasourceManager()   // 初始化数据源管理

initATMode():
  └── sql.InitAT()              // 仅初始化 AT 核心组件
                                 // (不初始化 TCC/XA/SAGA)
```

### 架构优势

**嵌入 `mysql.Driver` 的设计**：

```go
type SeataDB struct {
    *mysql.Driver  // 嵌入 MySQL 驱动
    resource      *Resource
    config        *Config
}
```

**优点**：
1. ✅ 自动继承所有 MySQL 方法 (`TableFields`, `Open`, `GetChars` 等)
2. ✅ 减少约 77 行重复代码
3. ✅ 符合 GoFrame 推荐的驱动扩展模式
4. ✅ 职责分离：`mysql.Driver` 负责 MySQL，`SeataDB` 负责 Seata

## ⚙️ 配置说明

### Seata 配置文件 (seatago.yml)

完整配置示例：

```yaml
seata:
  enabled: true
  application-id: my-application
  tx-service-group: default_tx_group
  
  client:
    rm:
      async-commit-buffer-limit: 10000
      report-retry-count: 5
      table-meta-check-enable: false
      report-success-enable: false
      lock:
        retry-interval: 30s
        retry-times: 10
        retry-policy-branch-rollback-on-conflict: true
    
    tm:
      commit-retry-count: 5
      rollback-retry-count: 5
      default-global-transaction-timeout: 60s
      degrade-check: false
    
    undo:
      data-validation: true
      log-serialization: jackson
      log-table: undo_log
      only-care-update-columns: true
      compress:
        enable: true
        type: zip
        threshold: 64k
  
  service:
    vgroup-mapping:
      default_tx_group: default
    grouplist:
      default: 127.0.0.1:8091
    enable-degrade: false
    disable-global-transaction: false
  
  transport:
    shutdown:
      wait: 3s
    type: TCP
    server: NIO
    heartbeat: true
    serialization: seata
    compressor: none
    enable-tm-client-batch-send-request: false
    enable-rm-client-batch-send-request: true
    rpc-rm-request-timeout: 30s
    rpc-tm-request-timeout: 30s
  
  getty:
    reconnect-interval: 0
    connection-num: 1
    session:
      compress-encoding: false
      tcp-no-delay: true
      tcp-keep-alive: true
      keep-alive-period: 120s
      tcp-r-buf-size: 262144
      tcp-w-buf-size: 65536
      tcp-read-timeout: 1s
      tcp-write-timeout: 5s
      wait-timeout: 1s
      max-msg-len: 16498688
      session-name: client
      cron-period: 1s
```

### 配置项说明

| 配置项 | 说明 | 默认值 |
|----------|------|----------|
| `application-id` | 应用 ID | `"gf-app"` |
| `tx-service-group` | 事务服务组 | `"default_tx_group"` |
| `service.vgroup-mapping` | 事务组到集群的映射 | - |
| `service.grouplist` | Seata Server 地址列表 | `"127.0.0.1:8091"` |
| `client.undo.log-table` | Undo Log 表名 | `"undo_log"` |
| `client.undo.only-care-update-columns` | 只关注更新列 | `true` |

## 💾 数据库准备

### 创建 Undo Log 表

在业务数据库中执行：

```sql
CREATE TABLE IF NOT EXISTS `undo_log` (
  `id` bigint(20) NOT NULL AUTO_INCREMENT,
  `branch_id` bigint(20) NOT NULL,
  `xid` varchar(100) NOT NULL,
  `context` varchar(128) NOT NULL,
  `rollback_info` longblob NOT NULL,
  `log_status` int(11) NOT NULL,
  `gmt_create` datetime NOT NULL,
  `gmt_modified` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `ux_undo_log` (`xid`,`branch_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
```

## 📘 API 使用

### 基本使用

```go
import (
    _ "github.com/gogf/gf/contrib/drivers/seata_mysql/v2"
    "github.com/gogf/gf/v2/database/gdb"
    "github.com/gogf/gf/v2/frame/g"
)

func main() {
    // 1. 初始化 Seata
    config := seata_mysql.DefaultConfig()
    config.Enabled = true
    err := seata_mysql.Init(config)
    if err != nil {
        panic(err)
    }
    
    // 2. 配置数据库
    gdb.SetConfig(gdb.Config{
        "default": gdb.ConfigGroup{
            gdb.ConfigNode{
                Type: "seata-at-mysql",
                Link: "mysql:root:password@tcp(127.0.0.1:3306)/mydb",
            },
        },
    })
    
    // 3. 使用数据库
    db := g.DB()
    db.Model("user").Insert(g.Map{
        "name": "test",
    })
}
```

### 分布式事务示例

```go
import (
    "github.com/gogf/gf/v2/database/gdb"
    "github.com/gogf/gf/v2/frame/g"
    "github.com/seata/seata-go/pkg/tm"
)

func BusinessService(ctx context.Context) error {
    // 开启全局事务
    return tm.WithGlobalTx(ctx, &tm.GtxConfig{
        Name:    "business-service",
        Timeout: 60000, // 60秒
    }, func(ctx context.Context) error {
        // 本地数据库操作
        db := g.DB()
        _, err := db.Model("user").Ctx(ctx).Insert(g.Map{
            "name": "test",
        })
        if err != nil {
            return err
        }
        
        // 调用其他微服务
        err = callOtherService(ctx)
        if err != nil {
            return err // 自动回滚
        }
        
        return nil // 自动提交
    })
}
```

## ⚠️ 注意事项

1. **配置文件路径**：必须通过环境变量 `SEATA_GO_CONFIG_PATH` 设置
2. **Undo Log 表**：必须在每个业务数据库中创建
3. **Seata Server**：完整模式下需要启动 Seata Server
4. **网络连接**：确保应用能连接到 Seata Server
5. **事务组配置**：`vgroup-mapping` 必须与 `tx-service-group` 匹配

## 🐛 常见问题

### Q1: 报错 "system variable SEATA_GO_CONFIG_PATH is empty"

**原因**：未设置环境变量

**解决**：
```bash
export SEATA_GO_CONFIG_PATH=/path/to/seatago.yml
```

或者使用精简模式（不设置环境变量）

### Q2: 无法连接到 Seata Server

**检查项**：
1. Seata Server 是否启动
2. `service.grouplist` 配置是否正确
3. 网络是否可达

### Q3: Undo Log 未生成

**检查项**：
1. `undo_log` 表是否存在
2. 数据库连接是否正常
3. 是否使用了正确的驱动类型：`seata-at-mysql`

## 🔗 相关资源

- [Seata 官网](https://seata.io/)
- [Seata-Go GitHub](https://github.com/seata/seata-go)
- [GoFrame 官网](https://goframe.org/)
- [GF 数据库 ORM](https://goframe.org/pages/viewpage.action?pageId=1114119)

## 📝 License

MIT License
