# gtransport Modbus Request/Response Models and ProcessImage Design

**Goal**

Add protocol-level Modbus request/response models and a separate `ProcessImage` component to `net/gtransport/modbus`, while keeping the codec boundary strictly stateless.

## Scope

This design covers three units inside `net/gtransport/modbus`:

- request models for supported Modbus function codes
- response models for normal and exception responses
- a standalone `ProcessImage` abstraction with an in-memory implementation

This design applies only to the classic function codes already supported by the codec:

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

- TCP sticky-packet and half-packet handling
- RTU frame detection and resynchronization
- MBAP length and protocol validation
- RTU CRC validation
- supported-function filtering
- static payload checks such as `byteCount`, quantity limits, and address continuity

The codec is not responsible for:

- `Unit ID` / `Slave ID` whitelist or device matching
- `ProcessImage` capacity or initialization state
- request execution against memory or hardware
- device policy, timeout, or data freshness
- business semantics of register values

The new request/response layer is responsible for:

- turning already-framed Modbus ADUs into typed request/response models
- preserving transport metadata needed to re-encode replies
- performing model-level static validation specific to the typed payload

The `ProcessImage` layer is responsible for:

- representing the four Modbus data areas
- enforcing read-only vs read-write boundaries
- performing address bounds checks against configured capacity
- reading and writing semantic values (`bool`, `uint16`)

## Architecture

The design uses two layers above the codec:

1. Protocol model layer
   - Parses framed Modbus TCP/RTU bytes into strongly typed request/response values
   - Encodes typed responses back into ADUs
   - Does not own any mutable device state
2. State layer
   - Provides `ProcessImage` as the storage abstraction for coils, discrete inputs, holding registers, and input registers
   - Does not know TCP vs RTU framing

Execution remains outside this design. A later handler/service layer will map:

`typed request -> ProcessImage operation -> typed response`

## File Layout

Add the following files under `net/gtransport/modbus`:

- `modbus_req.go`
  - request interfaces and typed request structs
  - request parsing entrypoints
  - request-level static validation
- `modbus_resp.go`
  - response interfaces and typed response structs
  - normal and exception response builders
  - response encoding entrypoints
- `process_image.go`
  - `ProcessImage` interface
  - shared errors for address and write-policy violations
- `process_image_memory.go`
  - in-memory `ProcessImage` implementation
- `modbus_req_test.go`
- `modbus_resp_test.go`
- `process_image_z_unit_test.go`

## Transport Metadata

Typed requests and responses need a small shared metadata header so the protocol model layer can preserve framing context without pulling transport logic into `ProcessImage`.

```go
type TransportKind byte

const (
    TransportTCP TransportKind = iota + 1
    TransportRTU
)

type ADUMeta struct {
    Transport     TransportKind
    TransactionID uint16
    UnitID        byte
}
```

Rules:

- `TransactionID` is meaningful only for TCP
- `UnitID` is preserved but not semantically validated against a device identity
- RTU parsing strips CRC before constructing typed requests
- TCP parsing strips MBAP before constructing typed requests

## Request Model

Requests use a small common interface and one concrete type per function code.

```go
type Request interface {
    Meta() ADUMeta
    FunctionCode() FunctionCode
}
```

Concrete request types:

- `ReadCoilsRequest`
- `ReadDiscreteInputsRequest`
- `ReadHoldingRegistersRequest`
- `ReadInputRegistersRequest`
- `WriteSingleCoilRequest`
- `WriteSingleRegisterRequest`
- `WriteMultipleCoilsRequest`
- `WriteMultipleRegistersRequest`

Field guidance:

- read requests store `StartAddress uint16` and `Quantity uint16`
- single-write requests store `Address uint16` and either `Value bool` or `Value uint16`
- multi-write requests store `StartAddress uint16` and either `Values []bool` or `Values []uint16`
- no request type stores raw CRC or raw MBAP bytes

Parsing entrypoints:

- `ParseTCPRequest(frame []byte) (Request, error)`
- `ParseRTURequest(frame []byte) (Request, error)`

The parser may assume the codec already rejected malformed envelopes. It still validates that the framed bytes can be represented as a supported typed request.

## Response Model

Responses mirror protocol semantics rather than forcing a single catch-all struct.

```go
type Response interface {
    Meta() ADUMeta
    FunctionCode() FunctionCode
    IsException() bool
}
```

Concrete response types:

- `ReadBitsResponse`
- `ReadRegistersResponse`
- `WriteSingleCoilResponse`
- `WriteSingleRegisterResponse`
- `WriteMultipleCoilsResponse`
- `WriteMultipleRegistersResponse`
- `ExceptionResponse`

Field guidance:

- read-bit responses store `Values []bool`
- read-register responses store `Values []uint16`
- single-write responses echo `Address` and written value
- multi-write responses store `StartAddress` and accepted quantity
- exception responses store `Function FunctionCode` and `ExceptionCode byte`

Encoding entrypoints:

- `EncodeTCPResponse(resp Response) ([]byte, error)`
- `EncodeRTUResponse(resp Response) ([]byte, error)`

Response encoding must:

- rebuild MBAP length for TCP
- append CRC for RTU
- encode exception responses as `function|0x80`

## Model-Level Validation

The request/response layer performs only static checks that are intrinsic to the typed model.

Included checks:

- supported function code membership
- fixed-length payload shape for read and single-write requests
- `quantity` range checks for supported operations
- `startAddress + quantity` continuity check using 16-bit address space rules
- `FC05` value must be `0x0000` or `0xFF00` when parsing raw bytes
- `FC15` packed coil byte count must match `ceil(quantity/8)`
- `FC16` register byte count must equal `quantity * 2`
- read response byte counts implied by encoded values
- exception response format correctness

Excluded checks:

- whether an address exists in a specific `ProcessImage`
- whether a device permits a function code at runtime
- whether a broadcast request should suppress a response
- request/response correlation across transactions

## ProcessImage

`ProcessImage` is a separate stateful component for four independent address spaces.

```go
type ProcessImage interface {
    ReadCoils(start uint16, quantity uint16) ([]bool, error)
    ReadDiscreteInputs(start uint16, quantity uint16) ([]bool, error)
    ReadHoldingRegisters(start uint16, quantity uint16) ([]uint16, error)
    ReadInputRegisters(start uint16, quantity uint16) ([]uint16, error)

    WriteSingleCoil(address uint16, value bool) error
    WriteSingleRegister(address uint16, value uint16) error
    WriteMultipleCoils(start uint16, values []bool) error
    WriteMultipleRegisters(start uint16, values []uint16) error
}
```

V1 deliberately excludes:

- locking or schema-freeze behavior
- timestamps, quality flags, and freshness policies
- sparse addressing optimizations
- callbacks, subscriptions, or change notifications
- protocol parsing or transport metadata

## In-Memory Implementation

The first implementation is a simple contiguous in-memory store.

```go
type MemoryProcessImage struct {
    coils            []bool
    discreteInputs   []bool
    holdingRegisters []uint16
    inputRegisters   []uint16
}
```

Constructor:

```go
func NewMemoryProcessImage(
    coilCount int,
    discreteInputCount int,
    holdingRegisterCount int,
    inputRegisterCount int,
) *MemoryProcessImage
```

Behavior:

- reads return copies so callers cannot mutate internal slices by alias
- `DiscreteInputs` and `InputRegisters` are read-only through the public interface
- multi-write operations fail if any part of the requested range is outside the configured capacity
- zero-quantity reads and zero-length writes are rejected as invalid usage

## Error Model

The request/response layer should return ordinary Go errors with clear protocol context, for example:

- unsupported function code
- invalid packed coil byte count
- invalid exception response function

`ProcessImage` should expose reusable errors for stateful failures, for example:

- address out of range
- quantity out of range
- write not allowed

These errors support later mapping into Modbus exception codes by the future handler layer.

## Testing Strategy

### Request/Response tests

- parse one valid request for each supported function code
- reject invalid `FC05` coil values
- reject invalid `FC15` / `FC16` byte counts
- reject address continuity overflow
- encode one valid normal response for each response shape
- encode valid exception responses
- reject malformed exception responses
- verify TCP vs RTU metadata preservation

### ProcessImage tests

- read each data area successfully within bounds
- reject out-of-range reads and writes
- verify read-only areas remain read-only by interface shape
- verify multi-write atomic bounds enforcement
- verify returned slices are copies

## Non-Goals

- execute Modbus requests against `ProcessImage`
- build a server-side dispatcher in this phase
- support user-configurable `Unit ID` policies
- add non-standard function codes
- add wire-format auto-detection between TCP and RTU
