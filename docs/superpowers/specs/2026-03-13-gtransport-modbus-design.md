# gtransport Modbus Codec Design

**Goal**

Add Modbus protocol codecs on top of `net/gtransport` without polluting the core transport package. The first version must support both Modbus TCP and Modbus RTU and keep the `gtransport` core focused on generic framing.

## Package Placement

Modbus is protocol-specific and carries more domain knowledge than the generic codecs in `gtransport`. It should live in a protocol subpackage:

`github.com/gogf/gf/v2/net/gtransport/modbus`

This keeps the public structure clean:

- `gtransport` contains generic framing and transport primitives
- `gtransport/modbus` contains Modbus-specific framing rules

## Public API

The first version should stay minimal:

```go
package modbus

func NewTCP() gtransport.Codec
func NewRTU() gtransport.Codec
```

Do not add options in the first version. If later requirements justify tuning, add explicit constructors such as `NewTCPWithOption` and `NewRTUWithOption`.

## Data Shape and Codec Responsibility

### Modbus TCP

`NewTCP()` should operate on complete Modbus TCP ADUs.

- `Decode` returns the full TCP ADU:
  - Transaction ID
  - Protocol ID
  - Length
  - Unit ID
  - PDU
- `Encode` accepts a full TCP ADU and rewrites the MBAP `Length` field before output

The codec must not trust a caller-provided MBAP length value.

### Modbus RTU

`NewRTU()` should operate on RTU frames without exposing CRC to the caller.

- `Decode` validates CRC and returns the RTU payload without the trailing CRC:
  - Address
  - Function
  - Data
- `Encode` accepts RTU payload without CRC and appends the correct CRC automatically

The caller should never need to calculate or strip CRC manually.

## RTU Framing Strategy

The RTU codec should not depend on serial-line idle timing. In `gtransport`, framing must be derived from bytes already read, not physical layer silence.

RTU frame completeness should therefore be determined by Modbus function rules:

- minimum bytes for address and function
- fixed-length request/response formats where applicable
- byte-count-driven formats where applicable
- exception responses using `function | 0x80`

This means the RTU codec is intentionally protocol-aware, which is exactly why it belongs in the Modbus subpackage instead of the `gtransport` root package.

## Supported Function Codes in V1

The first version should support the common public function codes:

- `0x01` Read Coils
- `0x02` Read Discrete Inputs
- `0x03` Read Holding Registers
- `0x04` Read Input Registers
- `0x05` Write Single Coil
- `0x06` Write Single Register
- `0x0F` Write Multiple Coils
- `0x10` Write Multiple Registers
- exception responses (`function | 0x80`)

For unsupported function codes, return an explicit error instead of guessing a frame shape.

## Error Handling

The codec should fail clearly in these cases:

- incomplete MBAP header
- invalid TCP length field
- RTU frame shorter than the minimum valid structure
- unsupported RTU function code
- invalid RTU CRC
- frame lengths exceeding internal safety bounds

The behavior should remain deterministic: incomplete data returns `gtransport.ErrNeedMoreData`, malformed frames return descriptive errors.

## Non-Goals for V1

The first version should not include:

- configurable protocol options
- vendor-specific/private function-code heuristics
- serial idle-gap framing
- PDU-only abstractions
- gateway-level translation between RTU and TCP
- request/response correlation helpers

The purpose of this work is to provide stable protocol codecs, not a full Modbus stack.

## Recommended Next Generic Codec After Modbus

If `gtransport` later needs another generic framing codec comparable to common Netty usage, the next high-value addition should be a varint length-prefixed codec. That belongs in the main `gtransport` package because it is generic framing, unlike Modbus.
