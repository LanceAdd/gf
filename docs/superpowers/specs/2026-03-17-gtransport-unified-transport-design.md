# gtransport Unified Transport Design

**Goal**

Redesign `net/gtransport` around one public transport model that treats connection lifecycle management as the default concern, while still supporting server-side accepted connections through a thin wrapping entry point.

The new design must make automatic reconnect and long-idle detection first-class capabilities without forcing users to choose between parallel transport types.

## Problem

The current package has started to drift into two models:

- a fixed-connection `Transport`
- an additive `ManagedTransport`

That split increases cognitive load for a foundational component:

- users must choose between two transport types up front
- transport configuration starts to fragment into separate option sets
- method semantics stop lining up cleanly
- future documentation and examples would have to teach two parallel paths

This is not a good long-term API shape for a low-level framework component.

At the same time, the real requirement has changed. `gtransport` is no longer just about frame decoding over an already-open stream. It now needs to support:

- active connection establishment
- automatic reconnect for future operations
- detection of long periods without incoming data

Those are connection lifecycle concerns, not just framing concerns.

## Design Direction

`gtransport` should expose one public transport model:

```go
type Transport struct{}
```

Users should create it through one of two clearly named entry points:

```go
func Dial(connector Connector, codec Codec, opts ...DialOption) *Transport
func Wrap(conn io.ReadWriteCloser, codec Codec, opts ...WrapOption) *Transport
```

This design keeps a single public model while preserving the two legitimate connection sources:

- outbound/client-style transport that owns connection establishment
- inbound/server-style transport that wraps an already accepted connection

The package should no longer present `ManagedTransport` as a long-term public type.

## Core Principles

- One public transport type, not two parallel transport types.
- Connection lifecycle is a first-class transport responsibility.
- Framed I/O remains the core operation surface.
- Failed operations are never transparently replayed.
- Idle detection is transport observability, not transport policy.
- Server accepted connections remain supported, but they do not pretend to have reconnect semantics.
- The API should be optimized for clarity over configuration breadth.

## Public API

Recommended public shape:

```go
type Connector func(ctx context.Context) (io.ReadWriteCloser, error)

type DialOption func(*dialOptions)
type WrapOption func(*wrapOptions)

type State int

const (
    StateIdle State = iota
    StateConnecting
    StateReady
    StateClosed
)

func Dial(connector Connector, codec Codec, opts ...DialOption) *Transport
func Wrap(conn io.ReadWriteCloser, codec Codec, opts ...WrapOption) *Transport

func (t *Transport) ReadFrame(ctx context.Context) ([]byte, error)
func (t *Transport) WriteFrame(ctx context.Context, frame []byte) error
func (t *Transport) Close() error

func (t *Transport) State() State
func (t *Transport) LastReadAt() time.Time
func (t *Transport) LastWriteAt() time.Time
func (t *Transport) IdleFor(now time.Time) time.Duration
```

This intentionally removes `ManagedTransport` from the long-term public story and avoids keeping both `New` and `NewManaged` as equally valid primary constructors.

## Entry Point Semantics

### Dial

`Dial` creates a transport that owns connection establishment.

It is the primary API for:

- client connections
- device communication
- reconnectable long-lived streams
- scenarios that require idle-read observability across reconnectable sessions

`Dial` may create a connection lazily on first read or write, or eagerly if implementation later chooses to support that, but its public contract is that it manages future availability.

### Wrap

`Wrap` creates a transport around an already established `io.ReadWriteCloser`.

It is the primary API for:

- server-side `Accept()` flows
- wrapping already-open pipes or test connections
- cases where connection lifecycle is owned externally

`Wrap` does not reconnect. Once its underlying connection fails, that transport instance is terminal.

## Read/Write Semantics

Both creation paths return the same `Transport` type and expose the same methods:

```go
ReadFrame(ctx)
WriteFrame(ctx, frame)
```

### Required rules

- `ctx` covers the whole operation, not just the connect phase.
- A failed read or write returns the observed error directly.
- The failed operation is not retried transparently.
- For `Dial`, the transport may attempt to reconnect on a later call.
- For `Wrap`, a connection failure ends the transport's useful lifetime.

This rule must remain stable because automatic replay of failed framed operations is unsafe:

- a write may already have been partly or fully received by the peer
- a read failure may leave frame boundary certainty behind on the old connection

## Context Semantics

The API must not expose a `context.Context` parameter unless the transport makes a real attempt to honor it for the entire operation.

That means:

- connect waiting must observe `ctx`
- read blocking must observe `ctx`
- write blocking must observe `ctx`

Where the underlying connection supports deadlines, the transport should translate `ctx` into deadline behavior.

If a specific connection type cannot be interrupted cleanly, the documentation may note that limitation, but the transport should still aim for one consistent semantic model rather than a split between connect-time context and I/O-time context.

## Idle Semantics

Idle detection is defined as the duration since the last successful frame read on the current active connection.

Rules:

- `LastReadAt()` reports the last successful full-frame read time.
- `LastWriteAt()` reports the last successful full-frame write time.
- `IdleFor(now)` reports duration since the last successful read.
- If the current connection became ready but has never produced a successful read, the idle baseline is the ready time of that connection.
- If there is no active connection, `IdleFor(now)` returns `0`.

This keeps idle semantics focused on "connected but silent" rather than "overall unavailable".

## State Semantics

The public state model should stay minimal:

- `StateIdle`
- `StateConnecting`
- `StateReady`
- `StateClosed`

### Dial state rules

- initial state is `StateIdle`
- a read or write with no active connection may move to `StateConnecting`
- a successful connect moves to `StateReady`
- a connection failure moves back to `StateIdle`
- `Close` moves to `StateClosed`

### Wrap state rules

- initial state is `StateReady`
- connection failure moves directly to `StateClosed`
- `Close` moves to `StateClosed`

`Wrap` should not pretend to be recoverable after connection loss.

## Option Strategy

The package should deliberately limit public configuration surface.

### Shared transport concerns

These are legitimate transport-level options for both `Dial` and `Wrap`:

- `WithReadTimeout(d time.Duration)`
- `WithMaxBufferBytes(n int)`
- `WithIdleTimeout(d time.Duration)`

### Dial-only concerns

These are meaningful only for `Dial`:

- `WithConnectTimeout(d time.Duration)`
- `WithReconnectBackoff(func(attempt int) time.Duration)`

### Excluded from first-class API

Do not make callback-based transport events part of the primary API surface for the first version of this redesign:

- no `WithOnStateChange`
- no `WithOnIdle`

Reason:

- callback timing and concurrency semantics are hard to keep stable
- callbacks push a low-level transport toward becoming an event framework
- foundational transport code should expose facts, not policy hooks

The package should expose state and timestamps. Higher layers can decide whether they want polling, monitoring, or event translation.

## Internal Architecture

The implementation should keep one internal framing engine and one public transport type.

Recommended structure:

- `Transport` stores the common public state and lifecycle
- one internal mode indicates whether it was created by `Dial` or `Wrap`
- framing logic remains reused rather than duplicated
- fixed-connection framing is still an internal primitive, but not a separate public model

This keeps the implementation layered without exposing the layering split as a user-facing type split.

## Error Handling

Stable exported errors may be retained for:

- closed transport
- missing connector
- nil connection returned by connector

Connection-level read/write failures should keep returning underlying errors.

Error messages should remain lowercase and have no trailing punctuation.

## Migration Strategy

Migration should be staged.

### Phase 1

Introduce the new primary API:

- `Dial`
- `Wrap`
- unified `Transport`

Keep existing APIs only as temporary compatibility shims:

- `New` becomes a deprecated alias for `Wrap`
- `NewManaged` and `ManagedTransport` become deprecated compatibility paths toward `Dial`

All new examples, docs, and tests should switch to the new API immediately.

### Phase 2

After a deprecation window:

- remove `New`
- remove `ManagedTransport`
- remove `ManagedOption`
- remove `NewManaged`

The final public creation API should contain only `Dial` and `Wrap`.

## Documentation Strategy

Documentation should teach one story only:

- use `Dial` for transport-owned outbound connections
- use `Wrap` for accepted or already-open connections

Avoid teaching `ManagedTransport` as a peer concept once the redesign begins.

## Testing Strategy

Tests should lock the unified design, not the transitional types.

Required coverage:

- public API shape for `Transport`, `Dial`, and `Wrap`
- `Dial` reconnects on later operations after failure
- `Wrap` does not reconnect after failure
- failed operations are not transparently replayed
- `ctx` is honored across connect and I/O paths
- idle baseline uses ready time before first successful read
- idle resets after a successful read
- `Close` reliably unblocks connect and waiter paths

## Non-Goals

- transparent replay of failed operations
- callback-heavy event API in the base transport package
- built-in heartbeat strategy
- built-in alarms, logging sinks, or business policy
- making server accepted connections appear reconnectable

## Recommendation

The package should evolve toward a single public `Transport` model with `Dial` and `Wrap` constructors.

This design matches the real requirement shift, reduces user choice overload, and keeps `gtransport` small enough to remain a foundation component rather than becoming a transport framework.
