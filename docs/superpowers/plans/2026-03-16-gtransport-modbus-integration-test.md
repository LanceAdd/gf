# gtransport Modbus Integration Test Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add end-to-end transport tests proving that `gtransport.Transport` and the Modbus TCP/RTU codecs work correctly together for sticky packets, partial frames, and RTU resynchronization.

**Architecture:** Add a dedicated integration test file under `net/gtransport` using `net.Pipe()`. Write failing tests first for the four agreed scenarios. Only change production code if a new integration test exposes a genuine transport/codec interaction bug.

**Tech Stack:** Go, `go test`, `net.Pipe()`, `gtransport.Transport`, Modbus TCP/RTU codecs

---

## Chunk 1: Transport-Level Integration Tests

### Task 1: Add failing TCP integration tests

**Files:**
- Create: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`
- Test: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`

- [ ] **Step 1: Write the sticky-packet TCP test**

Add a test that:

- builds two valid TCP ADUs
- writes them back-to-back into a pipe
- calls `ReadFrame()` twice
- asserts the two returned frames match the original ADUs in order

- [ ] **Step 2: Write the TCP partial-frame completion test**

Add a test that:

- writes only part of a TCP ADU
- starts `ReadFrame()` before the rest arrives
- writes the remaining bytes
- asserts the read returns the full frame

- [ ] **Step 3: Run focused TCP integration tests to verify they fail or pass appropriately**

Run: `go test ./net/gtransport -run 'ModbusTCP' -count=1`
Expected: PASS if transport and codec already integrate correctly; FAIL only if a real interaction bug exists

### Task 2: Add failing RTU integration tests

**Files:**
- Modify: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`
- Test: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`

- [ ] **Step 1: Write the RTU noise-prefix recovery test**

Add a test that:

- writes raw noise bytes followed by a valid RTU frame
- calls `ReadFrame()`
- asserts the returned frame is the valid RTU payload

- [ ] **Step 2: Write the RTU partial-frame completion test**

Add a test that:

- writes only part of a valid RTU frame
- starts `ReadFrame()`
- writes the remainder later
- asserts the returned frame is the RTU payload without CRC

- [ ] **Step 3: Run focused RTU integration tests**

Run: `go test ./net/gtransport -run 'ModbusRTU' -count=1`
Expected: PASS if current transport/codec integration is correct; FAIL only if a real interaction bug exists

### Task 3: Full verification and scope review

**Files:**
- Create or Modify: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`
- Test: `net/gtransport/gtransport_test.go`

- [ ] **Step 1: Run the full gtransport package tests**

Run: `go test ./net/gtransport -count=1`
Expected: PASS

- [ ] **Step 2: Review scope**

Confirm:

- no Modbus production code changed unless required by a failing integration test
- no new protocol rules were introduced
- tests stay transport-level rather than duplicating codec unit coverage

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-03-16-gtransport-modbus-integration-test-design.md docs/superpowers/plans/2026-03-16-gtransport-modbus-integration-test.md net/gtransport/gtransport_z_unit_modbus_integration_test.go
git commit -m "test: add modbus transport integration coverage"
```
