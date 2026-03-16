# gtransport Modbus Examples Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add two runnable Modbus TCP examples that separately demonstrate the recommended API path and the advanced API path.

**Architecture:** Keep both examples on the same simple TCP read-holding-register scenario so the only meaningful difference is API usage. The recommended example should use the one-call frame handler path; the advanced example should explicitly parse, execute, and encode while inserting a visible custom step.

**Tech Stack:** Go, `go build`, `go test`, `net/gtransport`, `net/gtransport/modbus`, local example modules

---

## File Structure

- Create: `net/gtransport/example/modbus_recommended/go.mod`
- Create: `net/gtransport/example/modbus_recommended/main.go`
- Create: `net/gtransport/example/modbus_advanced/go.mod`
- Create: `net/gtransport/example/modbus_advanced/main.go`
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`
- Reference: `docs/superpowers/specs/2026-03-16-gtransport-modbus-examples-design.md`
- Reference: `net/gtransport/example/tcp_echo/main.go`
- Reference: `net/gtransport/modbus/handle_request.go`
- Reference: `net/gtransport/modbus/modbus_req.go`
- Reference: `net/gtransport/modbus/modbus_executor.go`
- Reference: `net/gtransport/modbus/modbus_resp.go`

## Chunk 1: Recommended Example

### Task 1: Add runnable recommended Modbus example

**Files:**
- Create: `net/gtransport/example/modbus_recommended/go.mod`
- Create: `net/gtransport/example/modbus_recommended/main.go`

- [ ] **Step 1: Write the failing verification**

Try to build the new example path after creating only the module skeleton.

Run: `cd net/gtransport/example/modbus_recommended && go build ./...`
Expected: FAIL until `main.go` exists and uses the package correctly

- [ ] **Step 2: Write the minimal example**

Implement a minimal local TCP example that:

- starts a listener
- creates a Modbus TCP transport
- preloads one holding register in `MemoryProcessImage`
- reads one request frame
- handles it using `HandleTCPRequestFrame`
- writes one response frame
- has a client send one `FC03` request and print the result

- [ ] **Step 3: Build the example**

Run: `cd net/gtransport/example/modbus_recommended && go build ./...`
Expected: PASS

- [ ] **Step 4: Run package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/example/modbus_recommended/go.mod net/gtransport/example/modbus_recommended/main.go
git commit -m "feat: add recommended modbus example"
```

## Chunk 2: Advanced Example

### Task 2: Add runnable advanced Modbus example

**Files:**
- Create: `net/gtransport/example/modbus_advanced/go.mod`
- Create: `net/gtransport/example/modbus_advanced/main.go`

- [ ] **Step 1: Write the failing verification**

Try to build the new example path after creating only the module skeleton.

Run: `cd net/gtransport/example/modbus_advanced && go build ./...`
Expected: FAIL until `main.go` exists and uses the package correctly

- [ ] **Step 2: Write the minimal example**

Implement a minimal local TCP example that:

- starts a listener
- creates a Modbus TCP transport
- preloads one holding register in `MemoryProcessImage`
- reads one request frame
- parses it using `ParseTCPRequest`
- performs one visible custom step on the typed request
- executes with `ExecuteRequest`
- encodes with `EncodeTCPResponse`
- writes one response frame
- has a client send one `FC03` request and print the result

- [ ] **Step 3: Build the example**

Run: `cd net/gtransport/example/modbus_advanced && go build ./...`
Expected: PASS

- [ ] **Step 4: Run package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/example/modbus_advanced/go.mod net/gtransport/example/modbus_advanced/main.go
git commit -m "feat: add advanced modbus example"
```

## Chunk 3: Documentation Index

### Task 3: Link the new examples from package docs

**Files:**
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 1: Write the failing doc checklist**

Checklist:

- README mentions the recommended example
- README mentions the advanced example
- wording matches the API layering spec

- [ ] **Step 2: Update the README files**

Add short references to:

- `example/modbus_recommended`
- `example/modbus_advanced`

Explain that one is the default high-level path and the other is the low-level composable path.

- [ ] **Step 3: Run module verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add net/gtransport/README.md net/gtransport/README.zh-CN.md
git commit -m "docs: link modbus api examples"
```
