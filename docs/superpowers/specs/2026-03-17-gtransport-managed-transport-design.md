# gtransport Managed Transport Design

**Goal**

Add a new managed transport API that can create its own stream connection, recover future availability after disconnects, and expose idle-read observability without changing the semantics of the existing `Transport`.

## Scope

This design adds a new connection-managing transport type under `net/gtransport`.

It covers:

- a connector-based constructor
- connection state management
- reconnect-on-next-operation behavior
- read/write activity timestamps
- idle-read notification hooks
- tests and package documentation updates

It does not redesign codec behavior or the existing fixed-connection `Transport`.

## Problem

The current `Transport` is intentionally small. It accepts one already-open `io.ReadWriteCloser` and performs framed reads and writes on that fixed connection.

That shape is good for simple framing, but it leaves two repeated concerns to callers:

- creating and recreating the underlying connection
- monitoring how long the connection has gone without receiving frames

Projects that need reconnect behavior and idle-read visibility currently have to wrap `Transport` themselves. That leads to repeated state machines, uneven error handling, and inconsistent notification semantics.

## Design Principles

- Keep the existing `Transport` unchanged.
- Add connection management as a new, explicit API surface.
- Recover future connection availability, but do not transparently retry the failed read or write that observed the disconnect.
- Treat idle detection as transport observability, not as built-in business policy.
- Keep codec and framed I/O logic reused instead of duplicated.

## Chosen Approach

Add a new `ManagedTransport` type and a new connector-based constructor:

```go
type Connector func(ctx context.Context) (io.ReadWriteCloser, error)

type ManagedTransport struct{}

func NewManaged(connector Connector, codec Codec, opts ...ManagedOption) *ManagedTransport
```

`ManagedTransport` owns connection lifecycle. It creates a connection on demand, wraps that connection with the existing framing logic, and exposes state and idle-read observation methods.

The existing constructor remains unchanged:

```go
func New(conn io.ReadWriteCloser, codec Codec, opts ...Option) *Transport
```

This keeps backward compatibility and preserves the current package boundary for callers that only want framing.

## Public API

Recommended public shape:

```go
type Connector func(ctx context.Context) (io.ReadWriteCloser, error)

type ManagedOption func(*ManagedTransport)

type State int

func NewManaged(connector Connector, codec Codec, opts ...ManagedOption) *ManagedTransport

func (t *ManagedTransport) ReadFrame(ctx context.Context) ([]byte, error)
func (t *ManagedTransport) WriteFrame(ctx context.Context, frame []byte) error
func (t *ManagedTransport) Close() error

func (t *ManagedTransport) LastReadAt() time.Time
func (t *ManagedTransport) LastWriteAt() time.Time
func (t *ManagedTransport) IdleFor(now time.Time) time.Duration
func (t *ManagedTransport) State() State
```

Recommended managed options:

- `WithConnectTimeout(d time.Duration)`
- `WithReconnectBackoff(backoff func(attempt int) time.Duration)`
- `WithIdleTimeout(d time.Duration)`
- `WithOnStateChange(func(State))`
- `WithOnIdle(func(IdleEvent))`

`ReadFrame` and `WriteFrame` should accept `context.Context` because managed operations may need to wait for connect attempts or caller cancellation.

## State Model

`ManagedTransport` should maintain a small state machine:

- `StateIdle`: no active connection is currently ready
- `StateConnecting`: a connect attempt is in progress
- `StateReady`: a framed transport backed by a live connection is available
- `StateClosed`: the managed transport has been permanently closed

Transitions:

- initial state is `StateIdle`
- `ReadFrame` or `WriteFrame` in `StateIdle` triggers a connect attempt and enters `StateConnecting`
- successful connect moves to `StateReady`
- connection-level read or write failure invalidates the active connection and moves back to `StateIdle`
- `Close` moves to `StateClosed` and prevents future reconnects

## Reconnect Semantics

Reconnect behavior should be explicit and conservative.

Rules:

- if there is no active connection, the next read or write operation attempts to connect
- if an active connection fails during a read or write, that call returns the observed error
- after such a failure, the managed transport discards the connection and becomes reconnectable for the next call
- the failed read or write is not retried automatically

This rule is important because the failed operation may be only partially complete:

- a write may have been partly or fully received by the peer before the error surfaced
- a read failure may leave frame-boundary certainty behind on the old connection

Automatic replay would therefore risk duplicate requests or ambiguous protocol behavior.

## Idle-Read Observability

Idle-read visibility belongs in the transport layer, but the action taken after detecting idle should remain the caller's responsibility.

Observation methods:

- `LastReadAt()` returns the timestamp of the last successfully decoded frame
- `LastWriteAt()` returns the timestamp of the last successful `WriteFrame`
- `IdleFor(now)` returns how long the managed transport has gone without a successful read

Optional notification:

```go
type IdleEvent struct {
    Since   time.Time
    IdleFor time.Duration
    HasConn bool
    State   State
}
```

Notification semantics:

- `WithIdleTimeout` enables idle tracking with a threshold
- `WithOnIdle` registers a callback
- when the transport remains without a successful read for at least the configured duration, invoke `OnIdle`
- emit one callback per idle period, not a continuous stream of duplicate callbacks
- a successful read clears the current idle period and permits a future idle callback

The callback is an observation hook only. It must not implicitly close, reconnect, or write heartbeats on behalf of the caller.

## Initial Idle Baseline

A connected transport that has never successfully read a frame should still be observable as idle.

To support that, idle duration should be based on:

- the last successful read time, if one exists
- otherwise the time when the current connection entered `StateReady`

This lets callers detect "connected but silent" peers.

## Internal Structure

Implementation should reuse the existing `Transport` for framed I/O once a connection is established.

Recommended fields for `ManagedTransport`:

- connector and codec references
- managed options and callback references
- mutex-protected active connection and active `Transport`
- current state
- connect attempt counter or backoff bookkeeping
- `lastReadAt`
- `lastWriteAt`
- `readyAt`
- per-idle-period notification marker

The implementation should avoid copying framing logic into a second transport stack.

## Error Handling

Add explicit managed transport errors for stable behavior checks where needed:

- closed transport
- missing connector
- canceled connect attempt

Connection-level read and write failures should continue returning the underlying error so callers can inspect it.

Error messages should remain lowercase and without trailing punctuation to match repository conventions.

## Concurrency

Concurrency expectations should be explicit:

- concurrent writes remain serialized
- only one connect attempt should run at a time
- reads and writes should observe a consistent active connection snapshot
- invalidating a failed connection must not close a newly replaced connection by mistake

The implementation should carefully separate "the connection that failed" from "the currently installed connection" when tearing down state.

## Testing Strategy

Add focused unit tests that cover:

- public API shape for `NewManaged` and managed methods
- first operation triggers connect
- failed read returns error and allows the next call to reconnect
- failed write returns error and allows the next call to reconnect
- current failed operation is not transparently replayed
- `LastReadAt` updates only after successful reads
- `IdleFor` works both after reads and while connected-without-data
- `OnIdle` fires once per idle period and resets after the next successful read
- `Close` permanently disables reconnect

Tests should use GoFrame's preferred unit-test naming pattern for new test files.

## File Layout

Expected implementation files:

- `net/gtransport/gtransport_managed.go`
- `net/gtransport/gtransport_managed_z_unit_test.go`
- `net/gtransport/README.md`
- `net/gtransport/README.zh-CN.md`

The existing `gtransport.go` file should remain focused on the fixed-connection transport.

## Non-Goals

- transparent replay of failed reads or writes
- built-in heartbeat policy
- built-in business alarms or logging sinks
- automatic background reconnect loops without caller activity
- merging managed lifecycle behavior into the existing `Transport`

## Compatibility

This design is additive.

- existing `Transport` behavior remains unchanged
- existing constructors remain unchanged
- codec implementations remain unchanged
- callers can opt into `ManagedTransport` only when they need connection lifecycle and idle observability
