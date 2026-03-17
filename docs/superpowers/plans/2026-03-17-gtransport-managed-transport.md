# gtransport Managed Transport Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a new managed transport that can create connections on demand, recover future availability after disconnects, and expose idle-read observability without changing the existing fixed-connection transport API.

**Architecture:** Keep `Transport` as the small framing primitive over a fixed `io.ReadWriteCloser`. Add a separate `ManagedTransport` that owns connection lifecycle, wraps an active fixed transport when connected, returns errors from failed operations without transparent replay, and tracks read/write activity timestamps plus one-shot idle notifications.

**Tech Stack:** Go, `go test`, `context.Context`, `time`, `sync`, `net/gtransport`

---

## File Structure

- Create: `net/gtransport/gtransport_managed.go`
- Create: `net/gtransport/gtransport_managed_z_unit_test.go`
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Reference: `docs/superpowers/specs/2026-03-17-gtransport-managed-transport-design.md`
- Reference: `net/gtransport/gtransport.go`
- Reference: `net/gtransport/gtransport_test.go`

## Chunk 1: Public API and Test Scaffold

### Task 1: Lock the managed public API shape with tests

**Files:**
- Create: `net/gtransport/gtransport_managed_z_unit_test.go`

- [ ] **Step 1: Write the failing API-shape test**

Add a test that locks these symbols:

```go
var _ = NewManaged
type ctor func(Connector, Codec, ...ManagedOption) *ManagedTransport
var _ ctor = NewManaged
_ = (&ManagedTransport{}).ReadFrame
_ = (&ManagedTransport{}).WriteFrame
_ = (&ManagedTransport{}).Close
_ = (&ManagedTransport{}).LastReadAt
_ = (&ManagedTransport{}).LastWriteAt
_ = (&ManagedTransport{}).IdleFor
_ = (&ManagedTransport{}).State
```

- [ ] **Step 2: Run the focused test to verify it fails**

Run: `cd net/gtransport && go test ./... -run TestManagedTransportPublicAPIShape -count=1`
Expected: FAIL with undefined managed transport symbols

- [ ] **Step 3: Add the minimal declarations**

Create `gtransport_managed.go` with placeholder declarations for:

- `Connector`
- `ManagedOption`
- `State`
- `ManagedTransport`
- `NewManaged`
- managed methods

- [ ] **Step 4: Re-run the focused test**

Run: `cd net/gtransport && go test ./... -run TestManagedTransportPublicAPIShape -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport_managed.go net/gtransport/gtransport_managed_z_unit_test.go
git commit -m "feat: add managed transport api scaffold"
```

## Chunk 2: Connection Lifecycle

### Task 2: Implement on-demand connect and reconnectable future operations

**Files:**
- Modify: `net/gtransport/gtransport_managed.go`
- Modify: `net/gtransport/gtransport_managed_z_unit_test.go`

- [ ] **Step 1: Write failing lifecycle tests**

Add tests for:

- first `ReadFrame` triggers one connector call
- first `WriteFrame` triggers one connector call
- failed read returns the underlying error and the next call reconnects
- failed write returns the underlying error and the next call reconnects
- failed write is not transparently replayed
- `Close` prevents future reconnect

Use test doubles for:

- connector call counting
- scripted connections that fail on demand
- deterministic frame payloads

- [ ] **Step 2: Run the lifecycle-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestManagedTransport(Connect|Reconnect|Close)' -count=1`
Expected: FAIL because lifecycle behavior is not implemented yet

- [ ] **Step 3: Implement the lifecycle state machine**

Implement in `gtransport_managed.go`:

- mutex-protected current state
- single active connect attempt at a time
- active connection installation
- invalidation of failed connection snapshots
- conservative reconnect-on-next-call behavior
- `Close` teardown and closed-state checks

- [ ] **Step 4: Re-run the lifecycle-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestManagedTransport(Connect|Reconnect|Close)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/gtransport_managed.go net/gtransport/gtransport_managed_z_unit_test.go
git commit -m "feat: add managed transport connection lifecycle"
```

## Chunk 3: Activity Tracking and Idle Notification

### Task 3: Add read/write timestamps and idle observability

**Files:**
- Modify: `net/gtransport/gtransport_managed.go`
- Modify: `net/gtransport/gtransport_managed_z_unit_test.go`

- [ ] **Step 1: Write failing activity and idle tests**

Add tests for:

- `LastReadAt` updates only after a successful `ReadFrame`
- `LastWriteAt` updates only after a successful `WriteFrame`
- `IdleFor` uses `readyAt` when no read has completed yet
- `IdleFor` uses `lastReadAt` after a successful read
- `OnIdle` fires once per idle period
- a successful read resets the one-shot idle notification gate

- [ ] **Step 2: Run the activity-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestManagedTransport(Activity|Idle)' -count=1`
Expected: FAIL because timestamps and idle hooks are incomplete

- [ ] **Step 3: Implement activity bookkeeping**

Add:

- `lastReadAt`
- `lastWriteAt`
- `readyAt`
- idle timeout configuration
- one-shot idle notification tracking
- `LastReadAt`, `LastWriteAt`, `IdleFor`, and `State` implementations

Update successful read/write paths to record timestamps and reset idle period state correctly.

- [ ] **Step 4: Re-run the activity-focused tests**

Run: `cd net/gtransport && go test ./... -run 'TestManagedTransport(Activity|Idle)' -count=1`
Expected: PASS

- [ ] **Step 5: Run package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add net/gtransport/gtransport_managed.go net/gtransport/gtransport_managed_z_unit_test.go
git commit -m "feat: add managed transport idle observability"
```

## Chunk 4: Package Documentation

### Task 4: Document the new managed transport boundary

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 1: Write the failing doc checklist**

Checklist:

- README distinguishes `Transport` from `ManagedTransport`
- README explains that reconnect restores future availability, not failed operations
- README explains idle observation methods and callback semantics

- [ ] **Step 2: Update package documentation**

Document:

- when to use `New` versus `NewManaged`
- the connector-based constructor
- the no-transparent-retry rule
- `LastReadAt`, `LastWriteAt`, `IdleFor`, and `WithOnIdle`

- [ ] **Step 3: Run package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add net/gtransport/README.md net/gtransport/README.zh-CN.md
git commit -m "docs: describe managed transport"
```
