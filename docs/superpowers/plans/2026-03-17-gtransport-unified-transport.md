# gtransport Unified Transport Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the split `Transport` / `ManagedTransport` public model with one unified `Transport` type that is created by `Dial` or `Wrap`, supports reconnect for outbound connections, and exposes consistent idle-read observability.

**Architecture:** Keep one internal framing engine and one public transport type. `Wrap` will cover accepted or already-open connections without reconnect semantics, while `Dial` will own connect/reconnect lifecycle. The migration is staged: introduce the unified API and compatibility shims first, then remove the transitional managed API once the new path is fully covered by tests and docs.

**Tech Stack:** Go, `context`, `time`, `sync`, `io`, `go test`, `net/gtransport`

---

## File Structure

- Modify: `net/gtransport/gtransport.go`
  Public `Transport` type, shared state, unified read/write methods, shared options that apply to both `Dial` and `Wrap`.
- Create: `net/gtransport/gtransport_dial.go`
  Dial-mode constructor and connect/reconnect lifecycle logic.
- Create: `net/gtransport/gtransport_wrap.go`
  Wrap-mode constructor and fixed-connection lifecycle rules.
- Modify: `net/gtransport/gtransport_managed.go`
  Temporary compatibility shim from `ManagedTransport` / `NewManaged` toward the unified model, then eventual removal in a later chunk.
- Modify: `net/gtransport/gtransport_test.go`
  Replace old public API shape assertions with unified `Transport`, `Dial`, and `Wrap` coverage.
- Modify: `net/gtransport/gtransport_managed_z_unit_test.go`
  Convert managed tests into unified `Dial` behavior tests, then remove once no longer needed.
- Create: `net/gtransport/gtransport_dial_z_unit_test.go`
  Focused tests for reconnect, connect timeout, waiters, close behavior, and no transparent replay.
- Create: `net/gtransport/gtransport_wrap_z_unit_test.go`
  Focused tests for accepted-connection behavior, terminal close semantics, and idle behavior.
- Create: `net/gtransport/gtransport_context_z_unit_test.go`
  Tests that lock `ctx` semantics across connect, read, and write paths.
- Modify: `net/gtransport/README.md`
  Rewrite docs around `Dial` and `Wrap`.
- Modify: `net/gtransport/README.zh-CN.md`
  Rewrite Chinese docs around `Dial` and `Wrap`.
- Modify: `net/gtransport/example/tcp_echo/main.go`
  Migrate accepted-connection example to `Wrap`.
- Modify: `net/gtransport/example/gtcp_echo/main.go`
  Migrate accepted-connection example to `Wrap`.
- Modify: `net/gtransport/example/modbus_recommended/main.go`
  Migrate accepted-connection example to `Wrap`.
- Modify: `net/gtransport/example/modbus_advanced/main.go`
  Migrate accepted-connection example to `Wrap`.
- Reference: `docs/superpowers/specs/2026-03-17-gtransport-unified-transport-design.md`

## Chunk 1: Lock the Unified Public API

### Task 1: Replace the public API shape tests

**Files:**
- Modify: `net/gtransport/gtransport_test.go`
- Create: `net/gtransport/gtransport_dial_z_unit_test.go`
- Create: `net/gtransport/gtransport_wrap_z_unit_test.go`

- [ ] **Step 1: Write the failing unified API shape test**

Add assertions for:

```go
var _ = Dial
var _ = Wrap

type dialCtor func(Connector, Codec, ...DialOption) *Transport
type wrapCtor func(io.ReadWriteCloser, Codec, ...WrapOption) *Transport

var _ dialCtor = Dial
var _ wrapCtor = Wrap

_ = (&Transport{}).ReadFrame
_ = (&Transport{}).WriteFrame
_ = (&Transport{}).Close
_ = (&Transport{}).State
_ = (&Transport{}).LastReadAt
_ = (&Transport{}).LastWriteAt
_ = (&Transport{}).IdleFor
```

- [ ] **Step 2: Run the focused API-shape tests and verify they fail**

Run: `cd net/gtransport && go test ./... -run 'Test(PublicAPIShape|TransportUnifiedAPIShape)' -count=1`
Expected: FAIL with undefined `Dial`, `Wrap`, `DialOption`, or `WrapOption`

- [ ] **Step 3: Add the minimal unified declarations**

Introduce placeholder declarations in the transport package for:

- `Dial`
- `Wrap`
- `DialOption`
- `WrapOption`
- unified `Transport` method signatures with `context.Context`

- [ ] **Step 4: Re-run the focused API-shape tests**

Run: `cd net/gtransport && go test ./... -run 'Test(PublicAPIShape|TransportUnifiedAPIShape)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport.go net/gtransport/gtransport_test.go net/gtransport/gtransport_dial_z_unit_test.go net/gtransport/gtransport_wrap_z_unit_test.go
git commit -m "feat: lock unified gtransport api shape"
```

## Chunk 2: Implement Wrap Mode on the Unified Transport

### Task 2: Port fixed-connection framing to `Wrap`

**Files:**
- Modify: `net/gtransport/gtransport.go`
- Create: `net/gtransport/gtransport_wrap.go`
- Create: `net/gtransport/gtransport_wrap_z_unit_test.go`

- [ ] **Step 1: Write failing wrap-mode tests**

Cover:

- `Wrap` starts in `StateReady`
- wrapped connections read and write frames successfully
- wrapped short writes still flush complete encoded payload
- wrapped connection failure closes the transport permanently
- `IdleFor` uses ready time before first read and last-read time after a successful read

- [ ] **Step 2: Run the wrap-focused tests and verify they fail**

Run: `cd net/gtransport && go test ./... -run 'TestTransportWrap' -count=1`
Expected: FAIL because wrap-mode lifecycle is not implemented yet

- [ ] **Step 3: Move fixed-connection behavior behind `Wrap`**

Implement:

- `Wrap(conn, codec, opts...)`
- shared read buffer and encoding logic
- wrap-specific lifecycle rule: connection failure transitions to `StateClosed`
- shared timestamp bookkeeping for read and write success

- [ ] **Step 4: Re-run the wrap-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestTransportWrap' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport.go net/gtransport/gtransport_wrap.go net/gtransport/gtransport_wrap_z_unit_test.go
git commit -m "feat: add unified transport wrap mode"
```

## Chunk 3: Implement Dial Mode and Reconnect Semantics

### Task 3: Port managed lifecycle into unified `Transport`

**Files:**
- Modify: `net/gtransport/gtransport.go`
- Create: `net/gtransport/gtransport_dial.go`
- Create: `net/gtransport/gtransport_dial_z_unit_test.go`
- Modify: `net/gtransport/gtransport_managed.go`

- [ ] **Step 1: Write failing dial-mode lifecycle tests**

Cover:

- first read triggers one connect attempt
- first write triggers one connect attempt
- failed read returns the underlying error and the next call reconnects
- failed write returns the underlying error and the next call reconnects
- failed write is not replayed transparently
- `Close` prevents future reconnect attempts
- only one connect attempt runs while multiple goroutines wait

- [ ] **Step 2: Run the dial-focused tests and verify they fail**

Run: `cd net/gtransport && go test ./... -run 'TestTransportDial' -count=1`
Expected: FAIL because unified dial lifecycle is incomplete

- [ ] **Step 3: Implement dial-mode lifecycle**

Implement:

- `Dial(connector, codec, opts...)`
- dial-mode state machine
- single in-flight connect attempt
- reconnect-on-later-call behavior
- connect timeout and reconnect backoff handling
- compatibility shim from `NewManaged` to `Dial`

- [ ] **Step 4: Re-run the dial-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestTransportDial' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport.go net/gtransport/gtransport_dial.go net/gtransport/gtransport_dial_z_unit_test.go net/gtransport/gtransport_managed.go
git commit -m "feat: add unified transport dial mode"
```

## Chunk 4: Make `ctx` Cover the Whole Operation

### Task 4: Lock end-to-end context semantics

**Files:**
- Modify: `net/gtransport/gtransport.go`
- Modify: `net/gtransport/gtransport_dial.go`
- Modify: `net/gtransport/gtransport_wrap.go`
- Create: `net/gtransport/gtransport_context_z_unit_test.go`

- [ ] **Step 1: Write failing context tests**

Cover:

- canceled `ctx` aborts waiting for another goroutine's connect attempt
- canceled `ctx` aborts connect timeout waiting
- canceled `ctx` aborts `ReadFrame(ctx)` on a deadline-capable wrapped connection
- canceled `ctx` aborts `WriteFrame(ctx, frame)` on a deadline-capable wrapped connection
- `Close` unblocks pending dial waiters and pending I/O waiters

- [ ] **Step 2: Run the context-focused tests and verify they fail**

Run: `cd net/gtransport && go test ./... -run 'TestTransportContext' -count=1`
Expected: FAIL because `ctx` currently does not cover the entire operation

- [ ] **Step 3: Implement unified context handling**

Implement:

- per-operation deadline plumbing derived from `ctx`
- safe restore or clear of temporary deadlines after operations
- close-path signaling that wakes waiters blocked on connect or I/O

- [ ] **Step 4: Re-run the context-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestTransportContext' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport.go net/gtransport/gtransport_dial.go net/gtransport/gtransport_wrap.go net/gtransport/gtransport_context_z_unit_test.go
git commit -m "feat: honor context across transport operations"
```

## Chunk 5: Collapse Transitional APIs Into Compatibility Shims

### Task 5: Deprecate old constructors and managed types

**Files:**
- Modify: `net/gtransport/gtransport.go`
- Modify: `net/gtransport/gtransport_managed.go`
- Modify: `net/gtransport/gtransport_managed_z_unit_test.go`

- [ ] **Step 1: Write failing compatibility tests**

Cover:

- `NewManaged` delegates to unified dial behavior
- compatibility paths preserve documented no-replay semantics
- deprecated paths still expose the same observable behavior during transition

- [ ] **Step 2: Run the compatibility-focused tests and verify they fail**

Run: `cd net/gtransport && go test ./... -run 'TestTransportCompatibility' -count=1`
Expected: FAIL because old constructors are not yet unified compatibility shims

- [ ] **Step 3: Implement compatibility layer**

Implement:

- `NewManaged` as a deprecated wrapper around `Dial`
- deprecation comments on old constructors and types
- removal or consolidation of duplicate managed-only option plumbing where possible

- [ ] **Step 4: Re-run the compatibility-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestTransportCompatibility' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport.go net/gtransport/gtransport_managed.go net/gtransport/gtransport_managed_z_unit_test.go
git commit -m "refactor: deprecate split gtransport constructors"
```

## Chunk 5A: Remove Final Compatibility Constructor

### Task 5A: Remove `New` and finish the public API cleanup

**Files:**
- Modify: `net/gtransport/gtransport.go`
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 1: Confirm no code or user-facing docs still depend on `New`**

Run: `rg -n 'gtransport\.New\(' net/gtransport`
Expected: no remaining call sites to `gtransport.New`

- [ ] **Step 2: Remove `New` and its compatibility-only option alias**

Delete the remaining compatibility-only declarations:

- `New`
- `Option`

Keep:

- `Dial`
- `Wrap`
- `DialOption`
- `WrapOption`

- [ ] **Step 3: Update docs to state the final API contains only `Dial` and `Wrap`**

Rewrite any remaining compatibility wording so the final public story is:

- use `Dial` for outbound reconnectable transport
- use `Wrap` for accepted or already-open connections

- [ ] **Step 4: Run package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport.go net/gtransport/README.md net/gtransport/README.zh-CN.md docs/superpowers/specs/2026-03-17-gtransport-unified-transport-design.md docs/superpowers/plans/2026-03-17-gtransport-unified-transport.md
git commit -m "refactor: remove final gtransport compatibility constructor"
```

## Chunk 6: Rewrite Docs and Examples Around `Dial` and `Wrap`

### Task 6: Update package documentation and examples

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Modify: `net/gtransport/example/tcp_echo/main.go`
- Modify: `net/gtransport/example/gtcp_echo/main.go`
- Modify: `net/gtransport/example/modbus_recommended/main.go`
- Modify: `net/gtransport/example/modbus_advanced/main.go`

- [ ] **Step 1: Write the failing documentation checklist**

Checklist:

- docs present `Dial` as outbound managed transport
- docs present `Wrap` as accepted-connection transport
- docs remove `ManagedTransport` as a primary recommendation
- docs explain no transparent replay
- examples compile with the new constructors

- [ ] **Step 2: Update docs and examples**

Rewrite:

- package overview around one `Transport`
- example code to call `Wrap` for accepted connections
- wording that distinguishes reconnectable `Dial` from terminal `Wrap`

- [ ] **Step 3: Run package tests and example builds**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

Run: `cd net/gtransport/example/tcp_echo && go test ./... -count=1`
Expected: PASS

Run: `cd net/gtransport/example/gtcp_echo && go test ./... -count=1`
Expected: PASS

Run: `cd net/gtransport/example/modbus_recommended && go test ./... -count=1`
Expected: PASS

Run: `cd net/gtransport/example/modbus_advanced && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add net/gtransport/README.md net/gtransport/README.zh-CN.md net/gtransport/example/tcp_echo/main.go net/gtransport/example/gtcp_echo/main.go net/gtransport/example/modbus_recommended/main.go net/gtransport/example/modbus_advanced/main.go
git commit -m "docs: unify gtransport around dial and wrap"
```

## Chunk 7: Final Verification and Cleanup

### Task 7: Verify the unified model end to end

**Files:**
- Modify: `net/gtransport/gtransport_test.go`
- Modify: `net/gtransport/gtransport_z_unit_modbus_integration_test.go`

- [ ] **Step 1: Add or update final end-to-end tests**

Cover:

- modbus integration through `Wrap`
- read/write and reconnect expectations through `Dial`
- idle and state reporting across reconnect cycles

- [ ] **Step 2: Run full package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 3: Run targeted race-style verification where applicable**

Run: `cd net/gtransport && go test ./... -count=1 -run 'TestTransport(Dial|Wrap|Context)'`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add net/gtransport/gtransport_test.go net/gtransport/gtransport_z_unit_modbus_integration_test.go
git commit -m "test: verify unified gtransport behavior"
```
