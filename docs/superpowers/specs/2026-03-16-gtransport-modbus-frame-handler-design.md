# gtransport Modbus Frame Handler Design

**Goal**

Add a thin Modbus frame handler layer that connects typed request parsing, executor dispatch, and typed response encoding for both TCP and RTU frames.

## Scope

This design covers one focused sub-project inside `net/gtransport/modbus`:

- add explicit TCP and RTU request-frame handler helpers
- connect existing request parsing, request execution, and response encoding into one frame-level flow
- provide frame-level integration coverage for normal responses and exception responses

This design does not change:

- `gtransport.Transport`
- existing Modbus codec framing responsibilities
- `ProcessImage` responsibilities
- executor request-to-response semantics

## Confirmed Boundary

The package boundary remains layered:

- codec layer
  - validates MBAP / CRC / static payload legality
  - converts byte frames into typed requests and typed responses back into frames
- executor layer
  - executes typed requests against `ProcessImage`
  - maps protocol-level execution failures to typed `ExceptionResponse`
- frame handler layer
  - composes parse -> execute -> encode into one stateless helper

The frame handler is intentionally not a transport loop, not a server abstraction, and not a policy layer.

## API Shape

The integration layer should expose explicit transport-specific helpers:

```go
func HandleTCPRequestFrame(frame []byte, image ProcessImage) ([]byte, error)
func HandleRTURequestFrame(frame []byte, image ProcessImage) ([]byte, error)
func HandleRTURequestPayload(payload []byte, image ProcessImage) ([]byte, error)
```

The design intentionally does not use a single auto-detecting `HandleRequestFrame` helper because:

- TCP and RTU request frames have different wire shapes
- RTU includes CRC while TCP includes MBAP
- callers already know which codec they are using
- explicit entry points keep error behavior predictable

## Flow

### TCP

```go
ParseTCPRequest(frame) -> ExecuteRequest(req, image) -> EncodeTCPResponse(resp)
```

### RTU

```go
ParseRTURequest(frame) -> ExecuteRequest(req, image) -> EncodeRTUResponse(resp)
```

### RTU Transport Payload

For `gtransport.New(..., modbus.NewRTU())`, transport reads and writes decoded RTU payloads rather than raw CRC-bearing ADUs. That integration path should use:

```go
validateModbusPayload(payload) -> parseRequest(meta, payload) -> ExecuteRequest(req, image) -> encodeResponsePayload(resp)
```

Return rules:

- parse failure returns Go `error`
- `nil image` returns Go `error`
- successful execution returns a normal encoded response frame
- protocol-level execution failure returns an encoded Modbus exception frame

The frame handler does not inspect `SlaveID` semantics beyond preserving the behavior already defined by the executor.

## File Layout

Add the following files under `net/gtransport/modbus`:

- `handle_request.go`
  - `HandleTCPRequestFrame`
  - `HandleRTURequestFrame`
  - `HandleRTURequestPayload`
  - minimal shared helper if needed
- `handle_request_z_unit_test.go`
  - frame-level integration tests

Existing files reused without boundary changes:

- `modbus_req.go`
- `modbus_resp.go`
- `modbus_executor.go`
- `process_image.go`
- `process_image_memory.go`

No files under `net/gtransport` root need API changes for this sub-project.

## Error Model

The frame handler should stay thin and preserve existing layer semantics:

- request parsing errors remain Go `error`
- executor misuse errors remain Go `error`
- executor protocol exceptions are encoded and returned as normal response frames

This means the frame handler should not re-map errors that were already classified by the executor.

## Testing Strategy

Add frame-level tests that validate the full byte-frame closure:

### Happy path

- TCP read request frame -> normal TCP response frame
- TCP write request frame -> normal TCP response frame and `MemoryProcessImage` state change
- RTU read request frame -> normal RTU response frame
- RTU write request frame -> normal RTU response frame and `MemoryProcessImage` state change

### Exception path

- address out of range -> encoded exception response frame
- invalid quantity / empty write values -> encoded exception response frame
- unsupported function -> encoded exception response frame

### Misuse path

- `nil image` -> Go `error`
- invalid TCP frame -> Go `error`
- invalid RTU frame -> Go `error`

These tests should assert actual encoded bytes, not just typed response values, because the purpose of this layer is frame-level composition.

## Non-Goals

- automatic TCP/RTU protocol detection
- `ServeOnce` helper
- long-running server loop
- slave filtering
- broadcast no-response behavior
- timeout, retry, or transport lifecycle policy
- new public API in `net/gtransport`

## Compatibility

This design is additive within `net/gtransport/modbus`.

It introduces new helpers without changing the existing typed request/response or executor APIs. Existing callers can keep using parse/execute/encode directly if they want lower-level control.
