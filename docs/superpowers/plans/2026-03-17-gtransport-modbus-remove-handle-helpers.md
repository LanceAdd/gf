# Modbus Remove Handle Helpers Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Remove the public `Handle*` helper APIs from `net/gtransport/modbus` so the package exposes only the composable Modbus flow.

**Architecture:** Delete the helper entry points and update tests, examples, and docs to consistently use `Parse* -> ExecuteRequest -> Encode*`. Keep `ParseRTURequestPayload` and `EncodeRTUResponsePayload` as first-class APIs for the RTU transport payload path.

**Tech Stack:** Go 1.23+, `net/gtransport/modbus`, Go `testing`

---

## Chunk 1: API removal

### Task 1: Make tests fail against the removed API

**Files:**
- Modify: `net/gtransport/modbus/modbus_z_unit_test.go`
- Modify: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`
- Delete: `net/gtransport/modbus/handle_request_z_unit_test.go`

- [ ] **Step 1: Write the failing test changes**
Remove `Handle*` API shape assertions and rewrite integration coverage to the composable flow.

- [ ] **Step 2: Run test to verify it fails**
Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: FAIL until all helper references are removed or replaced.

### Task 2: Delete helper implementation and migrate examples/docs

**Files:**
- Delete: `net/gtransport/modbus/handle_request.go`
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/example/modbus_rtu/main.go`
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 3: Write minimal implementation**
Delete helper APIs and switch all remaining call sites to `Parse* -> ExecuteRequest -> Encode*`.

- [ ] **Step 4: Run regression verification**
Run: `cd net/gtransport && go test ./modbus -count=1 && go test ./... -count=1`
Expected: PASS.
