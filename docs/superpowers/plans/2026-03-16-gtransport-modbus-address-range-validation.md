# gtransport Modbus Address Range Validation Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reject Modbus request frames whose `startAddress + quantity` exceeds the 16-bit Modbus address space while keeping codec responsibility stateless and shared across TCP and RTU.

**Architecture:** Keep the new rule inside `net/gtransport/modbus/modbus_validation.go`, where request-shaped payload validation already lives. Extend tests first in the transport-specific test files, then add the minimal shared helper and wire it only into request validators for `0x01/0x02/0x03/0x04/0x0F/0x10`.

**Tech Stack:** Go, `go test`, shared codec validation helpers, Modbus TCP/RTU codecs

---

## Chunk 1: Tests and Shared Validation

### Task 1: Add failing transport tests for address-range overflow

**Files:**
- Modify: `net/gtransport/modbus/modbus_tcp_test.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_tcp_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Write the failing TCP tests**

```go
func TestTCPDecodeAllowsReadRequestAtAddressBoundary(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0xFF, 0xFF, 0x00, 0x01}
	_, _, err := NewTCP().Decode(frame)
	if err != nil {
		t.Fatalf("expected boundary request to be valid, got %v", err)
	}
}

func TestTCPDecodeRejectsReadRequestAddressRangeOverflow(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0xFF, 0xFF, 0x00, 0x02}
	_, _, err := NewTCP().Decode(frame)
	if err == nil {
		t.Fatal("expected address range error")
	}
}
```

- [ ] **Step 2: Write the failing RTU tests**

```go
func TestRTUDecodeRejectsReadRequestAddressRangeOverflow(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x03, 0xFF, 0xFF, 0x00, 0x02})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestRTUEncodeRejectsWriteMultipleRegistersAddressRangeOverflow(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x10, 0xFF, 0xFE, 0x00, 0x03, 0x06, 0x00, 0x0A, 0x00, 0x0B, 0x00, 0x0C})
	if err == nil {
		t.Fatal("expected address range error")
	}
}
```

- [ ] **Step 3: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'AddressRange|Boundary' -count=1`
Expected: FAIL because the new validation does not exist yet

### Task 2: Add the minimal shared address-range validator

**Files:**
- Modify: `net/gtransport/modbus/modbus_validation.go`
- Test: `net/gtransport/modbus/modbus_tcp_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Add a helper for request-shaped address range validation**

```go
func validateAddressRange(startAddress, quantity uint16) error {
	endExclusive := int(startAddress) + int(quantity)
	if endExclusive > 65536 {
		return fmt.Errorf("invalid modbus address range start=%d quantity=%d", startAddress, quantity)
	}
	return nil
}
```

- [ ] **Step 2: Call the helper from request validators only**

```go
if len(data) == 4 {
	startAddress := binary.BigEndian.Uint16(data[0:2])
	quantity := binary.BigEndian.Uint16(data[2:4])
	if err := validateAddressRange(startAddress, quantity); err != nil {
		return err
	}
}
```

Apply the same pattern to:

- read-bit request validation
- read-register request validation
- write-multiple-coils request validation
- write-multiple-registers request validation

- [ ] **Step 3: Keep response-path validation unchanged**

Do not add the helper to:

- read responses (`byteCount` shape)
- `0x05`
- `0x06`
- exception payloads
- write confirmation responses

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'AddressRange|Boundary' -count=1`
Expected: PASS

### Task 3: Run package verification and review the result

**Files:**
- Modify: `net/gtransport/modbus/modbus_validation.go`
- Modify: `net/gtransport/modbus/modbus_tcp_test.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Run the full modbus package tests**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 2: Review for scope control**

Confirm:
- no transport framing logic was changed
- no `ProcessImage` semantics were introduced
- no new supported function codes were added

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-03-16-gtransport-modbus-address-range-validation-design.md docs/superpowers/plans/2026-03-16-gtransport-modbus-address-range-validation.md net/gtransport/modbus/modbus_validation.go net/gtransport/modbus/modbus_tcp_test.go net/gtransport/modbus/modbus_rtu_test.go
git commit -m "feat: validate modbus request address ranges"
```
