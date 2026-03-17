# gtransport Modbus API Surface and ProcessImage Locking Design

## Goal

收敛 `net/gtransport/modbus` 的 API 心智：把 parse/request/execute/encode 这条可组合主线重新放回中心位置，同时让默认内存态 `MemoryProcessImage` 更适合服务端并发读多写少场景。

## Current Problem

### Handle helpers

当前包里同时暴露：

- `Parse*`
- `ExecuteRequest`
- `Encode*`
- `Handle*`

这让 `HandleTCPRequestFrame` / `HandleRTURequestFrame` / `HandleRTURequestPayload` 变得过于显眼，容易让使用者把 helper 误当成底层主 API，而不是兼容性的便捷封装。

### MemoryProcessImage concurrency

`MemoryProcessImage` 当前明确声明“NOT safe for concurrent use”。这和典型 Modbus server 使用场景不匹配：默认实现往往会被多个连接共享，而读寄存器远多于写寄存器。

## Decision

### 1. API surface

不立即删除 `Handle*`，避免不必要的破坏式变更；但要明确降级它们的定位：

- `modbus.go` 的包注释与 README 先介绍 composable API
- `Handle*` 只保留为 optional compatibility helpers
- `Handle*` 注释改成 `Deprecated:`，引导使用 `Parse* -> ExecuteRequest -> Encode*`

这样做可以达到两个目标：

- 底层组件主路径回到 composable flow
- 已有调用方暂时不被破坏

### 2. ProcessImage concurrency

不修改 `ProcessImage` 接口，只修改默认内存实现：

- 在 `MemoryProcessImage` 内部增加 `sync.RWMutex`
- `Read*` 使用 `RLock`
- `Write*` 使用 `Lock`

这保持了接口解耦，同时让默认实现成为更安全的“开箱即用”选项。

## Non-Goals

- 不给 `ProcessImage` 接口引入锁或事务语义
- 不保证多次读写调用之间的事务一致性
- 不在本次直接删除 `Handle*`

## Testing

- 添加一个并发读写测试
- 用 `go test -race ./modbus -run TestMemoryProcessImageConcurrentReadWrite -count=1` 做红绿验证
- 再跑 `go test ./... -count=1` 作为包级回归
