# gtransport Modbus Examples Design

**Goal**

Add two runnable Modbus examples that show the difference between the recommended high-level API path and the advanced low-level composable API path.

## Scope

This design covers two example programs under `net/gtransport/example`:

- one example for the recommended Modbus API path
- one example for the advanced Modbus API path

Both examples are intentionally minimal and focus on API usage rather than protocol breadth.

## Problem

The package now has a clear API layering model, but documentation alone is not enough for users to internalize the difference between:

- a standard one-call request handler path
- a manual parse/execute/encode path

Runnable examples are the fastest way to make that distinction concrete.

## Design Principle

The two examples should differ in API path, not in scenario complexity.

Rules:

- same protocol family in both examples
- same basic request/response scenario
- different API usage path
- minimal surrounding code

This keeps the comparison clean.

## Chosen Approach

Use two separate TCP examples.

### Recommended Example

Directory:

- `net/gtransport/example/modbus_recommended`

Server-side flow:

```go
frame, err := tr.ReadFrame()
respFrame, err := modbus.HandleTCPRequestFrame(frame, image)
err = tr.WriteFrame(respFrame)
```

### Advanced Example

Directory:

- `net/gtransport/example/modbus_advanced`

Server-side flow:

```go
req, err := modbus.ParseTCPRequest(frame)
resp, err := modbus.ExecuteRequest(req, image)
out, err := modbus.EncodeTCPResponse(resp)
```

The advanced example should insert one visible custom step between parse and encode, such as logging the concrete request type or validating the request kind before execution.

## Why TCP First

Both examples should use Modbus TCP first because:

- the teaching goal is API layering, not RTU-specific transport semantics
- TCP avoids introducing RTU `frame` versus `payload` decisions into the first comparison
- the examples stay focused on the difference between high-level and low-level APIs

RTU examples can be added later as a separate follow-up if needed.

## Shared Scenario

Both examples should use the same simple scenario:

- start a local TCP listener
- create a `MemoryProcessImage`
- preload a holding register value
- client sends one `FC03` read holding registers request
- server returns one successful response
- client prints the value

This makes the API difference obvious because the protocol scenario does not change.

## File Layout

Add:

- `net/gtransport/example/modbus_recommended/go.mod`
- `net/gtransport/example/modbus_recommended/main.go`
- `net/gtransport/example/modbus_advanced/go.mod`
- `net/gtransport/example/modbus_advanced/main.go`

Potential doc follow-up:

- update `net/gtransport/README.md`
- update `net/gtransport/README.zh-CN.md`

## Example Responsibilities

### modbus_recommended

Demonstrates:

- `gtransport.New(..., modbus.NewTCP(), ...)`
- `modbus.HandleTCPRequestFrame`
- `MemoryProcessImage` as the backing state

Should communicate:

- transport handles framed I/O
- handler handles protocol processing
- process image provides the data

### modbus_advanced

Demonstrates:

- `modbus.ParseTCPRequest`
- optional custom logic on the typed request
- `modbus.ExecuteRequest`
- `modbus.EncodeTCPResponse`

Should communicate:

- developers can intercept typed requests
- developers can keep standard execution while still inserting custom policy or logging
- low-level APIs are first-class, not hidden escape hatches

## Non-Goals

- RTU example in this batch
- long-running production server abstraction
- multi-request concurrency example
- full register-map design example
- broadcast semantics

## Compatibility

This design is additive only. It adds runnable examples and does not change existing Modbus behavior or public APIs.
