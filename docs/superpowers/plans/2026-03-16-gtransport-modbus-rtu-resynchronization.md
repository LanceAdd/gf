# gtransport Modbus RTU Resynchronization Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make RTU decoding skip invalid leading bytes and return the first later valid RTU frame while preserving existing framing, CRC, and payload validation rules.

**Architecture:** Keep the existing RTU frame-shape logic and shared payload validation as-is. Add tests first in `modbus_rtu_test.go`, then refactor `rtuCodec.Decode` into a small scan loop over candidate offsets, returning `ErrNeedMoreData` only when the remaining bytes at the current candidate could still become a valid frame.

**Tech Stack:** Go, `go test`, RTU codec scanning logic, shared Modbus validation helpers

---

## Chunk 1: RTU Decode Resynchronization

### Task 1: Add failing RTU resynchronization tests

**Files:**
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Write a noise-prefix recovery test**

```go
func TestRTUDecodeSkipsNoisePrefixToValidFrame(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	frame := append([]byte{0x99, 0x88}, appendCRC(payload)...)

	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}
```

- [ ] **Step 2: Write invalid-prefix recovery tests**

```go
func TestRTUDecodeSkipsUnsupportedFunctionPrefixToValidFrame(t *testing.T) {
	validPayload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	frame := append([]byte{0x01, 0x11}, appendCRC(validPayload)...)
	_, _, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
}

func TestRTUDecodeSkipsInvalidCRCPrefixToValidFrame(t *testing.T) {
	bad := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00}
	validPayload := []byte{0x01, 0x03, 0x00, 0x01, 0x00, 0x01}
	frame := append(bad, appendCRC(validPayload)...)
	_, _, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
}
```

- [ ] **Step 3: Write the incomplete-tail test**

```go
func TestRTUDecodeReturnsNeedMoreDataForRecoverablePartialFrame(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	frame := append([]byte{0x99}, appendCRC(payload)...)
	_, _, err := NewRTU().Decode(frame[:len(frame)-1])
	if err != gtransport.ErrNeedMoreData {
		t.Fatalf("expected ErrNeedMoreData, got %v", err)
	}
}
```

- [ ] **Step 4: Run focused RTU tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'Skips|RecoverablePartialFrame' -count=1`
Expected: FAIL because `Decode` still stops at the first invalid prefix

### Task 2: Implement minimal RTU scan-based resynchronization

**Files:**
- Modify: `net/gtransport/modbus/modbus_rtu.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Add a small per-candidate decode helper or equivalent loop structure**

```go
for offset := 0; offset < len(in); offset++ {
	payload, frameLength, status := c.decodeCandidate(in[offset:])
	switch status {
	case candidateValid:
		return payload, offset + frameLength, nil
	case candidateInvalid:
		continue
	case candidateNeedMore:
		return nil, 0, gtransport.ErrNeedMoreData
	}
}
```

- [ ] **Step 2: Reuse current frame-length, CRC, and payload validation logic**

The implementation must preserve:

- `frameLength`
- `hasValidCRC`
- `validateModbusPayload`

Do not introduce a second, competing RTU framing algorithm.

- [ ] **Step 3: Treat deterministic invalid candidates as skippable**

Skippable cases:

- unsupported function
- CRC mismatch on a complete candidate
- payload validation failure on a complete candidate

- [ ] **Step 4: Return `ErrNeedMoreData` only for incomplete current candidates**

If the current candidate start does not yet have enough bytes to decide validity, stop scanning and return `gtransport.ErrNeedMoreData`.

- [ ] **Step 5: Run focused RTU tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'Skips|RecoverablePartialFrame' -count=1`
Expected: PASS

### Task 3: Run full package verification

**Files:**
- Modify: `net/gtransport/modbus/modbus_rtu.go`
- Modify: `net/gtransport/modbus/modbus_rtu_test.go`
- Test: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Run the full modbus package tests**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 2: Review scope control**

Confirm:
- no TCP files changed
- no shared payload validation behavior changed
- no RTU encode behavior changed

- [ ] **Step 3: Commit**

```bash
git add docs/superpowers/specs/2026-03-16-gtransport-modbus-rtu-resynchronization-design.md docs/superpowers/plans/2026-03-16-gtransport-modbus-rtu-resynchronization.md net/gtransport/modbus/modbus_rtu.go net/gtransport/modbus/modbus_rtu_test.go
git commit -m "feat: resynchronize modbus rtu decoding"
```
