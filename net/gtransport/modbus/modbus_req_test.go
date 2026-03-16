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
