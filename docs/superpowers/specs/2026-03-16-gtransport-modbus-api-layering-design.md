# gtransport Modbus API Layering Design

**Goal**

Define a clear public API layering strategy for `net/gtransport/modbus` so developers can use either a high-level standard execution path or a low-level composable path without mixing transport and protocol semantics.

## Scope

This design covers the public-facing API organization of the existing Modbus package:

- classify existing APIs into recommended and advanced layers
- define semantic rules for `TCP frame`, `RTU frame`, and `RTU payload`
- define documentation priorities for the package

This design does not add new protocol behavior. It only standardizes how the current APIs should be presented and explained.

## Problem

The package now exposes multiple valid ways to use Modbus:

- parse requests directly
- execute typed requests
- handle full request frames
- handle decoded RTU transport payloads
- access `ProcessImage` directly

That flexibility is useful, but without an explicit layering model it creates two risks:

- users do not know which API is the default path
- users confuse raw RTU ADUs with transport-decoded RTU payloads

The package should keep both low-level and high-level capabilities, but the public narrative must make the intended path obvious.

## Design Principle

The package should expose one coherent layered API, not two competing API families.

Rules:

- low-level APIs stay public
- high-level APIs are built by composing low-level APIs
- documentation leads with the high-level path
- semantic boundaries are explicit and never inferred

This keeps the package flexible for advanced users while giving ordinary users a stable default workflow.

## API Layers

### Layer 1: Codec and Model Layer

This is the advanced path for developers who want protocol building blocks.

Primary APIs:

```go
ParseTCPRequest(frame []byte) (Request, error)
ParseRTURequest(frame []byte) (Request, error)
EncodeTCPResponse(resp Response) ([]byte, error)
EncodeRTUResponse(resp Response) ([]byte, error)
```

Related types:

- `Request`
- `Response`
- `ADUMeta`
- typed request structs
- typed response structs

Use this layer when the developer wants to:

- inspect or route typed requests manually
- insert custom business logic between parse and response creation
- bypass `ExecuteRequest`
- construct typed responses directly

### Layer 2: Execution Layer

This is the middle layer for developers who want typed requests but standard Modbus execution semantics.

Primary API:

```go
ExecuteRequest(req Request, image ProcessImage) (Response, error)
```

Related types:

- `ProcessImage`
- `NewMemoryProcessImage(...)`

Use this layer when the developer wants to:

- receive typed requests from their own source
- delegate standard request execution to the package
- preserve standard metadata propagation and exception mapping

### Layer 3: Handling Layer

This is the recommended path for most developers.

Primary APIs:

```go
HandleTCPRequestFrame(frame []byte, image ProcessImage) ([]byte, error)
HandleRTURequestFrame(frame []byte, image ProcessImage) ([]byte, error)
HandleRTURequestPayload(payload []byte, image ProcessImage) ([]byte, error)
```

Use this layer when the developer wants to:

- process a single request in one call
- avoid manually stitching parse, execute, and encode
- keep standard protocol behavior by default

## Semantic Rules

These names must stay strict across code, docs, and examples.

### TCP Frame

`TCP frame` means a complete Modbus TCP ADU including MBAP header.

Applies to:

- `ParseTCPRequest`
- `HandleTCPRequestFrame`
- `EncodeTCPResponse`

### RTU Frame

`RTU frame` means a raw Modbus RTU ADU including CRC.

Applies to:

- `ParseRTURequest`
- `HandleRTURequestFrame`
- `EncodeRTUResponse`

### RTU Payload

`RTU payload` means the decoded RTU content seen by `gtransport.New(..., modbus.NewRTU())` after CRC validation and stripping.

Applies to:

- `HandleRTURequestPayload`

This is intentionally separate from raw RTU frame handling.

## Naming Rules

The package should not introduce ambiguous generic names such as:

- `HandleRequest`
- `ParseRequest`
- `EncodeResponse`

Every public API must make both transport and data shape clear.

Allowed patterns:

- `TCP + Frame`
- `RTU + Frame`
- `RTU + Payload`

Local variable naming should follow the same rule:

- raw ADUs use `frame`
- transport-decoded RTU content uses `payload`

## Recommended Public Narrative

Documentation should present the package in this order:

1. `Recommended APIs`
2. `Advanced APIs`
3. `Semantic Rules`
4. `Common Flows`

The recommended path should lead with:

- `HandleTCPRequestFrame`
- `HandleRTURequestFrame`
- `HandleRTURequestPayload`
- `ExecuteRequest`

The advanced path should document:

- parse APIs
- encode APIs
- typed request/response models
- direct `ProcessImage` usage

## Common Flows

### Recommended TCP Flow

```go
respFrame, err := modbus.HandleTCPRequestFrame(frame, image)
```

### Recommended RTU Raw Flow

```go
respFrame, err := modbus.HandleRTURequestFrame(frame, image)
```

### Recommended RTU Transport Flow

```go
payload, err := tr.ReadFrame()
respPayload, err := modbus.HandleRTURequestPayload(payload, image)
err = tr.WriteFrame(respPayload)
```

### Advanced Manual Flow

```go
req, err := modbus.ParseTCPRequest(frame)
resp, err := modbus.ExecuteRequest(req, image)
out, err := modbus.EncodeTCPResponse(resp)
```

## Compatibility Guidance

This design is additive in intent, but it changes package guidance:

- high-level APIs become the documented default path
- low-level APIs remain public and supported
- `HandleRTURequestPayload` is treated as a first-class API, not as an implementation detail

## Non-Goals

- changing current protocol behavior
- hiding low-level APIs
- introducing automatic protocol detection
- adding new transport abstractions
- adding server lifecycle APIs in this design
