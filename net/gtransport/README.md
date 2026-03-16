# gtransport

`gtransport` provides framed read/write over stream connections using a fixed codec per transport.

It is intentionally small:

- `Codec` converts between raw stream bytes and complete frames.
- `Transport` performs synchronous `ReadFrame` / `WriteFrame` operations on an `io.ReadWriteCloser`.
- Business logic stays outside the package.

## Public API

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

## Options

- `WithReadTimeout(d time.Duration)`
- `WithMaxBufferBytes(n int)`

## Common Codecs

- `NewDelimiter(delim []byte, maxPayloadBytes int, strip bool)`
- `NewLine(maxPayloadBytes int, strip bool)`
  Decodes both `LF` and `CRLF` terminated frames, and encodes using `LF`.
- `NewFixedLength(length int)`
- `NewLengthPrefixed(fieldBytes int, order binary.ByteOrder, maxPayloadBytes int)`

## Advanced Codec

- `NewLengthField(opt LengthFieldOption)`

Use this only when the simple length-prefixed constructor is not enough, for
example when the length field is embedded inside a custom frame header.

## Protocol Codecs

Protocol-aware codecs should live in subpackages instead of the `gtransport`
 root package.

- `gtransport/modbus`
  - `modbus.NewTCP()`
  - `modbus.NewRTU()`

`modbus.NewTCP()` works on full Modbus TCP ADUs and rewrites the MBAP `Length`
field during encoding.

`modbus.NewRTU()` validates CRC during decoding, returns RTU payload without
CRC, and appends CRC during encoding.

Both Modbus codecs also validate supported function-code payload legality for
the built-in function set, including quantity ranges, byte-count consistency,
single-coil values, and exception frame shape.

The built-in Modbus codecs intentionally stop at protocol-static validation.
They do not enforce deployment or application policy such as Unit ID
whitelists, device ownership, ProcessImage bounds, or register-map semantics.

## Modbus APIs

The `gtransport/modbus` package exposes two usage paths.

### Recommended APIs

Use these when you want standard Modbus behavior with minimal glue code:

- `modbus.HandleTCPRequestFrame(frame, image)`
- `modbus.HandleRTURequestFrame(frame, image)`
- `modbus.HandleRTURequestPayload(payload, image)`
- `modbus.ExecuteRequest(req, image)`

### Advanced APIs

Use these when you want protocol building blocks and your own control flow:

- `modbus.ParseTCPRequest(frame)`
- `modbus.ParseRTURequest(frame)`
- `modbus.EncodeTCPResponse(resp)`
- `modbus.EncodeRTUResponse(resp)`
- typed Modbus request and response models
- `modbus.ProcessImage`

### Semantic Rules

- `TCP frame` means a full Modbus TCP ADU.
- `RTU frame` means a raw Modbus RTU ADU with CRC.
- `RTU payload` means the decoded CRC-stripped content returned by `gtransport.New(..., modbus.NewRTU())`.

Use `HandleRTURequestFrame` for raw RTU ADUs and `HandleRTURequestPayload` for
transport-decoded RTU payloads.

### Common Flows

Recommended TCP flow:

```go
frame, err := tr.ReadFrame()
if err != nil {
    return err
}

respFrame, err := modbus.HandleTCPRequestFrame(frame, image)
if err != nil {
    return err
}

return tr.WriteFrame(respFrame)
```

Recommended raw RTU flow:

```go
respFrame, err := modbus.HandleRTURequestFrame(frame, image)
```

Recommended RTU transport flow with `modbus.NewRTU()`:

```go
payload, err := tr.ReadFrame()
if err != nil {
    return err
}

respPayload, err := modbus.HandleRTURequestPayload(payload, image)
if err != nil {
    return err
}

return tr.WriteFrame(respPayload)
```

Advanced manual flow:

```go
req, err := modbus.ParseTCPRequest(frame)
if err != nil {
    return err
}

resp, err := modbus.ExecuteRequest(req, image)
if err != nil {
    return err
}

out, err := modbus.EncodeTCPResponse(resp)
if err != nil {
    return err
}
```

## Example

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

## Design Notes

- One transport binds to one fixed codec.
- Reads are synchronous and decode one full frame at a time.
- Writes are synchronous and serialized internally.
- There is no handler chain, channel API, or built-in business processing flow.
