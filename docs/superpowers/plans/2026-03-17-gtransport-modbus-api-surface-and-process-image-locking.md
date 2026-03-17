# Modbus API Surface And ProcessImage Locking Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Re-center `net/gtransport/modbus` on its composable API flow and make `MemoryProcessImage` safe for concurrent read-heavy use.

**Architecture:** Keep the existing helper functions for compatibility but demote them in docs and function comments so the primary path becomes `Parse* -> ExecuteRequest -> Encode*`. Add a `sync.RWMutex` to `MemoryProcessImage` so the default in-memory implementation tolerates concurrent reads and writes without changing the `ProcessImage` interface.

**Tech Stack:** Go 1.23+, `net/gtransport/modbus`, Go `testing`, race detector

---

## Chunk 1: Concurrent-safe MemoryProcessImage

### Task 1: Add a failing concurrent test

**Files:**
- Modify: `net/gtransport/modbus/process_image_z_unit_test.go`

- [ ] **Step 1: Write the failing test**
Add `TestMemoryProcessImageConcurrentReadWrite` that runs readers and writers against one shared image.

- [ ] **Step 2: Run race test to verify it fails**
Run: `cd net/gtransport && go test -race ./modbus -run TestMemoryProcessImageConcurrentReadWrite -count=1`
Expected: FAIL with a race report before locks are added.

### Task 2: Add RWMutex protection

**Files:**
- Modify: `net/gtransport/modbus/process_image_memory.go`

- [ ] **Step 3: Write minimal implementation**
Add `sync.RWMutex`; guard all reads with `RLock` and writes with `Lock`.

- [ ] **Step 4: Run race test to verify it passes**
Run: `cd net/gtransport && go test -race ./modbus -run TestMemoryProcessImageConcurrentReadWrite -count=1`
Expected: PASS.

## Chunk 2: API surface guidance

### Task 3: Demote helper APIs in docs

**Files:**
- Modify: `net/gtransport/modbus/handle_request.go`
- Modify: `net/gtransport/modbus/modbus.go`
- Modify: `net/gtransport/README.md`
- Modify: `net/gtransport/README.zh-CN.md`

- [ ] **Step 5: Update comments and docs**
Mark `Handle*` as deprecated compatibility helpers and move composable APIs to the front of the package/docs narrative.

- [ ] **Step 6: Run package regression**
Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS.
