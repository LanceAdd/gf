# gtransport Modbus Test Matrix Hardening Design

**Goal**

Expand `net/gtransport/modbus` test coverage so the existing protocol-static validation behavior is protected more evenly across supported function-code groups and exception-frame handling.

## Scope

This design applies only to tests under:

- `net/gtransport/modbus/modbus_tcp_test.go`
- `net/gtransport/modbus/modbus_rtu_test.go`

It does not intentionally change:

- public API
- supported function codes
- transport behavior
- validation rules already implemented in production code

## Problem

The current package behavior is broader than the current test matrix.

Coverage is currently concentrated around:

- TCP framing basics
- RTU CRC and framing basics
- `0x03`, `0x05`, `0x0F`, `0x10`
- RTU resynchronization

But the matrix is still uneven in three protocol-static areas:

1. Exception-frame decode behavior is not directly protected on both transports
2. `0x02`, `0x04`, and `0x06` rely on shared validators but lack dedicated regression tests
3. Address-range overflow protection is only directly exercised for a subset of the function-code families that now enforce it

## Validation Boundary

This work is test-only. It validates that the codec continues to enforce protocol-static rules it already claims to support.

It does not add tests for:

- Unit ID whitelist policy
- ProcessImage semantics
- application-level register meanings
- unsupported vendor/private functions

## Required Coverage

### Exception Frames

Add direct decode coverage for:

- TCP exception frame acceptance for a supported base function
- RTU exception frame acceptance for a supported base function
- rejection of malformed exception length
- rejection of exception frames whose base function is unsupported

### Function-Code Family Coverage

Add direct regression tests for:

- `0x02` Read Discrete Inputs
- `0x04` Read Input Registers
- `0x06` Write Single Register

The tests should prove these function families behave correctly in at least one representative success or failure path relevant to their dedicated validators.

### Address-Range Coverage

Add address-range overflow coverage for the remaining request families that already enforce the shared range rule:

- `0x01`
- `0x02`
- `0x04`
- `0x0F`

This should cover at least one TCP or RTU path per function family, without redundantly duplicating the same assertion on both transports unless it adds real value.

## Test Design Principles

- Prefer narrow regression tests with one clear behavior each
- Reuse existing test style and helper conventions
- Keep transport-specific assertions in the transport-specific test file
- Avoid broad table-driven rewrites unless necessary

## Expected Outcome

After this work:

- the test matrix more evenly reflects the current implementation
- exception-frame behavior is directly protected
- shared validator branches are less dependent on indirect coverage
- future refactors are less likely to silently weaken a supported function family

## Non-Goals

This work does not:

- introduce new production features
- redesign the validators
- add new codec options
- broaden scope into policy validation
