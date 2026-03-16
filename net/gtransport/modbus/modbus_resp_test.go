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
