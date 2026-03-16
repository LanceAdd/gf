# gtransport Modbus Integration Test Design

**Goal**

Add transport-level integration tests that verify `gtransport.Transport` and `gtransport/modbus` codecs work correctly together on real stream behavior such as sticky packets, partial frames, and RTU byte-stream recovery.

## Scope

This work adds tests under `net/gtransport`.

The tests cover integration between:

- `gtransport.Transport`
- `modbus.NewTCP()`
- `modbus.NewRTU()`
- stream-style connections created with `net.Pipe()`

This design does not intentionally change production behavior.

## Problem

The current test suite strongly validates the Modbus codec package in isolation, but it does not yet directly protect the end-to-end behavior of:

- `Transport.ReadFrame`
- internal buffering across multiple reads
- codec `consumed` semantics on real stream input
- RTU resynchronization when invalid bytes arrive before a valid frame

That leaves a gap between codec-level correctness and transport-level behavior.

## Test Boundary

These tests validate integration behavior only. They do not add new protocol requirements.

They are not responsible for:

- Modbus business semantics
- Unit ID policy
- ProcessImage behavior
- concurrency stress testing
- real network reliability

## Transport Setup

Use `net.Pipe()` as the connection primitive.

Reasons:

- real `io.ReadWriteCloser` behavior
- deterministic in-memory transport
- no external network dependencies
- enough realism to exercise buffering and read/write sequencing

Each test should create a `Transport` with a Modbus codec and use goroutines where needed to drive the write side while the read side blocks in `ReadFrame`.

Use `WithReadTimeout(...)` to prevent hangs, but keep assertions focused on framing behavior rather than timeout mechanics.

## Required Test Cases

### TCP Sticky Frames

Write two complete Modbus TCP ADUs into the same stream back-to-back.

Expected behavior:

- first `ReadFrame()` returns the first ADU
- second `ReadFrame()` returns the second ADU
- no bytes are merged or lost

This verifies transport buffering plus codec `consumed` behavior for TCP.

### TCP Partial Frame Completion

Write only part of a Modbus TCP ADU, then later write the rest.

Expected behavior:

- `ReadFrame()` waits until the ADU is complete
- once the remaining bytes arrive, `ReadFrame()` returns the full ADU

This verifies partial-frame buffering at the transport layer.

### RTU Noise-Prefix Recovery

Write raw bytes containing:

- invalid prefix bytes
- then one valid RTU frame

Expected behavior:

- `ReadFrame()` returns the valid RTU payload
- invalid prefix bytes are effectively discarded through codec resynchronization

This verifies that RTU resynchronization works through the full transport stack, not only via direct codec calls.

### RTU Partial Frame Completion

Write part of a valid RTU frame, then later write the remainder including CRC.

Expected behavior:

- `ReadFrame()` waits for completion
- once complete bytes arrive, `ReadFrame()` returns the RTU payload without CRC

This verifies that partial RTU frames are buffered rather than misclassified as noise.

## Architecture

Add a dedicated transport-level test file in `net/gtransport`, separate from codec unit tests.

The tests should:

- create a pipe pair
- wrap one side with `Transport`
- write raw bytes or encoded frames into the other side
- assert on `ReadFrame()` results

For RTU noise-prefix recovery, write raw bytes directly to the pipe instead of using `WriteFrame()`, because the test must include intentionally invalid bytes ahead of a valid frame.

## Non-Goals

This design does not include:

- TCP exception-frame integration tests
- RTU multiple-frame recovery chains
- benchmark coverage
- race/load testing
- production code refactoring unless a new integration test exposes a real bug
