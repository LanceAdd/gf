# gtransport Modbus Address Range Validation Design

**Goal**

Add stateless address-range validation to `net/gtransport/modbus` so request frames that carry `startAddress + quantity` are rejected when the requested range exceeds the Modbus 16-bit address space.

## Scope

This design applies only to the existing Modbus codec package:

- `github.com/gogf/gf/v2/net/gtransport/modbus`

It extends validation for the currently supported function codes:

- `0x01` Read Coils
- `0x02` Read Discrete Inputs
- `0x03` Read Holding Registers
- `0x04` Read Input Registers
- `0x0F` Write Multiple Coils
- `0x10` Write Multiple Registers

## Validation Boundary

The codec remains a stateless protocol validator. It is responsible for:

- transport envelope validation
- frame completeness detection
- CRC validation for RTU
- MBAP validation for TCP
- static field validation for supported function codes
- static request address-range validation using only bytes present in the current frame

The codec is not responsible for:

- `ProcessImage` bounds
- whether a specific address is initialized
- device-specific register maps
- slave/unit routing legitimacy
- broadcast semantics
- request/response correlation

## Problem

The current shared payload validators enforce quantity limits, byte-count consistency, and fixed request/response shapes, but they do not reject request frames whose requested address range crosses the Modbus 16-bit address boundary.

Examples that should be rejected:

- `startAddress = 0xFFFF, quantity = 2`
- `startAddress = 65530, quantity = 10`

Examples that should remain valid:

- `startAddress = 0xFFFF, quantity = 1`
- `startAddress = 65535 - quantity + 1`

## Rule

For request-shaped payloads that include both `startAddress` and `quantity`, compute:

`endExclusive = startAddress + quantity`

The frame is valid only when:

- `quantity` already satisfies the existing function-specific quantity limits
- `endExclusive <= 65536`

This uses exclusive-end semantics so that:

- `startAddress = 65535, quantity = 1` is valid
- any request that would need an address greater than `65535` is invalid

## Function-Code Matrix

### `0x01` Read Coils

- Request shape: fixed 6-byte payload
- New validation: reject if `startAddress + quantity > 65536`
- Response shape: unchanged

### `0x02` Read Discrete Inputs

- Request shape: fixed 6-byte payload
- New validation: reject if `startAddress + quantity > 65536`
- Response shape: unchanged

### `0x03` Read Holding Registers

- Request shape: fixed 6-byte payload
- New validation: reject if `startAddress + quantity > 65536`
- Response shape: unchanged

### `0x04` Read Input Registers

- Request shape: fixed 6-byte payload
- New validation: reject if `startAddress + quantity > 65536`
- Response shape: unchanged

### `0x0F` Write Multiple Coils

- Request shape: variable-length payload with `byteCount`
- New validation: reject if `startAddress + quantity > 65536`
- Response shape: unchanged

### `0x10` Write Multiple Registers

- Request shape: variable-length payload with `byteCount`
- New validation: reject if `startAddress + quantity > 65536`
- Response shape: unchanged

### `0x05` and `0x06`

No new address-range validation is added. These functions operate on a single address and do not carry a `quantity` field, so there is no multi-address range to validate.

### Responses and Exceptions

No new address-range validation is added for:

- read responses
- write confirmation responses
- exception responses

Those payloads do not contain the full `startAddress + quantity` request semantics required for this check.

## Architecture

Keep the transport-specific codecs unchanged in responsibility:

- TCP validates MBAP and delegates payload validation
- RTU derives frame length, validates CRC, and delegates payload validation

Add the new rule inside the shared payload validators in `modbus_validation.go` so that:

- TCP and RTU get identical behavior
- frame detection remains separate from payload legality
- the change stays local to one validation layer

Implementation should use a small helper that:

- reads `startAddress` and `quantity` from a request-shaped payload
- converts them to `int`
- rejects frames where `startAddress + quantity > 65536`

## Error Handling

When the current bytes are deterministically invalid, return an error immediately rather than `gtransport.ErrNeedMoreData`.

Error messages should stay descriptive, lower-case, and scoped to Modbus payload validation, consistent with the existing `invalid modbus ...` style in this package. A direct error such as `invalid modbus address range` is sufficient.

## Testing

Add coverage for both transports.

Required passing cases:

- TCP read request at the upper boundary: `startAddress = 0xFFFF, quantity = 1`
- RTU batch write request at the upper boundary: `startAddress + quantity = 65536`

Required failing cases:

- TCP read request crossing the boundary
- RTU read request crossing the boundary
- TCP write-multiple request crossing the boundary
- RTU write-multiple request crossing the boundary

Regression requirement:

- Existing byte-count, CRC, MBAP, and supported-function behavior remains unchanged.
