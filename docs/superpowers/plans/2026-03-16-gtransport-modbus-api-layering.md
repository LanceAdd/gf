# gtransport Modbus API Layering Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align package documentation and public guidance with the agreed Modbus API layering: recommended path first, advanced path second, and strict `frame` versus `payload` semantics.

**Architecture:** Keep existing behavior unchanged and update only package-facing documentation, comments, and API guidance. The work should explain the current layered API rather than introduce new execution logic.

**Tech Stack:** Go, Markdown docs, `go test`, `net/gtransport`, `net/gtransport/modbus`

---

## File Structure

- Modify: `net/gtransport/modbus/modbus.go`
  - package-level comment if needed
- Modify: `net/gtransport/README.md`
  - recommended versus advanced Modbus API guidance
- Modify: `net/gtransport/README.zh-CN.md`
  - Chinese version of the same guidance
- Modify: `docs/superpowers/specs/2026-03-16-gtransport-modbus-api-layering-design.md`
  - if minor wording adjustments are needed after implementation feedback
- Reference: `net/gtransport/modbus/handle_request.go`
- Reference: `net/gtransport/modbus/modbus_req.go`
- Reference: `net/gtransport/modbus/modbus_resp.go`
- Reference: `net/gtransport/modbus/modbus_executor.go`
- Reference: `net/gtransport/modbus/modbus_test.go`

## Chunk 1: Package Guidance

### Task 1: Update package-facing Modbus API guidance

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Modify: `net/gtransport/modbus/modbus.go`

- [ ] **Step 1: Write the failing doc/API-shape checks**

Decide the concrete checks to add or update so the layered API stays visible. At minimum:

- package guidance mentions recommended APIs first
- documentation distinguishes advanced APIs from recommended APIs
- RTU `frame` versus `payload` semantics are explicit

If a lightweight API shape test is needed, add it to `net/gtransport/modbus/modbus_test.go`.

- [ ] **Step 2: Run focused verification to establish baseline**

Run: `cd net/gtransport && go test ./modbus -run 'TestPublicAPIShape' -count=1`
Expected: PASS, confirming the current API baseline before doc-only changes

- [ ] **Step 3: Write the minimal documentation updates**

Update docs so they present:

- `HandleTCPRequestFrame`
- `HandleRTURequestFrame`
- `HandleRTURequestPayload`
- `ExecuteRequest`

as the recommended path, and present:

- `ParseTCPRequest`
- `ParseRTURequest`
- `EncodeTCPResponse`
- `EncodeRTUResponse`
- typed request/response models
- direct `ProcessImage` usage

as the advanced path.

- [ ] **Step 4: Run package verification**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus.go net/gtransport/README.md net/gtransport/README.zh-CN.md
git commit -m "docs: clarify modbus api layering"
```

## Chunk 2: Examples and Semantic Consistency

### Task 2: Ensure examples and wording respect frame/payload rules

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Reference: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`
- Reference: `net/gtransport/modbus/handle_request_z_unit_test.go`

- [ ] **Step 1: Write the failing consistency checklist**

Make a checklist for:

- no example uses `HandleRTURequestFrame` when the source is `gtransport.New(..., modbus.NewRTU())`
- no text describes `ParseRTURequest` as payload-aware
- examples use `frame` and `payload` consistently

- [ ] **Step 2: Audit docs against the checklist**

Review each relevant example and wording site against the checklist.
Expected: identify any mismatches before editing

- [ ] **Step 3: Apply the minimal wording/example fixes**

Update examples and prose so:

- raw RTU ADU flows use `HandleRTURequestFrame`
- transport-decoded RTU flows use `HandleRTURequestPayload`
- payload examples name payloads as `payload`, not `frame`

- [ ] **Step 4: Run module verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/README.md net/gtransport/README.zh-CN.md
git commit -m "docs: align modbus frame and payload guidance"
```

## Chunk 3: Final Review

### Task 3: Final doc review and status check

**Files:**
- Reference: `docs/superpowers/specs/2026-03-16-gtransport-modbus-api-layering-design.md`
- Reference: `net/gtransport/README.md`
- Reference: `net/gtransport/README.zh-CN.md`
- Reference: `net/gtransport/modbus/modbus.go`

- [ ] **Step 1: Re-read the spec and compare final docs**

Check that the final documentation matches the agreed layering:

- recommended path first
- advanced path second
- strict semantic rules for TCP frame, RTU frame, and RTU payload

- [ ] **Step 2: Run final verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Check git status**

Run: `git status --short`
Expected: only intended tracked changes plus pre-existing unrelated untracked files

- [ ] **Step 4: Commit any final wording fixes if needed**

```bash
git add net/gtransport/modbus/modbus.go net/gtransport/README.md net/gtransport/README.zh-CN.md
git commit -m "docs: finalize modbus api guidance"
```
