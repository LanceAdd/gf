# gtransport Modbus API Convergence Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Realign `net/gtransport/modbus` around a composable low-level-first API while preserving helper compatibility and completing the RTU payload low-level path.

**Architecture:** Keep the existing parse/execute/encode layering as the core model and demote `Handle*` APIs to documented convenience wrappers. Add explicit RTU payload low-level entry points so TCP frame, RTU frame, and RTU payload paths become symmetric in both docs and code. Update tests and examples to teach the composable path first while preserving helper behavior.

**Tech Stack:** Go 1.23+, GoFrame `gtransport`, Go unit tests, example modules

---

## Chunk 1: Public API Surface and Documentation Narrative

### Task 1: Reclassify package-level API narrative

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Test: `net/gtransport/modbus/modbus_test.go`
- Check: `net/gtransport/README.md`

- [ ] **Step 1: Write the failing API-shape/doc expectation test**

Add or extend a package-level API-shape test to reference the intended public low-level APIs and any newly added RTU payload low-level APIs.

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -run TestPublicAPIShape -count=1`
Expected: FAIL when new RTU payload low-level APIs are not yet defined.

- [ ] **Step 3: Update package comment narrative**

Rework `net/gtransport/modbus/modbus.go` comments to use:
- `Composable APIs`
- `Standard Execution Layer`
- `Convenience Helpers`

Keep helper APIs public, but stop describing them as recommended primary entry points.

- [ ] **Step 4: Run targeted test to verify it passes**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -run TestPublicAPIShape -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus.go net/gtransport/modbus/modbus_test.go
git commit -m "refactor: clarify modbus api layering"
```

### Task 2: Update README narrative to teach composition first

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 1: Update the Modbus sections**

Change the narrative so README leads with:
- composable flow: `Parse -> Execute -> Encode`
- helper flow as convenience
- explicit TCP frame / RTU frame / RTU payload semantics

- [ ] **Step 2: Verify doc consistency manually**

Check that README no longer calls helper APIs the default recommended path and that the semantics are consistent with the package comment.

- [ ] **Step 3: Commit**

```bash
git add net/gtransport/README.md net/gtransport/README.zh-CN.md
git commit -m "docs: teach composable modbus apis first"
```

## Chunk 2: RTU Payload Low-Level API Symmetry

### Task 3: Add RTU payload parse and encode APIs

**Files:**
- Modify: `net/gtransport/modbus/handle_request.go`
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/modbus/modbus_test.go`
- Test: `net/gtransport/modbus/handle_request_z_unit_test.go`
- Test: `net/gtransport/modbus/modbus_req_test.go`
- Test: `net/gtransport/modbus/modbus_resp_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:
- `ParseRTURequestPayload(payload []byte) (Request, error)`
- `EncodeRTUResponsePayload(resp Response) ([]byte, error)`
- equivalence with `HandleRTURequestPayload` for the same request/response path

Example target behavior:

```go
req, err := ParseRTURequestPayload([]byte{0x11, 0x03, 0x00, 0x00, 0x00, 0x01})
if err != nil { t.Fatal(err) }
if req.Meta().Transport != TransportRTU { t.Fatal("expected RTU transport") }
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -run 'TestParseRTURequestPayload|TestEncodeRTUResponsePayload|TestHandleRTURequestPayload' -count=1`
Expected: FAIL because the new public APIs are not yet implemented.

- [ ] **Step 3: Implement the minimal public APIs**

Expose thin public wrappers around the existing private RTU payload logic instead of duplicating parsing/encoding behavior.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -run 'TestParseRTURequestPayload|TestEncodeRTUResponsePayload|TestHandleRTURequestPayload' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/handle_request.go net/gtransport/modbus/modbus.go net/gtransport/modbus/modbus_test.go net/gtransport/modbus/handle_request_z_unit_test.go net/gtransport/modbus/modbus_req_test.go net/gtransport/modbus/modbus_resp_test.go
git commit -m "feat: add modbus rtu payload low-level apis"
```

### Task 4: Preserve helper thinness and symmetry

**Files:**
- Modify: `net/gtransport/modbus/handle_request.go`
- Test: `net/gtransport/modbus/handle_request_z_unit_test.go`

- [ ] **Step 1: Add explicit symmetry test**

Write a test that proves:
- `HandleRTURequestPayload(payload, image)` produces the same bytes as
- `ParseRTURequestPayload(payload)` -> `ExecuteRequest(req, image)` -> `EncodeRTUResponsePayload(resp)`

- [ ] **Step 2: Run test to verify it fails if symmetry is broken**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -run TestHandleRTURequestPayloadMatchesComposableFlow -count=1`
Expected: FAIL before the symmetry assertion is correctly wired.

- [ ] **Step 3: Keep helper implementation as a thin wrapper**

Ensure the helper continues to delegate through parse -> execute -> encode and does not gain independent protocol logic.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -run TestHandleRTURequestPayloadMatchesComposableFlow -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/handle_request.go net/gtransport/modbus/handle_request_z_unit_test.go
git commit -m "test: lock modbus helper symmetry"
```

## Chunk 3: Example and Full Verification

### Task 5: Reposition examples around composable-first usage

**Files:**
- Modify: `net/gtransport/example/modbus_recommended/main.go`
- Modify: `net/gtransport/example/modbus_advanced/main.go`

- [ ] **Step 1: Update example narrative and output**

Adjust example comments/output so:
- the advanced/composable example reads as the primary/default model
- the helper example reads as the convenience path

Keep code behavior simple and compilable.

- [ ] **Step 2: Run example module builds**

Run:

```bash
cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport/example/modbus_recommended && go test ./...
cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport/example/modbus_advanced && go test ./...
```

Expected: PASS with `[no test files]`

- [ ] **Step 3: Commit**

```bash
git add net/gtransport/example/modbus_recommended/main.go net/gtransport/example/modbus_advanced/main.go
git commit -m "docs: reposition modbus examples"
```

### Task 6: Full module verification

**Files:**
- Verify: `net/gtransport/modbus/*.go`
- Verify: `net/gtransport/example/modbus_recommended`
- Verify: `net/gtransport/example/modbus_advanced`

- [ ] **Step 1: Run full modbus package tests**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 2: Run full gtransport package tests**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Run vet**

Run: `cd /c/Users/lanceadd/GolandProjects/gf/net/gtransport && go vet ./...`
Expected: PASS with no output

- [ ] **Step 4: Commit**

```bash
git add net/gtransport
git commit -m "refactor: converge modbus api layering"
```
