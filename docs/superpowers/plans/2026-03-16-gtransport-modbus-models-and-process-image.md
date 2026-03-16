# gtransport Modbus Request/Response Models and ProcessImage Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add typed Modbus request/response models plus a standalone in-memory `ProcessImage` component under `net/gtransport/modbus`.

**Architecture:** Keep the codec unchanged as the stateless wire validator. Add a protocol model layer that parses and encodes framed Modbus TCP/RTU messages into typed request/response values, then add a separate `ProcessImage` abstraction and memory-backed implementation for the four Modbus data areas.

**Tech Stack:** Go, `go test`, `net/gtransport/modbus`, Modbus TCP/RTU framing rules

---

## Chunk 1: Typed Request Models

### Task 1: Add request interfaces and read-request parsing

**Files:**
- Create: `net/gtransport/modbus/modbus_req.go`
- Create: `net/gtransport/modbus/modbus_req_test.go`
- Test: `net/gtransport/modbus/modbus_req_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `ParseTCPRequest` parsing valid `FC01` and `FC03` requests
- `ParseRTURequest` parsing valid `FC02` and `FC04` requests
- returned types expose correct `ADUMeta`, function code, start address, and quantity

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Parse(TCP|RTU)Request(Read(Coils|DiscreteInputs|HoldingRegisters|InputRegisters)))' -count=1`
Expected: FAIL with missing request parser or request types

- [ ] **Step 3: Write the minimal implementation**

Implement:

- `TransportKind`
- `ADUMeta`
- `Request` interface
- `ReadCoilsRequest`
- `ReadDiscreteInputsRequest`
- `ReadHoldingRegistersRequest`
- `ReadInputRegistersRequest`
- `ParseTCPRequest`
- `ParseRTURequest`

Parsing rules:

- strip MBAP for TCP and CRC for RTU
- preserve `TransactionID` for TCP
- preserve `UnitID` for both transports
- reject unsupported function codes
- decode `StartAddress` and `Quantity` from the PDU

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Parse(TCP|RTU)Request(Read(Coils|DiscreteInputs|HoldingRegisters|InputRegisters)))' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_req.go net/gtransport/modbus/modbus_req_test.go
git commit -m "feat: add modbus typed read requests"
```

### Task 2: Add write-request parsing and request validation

**Files:**
- Modify: `net/gtransport/modbus/modbus_req.go`
- Modify: `net/gtransport/modbus/modbus_req_test.go`
- Test: `net/gtransport/modbus/modbus_req_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- valid `FC05`, `FC06`, `FC15`, and `FC16` requests
- invalid `FC05` coil value
- invalid packed byte count for `FC15`
- invalid byte count for `FC16`
- address continuity overflow rejection

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Parse(TCP|RTU)Request(Write|Invalid))' -count=1`
Expected: FAIL with missing write request support or missing validation

- [ ] **Step 3: Write the minimal implementation**

Implement:

- `WriteSingleCoilRequest`
- `WriteSingleRegisterRequest`
- `WriteMultipleCoilsRequest`
- `WriteMultipleRegistersRequest`

Validation rules:

- `FC05` accepts only `0x0000` and `0xFF00`
- `FC15` byte count must equal `ceil(quantity/8)`
- `FC16` byte count must equal `quantity * 2`
- multi-write request values are expanded into `[]bool` or `[]uint16`
- zero quantity and 16-bit address overflow are rejected

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Parse(TCP|RTU)Request(Write|Invalid))' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_req.go net/gtransport/modbus/modbus_req_test.go
git commit -m "feat: add modbus typed write requests"
```

## Chunk 2: Typed Response Models

### Task 3: Add normal response types and encoding

**Files:**
- Create: `net/gtransport/modbus/modbus_resp.go`
- Create: `net/gtransport/modbus/modbus_resp_test.go`
- Test: `net/gtransport/modbus/modbus_resp_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `EncodeTCPResponse` encoding a `ReadBitsResponse`
- `EncodeTCPResponse` encoding a `ReadRegistersResponse`
- `EncodeRTUResponse` encoding a `WriteSingleRegisterResponse`
- `EncodeRTUResponse` encoding a `WriteMultipleRegistersResponse`

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestEncode(TCP|RTU)Response' -count=1`
Expected: FAIL with missing response encoder or response types

- [ ] **Step 3: Write the minimal implementation**

Implement:

- `Response` interface
- `ReadBitsResponse`
- `ReadRegistersResponse`
- `WriteSingleCoilResponse`
- `WriteSingleRegisterResponse`
- `WriteMultipleCoilsResponse`
- `WriteMultipleRegistersResponse`
- `EncodeTCPResponse`
- `EncodeRTUResponse`

Encoding rules:

- TCP rewrites MBAP length from encoded PDU length
- RTU appends CRC
- read-bit responses pack `[]bool` into LSB-first bytes
- read-register responses emit big-endian register bytes

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestEncode(TCP|RTU)Response' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_resp.go net/gtransport/modbus/modbus_resp_test.go
git commit -m "feat: add modbus typed responses"
```

### Task 4: Add exception responses and response validation

**Files:**
- Modify: `net/gtransport/modbus/modbus_resp.go`
- Modify: `net/gtransport/modbus/modbus_resp_test.go`
- Test: `net/gtransport/modbus/modbus_resp_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- valid TCP exception response encoding
- valid RTU exception response encoding
- rejecting unsupported base function codes in exception responses
- rejecting empty read responses when quantity would be impossible by shape

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Exception|Invalid).*Response' -count=1`
Expected: FAIL with missing exception support or missing validation

- [ ] **Step 3: Write the minimal implementation**

Implement:

- `ExceptionResponse`
- `IsException() bool`
- response-side validation helpers for supported base function codes

Validation rules:

- exception response function is encoded as `baseFunction | 0x80`
- base function must be one of the supported classic function codes
- normal response shapes must remain internally consistent with their data payloads

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Exception|Invalid).*Response' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_resp.go net/gtransport/modbus/modbus_resp_test.go
git commit -m "feat: add modbus exception responses"
```

## Chunk 3: ProcessImage Abstraction and Memory Store

### Task 5: Add `ProcessImage` interface and in-memory store

**Files:**
- Create: `net/gtransport/modbus/process_image.go`
- Create: `net/gtransport/modbus/process_image_memory.go`
- Create: `net/gtransport/modbus/process_image_z_unit_test.go`
- Test: `net/gtransport/modbus/process_image_z_unit_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- reading each data area within bounds
- writing coils and holding registers within bounds
- out-of-range reads and writes returning errors
- returned slices being copies rather than aliases

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestMemoryProcessImage' -count=1`
Expected: FAIL with missing `ProcessImage` or memory implementation

- [ ] **Step 3: Write the minimal implementation**

Implement:

- `ProcessImage` interface
- `MemoryProcessImage`
- `NewMemoryProcessImage`
- range-check helpers
- copy-on-read behavior

Behavior rules:

- coils and holding registers are writable
- discrete inputs and input registers are exposed as read-only by interface shape
- constructor capacity arguments use `int` so a full 65536-entry address space remains representable
- zero-quantity reads and zero-length multi-writes return errors

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestMemoryProcessImage' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/process_image.go net/gtransport/modbus/process_image_memory.go net/gtransport/modbus/process_image_z_unit_test.go
git commit -m "feat: add modbus process image memory store"
```

### Task 6: Add write-range and multi-value edge-case coverage

**Files:**
- Modify: `net/gtransport/modbus/process_image_memory.go`
- Modify: `net/gtransport/modbus/process_image_z_unit_test.go`
- Test: `net/gtransport/modbus/process_image_z_unit_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `WriteMultipleCoils` rejecting partial out-of-range writes without mutating prior values
- `WriteMultipleRegisters` rejecting partial out-of-range writes without mutating prior values
- zero-length multi-write rejection

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestMemoryProcessImage(AtomicBounds|ZeroLength)' -count=1`
Expected: FAIL with missing atomic bounds behavior

- [ ] **Step 3: Write the minimal implementation**

Ensure:

- multi-write operations validate the full target range before mutating state
- zero-length writes return an error immediately

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestMemoryProcessImage(AtomicBounds|ZeroLength)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/process_image_memory.go net/gtransport/modbus/process_image_z_unit_test.go
git commit -m "test: harden modbus process image edge cases"
```

## Chunk 4: Integration and Regression Verification

### Task 7: Verify coexistence with existing codec tests

**Files:**
- Modify: `net/gtransport/modbus/modbus_test.go`
- Test: `net/gtransport/modbus/modbus_req_test.go`
- Test: `net/gtransport/modbus/modbus_resp_test.go`
- Test: `net/gtransport/modbus/process_image_z_unit_test.go`
- Test: `net/gtransport/modbus/modbus_tcp_test.go`
- Test: `net/gtransport/modbus/modbus_rtu_test.go`

- [ ] **Step 1: Add any missing package-level API shape assertions**

Ensure package-level tests reference:

- request parser entrypoints
- response encoder entrypoints
- `ProcessImage`
- `NewMemoryProcessImage`

- [ ] **Step 2: Run the full modbus package tests**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 3: Run the full gtransport module tests**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Review scope**

Confirm:

- codec responsibilities remain stateless
- `UnitID` policy was not introduced
- `ProcessImage` stayed independent from transport framing
- no handler/service execution layer was added

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "test: verify modbus models and process image integration"
```
