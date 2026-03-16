# gtransport Rewrite Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `net/gpipeline` in place with a smaller `gtransport` package centered on `Codec` and synchronous `Transport`.

**Architecture:** Collapse the existing pipeline/channel/handler framework into a `Codec` + `Transport` design. `Transport` owns buffered framed I/O over a fixed `Conn`, while protocol-specific framing lives in concrete `Codec` implementations. Business processing stays outside the package.

**Tech Stack:** Go, `io.ReadWriteCloser`, framed stream codecs, Go test

---

## Chunk 1: Public API Reset

### Task 1: Rename the package surface to `gtransport`

**Files:**
- Modify: `net/gpipeline/go.mod`
- Modify: `net/gpipeline/*.go`
- Modify: `net/gpipeline/example/tcp_echo/main.go`
- Modify: `net/gpipeline/example/gtcp_echo/main.go`

- [ ] **Step 1: Write the failing test**

Add a package-level API test that references:
- `type Codec interface`
- `type Transport struct`
- `func New(conn Conn, codec Codec, opts ...Option) *Transport`
- `func (t *Transport) ReadFrame() ([]byte, error)`
- `func (t *Transport) WriteFrame(frame []byte) error`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestPublicAPIShape -count=1`
Expected: FAIL with missing `Transport`/`New`/`ReadFrame`/`WriteFrame`

- [ ] **Step 3: Write minimal implementation**

Rename package declarations from `gpipeline` to `gtransport` and introduce the new top-level API surface while removing references to `Pipeline`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestPublicAPIShape -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "refactor: reset gpipeline to gtransport api"
```

### Task 2: Remove pipeline-only concepts

**Files:**
- Modify: `net/gpipeline/gpipeline.go`
- Delete: `net/gpipeline/gpipeline_handler.go`
- Modify: `net/gpipeline/gpipeline_internal_test.go`
- Modify: `net/gpipeline/gpipeline_z_unit_test.go`

- [ ] **Step 1: Write the failing test**

Add/adjust tests so they no longer rely on:
- `Start`
- `Receive`
- `Send`
- `TrySend`
- `SendCh`
- `WithHandler`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestTransportWithoutPipelineLifecycle -count=1`
Expected: FAIL while old API is still required by tests or code

- [ ] **Step 3: Write minimal implementation**

Delete the handler/channel lifecycle code and keep only synchronous framed read/write behavior.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestTransportWithoutPipelineLifecycle -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "refactor: remove pipeline lifecycle and handlers"
```

## Chunk 2: Transport Core

### Task 3: Implement synchronous framed reads

**Files:**
- Modify: `net/gpipeline/gpipeline.go`
- Modify: `net/gpipeline/gpipeline_internal_test.go`

- [ ] **Step 1: Write the failing test**

Add tests covering:
- partial reads across multiple `Read` calls
- invalid decoder output protection
- max buffer enforcement
- user `Close()` interrupting `ReadFrame()`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'TestTransport_(ReadFrame|Close)' -count=1`
Expected: FAIL on unimplemented `ReadFrame` behavior

- [ ] **Step 3: Write minimal implementation**

Move the current `readFrame` logic into `Transport.ReadFrame()`, preserving:
- decoder contract validation
- read timeout support
- decode buffer shrinking

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'TestTransport_(ReadFrame|Close)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "feat: add synchronous framed reads"
```

### Task 4: Implement synchronous framed writes

**Files:**
- Modify: `net/gpipeline/gpipeline.go`
- Modify: `net/gpipeline/gpipeline_internal_test.go`

- [ ] **Step 1: Write the failing test**

Add tests covering:
- encoded write output
- short write retry until full frame is written
- concurrent `WriteFrame` calls serialize correctly
- write after close returns a connection error

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run TestTransport_WriteFrame -count=1`
Expected: FAIL on missing `WriteFrame` or incorrect short-write handling

- [ ] **Step 3: Write minimal implementation**

Implement `WriteFrame` with:
- `Codec.Encode`
- full-write loop
- mutex-protected serialized writes

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run TestTransport_WriteFrame -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "feat: add synchronous framed writes"
```

## Chunk 3: Codec Cleanup

### Task 5: Collapse codec API to a single `Codec` surface

**Files:**
- Modify: `net/gpipeline/gpipeline_decoder_delimiter.go`
- Modify: `net/gpipeline/gpipeline_decoder_fixed.go`
- Modify: `net/gpipeline/gpipeline_decoder_length_field.go`
- Modify: `net/gpipeline/gpipeline_decoder_line.go`
- Modify: `net/gpipeline/gpipeline_encoder_delimiter.go`
- Modify: `net/gpipeline/gpipeline_encoder_fixed.go`
- Modify: `net/gpipeline/gpipeline_encoder_length_field.go`
- Modify: `net/gpipeline/gpipeline_encoder_line.go`
- Modify: `net/gpipeline/gpipeline_length_field_api_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for the new constructors:
- `NewDelimiter`
- `NewLine`
- `NewFixedLength`
- `NewLengthPrefixed`
- `NewLengthField`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Test(NewDelimiter|NewLine|NewFixedLength|NewLengthPrefixed|NewLengthField)' -count=1`
Expected: FAIL with missing constructors

- [ ] **Step 3: Write minimal implementation**

Unify each codec pair into a single type implementing both `Decode` and `Encode`, and promote the simple constructors as the main API.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'Test(NewDelimiter|NewLine|NewFixedLength|NewLengthPrefixed|NewLengthField)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "refactor: unify codec implementations"
```

### Task 6: Simplify length-prefixed framing

**Files:**
- Modify: `net/gpipeline/gpipeline_decoder_length_field.go`
- Modify: `net/gpipeline/gpipeline_encoder_length_field.go`
- Modify: `net/gpipeline/gpipeline_length_field_api_test.go`

- [ ] **Step 1: Write the failing test**

Add tests for:
- `NewLengthPrefixed(2, binary.BigEndian, maxPayload)`
- `NewLengthPrefixed(4, binary.LittleEndian, maxPayload)`
- advanced `NewLengthField(LengthFieldOption{...})`

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -run 'Test(NewLengthPrefixed|NewLengthField)' -count=1`
Expected: FAIL before the new simplified constructor exists

- [ ] **Step 3: Write minimal implementation**

Introduce `LengthFieldOption` as the advanced type and layer `NewLengthPrefixed` on top of it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -run 'Test(NewLengthPrefixed|NewLengthField)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "feat: add simplified length prefixed codec"
```

## Chunk 4: Documentation and Verification

### Task 7: Update examples and package docs

**Files:**
- Modify: `net/gpipeline/example/tcp_echo/main.go`
- Modify: `net/gpipeline/example/gtcp_echo/main.go`
- Modify: `net/gpipeline/*.go` package comments as needed

- [ ] **Step 1: Write the failing test**

Use build-based validation for examples after updating imports and public API usage.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./... -count=1`
Expected: FAIL while examples still target the old API

- [ ] **Step 3: Write minimal implementation**

Rewrite the examples to use:
- `codec := gtransport.NewLengthPrefixed(...)`
- `tr := gtransport.New(conn, codec)`
- `ReadFrame` / `WriteFrame`

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gpipeline
git commit -m "docs: update examples for gtransport"
```

### Task 8: Final verification

**Files:**
- Test only

- [ ] **Step 1: Run native verification**

Run: `go test ./... -count=1`
Expected: PASS

- [ ] **Step 2: Run cross-target verification**

Run: `$env:GOARCH='386'; $env:GOOS='windows'; go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Inspect package API surface**

Confirm only the intended public API remains:
- `Conn`
- `Codec`
- `Transport`
- `Option`
- `New`
- `ReadFrame`
- `WriteFrame`
- `Close`
- `RawConn`
- `WithReadTimeout`
- `WithMaxBufferBytes`
- codec constructors
- `ErrNeedMoreData`

- [ ] **Step 4: Commit**

```bash
git add net/gpipeline
git commit -m "test: verify final gtransport rewrite"
```
