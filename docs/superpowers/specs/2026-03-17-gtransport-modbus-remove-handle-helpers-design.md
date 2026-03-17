# gtransport Modbus Remove Handle Helpers Design

## Goal

彻底移除 `net/gtransport/modbus` 中把 parse/execute/encode 串成一步的 `Handle*` helper，收敛 API surface，让底层使用路径只剩下 composable flow。

## Why

`HandleTCPRequestFrame`、`HandleRTURequestFrame`、`HandleRTURequestPayload` 会把调用方的控制点提前收走：

- 请求一旦 parse 完，就被直接送进 `ExecuteRequest`
- 响应何时生成、是否生成，也被 helper 固定

这和底层组件的定位不一致。底层组件更应该提供：

- `Parse*`
- `ExecuteRequest`
- `Encode*`

调用方自行决定中间是否接自己的业务逻辑、访问自己的状态层，或者完全绕开 `ProcessImage`。

## Options Considered

### 方案 A：保留 helper，仅 deprecated

优点：

- 兼容性最好

缺点：

- 公开 API 仍然显得冗余
- 文档即使弱化，也仍然要长期维护一条不推荐路径

### 方案 B：删除公开 helper，保留未导出共享流水线

优点：

- 对外 API 变干净

缺点：

- 包内仍残留一层价值很低的 parse -> execute -> encode 管道

### 方案 C：彻底删除 helper 与对应内部管道

优点：

- API surface 最小
- 文档、测试、示例都统一到 composable flow

缺点：

- 明确是 breaking change

## Decision

采用方案 C。

## Scope

- 删除 `net/gtransport/modbus/handle_request.go`
- 删除 `net/gtransport/modbus/handle_request_z_unit_test.go`
- 移除 `modbus_z_unit_test.go` 中对 `Handle*` 的 API shape 断言
- 把 `gtransport` 集成测试和 `example/modbus_rtu` 全部改成 composable flow
- README 和包注释只描述 `Parse* -> ExecuteRequest/custom logic -> Encode*`

## Non-Goals

- 不删除 `ParseRTURequestPayload`
- 不删除 `EncodeRTUResponsePayload`
- 不改变 `ExecuteRequest` 与 `ProcessImage` 的职责
