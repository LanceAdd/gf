# gtransport Modbus RTU Resynchronization Design

**Goal**

Add in-codec RTU resynchronization to `net/gtransport/modbus` so `Decode` can skip invalid leading bytes and return the first later valid RTU frame without pushing recovery responsibility to the caller.

## Scope

This design applies only to:

- `github.com/gogf/gf/v2/net/gtransport/modbus`

The change is limited to RTU decoding behavior. It does not change:

- Modbus TCP behavior
- RTU encoding behavior
- shared Modbus payload validation rules
- public constructors or public API

## Problem

The current RTU codec validates frame length, CRC, and payload legality only at offset `0`. If the input buffer begins with noise bytes, a malformed RTU frame, or an unsupported function-code prefix, `Decode` returns an error immediately instead of scanning forward to find a later valid frame.

That is insufficient for the stated codec goal of handling stream-oriented framing robustly when the byte stream can contain misaligned or corrupted leading bytes.

## Required Behavior

`rtuCodec.Decode` must scan forward through the input buffer one byte at a time until one of these outcomes occurs:

1. A valid RTU frame is found
2. The remaining bytes are insufficient to determine whether the current candidate could become a valid frame
3. No valid frame can exist in the current bytes without more data

### Valid Frame Found

When a valid frame is found:

- return the payload without trailing CRC, consistent with current RTU behavior
- return `consumed` equal to the number of bytes skipped plus the valid frame length

This lets the transport discard both noise bytes and the decoded frame in a single consume step.

### Deterministically Invalid Candidate

When the current candidate start is deterministically invalid, `Decode` must skip exactly one byte and continue scanning.

This includes:

- unsupported function code at the current candidate start
- enough bytes for a complete candidate frame, but CRC validation fails
- enough bytes for a complete candidate frame, CRC passes, but payload static validation fails

### Incomplete Candidate

When the bytes from the current candidate start are still insufficient to determine whether the candidate could become a valid frame, `Decode` must return `gtransport.ErrNeedMoreData` and stop scanning.

This prevents consuming a partial valid frame header as if it were noise.

## Reused Framing Rules

The existing RTU frame-shape logic remains authoritative:

- fixed-length request/response handling for `0x05` and `0x06`
- ambiguous request/response handling for `0x01/0x02/0x03/0x04`
- fixed-length response vs variable-length request handling for `0x0F/0x10`
- exception response handling via `function | 0x80`

Resynchronization changes only where decoding starts, not how a candidate frame is interpreted.

## Validation Boundary

The codec remains a stateless validator. It is responsible for:

- candidate frame-shape discovery
- CRC validation
- shared payload legality validation
- byte-stream recovery by skipping invalid leading bytes

The codec is not responsible for:

- semantic validation against device state
- process-image bounds
- message correlation
- recovering bytes after `ErrNeedMoreData`

## Architecture

Keep `frameLength` as the per-candidate shape detector. Do not move resynchronization policy into `frameLength`.

`Decode` should become a small scanning loop:

1. Try to decode from current offset
2. If candidate is valid, return it
3. If candidate is deterministically invalid, advance by one byte
4. If candidate needs more bytes, return `ErrNeedMoreData`

To keep the code focused, candidate decoding may be factored into a small helper that works on a slice beginning at the current offset and reports:

- payload
- candidate frame length
- candidate status: valid, invalid, need more data

The helper should reuse the existing CRC and payload validation code paths rather than duplicating them.

## Error Handling

After this change, RTU `Decode` should prefer resynchronization over returning hard errors for invalid leading bytes.

Hard errors are still acceptable only when the data at the chosen candidate start is validly framed but exceeds internal safety limits in a deterministic way. In practice, most bad-prefix cases should now be recovered by scanning.

`ErrNeedMoreData` remains the signal when the remaining bytes could still become a valid frame.

## Testing

Add RTU tests covering:

- noise prefix followed by a valid RTU frame
- unsupported-function prefix followed by a valid RTU frame
- invalid-CRC candidate followed by a valid RTU frame
- noise prefix followed by an incomplete RTU frame, which must return `ErrNeedMoreData`

Required assertions:

- decoded payload matches the later valid frame
- `consumed` includes skipped bytes plus the valid frame length
- partial trailing data is not consumed on `ErrNeedMoreData`

## Non-Goals

This design does not add:

- probabilistic CRC scanning across arbitrary lengths
- idle-gap timing behavior
- TCP resynchronization
- new function-code support
