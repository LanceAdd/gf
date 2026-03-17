# gtransport Modbus API Convergence Design

**Goal**

Realign `net/gtransport/modbus` around a composable, low-level-first API model that better fits a foundational library, while preserving existing high-level helpers for convenience and backward compatibility.

## Problem

The package currently presents what looks like two parallel API families:

- high-level request handlers such as `HandleTCPRequestFrame`
- low-level composable APIs such as `ParseTCPRequest`, `ExecuteRequest`, and `EncodeTCPResponse`

The implementation is already layered cleanly, but the public narrative still makes the helper layer look like the primary path. That is a mismatch for a foundational library, where developers usually need explicit, controllable, debuggable building blocks rather than a one-call wrapper.

The current shape also has an asymmetry:

- TCP and RTU frame flows expose both helper and low-level paths
- RTU payload flow exposes only the helper path

That makes the advanced/composable story incomplete for the exact transport-decoded RTU case that is easiest to confuse.

## Design Principle

`net/gtransport/modbus` should present one layered API, not two competing API families.

Rules:

- low-level composable APIs are the main public model
- execution stays explicit and inspectable
- helper APIs may remain, but as convenience wrappers only
- TCP frame, RTU frame, and RTU payload semantics must remain explicit and symmetric
- documentation and examples should teach composition first, convenience second

This keeps the package aligned with the needs of framework and systems developers while preserving ease of use for smaller integrations.

## Final API Layering

### Layer 1: Composable Protocol APIs

This is the primary public model.

Primary APIs:

```go
ParseTCPRequest(frame []byte) (Request, error)
ParseRTURequest(frame []byte) (Request, error)
EncodeTCPResponse(resp Response) ([]byte, error)
EncodeRTUResponse(resp Response) ([]byte, error)
```

Related public types:

- `Request`
- `Response`
- `ADUMeta`
- typed request structs
- typed response structs

Use this layer when the developer wants to:

- inspect requests before execution
- apply routing, authorization, filtering, logging, or observability
- construct custom responses directly
- control protocol flow explicitly

### Layer 2: Standard Execution Layer

This is the middle layer and remains a first-class public surface.

Primary APIs:

```go
ExecuteRequest(req Request, image ProcessImage) (Response, error)
```

Related public types:

- `ProcessImage`
- `NewMemoryProcessImage(...)`

Use this layer when the developer wants standard Modbus request execution semantics but still wants to own framing, parsing, or response construction boundaries.

### Layer 3: Convenience Helpers

This layer stays public for compatibility and small integrations, but it is no longer the default narrative.

Primary APIs:

```go
HandleTCPRequestFrame(frame []byte, image ProcessImage) ([]byte, error)
HandleRTURequestFrame(frame []byte, image ProcessImage) ([]byte, error)
HandleRTURequestPayload(payload []byte, image ProcessImage) ([]byte, error)
```

These helpers are thin wrappers over parse -> execute -> encode and should be documented as convenience helpers rather than recommended primary APIs.

## API Retention Strategy

### Keep and Promote as Primary

- `ParseTCPRequest`
- `ParseRTURequest`
- `EncodeTCPResponse`
- `EncodeRTUResponse`
- `ExecuteRequest`
- `ProcessImage`
- `NewMemoryProcessImage`

### Keep but Demote to Helper Status

- `HandleTCPRequestFrame`
- `HandleRTURequestFrame`
- `HandleRTURequestPayload`

These remain useful, but they should not define the package philosophy.

## Required Capability Gap Closure

If `RTU payload` remains a public semantic concept, it must also have a public low-level composable path.

Recommended additions:

```go
ParseRTURequestPayload(payload []byte) (Request, error)
EncodeRTUResponsePayload(resp Response) ([]byte, error)
```

That enables a fully composable RTU payload path:

```go
payload -> ParseRTURequestPayload -> ExecuteRequest -> EncodeRTUResponsePayload
```

Without that addition, the RTU payload path remains helper-only, which breaks the low-level-first model and leaves the transport-decoded RTU workflow asymmetric.

## Semantic Rules

These terms must remain strict across code, docs, and examples.

### TCP Frame

A `TCP frame` is a complete Modbus TCP ADU including the MBAP header.

### RTU Frame

An `RTU frame` is a raw Modbus RTU ADU including CRC.

### RTU Payload

An `RTU payload` is the CRC-stripped Modbus content returned by `gtransport.Wrap(..., modbus.NewRTU())` after transport decoding.

This semantic distinction is important enough that it must be represented consistently in both low-level and helper APIs.

## Request/Response Extensibility Position

Although `Request` and `Response` are interfaces, the package's current execution and encoding model is built around a closed set of built-in concrete types and internal type switching.

That means the package should document them as shared protocol model interfaces, not as an open-ended extension mechanism with guaranteed plug-in style extensibility.

Developers may still implement their own values where appropriate, but the package should not imply that arbitrary external request/response implementations are fully supported across all layers.

## Documentation Narrative

The package documentation should present APIs in this order:

1. `Composable APIs`
2. `Standard Execution Layer`
3. `Convenience Helpers`
4. semantic rules
5. common flows

Recommended documentation defaults:

- lead with `Parse -> Execute -> Encode`
- position `ExecuteRequest` as the standard execution step, not as a top-level helper
- describe `Handle*` as reduced-boilerplate wrappers
- give RTU payload semantics their own explicit subsection

## Example Strategy

Examples should reinforce the new narrative.

Recommended changes:

- make the current advanced example the primary example
- rename or describe it as the composable/default flow
- keep the current recommended helper example, but relabel it as helper/convenience flow
- add an RTU payload example showing transport-decoded payload composition

This is especially important because RTU payload is currently the easiest path to misunderstand.

## Migration Strategy

This convergence should be non-breaking in the first step.

Phase 1:

- keep all current public APIs
- change package comments and README narrative
- relabel helper APIs as convenience helpers
- add missing RTU payload low-level APIs
- update examples and tests to teach the composable path first

Phase 2, only if later justified:

- evaluate whether any helper APIs should move, deprecate, or remain permanently

No helper removal is required for the current convergence goal.

## Testing Expectations

The convergence work should preserve current behavior while clarifying API roles.

Tests should cover:

- existing helper APIs still behave the same
- new RTU payload low-level APIs match helper behavior for equivalent inputs
- composable flows and helper flows stay semantically aligned
- examples continue to compile

## Outcome

After this convergence:

- the package reads as a foundational composable library
- standard execution remains first-class but explicit
- helpers remain available without dominating the design
- TCP frame, RTU frame, and RTU payload flows become symmetric and easier to reason about
