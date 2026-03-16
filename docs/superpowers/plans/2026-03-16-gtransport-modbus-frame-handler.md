# gtransport Modbus Frame Handler Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add explicit TCP and RTU Modbus frame handlers that compose parse, execute, and encode into one stateless frame-level API.

**Architecture:** Keep the new layer inside `net/gtransport/modbus` as a thin adapter over existing request parsing, executor dispatch, and response encoding. Do not modify `gtransport.Transport` or add server-loop abstractions; the handler just turns one request frame into one response frame.

**Tech Stack:** Go, `go test`, `net/gtransport/modbus`, existing typed Modbus request/response models, existing `ProcessImage` and executor

---

## File Structure

- Create: `net/gtransport/modbus/handle_request.go`
  - explicit `HandleTCPRequestFrame`
  - explicit `HandleRTURequestFrame`
  - optional tiny shared helper if needed
- Create: `net/gtransport/modbus/handle_request_z_unit_test.go`
  - frame-level closure tests for TCP and RTU
- Reference: `net/gtransport/modbus/modbus_req.go`
  - request parsing entry points
- Reference: `net/gtransport/modbus/modbus_resp.go`
  - response encoding entry points
- Reference: `net/gtransport/modbus/modbus_executor.go`
  - request execution semantics and exception mapping
- Reference: `docs/superpowers/specs/2026-03-16-gtransport-modbus-frame-handler-design.md`

## Chunk 1: Happy Path Frame Handling

### Task 1: Add TCP and RTU read/write happy-path closure tests

**Files:**
- Create: `net/gtransport/modbus/handle_request_z_unit_test.go`
- Reference: `net/gtransport/modbus/modbus_req_test.go`
- Reference: `net/gtransport/modbus/modbus_resp_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `HandleTCPRequestFrame` read request -> encoded TCP read response
- `HandleTCPRequestFrame` write request -> encoded TCP write response and `MemoryProcessImage` state change
- `HandleRTURequestFrame` read request -> encoded RTU read response
- `HandleRTURequestFrame` write request -> encoded RTU write response and `MemoryProcessImage` state change

Each test should assert exact encoded bytes, not only typed values.

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestHandle(TCP|RTU)RequestFrame.*(Read|Write)' -count=1`
Expected: FAIL with undefined handler functions

- [ ] **Step 3: Write the minimal implementation**

Create `net/gtransport/modbus/handle_request.go` with:

```go
func HandleTCPRequestFrame(frame []byte, image ProcessImage) ([]byte, error)
func HandleRTURequestFrame(frame []byte, image ProcessImage) ([]byte, error)
```

Implement the minimal flow:

- parse request using the transport-specific parser
- execute request with `ExecuteRequest`
- encode response with the transport-specific encoder

Do not add protocol auto-detection or extra policy.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestHandle(TCP|RTU)RequestFrame.*(Read|Write)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/handle_request.go net/gtransport/modbus/handle_request_z_unit_test.go
git commit -m "feat: add modbus frame handler happy path"
```

## Chunk 2: Exception and Misuse Paths

### Task 2: Add frame-level exception and misuse coverage

**Files:**
- Modify: `net/gtransport/modbus/handle_request_z_unit_test.go`
- Modify: `net/gtransport/modbus/handle_request.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- TCP address out of range -> encoded TCP exception response
- RTU invalid quantity or empty write values -> encoded RTU exception response
- TCP unsupported function -> encoded TCP exception response
- `nil image` -> Go `error`
- invalid TCP request frame -> Go `error`
- invalid RTU request frame -> Go `error`

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestHandle(TCP|RTU)RequestFrame.*(Exception|Invalid|Nil)' -count=1`
Expected: FAIL where the new exception or misuse behaviors are not yet covered correctly

- [ ] **Step 3: Write the minimal implementation**

Adjust the handler implementation only as needed to satisfy the tests:

- keep parse errors as Go `error`
- keep `nil image` as Go `error`
- let executor-produced `ExceptionResponse` flow into normal response encoding

Do not add new exception mapping here; reuse executor behavior.

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestHandle(TCP|RTU)RequestFrame.*(Exception|Invalid|Nil)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/handle_request.go net/gtransport/modbus/handle_request_z_unit_test.go
git commit -m "test: cover modbus frame handler exceptions"
```

## Chunk 3: Final Verification

### Task 3: Verify package integration and public shape

**Files:**
- Reference: `net/gtransport/modbus/handle_request.go`
- Reference: `net/gtransport/modbus/handle_request_z_unit_test.go`
- Reference: `docs/superpowers/specs/2026-03-16-gtransport-modbus-frame-handler-design.md`

- [ ] **Step 1: Run package verification**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 2: Run module verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Check git status**

Run: `git status --short`
Expected: only intended tracked changes plus pre-existing unrelated untracked files

- [ ] **Step 4: Commit final integration batch if needed**

If Chunk 2 required implementation edits beyond tests and they are not yet committed:

```bash
git add net/gtransport/modbus/handle_request.go net/gtransport/modbus/handle_request_z_unit_test.go
git commit -m "feat: finalize modbus frame handler integration"
```
