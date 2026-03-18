# gtransport

`gtransport` provides framed I/O over stream connections.

The package is centered on three ideas:

- a `Codec` defines frame boundaries and encoding rules
- `WrapTransport` performs synchronous frame reads and writes on one fixed connection
- `DialTransport` adds managed dialing, reconnect, idle observation, and connection callbacks

## Primary API

```go
type Codec interface {
    Decode(in []byte) (frame []byte, consumed int, err error)
    Encode(frame []byte) ([]byte, error)
}

type FrameTransport interface {
    ReadFrame(ctx context.Context) ([]byte, error)
    WriteFrame(ctx context.Context, frame []byte) error
    Close() error
    State() State
}

type Observable interface {
    IdleFor(now time.Time) time.Duration
    LastReadAt() time.Time
    LastWriteAt() time.Time
}

func Wrap(conn io.ReadWriteCloser, codec Codec, opts ...WrapOption) *WrapTransport
func Dial(ctx context.Context, connector Connector, codec Codec, opts ...DialOption) *DialTransport
```

## Dial vs Wrap

Use `Dial` when the transport should own connection establishment and recover
future availability after disconnects.

Use `Wrap` when you already have a live connection, for example:

- server-side `Accept()` flows
- `net.Pipe` tests
- wrapping `gtcp.Conn` or another already-open stream

`Dial` may reconnect on a later operation after a connection failure.

`Wrap` does not reconnect. Once the wrapped connection fails, that transport is
terminal.

## Read/Write Semantics

- `ReadFrame(ctx)` and `WriteFrame(ctx, frame)` are the only primary framed I/O methods.
- `ctx` is intended to cover the whole operation, including connect waiting and I/O blocking when the underlying connection supports deadlines.
- failed operations return the observed error directly
- failed operations are never transparently replayed

That no-replay rule is intentional:

- a failed write may already have reached the peer
- a failed read may leave the old connection at an unknown frame boundary

## Observability

Shared observation surface:

- `State()` reports `idle`, `connecting`, `ready`, or `closed`
- `State` implements `fmt.Stringer`, so `%s`/`%v` output uses those same names
- `LastReadAt()` reports the last successful full-frame read
- `LastWriteAt()` reports the last successful full-frame write
- `IdleFor(now)` reports how long the active connection has gone without a successful read

If a connection is ready but has not yet produced a frame, idle time is measured
from the ready time of that connection.

Dial-only observation and lifecycle helpers:

- `LastFrameAt()` reports the last successful full-frame read and survives reconnects
- `EnsureConnected(ctx)` triggers a connect attempt when the dial transport is idle
- `WithIdleTimeout`, `WithOnIdle`, `WithOnConnect`, `WithOnConnectionLost`, and `WithOnConnectFail` expose watchdog-driven callbacks

## Options

Wrap options:

- `WithReadTimeout(d time.Duration)`
- `WithMaxBufferBytes(n int)`

Dial options:

- `WithDialReadTimeout(d time.Duration)`
- `WithDialMaxBufferBytes(n int)`
- `WithConnectTimeout(d time.Duration)`
- `WithReconnectBackoff(func(attempt int) time.Duration)`
- `WithMinStableDuration(d time.Duration)`
- `WithIdleTimeout(d time.Duration)`
- `WithOnIdle(func())`
- `WithOnConnect(func(first bool))`
- `WithOnConnectionLost(func(err error))`
- `WithOnConnectFail(func(err error))`
- `WithPollInterval(d time.Duration)`

## Common Codecs

- `NewDelimiter(delim []byte, maxPayloadBytes int, strip bool)`
- `NewLine(maxPayloadBytes int, strip bool)`
- `NewFixedLength(length int)`
- `NewLengthPrefixed(fieldBytes int, order binary.ByteOrder, maxPayloadBytes int)`
- `NewLengthField(opt LengthFieldOption)`

## Protocol Codecs

Protocol-aware codecs belong in subpackages.

- `gtransport/adcp`
  - `adcp.New(maxFrameLength ...int)`
- `gtransport/modbus`
  - `modbus.NewTCP()`
  - `modbus.NewRTU()`

## Recommended Flows

Accepted connection flow:

```go
tr := gtransport.Wrap(conn, codec, gtransport.WithReadTimeout(5*time.Second))
defer tr.Close()

frame, err := tr.ReadFrame(context.Background())
if err != nil {
    return err
}

return tr.WriteFrame(context.Background(), frame)
```

Reconnectable outbound flow:

```go
ctx := context.Background()
tr := gtransport.Dial(ctx, connector, codec, gtransport.WithConnectTimeout(3*time.Second))
defer tr.Close()

if err := tr.WriteFrame(ctx, payload); err != nil {
    return err
}
```

## Modbus Example

Composable/default flow:

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

RTU semantics stay explicit:

- `modbus.NewTCP()` is transport-oriented: its `Decode` scans the byte stream, may discard malformed candidates as noise, and resynchronizes to the next valid TCP ADU
- `ParseTCPRequest` is the strict frame validator/parser to use when callers need protocol-level errors for a specific Modbus TCP ADU
- `ParseRTURequest` / `EncodeRTUResponse` operate on raw RTU ADUs with CRC
- `ParseRTURequestPayload` / `EncodeRTUResponsePayload` operate on CRC-stripped RTU payloads returned by `modbus.NewRTU()`
- `ExecuteRequest` is optional; callers can also route typed requests into their own business/storage layers

Example modules:

- `example/wrap_echo` shows Wrap + LengthPrefixed codec echo
- `example/dial_reconnect` shows Dial with automatic reconnection
- `example/adcp_stream` shows decode-only ADCP payload reads with `adcp.New(adcp.DefaultMaxFrameLength * 2)`
- `example/modbus_tcp` shows composable Modbus TCP server
- `example/modbus_rtu` shows RTU codec over net.Pipe

## ADCP Example

Decode-only ADCP flow:

```go
tr := gtransport.Wrap(conn, adcp.New(16384), gtransport.WithReadTimeout(5*time.Second))
defer tr.Close()

payload, err := tr.ReadFrame(context.Background())
if err != nil {
    return err
}

return handleADCPPayload(payload)
```

ADCP codec semantics:

- `adcp.New()` keeps the default max frame length of `8192`
- `adcp.New(n)` uses `n` as the max frame length when `n > 0`
- `adcp.DefaultMaxFrameLength` exposes that default limit for callers that want to scale from it
- use `adcp.New(16384)` when the device can emit payloads larger than the default limit
- `adcp.New()` scans the stream for a 16-byte `0x80` sync preamble
- it validates the fixed metadata header and CRC16-CCITT checksum
- `ReadFrame` returns only the ADCP payload bytes
- `WriteFrame` is not supported with `adcp.New()` because the codec is decode-only in this version
- `example/adcp_stream` shows a complete net.Pipe-based usage flow

## Final API

The public creation API is:

- `Dial`
- `Wrap`

The public transport types are:

- `WrapTransport`
- `DialTransport`
- `FrameTransport`
- `Observable`
