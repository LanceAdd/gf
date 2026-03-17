# gtransport

`gtransport` provides framed I/O over stream connections through one public
`Transport` model.

The package is centered on three ideas:

- a `Codec` defines frame boundaries and encoding rules
- a `Transport` performs synchronous frame reads and writes
- transport lifecycle can come from either `Dial` or `Wrap`

## Primary API

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

`Transport` exposes transport facts, not transport policy:

- `State()` reports `idle`, `connecting`, `ready`, or `closed`
- `State` implements `fmt.Stringer`, so `%s`/`%v` output uses those same names
- `LastReadAt()` reports the last successful full-frame read
- `LastWriteAt()` reports the last successful full-frame write
- `IdleFor(now)` reports how long the active connection has gone without a successful read

If a connection is ready but has not yet produced a frame, idle time is measured
from the ready time of that connection.

## Options

Shared transport options:

- `WithReadTimeout(d time.Duration)`
- `WithMaxBufferBytes(n int)`

Dial-only options:

- `WithConnectTimeout(d time.Duration)`
- `WithReconnectBackoff(func(attempt int) time.Duration)`

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
tr := gtransport.Dial(connector, codec, gtransport.WithConnectTimeout(3*time.Second))
defer tr.Close()

if err := tr.WriteFrame(context.Background(), payload); err != nil {
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

Convenience helper flow still exists when you want less boilerplate:

```go
respFrame, err := modbus.HandleTCPRequestFrame(frame, image)
```

RTU semantics stay explicit:

- `ParseRTURequest` / `EncodeRTUResponse` operate on raw RTU ADUs with CRC
- `ParseRTURequestPayload` / `EncodeRTUResponsePayload` operate on CRC-stripped RTU payloads returned by `modbus.NewRTU()`
- `HandleRTURequestPayload` is the convenience wrapper around that payload path

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

The final public creation API is:

- `Dial`
- `Wrap`
