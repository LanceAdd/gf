# GTCP Frame Decoder Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pluggable frame-decoder support to `net/gtcp` with first-class fixed-length, custom-delimiter, and custom decoder capabilities, then expose server-side message handling based on decoders.

**Architecture:** Introduce a decoder abstraction at `Conn` level (`ReadFrame` + internal decode buffer + `ErrNeedMoreData`) and keep current `Recv*` APIs unchanged. Then add an opt-in server message mode (`SetDecoder` + `SetMessageHandler`) that decodes frames per connection and closes the connection immediately on protocol errors.

**Tech Stack:** Go, GoFrame `net/gtcp`, `gtest`, standard library `bytes`/`errors`/`io`.

---

## File Structure

Planned file ownership and responsibilities:

- Create: `net/gtcp/gtcp_decoder.go`
  - `Decoder` interface
  - `ErrNeedMoreData`
  - shared decoder validation helpers
- Create: `net/gtcp/gtcp_decoder_fixed.go`
  - fixed-length decoder implementation
- Create: `net/gtcp/gtcp_decoder_delimiter.go`
  - delimiter decoder implementation (`line` as special delimiter use-case)
- Modify: `net/gtcp/gtcp_conn.go`
  - add decode buffer fields
  - add `ReadFrame(decoder Decoder) ([]byte, error)`
  - add small internal helpers for append/consume/limit checks
- Modify: `net/gtcp/gtcp_server.go`
  - add decoder and message-handler fields
  - add `SetDecoder` and `SetMessageHandler`
  - add decoder-based accept handling mode
- Create: `net/gtcp/gtcp_z_unit_decoder_fixed_test.go`
  - fixed decoder unit tests
- Create: `net/gtcp/gtcp_z_unit_decoder_delimiter_test.go`
  - delimiter decoder unit tests
- Create: `net/gtcp/gtcp_z_unit_decoder_conn_test.go`
  - `Conn.ReadFrame` integration tests
- Create: `net/gtcp/gtcp_z_unit_decoder_server_test.go`
  - server decoder/message mode tests
- Modify: `net/gtcp/gtcp_z_unit_test.go` (if needed)
  - regression assertions for old server handler behavior

Implementation references:
- spec: `docs/superpowers/specs/2026-03-11-gtcp-frame-decoder-design.md`
- use @superpowers:test-driven-development during code tasks
- use @superpowers:verification-before-completion before claiming done

## Chunk 1: Decoder Contracts and Built-ins

### Task 1: Add decoder contract and sentinel error

**Files:**
- Create: `net/gtcp/gtcp_decoder.go`
- Test: `net/gtcp/gtcp_z_unit_decoder_fixed_test.go`

- [ ] **Step 1: Write failing test for interface-driven decode flow**

```go
// net/gtcp/gtcp_z_unit_decoder_fixed_test.go
func TestDecoderContract_NeedMoreData(t *testing.T) {
    _, _, err := NewFixedLengthDecoder(4).Decode([]byte("ab"))
    gtest.C(t, func(t *gtest.T) {
        t.Assert(err, ErrNeedMoreData)
    })
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./net/gtcp -run TestDecoderContract_NeedMoreData -count=1`
Expected: FAIL (`undefined: NewFixedLengthDecoder` and/or `undefined: ErrNeedMoreData`)

- [ ] **Step 3: Implement minimal decoder contract**

```go
var ErrNeedMoreData = errors.New("need more data")

type Decoder interface {
    Decode(in []byte) (frame []byte, consumed int, err error)
}
```

- [ ] **Step 4: Run test to verify partial pass**

Run: `go test ./net/gtcp -run TestDecoderContract_NeedMoreData -count=1`
Expected: still FAIL (fixed decoder not implemented yet)

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_decoder.go net/gtcp/gtcp_z_unit_decoder_fixed_test.go
git commit -m "feat(gtcp): add decoder contract and sentinel error"
```

### Task 2: Implement fixed-length decoder

**Files:**
- Create: `net/gtcp/gtcp_decoder_fixed.go`
- Modify: `net/gtcp/gtcp_z_unit_decoder_fixed_test.go`

- [ ] **Step 1: Add failing tests for fixed decoder**

```go
func TestFixedDecoder_Success(t *testing.T) {
    d := NewFixedLengthDecoder(4)
    frame, consumed, err := d.Decode([]byte("abcdEF"))
    gtest.C(t, func(t *gtest.T) {
        t.AssertNil(err)
        t.Assert(frame, []byte("abcd"))
        t.Assert(consumed, 4)
    })
}
```

- [ ] **Step 2: Run tests to verify fail**

Run: `go test ./net/gtcp -run TestFixedDecoder_ -count=1`
Expected: FAIL (`undefined: NewFixedLengthDecoder`)

- [ ] **Step 3: Implement fixed decoder with parameter checks**

```go
type fixedLengthDecoder struct { length int }
func NewFixedLengthDecoder(length int) Decoder
func (d *fixedLengthDecoder) Decode(in []byte) ([]byte, int, error)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./net/gtcp -run TestFixedDecoder_ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_decoder_fixed.go net/gtcp/gtcp_z_unit_decoder_fixed_test.go
git commit -m "feat(gtcp): add fixed-length decoder"
```

### Task 3: Implement delimiter decoder

**Files:**
- Create: `net/gtcp/gtcp_decoder_delimiter.go`
- Create: `net/gtcp/gtcp_z_unit_decoder_delimiter_test.go`

- [ ] **Step 1: Add failing delimiter tests**

```go
func TestDelimiterDecoder_Success(t *testing.T) {
    d := NewDelimiterDecoder([]byte("\r\n"), 1024, true)
    frame, consumed, err := d.Decode([]byte("hello\r\nworld"))
    gtest.C(t, func(t *gtest.T) {
        t.AssertNil(err)
        t.Assert(frame, []byte("hello"))
        t.Assert(consumed, 7)
    })
}
```

- [ ] **Step 2: Run tests to verify fail**

Run: `go test ./net/gtcp -run TestDelimiterDecoder_ -count=1`
Expected: FAIL (`undefined: NewDelimiterDecoder`)

- [ ] **Step 3: Implement delimiter decoder**

```go
type delimiterDecoder struct {
    delim          []byte
    maxFrameLen    int
    stripDelimiter bool
}
func NewDelimiterDecoder(delim []byte, maxFrameLen int, stripDelimiter bool) Decoder
func (d *delimiterDecoder) Decode(in []byte) ([]byte, int, error)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./net/gtcp -run TestDelimiterDecoder_ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_decoder_delimiter.go net/gtcp/gtcp_z_unit_decoder_delimiter_test.go
git commit -m "feat(gtcp): add delimiter decoder"
```

## Chunk 2: Conn.ReadFrame Integration

### Task 4: Add `Conn.ReadFrame` and connection decode buffer

**Files:**
- Modify: `net/gtcp/gtcp_conn.go`
- Create: `net/gtcp/gtcp_z_unit_decoder_conn_test.go`

- [ ] **Step 1: Add failing conn integration tests**

```go
func TestConn_ReadFrame_OneFramePerCall(t *testing.T) {
    // server writes "aa|bb|" once, client decoder delim='|'
    // first ReadFrame => "aa", second ReadFrame => "bb"
}
```

- [ ] **Step 2: Run tests to verify fail**

Run: `go test ./net/gtcp -run TestConn_ReadFrame_ -count=1`
Expected: FAIL (`Conn.ReadFrame undefined`)

- [ ] **Step 3: Implement minimal `ReadFrame` runtime**

Implementation checklist:
- add decode buffer field(s) to `Conn` (preserve remaining sticky bytes)
- parse existing buffer before reading new socket bytes
- use `ErrNeedMoreData` to continue reading
- enforce decode buffer max limit
- return one frame per `ReadFrame` call

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./net/gtcp -run TestConn_ReadFrame_ -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_conn.go net/gtcp/gtcp_z_unit_decoder_conn_test.go
git commit -m "feat(gtcp): add conn readframe integration"
```

### Task 5: Add custom decoder coverage

**Files:**
- Modify: `net/gtcp/gtcp_z_unit_decoder_conn_test.go`

- [ ] **Step 1: Add failing tests with custom decoder stub**

```go
type customDecoder struct{}
func (d *customDecoder) Decode(in []byte) ([]byte, int, error) { /* custom framing */ }
```

Test targets:
- custom decoder success
- custom decoder `ErrNeedMoreData`
- custom decoder protocol error path

- [ ] **Step 2: Run tests to verify fail**

Run: `go test ./net/gtcp -run TestConn_ReadFrame_CustomDecoder -count=1`
Expected: FAIL due to behavior not implemented correctly yet

- [ ] **Step 3: Refine `ReadFrame` to satisfy all custom decoder paths**

Implementation notes:
- reject invalid decoder outputs (`consumed < 0`, `consumed > len(buffer)`)
- reject inconsistent output (`frame != nil` with `consumed == 0` unless explicitly supported)

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./net/gtcp -run TestConn_ReadFrame_CustomDecoder -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_conn.go net/gtcp/gtcp_z_unit_decoder_conn_test.go
git commit -m "test(gtcp): cover custom decoder behavior on conn"
```

## Chunk 3: Server Decoder Mode

### Task 6: Add server decoder/message APIs

**Files:**
- Modify: `net/gtcp/gtcp_server.go`
- Create: `net/gtcp/gtcp_z_unit_decoder_server_test.go`

- [ ] **Step 1: Add failing server tests**

Test targets:
- old `SetHandler` path still works unchanged
- `SetDecoder + SetMessageHandler` emits decoded frames
- protocol error closes only current connection

- [ ] **Step 2: Run tests to verify fail**

Run: `go test ./net/gtcp -run TestServer_DecoderMode -count=1`
Expected: FAIL (`SetDecoder`/`SetMessageHandler` undefined)

- [ ] **Step 3: Implement server decoder mode**

Implementation checklist:
- extend `Server` struct with decoder + message handler fields
- add public setters
- in `Run`, choose mode:
  - normal handler mode when decoder unset
  - decoder message mode when decoder set
- on decode protocol error: close this conn immediately
- keep accept loop behavior unchanged for other conns

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./net/gtcp -run TestServer_DecoderMode -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_server.go net/gtcp/gtcp_z_unit_decoder_server_test.go
git commit -m "feat(gtcp): add server decoder message mode"
```

### Task 7: Regression verification for existing APIs

**Files:**
- Modify: `net/gtcp/gtcp_z_unit_test.go` (only if regression tests are missing)

- [ ] **Step 1: Add/adjust failing regression tests**

Focus:
- `SendRecv`, `RecvPkg`, `RecvLine`, `RecvTill` behavior unchanged

- [ ] **Step 2: Run targeted regression tests**

Run: `go test ./net/gtcp -run "TestConn_|TestSend|TestSendRecv|TestSendPkg|TestSendRecvPkg" -count=1`
Expected: PASS (or identify and fix regressions)

- [ ] **Step 3: Fix regressions if any**

Make minimal compatibility fixes only.

- [ ] **Step 4: Re-run regression tests**

Run: `go test ./net/gtcp -run "TestConn_|TestSend|TestSendRecv|TestSendPkg|TestSendRecvPkg" -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/gtcp_z_unit_test.go net/gtcp/gtcp_conn.go net/gtcp/gtcp_server.go
git commit -m "test(gtcp): keep existing tcp behaviors compatible"
```

## Chunk 4: Final Verification and Handoff

### Task 8: Full package verification

**Files:**
- No source changes expected

- [ ] **Step 1: Run full `gtcp` test package**

Run: `go test ./net/gtcp -count=1`
Expected: PASS

- [ ] **Step 2: Run full repository smoke tests (if practical)**

Run: `go test ./...`
Expected: PASS or known unrelated failures documented

- [ ] **Step 3: Capture final behavior summary**

Record:
- APIs added
- default protocol-error behavior
- compatibility guarantees

- [ ] **Step 4: Update docs/comments if needed**

At minimum add doc comments for:
- `Decoder`
- `ErrNeedMoreData`
- `ReadFrame`
- `SetDecoder`
- `SetMessageHandler`

- [ ] **Step 5: Commit**

```bash
git add net/gtcp/*.go net/gtcp/*decoder*_test.go docs/superpowers/specs/2026-03-11-gtcp-frame-decoder-design.md
git commit -m "feat(gtcp): introduce pluggable frame decoders for conn and server"
```

## Acceptance Criteria
- Fixed-length decoder works for half/sticky packets.
- Delimiter decoder works for half/sticky packets and max-length guard.
- Custom decoder fully supported via interface.
- `Conn.ReadFrame` returns exactly one frame per call.
- Server decoder mode supports per-frame callbacks.
- Protocol errors close current connection immediately.
- Existing `gtcp` APIs remain behavior-compatible.
- `go test ./net/gtcp -count=1` passes.

## Execution Notes
- Follow TDD strictly for each task (`test fail -> impl -> test pass`).
- Keep changes minimal and DRY; avoid pipeline overdesign in this iteration.
- Prefer small commits per task for easier review and rollback.

Plan complete and saved to `docs/superpowers/plans/2026-03-11-gtcp-frame-decoder.md`. Ready to execute?
