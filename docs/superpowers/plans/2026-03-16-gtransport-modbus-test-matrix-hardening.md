# gtransport Modbus Test Matrix Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add focused regression tests that more evenly cover existing Modbus protocol-static validation behavior across exception frames, under-covered function families, and remaining address-range branches.

**Architecture:** Change only the transport-specific Modbus test files. Add failing tests first for the specific existing behaviors we want protected, then run the package tests. If a new test exposes a real production gap, fix that production gap minimally in the existing codec code.

**Tech Stack:** Go, `go test`, Modbus TCP/RTU codec tests

---

## Chunk 1: Exception and Function-Family Coverage

### Task 1: Add failing exception-frame decode tests

**Files:**
- Modify: `net/gtransport/modbus/modbus_tcp_test.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_tcp_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Add TCP exception-frame tests**

Add tests for:

- supported-base-function exception frame decode succeeds
- malformed exception length is rejected
- unsupported-base-function exception frame is rejected

- [ ] **Step 2: Add RTU exception-frame tests**

Add tests for:

- supported-base-function exception frame decode succeeds
- malformed exception length is rejected
- unsupported-base-function exception frame is rejected

- [ ] **Step 3: Run focused exception tests to verify current behavior**

Run: `cd net/gtransport && go test ./modbus -run 'Exception' -count=1`
Expected: PASS if the behavior is already implemented; FAIL only if the current implementation does not match the documented support

### Task 2: Add dedicated tests for `0x02`, `0x04`, and `0x06`

**Files:**
- Modify: `net/gtransport/modbus/modbus_tcp_test.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_tcp_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Add `0x02` coverage**

Add at least one direct regression proving:

- request quantity validation or
- response byte-count validation

- [ ] **Step 2: Add `0x04` coverage**

Add at least one direct regression proving:

- request quantity validation or
- response even-byte-count validation

- [ ] **Step 3: Add `0x06` coverage**

Add at least one direct regression proving:

- fixed payload-length acceptance or
- malformed payload-length rejection

- [ ] **Step 4: Run focused family tests**

Run: `cd net/gtransport && go test ./modbus -run 'Discrete|InputRegisters|SingleRegister' -count=1`
Expected: PASS

## Chunk 2: Remaining Address-Range Branches

### Task 3: Add address-range coverage for remaining request families

**Files:**
- Modify: `net/gtransport/modbus/modbus_tcp_test.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_tcp_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Add direct overflow tests for the remaining branches**

Cover the shared address-range rule on:

- `0x01`
- `0x02`
- `0x04`
- `0x0F`

- [ ] **Step 2: Keep tests representative, not redundant**

Choose transport placement based on where the test reads most clearly. Do not duplicate the same branch on both transports unless needed.

- [ ] **Step 3: Run focused address-range tests**

Run: `cd net/gtransport && go test ./modbus -run 'AddressRange|ReadCoils|Discrete|InputRegisters|MultipleCoils' -count=1`
Expected: PASS

### Task 4: Run full verification

**Files:**
- Modify: `net/gtransport/modbus/modbus_tcp_test.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Run the full package tests**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 2: Review scope**

Confirm:

- no policy-validation tests were added
- no new function-code support was implied
- tests remain aligned with existing codec responsibility

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-03-16-gtransport-modbus-test-matrix-hardening-design.md docs/superpowers/plans/2026-03-16-gtransport-modbus-test-matrix-hardening.md net/gtransport/modbus/modbus_tcp_test.go net/gtransport/modbus/modbus_rtu_test.go
git commit -m "test: harden modbus codec coverage"
```
