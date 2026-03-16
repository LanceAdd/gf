# gtransport Modbus Executor Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a Modbus executor that runs typed requests against `ProcessImage`, emits typed responses, and normalizes package naming from `UnitID` to `SlaveID`.

**Architecture:** First rename metadata consistently so request, response, and executor code share one address-field term. Then add a thin `ExecuteRequest` dispatcher that maps the 8 supported function codes onto `ProcessImage`, returning normal responses on success and `ExceptionResponse` for protocol-level execution failures.

**Tech Stack:** Go, `go test`, `net/gtransport/modbus`, typed request/response models, memory-backed process image

---

## Chunk 1: Naming Normalization

### Task 1: Rename `UnitID` metadata to `SlaveID`

**Files:**
- Modify: `net/gtransport/modbus/modbus_req.go`
- Modify: `net/gtransport/modbus/modbus_resp.go`
- Modify: `net/gtransport/modbus/modbus_req_test.go`
- Modify: `net/gtransport/modbus/modbus_resp_test.go`
- Modify: `net/gtransport/modbus/modbus_test.go`
- Search: `net/gtransport/modbus`
- Test: `net/gtransport/modbus/modbus_req_test.go`
- Test: `net/gtransport/modbus/modbus_resp_test.go`
- Test: `net/gtransport/modbus/modbus_test.go`

- [ ] **Step 1: Write the failing tests**

Adjust tests so they assert:

- `ADUMeta.SlaveID`
- no remaining `UnitID` field access in request/response tests

- [ ] **Step 2: Search for remaining metadata references**

Run: `rg -n "UnitID|SlaveID" net/gtransport/modbus`
Expected: identify every code, test, and doc reference that must be updated in this implementation batch

- [ ] **Step 3: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Parse|Encode|PublicAPIShape)' -count=1`
Expected: FAIL with unknown field `UnitID` or missing field `SlaveID`

- [ ] **Step 4: Write the minimal implementation**

Rename:

- `ADUMeta.UnitID` -> `ADUMeta.SlaveID`

Update all direct metadata usage in:

- request parsing
- response encoding
- tests

- [ ] **Step 5: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'Test(Parse|Encode|PublicAPIShape)' -count=1`
Expected: PASS

- [ ] **Step 6: Run wider package verification**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS, proving the public metadata rename did not leave compile breaks inside the module

- [ ] **Step 7: Commit**

```bash
git add net/gtransport/modbus/modbus_req.go net/gtransport/modbus/modbus_resp.go net/gtransport/modbus/modbus_req_test.go net/gtransport/modbus/modbus_resp_test.go net/gtransport/modbus/modbus_test.go
git commit -m "refactor: rename modbus metadata to slave id"
```

## Chunk 2: Executor Happy Path

### Task 2: Add executor for supported read requests

**Files:**
- Create: `net/gtransport/modbus/modbus_executor.go`
- Create: `net/gtransport/modbus/modbus_executor_test.go`
- Test: `net/gtransport/modbus/modbus_executor_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `ReadCoilsRequest` -> `ReadBitsResponse`
- `ReadDiscreteInputsRequest` -> `ReadBitsResponse`
- `ReadHoldingRegistersRequest` -> `ReadRegistersResponse`
- `ReadInputRegistersRequest` -> `ReadRegistersResponse`
- metadata preservation for `Transport`, `TransactionID`, and `SlaveID`
- unsupported request function -> `ExceptionResponse, nil` with exception code `0x01`
- `nil req` or `nil image` caller-precondition violations -> Go `error`

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestExecuteRequest(Read|Metadata)' -count=1`
Expected: FAIL with missing `ExecuteRequest` or executor response logic

- [ ] **Step 3: Write the minimal implementation**

Implement:

- `ExecuteRequest(req Request, image ProcessImage) (Response, error)`
- read-request dispatch branches
- response constructors for read responses

Behavior:

- execute only the 4 read requests in this task
- preserve request metadata in the response
- return `ExceptionResponse, nil` with exception code `0x01` for unsupported request functions
- reserve Go `error` for nil request or internal impossible states
- treat `nil image` as caller misuse and return Go `error`

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestExecuteRequest(Read|Metadata)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_executor.go net/gtransport/modbus/modbus_executor_test.go
git commit -m "feat: add modbus executor read dispatch"
```

### Task 3: Add executor for supported write requests

**Files:**
- Modify: `net/gtransport/modbus/modbus_executor.go`
- Modify: `net/gtransport/modbus/modbus_executor_test.go`
- Test: `net/gtransport/modbus/modbus_executor_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `WriteSingleCoilRequest` success
- `WriteSingleRegisterRequest` success
- `WriteMultipleCoilsRequest` success
- `WriteMultipleRegistersRequest` success
- write responses echo correct address or quantity
- write responses preserve `Transport`, `TransactionID`, and `SlaveID`
- `MemoryProcessImage` state changes as expected

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestExecuteRequestWrite' -count=1`
Expected: FAIL with missing write-request dispatch

- [ ] **Step 3: Write the minimal implementation**

Add write dispatch branches that:

- call the matching `ProcessImage` method
- build the matching typed write response
- use `len(values)` to derive response quantity for multiple writes

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestExecuteRequestWrite' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_executor.go net/gtransport/modbus/modbus_executor_test.go
git commit -m "feat: add modbus executor write dispatch"
```

## Chunk 3: Exception Mapping and Final Verification

### Task 4: Add exception mapping for execution failures

**Files:**
- Modify: `net/gtransport/modbus/modbus_executor.go`
- Modify: `net/gtransport/modbus/modbus_executor_test.go`
- Test: `net/gtransport/modbus/modbus_executor_test.go`

- [ ] **Step 1: Write the failing tests**

Add tests for:

- `ErrProcessImageAddressOutOfRange` -> exception code `0x02`
- `ErrProcessImageQuantityOutOfRange` -> exception code `0x03`
- `ErrProcessImageWriteValuesEmpty` -> exception code `0x03`
- unsupported request implementation -> Go `error`
- `SlaveID == 0` still returns `Response, nil` rather than suppressing reply

- [ ] **Step 2: Run the focused tests to verify they fail**

Run: `cd net/gtransport && go test ./modbus -run 'TestExecuteRequest(Exception|InternalError)' -count=1`
Expected: FAIL with missing exception mapping

- [ ] **Step 3: Write the minimal implementation**

Implement:

- internal helper to build `ExceptionResponse`
- error mapping helper from `ProcessImage` errors to exception codes

Rules:

- only recognized process-image errors map to `ExceptionResponse`
- unrecognized errors remain Go `error`
- executor still does not implement broadcast suppression

- [ ] **Step 4: Run the focused tests to verify they pass**

Run: `cd net/gtransport && go test ./modbus -run 'TestExecuteRequest(Exception|InternalError)' -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus/modbus_executor.go net/gtransport/modbus/modbus_executor_test.go
git commit -m "feat: map modbus executor failures to exceptions"
```

### Task 5: Full verification and scope review

**Files:**
- Modify: `net/gtransport/modbus/modbus_test.go`
- Test: `net/gtransport/modbus/modbus_executor_test.go`
- Test: `net/gtransport/modbus/modbus_req_test.go`
- Test: `net/gtransport/modbus/modbus_resp_test.go`
- Test: `net/gtransport/modbus/process_image_z_unit_test.go`

- [ ] **Step 1: Add any missing API shape assertions**

Ensure package-level API tests reference:

- `ExecuteRequest`
- `ADUMeta.SlaveID`

- [ ] **Step 2: Run the full modbus package tests**

Run: `cd net/gtransport && go test ./modbus -count=1`
Expected: PASS

- [ ] **Step 3: Run the full gtransport module tests**

Run: `cd net/gtransport && go test ./... -count=1`
Expected: PASS

- [ ] **Step 4: Review scope**

Confirm:

- codec stayed stateless
- executor did not add broadcast suppression
- `SlaveID` naming is consistent across request, response, and executor code
- executor only uses `ProcessImage`, not transport I/O

- [ ] **Step 5: Commit**

```bash
git add net/gtransport/modbus
git commit -m "test: verify modbus executor integration"
```
