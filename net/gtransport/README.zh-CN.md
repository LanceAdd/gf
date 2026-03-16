# gtransport

`gtransport` 用于在流式连接上按固定 codec 进行分帧读写。

它的设计目标是尽量小而直接：

- `Codec` 负责原始字节流和完整帧之间的转换。
- `Transport` 负责在 `io.ReadWriteCloser` 上同步执行 `ReadFrame` / `WriteFrame`。
- 业务逻辑放在包外处理。

## 对外 API

```go
type Codec interface {
    Decode(in []byte) (frame []byte, consumed int, err error)
    Encode(frame []byte) ([]byte, error)
}

type Transport struct{}

func New(conn io.ReadWriteCloser, codec Codec, opts ...Option) *Transport
func (t *Transport) ReadFrame() ([]byte, error)
func (t *Transport) WriteFrame(frame []byte) error
func (t *Transport) Close() error
```

## 可选项

- `WithReadTimeout(d time.Duration)`
- `WithMaxBufferBytes(n int)`

## 常用 Codec

- `NewDelimiter(delim []byte, maxPayloadBytes int, strip bool)`
- `NewLine(maxPayloadBytes int, strip bool)`
  解码时同时支持 `LF` 和 `CRLF`，编码时固定输出 `LF`。
- `NewFixedLength(length int)`
- `NewLengthPrefixed(fieldBytes int, order binary.ByteOrder, maxPayloadBytes int)`

## 高级 Codec

- `NewLengthField(opt LengthFieldOption)`

只有在简单的长度前缀构造器不够用时再使用它，例如长度字段位于自定义头部中间。

## 协议型 Codec

带协议知识的 codec 应放在子包里，而不是继续堆到 `gtransport` 主包。

- `gtransport/modbus`
  - `modbus.NewTCP()`
  - `modbus.NewRTU()`

`modbus.NewTCP()` 面向完整的 Modbus TCP ADU，并在编码时自动重写 MBAP `Length` 字段。

`modbus.NewRTU()` 会在解码时校验 CRC，返回去掉 CRC 的 RTU 内容，并在编码时自动补 CRC。

这些 Modbus codec 还会校验内置功能码集合的静态 payload 合法性，包括数量范围、字节数一致性、单线圈写值以及异常帧形状。

这些内置 Modbus codec 故意只做到协议静态校验，不负责部署或业务策略，例如 SlaveID 白名单、设备归属、ProcessImage 边界或寄存器映射语义。

## Modbus API 分层

`gtransport/modbus` 同时提供推荐路径和高级路径。

### 推荐 API

当你想用最少样板代码得到标准 Modbus 行为时，优先使用：

- `modbus.HandleTCPRequestFrame(frame, image)`
- `modbus.HandleRTURequestFrame(frame, image)`
- `modbus.HandleRTURequestPayload(payload, image)`
- `modbus.ExecuteRequest(req, image)`

### 高级 API

当你想自己组合协议零件和控制流程时，使用：

- `modbus.ParseTCPRequest(frame)`
- `modbus.ParseRTURequest(frame)`
- `modbus.EncodeTCPResponse(resp)`
- `modbus.EncodeRTUResponse(resp)`
- typed Modbus request/response 模型
- `modbus.ProcessImage`

### 语义规则

- `TCP frame` 表示完整的 Modbus TCP ADU。
- `RTU frame` 表示带 CRC 的原始 Modbus RTU ADU。
- `RTU payload` 表示 `gtransport.New(..., modbus.NewRTU())` 返回的去 CRC 内容。

原始 RTU ADU 应使用 `HandleRTURequestFrame`，transport 解码后的 RTU 内容应使用 `HandleRTURequestPayload`。

## 示例

```go
codec := gtransport.NewLengthPrefixed(2, binary.BigEndian, 1024)
tr := gtransport.New(conn, codec)
defer tr.Close()

for {
    frame, err := tr.ReadFrame()
    if err != nil {
        return err
    }

    if err := tr.WriteFrame(frame); err != nil {
        return err
    }
}
```

## 设计说明

- 一个 transport 绑定一个固定 codec。
- 读取是同步的，每次返回一帧完整数据。
- 写入是同步的，内部自动串行化。
- 不提供 handler 链、channel API，也不内置业务处理流程。
