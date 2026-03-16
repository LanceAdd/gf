# gtransport Modbus File Split Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Split `net/gtransport/modbus/modbus.go` into smaller files by responsibility without changing API or behavior.

**Architecture:** Keep the package flat and keep tests unchanged. Move public constructors into `modbus.go`, TCP codec logic into `modbus_tcp.go`, RTU codec and CRC helpers into `modbus_rtu.go`, and shared payload validation into `modbus_validation.go`.

**Tech Stack:** Go, `gtransport.Codec`, Modbus TCP, Modbus RTU

---

## Chunk 1: Pure Refactor

### Task 1: Split the single Modbus source file by responsibility

**Files:**
- Modify: `net/gtransport/modbus/modbus.go`
- Create: `net/gtransport/modbus/modbus_tcp.go`
- Create: `net/gtransport/modbus/modbus_rtu.go`
- Create: `net/gtransport/modbus/modbus_validation.go`
- Test: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Use existing tests as the safety net**
- [ ] **Step 2: Run `go test ./modbus -count=1` and confirm green baseline**
- [ ] **Step 3: Split code by file responsibility with no behavior changes**
- [ ] **Step 4: Run `go test ./modbus -count=1` and confirm green**
- [ ] **Step 5: Run package-level verification**

