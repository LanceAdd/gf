# gtransport Modbus Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `gtransport/modbus` with minimal, protocol-aware Modbus TCP and Modbus RTU codecs.

**Architecture:** Keep `net/gtransport` unchanged as the generic transport layer and add a protocol subpackage `net/gtransport/modbus`. Implement two codecs, `NewTCP()` and `NewRTU()`, where TCP manages MBAP length and RTU manages CRC and function-code-based frame-length detection.

**Tech Stack:** Go, `gtransport.Codec`, binary framing, Modbus TCP, Modbus RTU, CRC16

---

## Chunk 1: Package Skeleton

### Task 1: Create the Modbus subpackage and public API

**Files:**
- Create: `net/gtransport/modbus/modbus.go`
- Create: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add tests that reference:
- `modbus.NewTCP()`
- `modbus.NewRTU()`
- returned values implementing `gtransport.Codec`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Test(NewTCP|NewRTU)' -count=1`
Expected: FAIL with missing package symbols

- [ ] **Step 3: Write minimal implementation**

Create the package, add the two constructors, and return codec stubs implementing `gtransport.Codec`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'Test(NewTCP|NewRTU)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "feat: add modbus codec package skeleton"
```

## Chunk 2: Modbus TCP Codec

### Task 2: Implement TCP decode

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for:
- incomplete MBAP header returns `gtransport.ErrNeedMoreData`
- valid full ADU decodes correctly
- invalid MBAP length returns an error

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestTCPDecode' -count=1`
Expected: FAIL on missing decode behavior

- [ ] **Step 3: Write minimal implementation**

Implement TCP `Decode`:
- wait for 7-byte MBAP header
- read MBAP length
- derive full ADU length
- return full ADU

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestTCPDecode' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "feat: implement modbus tcp decode"
```

### Task 3: Implement TCP encode

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for:
- encode rewrites MBAP length correctly
- malformed short ADU returns an error

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestTCPEncode' -count=1`
Expected: FAIL on missing encode behavior

- [ ] **Step 3: Write minimal implementation**

Implement TCP `Encode`:
- validate minimum header size
- recompute MBAP length from Unit ID + PDU
- return corrected full ADU

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestTCPEncode' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "feat: implement modbus tcp encode"
```

## Chunk 3: Modbus RTU Codec

### Task 4: Implement RTU decode for supported function codes

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for RTU `Decode` covering:
- incomplete data returns `gtransport.ErrNeedMoreData`
- valid CRC frame returns payload without CRC
- invalid CRC returns an error
- unsupported function code returns an error
- supported function codes `0x01/0x02/0x03/0x04/0x05/0x06/0x0F/0x10`
- exception response frame shape

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestRTUDecode' -count=1`
Expected: FAIL on missing RTU frame-length logic

- [ ] **Step 3: Write minimal implementation**

Implement RTU `Decode`:
- inspect address and function
- determine expected length from function rules
- validate CRC
- return RTU payload without CRC

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestRTUDecode' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "feat: implement modbus rtu decode"
```

### Task 5: Implement RTU encode

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for RTU `Encode`:
- appends correct CRC for a known frame
- rejects obviously too-short payloads

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestRTUEncode' -count=1`
Expected: FAIL on missing CRC append behavior

- [ ] **Step 3: Write minimal implementation**

Implement RTU `Encode`:
- accept payload without CRC
- append computed CRC16 (Modbus)
- return full RTU ADU

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestRTUEncode' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "feat: implement modbus rtu encode"
```

## Chunk 4: Integration and Documentation

### Task 6: Document and verify the new codecs

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Add or modify: package comments in `net/gtransport/modbus/*.go`

- [ ] **Step 1: Write the failing test**

Use build verification and package tests as the acceptance gate.

- [ ] **Step 2: Run verification to confirm current state**

Run: `go test ./... -count=1`
Expected: PASS before doc updates

- [ ] **Step 3: Write minimal implementation**

Document:
- `modbus.NewTCP()`
- `modbus.NewRTU()`
- TCP length-field behavior
- RTU CRC behavior

- [ ] **Step 4: Run final verification**

Run: `go test ./... -count=1`
Expected: PASS

Run: `$env:GOARCH='386'; $env:GOOS='windows'; go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport docs/superpowers/specs docs/superpowers/plans
git commit -m "docs: add modbus codec design and verification"
```
