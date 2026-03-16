# gtransport Modbus Executor Design

**Goal**

Add a thin Modbus executor layer that runs typed `Request` values against a `ProcessImage` and produces typed `Response` values, while normalizing naming from `UnitID` to `SlaveID` across the Modbus package.

## Scope

This design covers one focused sub-project inside `net/gtransport/modbus`:

- rename Modbus addressing metadata from `UnitID` to `SlaveID`
- add a request executor that maps the 8 supported classic function codes onto `ProcessImage`
- map protocol-level execution failures into `ExceptionResponse`

Supported function codes:

- `0x01` Read Coils
- `0x02` Read Discrete Inputs
- `0x03` Read Holding Registers
- `0x04` Read Input Registers
- `0x05` Write Single Coil
- `0x06` Write Single Register
- `0x0F` Write Multiple Coils
- `0x10` Write Multiple Registers

## Confirmed Boundary

The codec remains responsible for:

- framing
- CRC / MBAP validation
- static payload legality

The request/response model layer remains responsible for:

- typed protocol representation
- TCP/RTU parsing and encoding

The new executor layer is responsible for:

- dispatching typed requests by function code
- reading and writing `ProcessImage`
- turning execution failures that have Modbus meaning into `ExceptionResponse`

The executor is not responsible for:

- socket reads and writes
- stream lifecycle or session state
- broadcast special handling
- retries, timeouts, or device liveness
- policy around device identity beyond carrying `SlaveID`

## Naming Normalization

The package should use a single address-field term everywhere: `SlaveID`.

Required normalization:

- `ADUMeta.UnitID` becomes `ADUMeta.SlaveID`
- tests, helpers, response encoders, request parsers, and executor code all use `SlaveID`
- docs written by this sub-project also use `SlaveID`

This is intentionally package-wide. Mixing `UnitID` and `SlaveID` in adjacent layers would make executor code harder to reason about and would spread naming churn into every later task.

## Architecture

The executor should stay as a thin pure-function layer:

```go
func ExecuteRequest(req Request, image ProcessImage) (Response, error)
```

Flow:

1. Inspect the concrete `Request` type
2. Execute the corresponding `ProcessImage` operation
3. Build a typed normal `Response` on success
4. Map recognized execution failures to `ExceptionResponse`
5. Return a Go `error` only for internal faults that do not cleanly map to Modbus protocol semantics

This keeps execution logic centralized without introducing state, configuration, or service wiring before it is needed.

## File Layout

Add or modify the following files under `net/gtransport/modbus`:

- `modbus_req.go`
  - rename `UnitID` to `SlaveID`
- `modbus_resp.go`
  - rename `UnitID` to `SlaveID`
- `process_image.go`
  - keep `ProcessImage` and stateful error definitions
- `modbus_executor.go`
  - `ExecuteRequest`
  - request dispatch
  - error-to-exception mapping
- `modbus_executor_test.go`
  - executor happy path and exception path tests
- existing request/response tests
  - rename metadata assertions from `UnitID` to `SlaveID`

No new transport or networking files are needed.

## Executor API

Primary API:

```go
func ExecuteRequest(req Request, image ProcessImage) (Response, error)
```

Behavior contract:

- returns a normal typed `Response` for successful execution
- returns `ExceptionResponse, nil` for protocol-level execution failures
- returns non-nil Go `error` only for internal faults or impossible states

There is no executor struct in V1 because:

- no config has been identified
- no dependencies need injection
- no mutable state is needed
- a function is easier to test exhaustively

The executor trusts the typed request layer for codec-level static validation. It does not re-check byte counts, CRC-derived framing, or request quantity legality that should already have been enforced before a `Request` is constructed. Hand-constructed impossible request states may still surface as Go `error` when they no longer map cleanly to a Modbus exception.

## Request to Response Mapping

### Read requests

- `ReadCoilsRequest` -> `image.ReadCoils` -> `ReadBitsResponse` with function `0x01`
- `ReadDiscreteInputsRequest` -> `image.ReadDiscreteInputs` -> `ReadBitsResponse` with function `0x02`
- `ReadHoldingRegistersRequest` -> `image.ReadHoldingRegisters` -> `ReadRegistersResponse` with function `0x03`
- `ReadInputRegistersRequest` -> `image.ReadInputRegisters` -> `ReadRegistersResponse` with function `0x04`

### Write requests

- `WriteSingleCoilRequest` -> `image.WriteSingleCoil` -> `WriteSingleCoilResponse`
- `WriteSingleRegisterRequest` -> `image.WriteSingleRegister` -> `WriteSingleRegisterResponse`
- `WriteMultipleCoilsRequest` -> `image.WriteMultipleCoils` -> `WriteMultipleCoilsResponse`
- `WriteMultipleRegistersRequest` -> `image.WriteMultipleRegisters` -> `WriteMultipleRegistersResponse`

Response metadata rules:

- `SlaveID` is copied from the request metadata
- `Transport` is copied from the request metadata
- `TransactionID` is preserved for TCP responses
- `TransactionID` remains zero-value for RTU responses
- response function code matches the request function code except for exceptions, which encode `function | 0x80`

## Exception Mapping

The executor should prefer returning protocol exceptions over raw Go errors when the failure has a clear Modbus meaning.

Mapped failures:

- unsupported request function -> `ExceptionResponse, nil` with exception code `0x01` (`Illegal Function`)
- `ErrProcessImageAddressOutOfRange` -> exception code `0x02` (`Illegal Data Address`)
- `ErrProcessImageQuantityOutOfRange` -> exception code `0x03` (`Illegal Data Value`)
- `ErrProcessImageWriteValuesEmpty` -> exception code `0x03` (`Illegal Data Value`)

Unmapped failures stay as Go errors, for example:

- a nil `Request`
- malformed request state that should have been impossible after parsing
- unexpected `ProcessImage` implementation errors without protocol meaning

Unsupported-function source:

- if the executor receives a non-nil `Request` implementation whose `FunctionCode()` is outside the executor support matrix, it should still build `ExceptionResponse, nil` using the request metadata and original function code
- this preserves standard Modbus `Illegal Function` behavior even if future request implementations are added later

## Broadcast Handling

Broadcast is explicitly out of scope for V1.

Rules for now:

- `SlaveID == 0` is treated as an ordinary metadata value
- executor still returns a normal or exception `Response, nil`
- this applies uniformly to all 8 supported function codes in V1
- no special “execute but suppress reply” behavior is added

This avoids complicating the executor return contract before there is a concrete broadcast requirement.

## Error Model

The executor should use a small internal helper for exception construction so each dispatch branch does not duplicate metadata copying.

Suggested internal helper shape:

```go
func newExceptionResponse(meta ADUMeta, function FunctionCode, exceptionCode byte) ExceptionResponse
```

This helper is internal to the executor layer and not required as public API.

## Testing Strategy

### Rename coverage

- full-package search confirms there are no remaining `ADUMeta.UnitID` references in `net/gtransport/modbus`
- request parser tests assert `SlaveID`
- response encoder tests assert `SlaveID`
- package API shape test references `ADUMeta.SlaveID`

### Happy-path executor tests

- all 8 supported requests execute successfully against `MemoryProcessImage`
- read responses preserve values and function codes
- write responses echo address or quantity correctly
- metadata is preserved across execution

### Exception-path executor tests

- out-of-range address maps to exception `0x02`
- zero quantity or empty write values map to exception `0x03`
- unsupported request function maps to `ExceptionResponse, nil` with exception `0x01`
- unknown internal failure remains Go `error`
- `SlaveID == 0` still produces the same response shape as any other request

## Non-Goals

- broadcast no-response semantics
- pluggable handler registry
- concurrency policy inside `ProcessImage`
- request authorization or device selection
- network server integration
- retry logic or timeout behavior

## Compatibility

This sub-project intentionally accepts a package-level breaking change inside `net/gtransport/modbus`:

- `ADUMeta.UnitID` is renamed to `ADUMeta.SlaveID`

There is no compatibility alias or transition period in V1. The rename is expected to be applied consistently across code, tests, examples, and docs in the same implementation batch.
