# ADCP Max Frame Length Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `net/gtransport/adcp` accept an optional max-frame-length argument while preserving the existing default behavior.

**Architecture:** Keep the package decode-only and change only the constructor surface from fixed-default `adcp.New()` to `adcp.New(maxFrameLength ...int)`. Parse only the first optional argument, keep `8192` as the fallback default, and validate the behavior through focused unit tests before updating package documentation.

**Tech Stack:** Go 1.23+, `net/gtransport`, Go `testing`

---

## Chunk 1: Constructor configuration

### Task 1: Add failing tests for constructor configuration

**Files:**
- Modify: `net/gtransport/adcp/adcp_z_unit_test.go`
- Test: `net/gtransport/adcp/adcp_z_unit_test.go`

- [ ] **Step 1: Write the failing test**
Add tests that prove `adcp.New()` keeps the default max frame length, `adcp.New(custom)` accepts larger frames, and `adcp.New(0)` falls back to the default limit.

- [ ] **Step 2: Run test to verify it fails**
Run: `cd net/gtransport && go test ./adcp -count=1`
Expected: FAIL because the constructor still ignores optional max length input.

### Task 2: Implement optional max frame length

**Files:**
- Modify: `net/gtransport/adcp/adcp.go`
- Test: `net/gtransport/adcp/adcp_z_unit_test.go`

- [ ] **Step 3: Write minimal implementation**
Change `adcp.New` to accept an optional integer, resolve the effective max frame length from the first argument, and keep non-positive inputs on the default path.

- [ ] **Step 4: Run test to verify it passes**
Run: `cd net/gtransport && go test ./adcp -count=1`
Expected: PASS.

### Task 3: Update docs and re-verify

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 5: Update docs**
Document that `adcp.New()` keeps the default `8192` limit and `adcp.New(n)` overrides it.

- [ ] **Step 6: Run targeted verification**
Run: `cd net/gtransport && go test ./adcp -count=1 && go test ./... -run ADCP -count=1`
Expected: PASS.
