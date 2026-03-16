# GTCP Frame Decoder Design

## Overview
- Topic: Add Netty-like frame decoding capabilities to `net/gtcp`.
- Date: 2026-03-11
- Status: Approved by discussion

## Background
`gtcp` already provides:
- raw stream methods: `Recv`, `RecvLine`, `RecvTill`
- simple length-header protocol: `SendPkg`/`RecvPkg`

But it lacks a unified, pluggable decoder abstraction. This makes protocol evolution harder and pushes frame handling details into business code.

## Goals
- Add pluggable frame decoding to `Conn` as the first phase.
- Add `Server` message callback mode as the second phase.
- First built-in decoders:
  1. Fixed length
  2. Custom delimiter (line delimiter is a special case)
  3. Custom decoder interface

## Non-Goals
- No full Netty pipeline (encoder/decoder/handler chain) in this phase.
- No breaking changes to existing public APIs (`Recv*`, `Send*`, `RecvPkg*`, etc).

## Confirmed Product Decisions
- `ReadFrame` returns exactly one frame per call.
- Decoder protocol error handling: close the connection immediately.
- Rollout strategy:
  1. Phase 1: `Conn` level decoder abstraction
  2. Phase 2: `Server` level `SetDecoder + SetMessageHandler`

## Proposed API (Phase 1)
```go
package gtcp

import "errors"

var ErrNeedMoreData = errors.New("need more data")

type Decoder interface {
    // Decode tries parsing one frame from input bytes.
    // frame: decoded payload (single frame)
    // consumed: number of bytes consumed from input
    // err:
    //   - ErrNeedMoreData: half packet, continue reading
    //   - other errors: protocol error
    Decode(in []byte) (frame []byte, consumed int, err error)
}

func (c *Conn) ReadFrame(decoder Decoder) ([]byte, error)
```

Built-ins:
- `NewFixedLengthDecoder(length int) Decoder`
- `NewDelimiterDecoder(delim []byte, maxFrameLen int, stripDelimiter bool) Decoder`

## Decoder Runtime Model
Connection keeps an internal receive buffer for frame decoding.

Per `ReadFrame(decoder)`:
1. Try `decoder.Decode(buffer)` in a loop.
2. If frame decoded:
   - consume bytes
   - return one frame
3. If `ErrNeedMoreData`:
   - read more bytes from socket
   - append to buffer
   - continue
4. If protocol error:
   - return error to caller
   - caller closes connection (or server mode closes automatically)

Sticky packet handling:
- A single read may contain multiple frames.
- `ReadFrame` returns only one frame each call.
- Remaining bytes stay in internal buffer for subsequent calls.

Half packet handling:
- Decoder returns `ErrNeedMoreData`.
- Runtime keeps reading until enough bytes are available.

## Proposed API (Phase 2)
`Server` gains message mode while keeping existing handler mode:

```go
func (s *Server) SetDecoder(dec Decoder)
func (s *Server) SetMessageHandler(handler func(conn *Conn, frame []byte))
```

Server run behavior:
- If decoder is not configured: keep existing `SetHandler` behavior unchanged.
- If decoder is configured:
  - server reads stream and decodes frames
  - invoke `messageHandler` per frame
  - on protocol error: close this connection immediately
  - other connections remain unaffected

## Validation and Limits
- `FixedLengthDecoder(length <= 0)` -> invalid parameter error
- `DelimiterDecoder(delim empty)` -> invalid parameter error
- `maxFrameLen` guard required to prevent unbounded memory growth
- connection-level decode buffer max size guard required (DoS protection)

## Compatibility
- Existing APIs and behavior stay unchanged.
- New behavior is opt-in by using `ReadFrame` or `SetDecoder`.

## Test Strategy
### Unit tests (decoders)
- fixed-length half packet
- fixed-length sticky packet (multiple frames in one read)
- delimiter half packet (delimiter split across reads)
- delimiter sticky packet
- invalid config and frame-too-large errors

### Conn integration
- `ReadFrame` returns one frame per call
- remaining bytes preserved for next call
- protocol error path returns error

### Server integration
- old handler path regression
- decoder/message path integration
- protocol error closes current connection immediately

## Risks and Mitigations
- Risk: read loop deadlock/timeouts under malformed peer behavior
  - Mitigation: reuse existing conn deadline behavior and buffer limits
- Risk: API ambiguity if both handler modes are set
  - Mitigation: explicit runtime validation and deterministic precedence

## Milestones
1. Phase 1 API + decoder core + fixed/delimiter built-ins + tests
2. Phase 2 server integration + tests
3. docs updates and examples
