# gtransport Modbus Validation Hardening Design

**Goal**

Harden `net/gtransport/modbus` so the codec validates complete Modbus frame legality within the codec's stateless responsibility, instead of only validating framing and CRC.

## Scope

This design covers only static, stateless validation for the function codes already supported by `gtransport/modbus`:

- `0x01` Read Coils
- `0x02` Read Discrete Inputs
- `0x03` Read Holding Registers
- `0x04` Read Input Registers
- `0x05` Write Single Coil
- `0x06` Write Single Register
- `0x0F` Write Multiple Coils
- `0x10` Write Multiple Registers
- exception responses (`function | 0x80`)

This work applies to both Modbus TCP and Modbus RTU.

## Validation Boundary

The codec is responsible for:

- envelope validation
- frame completeness detection
- static function-code-specific length and field validation
- CRC validation for RTU
- MBAP validation and rewrite for TCP

The codec is not responsible for:

- request/response correlation
- transaction ordering
- broadcast semantics
- device address legitimacy
- Unit ID or slave-address whitelist policy
- ProcessImage bounds or initialization state
- application-level meaning of register values
- context-dependent validation that needs previous frames

## General Rules

- Return `gtransport.ErrNeedMoreData` only when additional bytes could still make the frame valid.
- Return an error immediately once the current bytes are deterministically invalid.
- Reject unsupported function codes explicitly.
- Keep validation deterministic and shared across TCP and RTU where the PDU rules are identical.

## Modbus TCP Requirements

- MBAP header must be at least 7 bytes.
- `Protocol ID` must be `0`.
- MBAP `Length` must be in `2..254`.
- Total ADU length must equal `6 + Length`.
- `Encode` must recompute and rewrite `Length`.
- After MBAP validation, TCP must validate Unit ID + PDU using the same function-specific rules as RTU payload validation.

## Modbus RTU Requirements

- RTU frame must include address, function, and CRC.
- Total ADU length must not exceed 256 bytes.
- CRC must be validated before accepting a frame.
- Framing must continue to rely on function-code-derived lengths, not serial idle timing.
- After frame-length detection, RTU must validate address + PDU using the same function-specific rules as TCP payload validation.

## Function-Specific Requirements

### `0x01` Read Coils

- Request payload length: fixed 6 bytes
- Request quantity: `1..2000`
- Response payload length: `3 + byteCount`
- Response byte count: `1..250`

### `0x02` Read Discrete Inputs

- Request payload length: fixed 6 bytes
- Request quantity: `1..2000`
- Response payload length: `3 + byteCount`
- Response byte count: `1..250`

### `0x03` Read Holding Registers

- Request payload length: fixed 6 bytes
- Request quantity: `1..125`
- Response payload length: `3 + byteCount`
- Response byte count: even and in `2..250`

### `0x04` Read Input Registers

- Request payload length: fixed 6 bytes
- Request quantity: `1..125`
- Response payload length: `3 + byteCount`
- Response byte count: even and in `2..250`

### `0x05` Write Single Coil

- Request payload length: fixed 6 bytes
- Response payload length: fixed 6 bytes
- Coil value must be `0x0000` or `0xFF00`

### `0x06` Write Single Register

- Request payload length: fixed 6 bytes
- Response payload length: fixed 6 bytes
- Register value may be any `uint16`

### `0x0F` Write Multiple Coils

- Request payload length: `7 + byteCount`
- Request quantity: `1..1968`
- Request byte count: `1..246`
- Request byte count must equal `ceil(quantity/8)`
- Response payload length: fixed 6 bytes
- Response quantity: `1..1968`

### `0x10` Write Multiple Registers

- Request payload length: `7 + byteCount`
- Request quantity: `1..123`
- Request byte count: `2..246`
- Request byte count must equal `quantity * 2`
- Response payload length: fixed 6 bytes
- Response quantity: `1..123`

### Exception Response

- Payload length: fixed 3 bytes
- Base function code must be in the supported set
- Exception code is accepted as any single byte

## Architecture

Validation should be centralized into shared helpers that operate on:

- transport kind: TCP or RTU
- payload kind: request, response, or exception
- address/unit id + function + function payload bytes

The transport-specific code should stay small:

- TCP: validate MBAP, rewrite MBAP length, delegate payload validation
- RTU: detect frame length, validate CRC, delegate payload validation

## Implementation Strategy

1. Add a shared validation layer for supported function codes.
2. Route both TCP and RTU encode/decode through the shared validators.
3. Expand tests to cover all supported function-code constraints across both transports.
4. Keep public API unchanged.
