# ADCP Codec Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add an ADCP stream codec under `net/gtransport/adcp` that finds ADCP frames in a byte stream, validates frame metadata and CRC, and returns only payload bytes from `ReadFrame`.

**Architecture:** Follow the existing `net/gtransport/modbus` package pattern by introducing a focused protocol subpackage that exposes `adcp.New() gtransport.Codec`. Keep the first version decode-only: the codec synchronizes on a 16-byte `0x80` preamble, parses the fixed 16-byte metadata header, waits for payload plus 4-byte CRC, validates CRC16-CCITT over the payload, and returns the payload bytes while resynchronizing across noise and malformed prefixes.

**Tech Stack:** Go 1.23+, `net/gtransport`, Go `testing`

---

## Chunk 1: ADCP codec core

### Task 1: Add failing decode tests

**Files:**
- Create: `net/gtransport/adcp/adcp_z_unit_test.go`
- Test: `net/gtransport/adcp/adcp_z_unit_test.go`

- [ ] **Step 1: Write the failing test**
Add focused tests for complete decode, partial frame returning `gtransport.ErrNeedMoreData`, CRC failure recovery, invalid header-field validation, and noise-prefix resynchronization.

- [ ] **Step 2: Run test to verify it fails**
Run: `cd net/gtransport && go test ./adcp -count=1`
Expected: FAIL because package implementation does not exist yet.

### Task 2: Implement minimal decode-only codec

**Files:**
- Create: `net/gtransport/adcp/adcp.go`
- Test: `net/gtransport/adcp/adcp_z_unit_test.go`

- [ ] **Step 3: Write minimal implementation**
Expose `adcp.New()`, implement sync-header scan, header parsing, payload-length checks, CRC16-CCITT validation, payload-only decode result, and an encode method that returns a clear unsupported error.

- [ ] **Step 4: Run test to verify it passes**
Run: `cd net/gtransport && go test ./adcp -count=1`
Expected: PASS.

### Task 3: Verify transport compatibility

**Files:**
- Test: `net/gtransport/adcp/adcp_z_unit_test.go`

- [ ] **Step 5: Run targeted package verification**
Run: `cd net/gtransport && go test ./... -run ADCP -count=1`
Expected: PASS for ADCP-targeted coverage.
