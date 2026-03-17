package modbus

import (
	"bytes"
	"testing"

	"github.com/gogf/gf/v2/net/gtransport"
)

func TestTCPDecodeNeedMoreData(t *testing.T) {
	_, _, err := NewTCP().Decode([]byte{0x00, 0x01, 0x00})
	if err != gtransport.ErrNeedMoreData {
		t.Fatalf("expected ErrNeedMoreData, got %v", err)
	}
}

func TestTCPDecodeFullADU(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	got, consumed, err := NewTCP().Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, frame) {
		t.Fatalf("expected %x, got %x", frame, got)
	}
}

func TestTCPDecodePreservesPartialHeaderCandidate(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01}
	got, consumed, err := NewTCP().Decode(frame)
	if err != gtransport.ErrNeedMoreData {
		t.Fatalf("expected ErrNeedMoreData, got %v", err)
	}
	if consumed != 0 {
		t.Fatalf("expected consumed=0, got %d", consumed)
	}
	if got != nil {
		t.Fatalf("expected nil frame, got %x", got)
	}
}

func TestTCPDecodeResynchronizesPastInvalidProtocolIDNoise(t *testing.T) {
	stream := []byte{
		0x00, 0x01, 0x00, 0x01, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A,
		0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A,
	}
	want := []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}

	got, consumed, err := NewTCP().Decode(stream)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(stream) {
		t.Fatalf("expected consumed=%d, got %d", len(stream), consumed)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestTCPDecodeAllowsReadHoldingRegistersAtAddressBoundary(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0xFF, 0xFF, 0x00, 0x01}
	got, consumed, err := NewTCP().Decode(frame)
	if err != nil {
		t.Fatalf("expected boundary request to be valid, got %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, frame) {
		t.Fatalf("expected %x, got %x", frame, got)
	}
}

func TestTCPDecodeExceptionResponse(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x03, 0x01, 0x83, 0x02}
	got, consumed, err := NewTCP().Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, frame) {
		t.Fatalf("expected %x, got %x", frame, got)
	}
}

func TestTCPDecodeResynchronizesPastMalformedExceptionNoise(t *testing.T) {
	stream := []byte{
		0x00, 0x01, 0x00, 0x00, 0x00, 0x04, 0x01, 0x83, 0x02, 0x00,
		0x00, 0x02, 0x00, 0x00, 0x00, 0x03, 0x01, 0x83, 0x02,
	}
	want := []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x03, 0x01, 0x83, 0x02}

	got, consumed, err := NewTCP().Decode(stream)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(stream) {
		t.Fatalf("expected consumed=%d, got %d", len(stream), consumed)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestTCPDecodeResynchronizesPastInvalidPayloadNoise(t *testing.T) {
	stream := []byte{
		0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x7E,
		0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A,
	}
	want := []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}

	got, consumed, err := NewTCP().Decode(stream)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(stream) {
		t.Fatalf("expected consumed=%d, got %d", len(stream), consumed)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestTCPEncodeRejectsWriteMultipleRegistersAddressRangeOverflow(t *testing.T) {
	_, err := NewTCP().Encode([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x10, 0xFF, 0xFE, 0x00, 0x03, 0x06, 0x00, 0x0A, 0x00, 0x0B, 0x00, 0x0C})
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestTCPEncodeRewritesLength(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x12, 0x34, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	got, err := NewTCP().Encode(frame)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	want := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestTCPEncodeRejectsLengthTooSmallFrame(t *testing.T) {
	_, err := NewTCP().Encode([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01})
	if err == nil {
		t.Fatal("expected length error")
	}
}

func TestTCPEncodeRejectsProtocolID(t *testing.T) {
	_, err := NewTCP().Encode([]byte{0x00, 0x01, 0x00, 0x01, 0x00, 0x00, 0x01, 0x03})
	if err == nil {
		t.Fatal("expected protocol id error")
	}
}

func TestTCPEncodeRejectsLengthTooLargeFrame(t *testing.T) {
	frame := make([]byte, 6+255)
	frame[6] = 0x01
	frame[7] = 0x03
	_, err := NewTCP().Encode(frame)
	if err == nil {
		t.Fatal("expected length error")
	}
}

func TestTCPEncodeRejectsWriteSingleCoilInvalidValue(t *testing.T) {
	_, err := NewTCP().Encode([]byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x05, 0x00, 0xAC, 0x12, 0x34})
	if err == nil {
		t.Fatal("expected coil value error")
	}
}

func TestTCPEncodeWriteSingleRegister(t *testing.T) {
	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x01, 0x06, 0x00, 0x64, 0x12, 0x34}
	got, err := NewTCP().Encode(frame)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	want := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x06, 0x00, 0x64, 0x12, 0x34}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}
