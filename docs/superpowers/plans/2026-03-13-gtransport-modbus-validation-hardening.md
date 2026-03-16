# gtransport Modbus Validation Hardening Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the stateless validation matrix for `net/gtransport/modbus` across the currently supported Modbus function codes without changing the public API.

**Architecture:** Introduce shared function-code validators that both TCP and RTU paths call after envelope checks. Keep TCP responsible for MBAP handling and RTU responsible for CRC and frame-length detection, but centralize all function-specific legality checks so encode and decode follow the same rules.

**Tech Stack:** Go, `gtransport.Codec`, Modbus TCP, Modbus RTU, MBAP, CRC16

---

## Chunk 1: Shared Validation Matrix

### Task 1: Add failing tests for the missing Modbus legality matrix

**Files:**
- Modify: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for the currently missing validation cases:
- TCP MBAP length upper bound
- TCP function-specific validation for supported PDUs
- RTU request quantity bounds for `0x01/0x02/0x03/0x04`
- RTU response byte-count rules for `0x01/0x02/0x03/0x04`
- single-coil value validation for `0x05`
- response quantity validation for `0x0F/0x10`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./modbus -count=1`
Expected: FAIL on missing legality checks

- [ ] **Step 3: Write minimal implementation**

Do not implement yet. Stop after the red tests are confirmed.

- [ ] **Step 4: Run test to verify it still fails for the expected reasons**

Run: `go test ./modbus -count=1`
Expected: FAIL only on the new cases

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_test.go
git commit -m "test: add modbus validation hardening coverage"
```

## Chunk 2: Shared Payload Validators

### Task 2: Centralize request, response, and exception validation

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`

- [ ] **Step 1: Write the failing test**

Use the tests from Chunk 1 as the red state.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./modbus -count=1`
Expected: FAIL on validation gaps

- [ ] **Step 3: Write minimal implementation**

Implement shared helpers for:
- request payload validation
- response payload validation
- exception payload validation
- shared quantity and byte-count checks per function code

Keep the helpers transport-agnostic.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus.go net/gtransport/modbus/modbus_test.go
git commit -m "feat: centralize modbus payload validation"
```

## Chunk 3: Wire Shared Validators into TCP and RTU

### Task 3: Apply shared validators consistently in encode and decode paths

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing test**

Add or extend tests that prove:
- TCP decode rejects function-specific illegal payloads
- TCP encode rejects function-specific illegal payloads
- RTU decode rejects function-specific illegal payloads
- RTU encode rejects function-specific illegal payloads

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./modbus -count=1`
Expected: FAIL on path-specific gaps

- [ ] **Step 3: Write minimal implementation**

Update:
- TCP `Decode`
- TCP `Encode`
- RTU `Decode`
- RTU `Encode`

to delegate to the shared validators at the correct boundary.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "feat: apply shared modbus validation across transports"
```

## Chunk 4: Documentation and Final Verification

### Task 4: Document the hardened validation behavior and run final verification

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Modify: `docs/superpowers/specs/2026-03-13-gtransport-modbus-validation-design.md`
- Modify: `docs/superpowers/plans/2026-03-13-gtransport-modbus-validation-hardening.md`

- [ ] **Step 1: Write the failing test**

Use final package verification as the acceptance gate.

- [ ] **Step 2: Run verification before doc edits**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Write minimal implementation**

Document:
- MBAP validation behavior
- RTU CRC and legality checks
- supported function-code validation coverage

- [ ] **Step 4: Run final verification**

Run: `go test ./... -count=1`
Expected: PASS

Run: `$env:GOARCH='386'; $env:GOOS='windows'; go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport docs/superpowers/specs docs/superpowers/plans
git commit -m "docs: describe modbus validation hardening"
```
