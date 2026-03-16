package modbus

import (
	"bytes"
	"testing"
)

func TestEncodeTCPResponseReadBits(t *testing.T) {
	resp := ReadBitsResponse{
		meta:     ADUMeta{Transport: TransportTCP, TransactionID: 0x0102, UnitID: 0x11},
		function: FunctionCode(0x01),
		Values:   []bool{true, false, true, true, false, false, true, false, true, false},
	}

	got, err := EncodeTCPResponse(resp)
	if err != nil {
		t.Fatalf("encode tcp response: %v", err)
	}

	want := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x05, 0x11, 0x01, 0x02, 0x4D, 0x01}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestEncodeTCPResponseReadRegisters(t *testing.T) {
	resp := ReadRegistersResponse{
		meta:     ADUMeta{Transport: TransportTCP, TransactionID: 0x0304, UnitID: 0x22},
		function: FunctionCode(0x03),
		Values:   []uint16{0x1234, 0x5678},
	}

	got, err := EncodeTCPResponse(resp)
	if err != nil {
		t.Fatalf("encode tcp response: %v", err)
	}

	want := []byte{0x03, 0x04, 0x00, 0x00, 0x00, 0x07, 0x22, 0x03, 0x04, 0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestEncodeRTUResponseWriteSingleRegister(t *testing.T) {
	resp := WriteSingleRegisterResponse{
		meta:    ADUMeta{Transport: TransportRTU, UnitID: 0x33},
		Address: 0x0064,
		Value:   0x1234,
	}

	got, err := EncodeRTUResponse(resp)
	if err != nil {
		t.Fatalf("encode rtu response: %v", err)
	}

	want := appendCRC([]byte{0x33, 0x06, 0x00, 0x64, 0x12, 0x34})
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestEncodeRTUResponseWriteMultipleRegisters(t *testing.T) {
	resp := WriteMultipleRegistersResponse{
		meta:         ADUMeta{Transport: TransportRTU, UnitID: 0x44},
		StartAddress: 0x0020,
		Quantity:     0x0002,
	}

	got, err := EncodeRTUResponse(resp)
	if err != nil {
		t.Fatalf("encode rtu response: %v", err)
	}

	want := appendCRC([]byte{0x44, 0x10, 0x00, 0x20, 0x00, 0x02})
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestEncodeTCPExceptionResponse(t *testing.T) {
	resp := ExceptionResponse{
		meta:          ADUMeta{Transport: TransportTCP, TransactionID: 0x0506, UnitID: 0x55},
		function:      FunctionCode(0x03),
		ExceptionCode: 0x02,
	}

	got, err := EncodeTCPResponse(resp)
	if err != nil {
		t.Fatalf("encode tcp exception response: %v", err)
	}

	want := []byte{0x05, 0x06, 0x00, 0x00, 0x00, 0x03, 0x55, 0x83, 0x02}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestEncodeRTUExceptionResponse(t *testing.T) {
	resp := ExceptionResponse{
		meta:          ADUMeta{Transport: TransportRTU, UnitID: 0x66},
		function:      FunctionCode(0x04),
		ExceptionCode: 0x03,
	}

	got, err := EncodeRTUResponse(resp)
	if err != nil {
		t.Fatalf("encode rtu exception response: %v", err)
	}

	want := appendCRC([]byte{0x66, 0x84, 0x03})
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestInvalidExceptionResponseRejectsUnsupportedBaseFunction(t *testing.T) {
	resp := ExceptionResponse{
		meta:          ADUMeta{Transport: TransportTCP, TransactionID: 0x0708, UnitID: 0x77},
		function:      FunctionCode(0x11),
		ExceptionCode: 0x01,
	}

	_, err := EncodeTCPResponse(resp)
	if err == nil {
		t.Fatal("expected unsupported function error")
	}
}

func TestInvalidReadBitsResponseRejectsEmptyValues(t *testing.T) {
	resp := ReadBitsResponse{
		meta:     ADUMeta{Transport: TransportTCP, TransactionID: 0x090A, UnitID: 0x88},
		function: FunctionCode(0x01),
	}

	_, err := EncodeTCPResponse(resp)
	if err == nil {
		t.Fatal("expected empty read bits response error")
	}
}

func TestInvalidReadRegistersResponseRejectsEmptyValues(t *testing.T) {
	resp := ReadRegistersResponse{
		meta:     ADUMeta{Transport: TransportTCP, TransactionID: 0x0B0C, UnitID: 0x99},
		function: FunctionCode(0x03),
	}

	_, err := EncodeTCPResponse(resp)
	if err == nil {
		t.Fatal("expected empty read registers response error")
	}
}
