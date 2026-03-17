# gtransport

`gtransport` 通过一个统一的 `Transport` 模型，为流式连接提供分帧读写能力。

这个包围绕三个核心概念展开：

- `Codec` 负责定义帧边界和编码规则
- `Transport` 负责同步读写完整帧
- transport 生命周期来自 `Dial` 或 `Wrap`

## 主 API

```go
type Codec interface {
    Decode(in []byte) (frame []byte, consumed int, err error)
    Encode(frame []byte) ([]byte, error)
}

type Transport struct{}

func Dial(connector Connector, codec Codec, opts ...DialOption) *Transport
func Wrap(conn io.ReadWriteCloser, codec Codec, opts ...WrapOption) *Transport

func (t *Transport) ReadFrame(ctx context.Context) ([]byte, error)
func (t *Transport) WriteFrame(ctx context.Context, frame []byte) error
func (t *Transport) Close() error

func (t *Transport) State() State
func (t *Transport) LastReadAt() time.Time
func (t *Transport) LastWriteAt() time.Time
func (t *Transport) IdleFor(now time.Time) time.Duration
```

## Dial 与 Wrap

当 transport 需要自己建连，并在断开后恢复后续可用性时，使用 `Dial`。

当你已经有一个可用连接时，使用 `Wrap`，典型场景包括：

- 服务端 `Accept()` 后的连接
- `net.Pipe` 测试连接
- `gtcp.Conn` 或其他已建立流连接的包装

`Dial` 可以在后续调用中尝试重连。

`Wrap` 不会重连。被包装连接一旦失效，该 transport 就进入终止状态。

## 读写语义

- `ReadFrame(ctx)` 和 `WriteFrame(ctx, frame)` 是唯一的主读写入口
- `ctx` 的目标语义是覆盖整次操作；当底层连接支持 deadline 时，它同时约束建连等待和 I/O 阻塞
- 失败的操作直接返回观察到的错误
- 不透明重放失败的操作

之所以坚持“不重放”，是因为：

- 写失败时，对端可能已经收到全部或部分数据
- 读失败时，旧连接上的帧边界可能已经不再确定

## 观测能力

`Transport` 暴露的是事实，不是策略：

- `State()` 返回 `idle`、`connecting`、`ready`、`closed`
- `State` 实现了 `fmt.Stringer`，因此 `%s`/`%v` 输出会直接使用这些名字
- `LastReadAt()` 返回最近一次成功读到完整帧的时间
- `LastWriteAt()` 返回最近一次成功写出完整帧的时间
- `IdleFor(now)` 返回当前活动连接已经多久没有成功读到数据

如果连接已经 ready，但还从未成功读到任何帧，则 idle 基线取该连接的 ready 时间。

## 可选项

共享 transport 可选项：

- `WithReadTimeout(d time.Duration)`
- `WithMaxBufferBytes(n int)`

仅 `Dial` 适用的可选项：

- `WithConnectTimeout(d time.Duration)`
- `WithReconnectBackoff(func(attempt int) time.Duration)`

## 常用 Codec

- `NewDelimiter(delim []byte, maxPayloadBytes int, strip bool)`
- `NewLine(maxPayloadBytes int, strip bool)`
- `NewFixedLength(length int)`
- `NewLengthPrefixed(fieldBytes int, order binary.ByteOrder, maxPayloadBytes int)`
- `NewLengthField(opt LengthFieldOption)`

## 协议型 Codec

带协议知识的 codec 应放在子包。

- `gtransport/adcp`
  - `adcp.New(maxFrameLength ...int)`
- `gtransport/modbus`
  - `modbus.NewTCP()`
  - `modbus.NewRTU()`

## 推荐流程

已建立连接流程：

```go
tr := gtransport.Wrap(conn, codec, gtransport.WithReadTimeout(5*time.Second))
defer tr.Close()

frame, err := tr.ReadFrame(context.Background())
if err != nil {
    return err
}

return tr.WriteFrame(context.Background(), frame)
```

可重连的主动连接流程：

```go
tr := gtransport.Dial(connector, codec, gtransport.WithConnectTimeout(3*time.Second))
defer tr.Close()

if err := tr.WriteFrame(context.Background(), payload); err != nil {
    return err
}
```

## Modbus 示例

默认推荐的可组合流程：

```go
tr := gtransport.Wrap(conn, modbus.NewTCP(), gtransport.WithReadTimeout(5*time.Second))
defer tr.Close()

frame, err := tr.ReadFrame(context.Background())
if err != nil {
    return err
}

req, err := modbus.ParseTCPRequest(frame)
if err != nil {
    return err
}

resp, err := modbus.ExecuteRequest(req, image)
if err != nil {
    return err
}

respFrame, err := modbus.EncodeTCPResponse(resp)
if err != nil {
    return err
}

return tr.WriteFrame(context.Background(), respFrame)
```

如果你只想减少样板代码，也仍然可以使用 helper 流程：

```go
respFrame, err := modbus.HandleTCPRequestFrame(frame, image)
```

RTU 语义需要严格区分：

- `ParseRTURequest` / `EncodeRTUResponse` 面向带 CRC 的原始 RTU ADU
- `ParseRTURequestPayload` / `EncodeRTUResponsePayload` 面向 `modbus.NewRTU()` 返回的去 CRC RTU payload
- `HandleRTURequestPayload` 是这条 payload 路径上的便捷封装

示例模块：

- `example/wrap_echo` 展示 Wrap + LengthPrefixed 编码的回显示例
- `example/dial_reconnect` 展示 Dial 自动重连
- `example/adcp_stream` 展示使用 `adcp.New(adcp.DefaultMaxFrameLength * 2)` 的只解码 ADCP 读取流程
- `example/modbus_tcp` 展示可组合的 Modbus TCP 服务端
- `example/modbus_rtu` 展示基于 net.Pipe 的 RTU 编码

## ADCP 示例

只解码的 ADCP 流程：

```go
tr := gtransport.Wrap(conn, adcp.New(16384), gtransport.WithReadTimeout(5*time.Second))
defer tr.Close()

payload, err := tr.ReadFrame(context.Background())
if err != nil {
    return err
}

return handleADCPPayload(payload)
```

ADCP codec 语义：

- `adcp.New()` 使用默认最大帧长 `8192`
- `adcp.New(n)` 会在 `n > 0` 时使用自定义最大帧长
- `adcp.DefaultMaxFrameLength` 导出了这个默认值，便于外部按基线放大
- 当设备 payload 可能超过默认上限时，可以直接使用 `adcp.New(16384)`
- `adcp.New()` 会在字节流中扫描 16 字节连续的 `0x80` 同步头
- 它会校验固定头字段和 CRC16-CCITT 校验值
- `ReadFrame` 成功后只返回 ADCP payload 字节
- 当前版本的 `adcp.New()` 是 decode-only codec，不支持 `WriteFrame`
- `example/adcp_stream` 展示了一个完整的 net.Pipe 用法

## 最终 API

最终公开创建入口只保留：

- `Dial`
- `Wrap`
