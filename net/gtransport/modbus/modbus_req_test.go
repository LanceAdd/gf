package modbus

import (
	"strings"
	"testing"
)

func TestParseTCPRequestReadCoils(t *testing.T) {
	frame := []byte{0x12, 0x34, 0x00, 0x00, 0x00, 0x06, 0x11, 0x01, 0x00, 0x64, 0x00, 0x0A}

	req, err := ParseTCPRequest(frame)
	if err != nil {
		t.Fatalf("parse tcp request: %v", err)
	}

	readReq, ok := req.(ReadCoilsRequest)
	if !ok {
		t.Fatalf("expected ReadCoilsRequest, got %T", req)
	}
	if readReq.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x1234, UnitID: 0x11}) {
		t.Fatalf("unexpected meta: %+v", readReq.Meta())
	}
	if readReq.FunctionCode() != FunctionCode(0x01) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readReq.FunctionCode()))
	}
	if readReq.StartAddress != 0x0064 || readReq.Quantity != 0x000A {
		t.Fatalf("unexpected read coils request: %+v", readReq)
	}
}

func TestParseTCPRequestReadHoldingRegisters(t *testing.T) {
	frame := []byte{0xAB, 0xCD, 0x00, 0x00, 0x00, 0x06, 0x22, 0x03, 0x00, 0x10, 0x00, 0x02}

	req, err := ParseTCPRequest(frame)
	if err != nil {
		t.Fatalf("parse tcp request: %v", err)
	}

	readReq, ok := req.(ReadHoldingRegistersRequest)
	if !ok {
		t.Fatalf("expected ReadHoldingRegistersRequest, got %T", req)
	}
	if readReq.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0xABCD, UnitID: 0x22}) {
		t.Fatalf("unexpected meta: %+v", readReq.Meta())
	}
	if readReq.FunctionCode() != FunctionCode(0x03) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readReq.FunctionCode()))
	}
	if readReq.StartAddress != 0x0010 || readReq.Quantity != 0x0002 {
		t.Fatalf("unexpected read holding registers request: %+v", readReq)
	}
}

func TestParseRTURequestReadDiscreteInputs(t *testing.T) {
	frame := appendCRC([]byte{0x33, 0x02, 0x00, 0x20, 0x00, 0x08})

	req, err := ParseRTURequest(frame)
	if err != nil {
		t.Fatalf("parse rtu request: %v", err)
	}

	readReq, ok := req.(ReadDiscreteInputsRequest)
	if !ok {
		t.Fatalf("expected ReadDiscreteInputsRequest, got %T", req)
	}
	if readReq.Meta() != (ADUMeta{Transport: TransportRTU, UnitID: 0x33}) {
		t.Fatalf("unexpected meta: %+v", readReq.Meta())
	}
	if readReq.FunctionCode() != FunctionCode(0x02) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readReq.FunctionCode()))
	}
	if readReq.StartAddress != 0x0020 || readReq.Quantity != 0x0008 {
		t.Fatalf("unexpected read discrete inputs request: %+v", readReq)
	}
}

func TestParseRTURequestReadInputRegisters(t *testing.T) {
	frame := appendCRC([]byte{0x44, 0x04, 0x01, 0x00, 0x00, 0x03})

	req, err := ParseRTURequest(frame)
	if err != nil {
		t.Fatalf("parse rtu request: %v", err)
	}

	readReq, ok := req.(ReadInputRegistersRequest)
	if !ok {
		t.Fatalf("expected ReadInputRegistersRequest, got %T", req)
	}
	if readReq.Meta() != (ADUMeta{Transport: TransportRTU, UnitID: 0x44}) {
		t.Fatalf("unexpected meta: %+v", readReq.Meta())
	}
	if readReq.FunctionCode() != FunctionCode(0x04) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readReq.FunctionCode()))
	}
	if readReq.StartAddress != 0x0100 || readReq.Quantity != 0x0003 {
		t.Fatalf("unexpected read input registers request: %+v", readReq)
	}
}

func TestParseRTURequestRejectsMissingCRC(t *testing.T) {
	_, err := ParseRTURequest([]byte{0x44, 0x04, 0x01, 0x00, 0x00, 0x03})
	if err == nil {
		t.Fatal("expected missing crc error")
	}
	if !strings.Contains(err.Error(), "crc") {
		t.Fatalf("expected crc error, got %v", err)
	}
}

func TestParseRTURequestRejectsInvalidCRC(t *testing.T) {
	frame := appendCRC([]byte{0x44, 0x04, 0x01, 0x00, 0x00, 0x03})
	frame[len(frame)-1] ^= 0xFF

	_, err := ParseRTURequest(frame)
	if err == nil {
		t.Fatal("expected invalid crc error")
	}
	if !strings.Contains(err.Error(), "crc") {
		t.Fatalf("expected crc error, got %v", err)
	}
}

func TestParseTCPRequestWriteSingleCoil(t *testing.T) {
	frame := []byte{0x00, 0x09, 0x00, 0x00, 0x00, 0x06, 0x55, 0x05, 0x00, 0xAC, 0xFF, 0x00}

	req, err := ParseTCPRequest(frame)
	if err != nil {
		t.Fatalf("parse tcp request: %v", err)
	}

	writeReq, ok := req.(WriteSingleCoilRequest)
	if !ok {
		t.Fatalf("expected WriteSingleCoilRequest, got %T", req)
	}
	if writeReq.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x0009, UnitID: 0x55}) {
		t.Fatalf("unexpected meta: %+v", writeReq.Meta())
	}
	if writeReq.FunctionCode() != FunctionCode(0x05) {
		t.Fatalf("unexpected function code: 0x%02x", byte(writeReq.FunctionCode()))
	}
	if writeReq.Address != 0x00AC || !writeReq.Value {
		t.Fatalf("unexpected write single coil request: %+v", writeReq)
	}
}

func TestParseTCPRequestWriteSingleRegister(t *testing.T) {
	frame := []byte{0x00, 0x0A, 0x00, 0x00, 0x00, 0x06, 0x66, 0x06, 0x00, 0x64, 0x12, 0x34}

	req, err := ParseTCPRequest(frame)
	if err != nil {
		t.Fatalf("parse tcp request: %v", err)
	}

	writeReq, ok := req.(WriteSingleRegisterRequest)
	if !ok {
		t.Fatalf("expected WriteSingleRegisterRequest, got %T", req)
	}
	if writeReq.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x000A, UnitID: 0x66}) {
		t.Fatalf("unexpected meta: %+v", writeReq.Meta())
	}
	if writeReq.FunctionCode() != FunctionCode(0x06) {
		t.Fatalf("unexpected function code: 0x%02x", byte(writeReq.FunctionCode()))
	}
	if writeReq.Address != 0x0064 || writeReq.Value != 0x1234 {
		t.Fatalf("unexpected write single register request: %+v", writeReq)
	}
}

func TestParseRTURequestWriteMultipleCoils(t *testing.T) {
	frame := appendCRC([]byte{0x77, 0x0F, 0x00, 0x13, 0x00, 0x0A, 0x02, 0x4D, 0x01})

	req, err := ParseRTURequest(frame)
	if err != nil {
		t.Fatalf("parse rtu request: %v", err)
	}

	writeReq, ok := req.(WriteMultipleCoilsRequest)
	if !ok {
		t.Fatalf("expected WriteMultipleCoilsRequest, got %T", req)
	}
	if writeReq.Meta() != (ADUMeta{Transport: TransportRTU, UnitID: 0x77}) {
		t.Fatalf("unexpected meta: %+v", writeReq.Meta())
	}
	if writeReq.FunctionCode() != FunctionCode(0x0F) {
		t.Fatalf("unexpected function code: 0x%02x", byte(writeReq.FunctionCode()))
	}
	if writeReq.StartAddress != 0x0013 {
		t.Fatalf("unexpected start address: %d", writeReq.StartAddress)
	}
	want := []bool{true, false, true, true, false, false, true, false, true, false}
	if len(writeReq.Values) != len(want) {
		t.Fatalf("unexpected values length: %d", len(writeReq.Values))
	}
	for i, v := range want {
		if writeReq.Values[i] != v {
			t.Fatalf("unexpected coil value at %d: %v", i, writeReq.Values[i])
		}
	}
}

func TestParseRTURequestWriteMultipleRegisters(t *testing.T) {
	frame := appendCRC([]byte{0x88, 0x10, 0x00, 0x20, 0x00, 0x02, 0x04, 0x12, 0x34, 0x56, 0x78})

	req, err := ParseRTURequest(frame)
	if err != nil {
		t.Fatalf("parse rtu request: %v", err)
	}

	writeReq, ok := req.(WriteMultipleRegistersRequest)
	if !ok {
		t.Fatalf("expected WriteMultipleRegistersRequest, got %T", req)
	}
	if writeReq.Meta() != (ADUMeta{Transport: TransportRTU, UnitID: 0x88}) {
		t.Fatalf("unexpected meta: %+v", writeReq.Meta())
	}
	if writeReq.FunctionCode() != FunctionCode(0x10) {
		t.Fatalf("unexpected function code: 0x%02x", byte(writeReq.FunctionCode()))
	}
	if writeReq.StartAddress != 0x0020 {
		t.Fatalf("unexpected start address: %d", writeReq.StartAddress)
	}
	want := []uint16{0x1234, 0x5678}
	if len(writeReq.Values) != len(want) {
		t.Fatalf("unexpected values length: %d", len(writeReq.Values))
	}
	for i, v := range want {
		if writeReq.Values[i] != v {
			t.Fatalf("unexpected register value at %d: 0x%04x", i, writeReq.Values[i])
		}
	}
}

func TestParseTCPRequestRejectsWriteSingleCoilInvalidValue(t *testing.T) {
	frame := []byte{0x00, 0x0B, 0x00, 0x00, 0x00, 0x06, 0x55, 0x05, 0x00, 0xAC, 0x12, 0x34}

	_, err := ParseTCPRequest(frame)
	if err == nil {
		t.Fatal("expected invalid coil value error")
	}
}

func TestParseRTURequestRejectsWriteMultipleCoilsInvalidByteCount(t *testing.T) {
	frame := appendCRC([]byte{0x77, 0x0F, 0x00, 0x13, 0x00, 0x0A, 0x01, 0x4D})

	_, err := ParseRTURequest(frame)
	if err == nil {
		t.Fatal("expected invalid byte count error")
	}
}

func TestParseRTURequestRejectsWriteMultipleRegistersInvalidByteCount(t *testing.T) {
	frame := appendCRC([]byte{0x88, 0x10, 0x00, 0x20, 0x00, 0x02, 0x03, 0x12, 0x34, 0x56})

	_, err := ParseRTURequest(frame)
	if err == nil {
		t.Fatal("expected invalid byte count error")
	}
}

func TestParseTCPRequestRejectsWriteMultipleRegistersAddressRangeOverflow(t *testing.T) {
	frame := []byte{0x00, 0x0C, 0x00, 0x00, 0x00, 0x0D, 0x99, 0x10, 0xFF, 0xFE, 0x00, 0x03, 0x06, 0x00, 0x0A, 0x00, 0x0B, 0x00, 0x0C}

	_, err := ParseTCPRequest(frame)
	if err == nil {
		t.Fatal("expected address range error")
	}
}
