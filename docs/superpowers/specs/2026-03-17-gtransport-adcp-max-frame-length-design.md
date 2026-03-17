# ADCP Max Frame Length Design

## Context

`net/gtransport/adcp` 当前通过 `adcp.New()` 暴露一个 decode-only codec，内部把最大帧长固定为 `8192`。这已经能覆盖默认场景，但当接入的 ADCP 设备 payload 较大时，调用方无法在不改源码的前提下放宽上限。

本次只补一个最小能力：让调用方在保持 `adcp.New()` 默认行为不变的前提下，按需传入自定义最大帧长。

## Options Considered

### 方案 A：`adcp.New(maxFrameLength ...int)`

优点：

- 保留现有 `adcp.New()` 调用方式
- 需要自定义时只需 `adcp.New(16384)`
- 改动范围最小

代价：

- 这个 API 只适合当前单一配置项
- 超过一个参数时，语义只能约定而不是靠类型系统限制

### 方案 B：`adcp.NewWithMaxFrameLength(n int)`

优点：

- 参数语义最直白

代价：

- 需要同时维护两个构造入口
- 比用户想要的调用方式更长

### 方案 C：`adcp.New(opts ...Option)`

优点：

- 最容易扩展

代价：

- 当前只有一个配置项，复杂度偏高

## Decision

采用方案 A：`adcp.New(maxFrameLength ...int) gtransport.Codec`。

语义约定：

- 不传参数时，继续使用默认最大帧长 `8192`
- 传入第一个参数且大于 `0` 时，使用该值
- 传入非正数时，回退到默认值
- 额外参数不新增语义，本次实现只读取第一个参数

## Impact

- `net/gtransport/adcp/adcp.go` 需要调整构造函数签名和默认值解析
- `net/gtransport/adcp/adcp_z_unit_test.go` 需要增加默认值、自定义值与非法值回退测试
- `net/gtransport/README.md` 与 `net/gtransport/README.zh-CN.md` 需要同步更新 API 说明
