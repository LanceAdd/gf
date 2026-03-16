package modbus

import (
	"bytes"
	"testing"

	"github.com/gogf/gf/v2/net/gtransport"
)

func TestRTUDecodeStripsCRC(t *testing.T) {
	frameWithCRC := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0xC5, 0xCD}
	want := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	got, consumed, err := NewRTU().Decode(frameWithCRC)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frameWithCRC) {
		t.Fatalf("expected consumed=%d, got %d", len(frameWithCRC), consumed)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestRTUDecodeReadResponse(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x04, 0x00, 0x01, 0x00, 0x02}
	frame := appendCRC(payload)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeReadResponsePartialNeedsMoreData(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x04, 0x00, 0x01, 0x00, 0x02}
	frame := appendCRC(payload)
	_, _, err := NewRTU().Decode(frame[:8])
	if err != gtransport.ErrNeedMoreData {
		t.Fatalf("expected ErrNeedMoreData, got %v", err)
	}
}

func TestRTUDecodeWriteMultipleRegistersResponse(t *testing.T) {
	payload := []byte{0x01, 0x10, 0x00, 0x00, 0x00, 0x02}
	frame := appendCRC(payload)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeRejectsReadCoilsResponseEmptyByteCount(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x01, 0x00})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUDecodeRejectsReadHoldingRegistersQuantityOutOfRange(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x7E})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUDecodeAllowsReadHoldingRegistersAtAddressBoundary(t *testing.T) {
	payload := []byte{0x01, 0x03, 0xFF, 0xFF, 0x00, 0x01}
	frame := appendCRC(payload)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("expected boundary request to be valid, got %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeRejectsReadHoldingRegistersAddressRangeOverflow(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x03, 0xFF, 0xFF, 0x00, 0x02})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestRTUDecodeRejectsReadHoldingRegistersResponseOddByteCount(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x03, 0x03, 0x00, 0x01, 0x00})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUDecodeRejectsWriteSingleCoilInvalidValue(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x05, 0x00, 0xAC, 0x12, 0x34})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected coil value error")
	}
}

func TestRTUDecodeExceptionResponse(t *testing.T) {
	payload := []byte{0x01, 0x83, 0x02}
	frame := appendCRC(payload)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeRejectsUnsupportedExceptionFunction(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x91, 0x01})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected unsupported function error")
	}
}

func TestRTUDecodeRejectsReadDiscreteInputsAddressRangeOverflow(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x02, 0xFF, 0xFF, 0x00, 0x02})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestRTUDecodeRejectsReadInputRegistersResponseOddByteCount(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x04, 0x03, 0x00, 0x01, 0x00})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUDecodeRejectsWriteMultipleCoilsQuantityOutOfRange(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x0F, 0x00, 0x13, 0x00, 0x00, 0x00})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUEncodeRejectsWriteMultipleCoilsAddressRangeOverflow(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x0F, 0xFF, 0xF8, 0x00, 0x10, 0x02, 0xCD, 0x01})
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestRTUDecodeRejectsWriteMultipleCoilsByteCountMismatch(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x0F, 0x00, 0x13, 0x00, 0x0A, 0x01, 0xCD})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUDecodeRejectsWriteMultipleRegistersQuantityOutOfRange(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUDecodeRejectsWriteMultipleRegistersByteCountMismatch(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x10, 0x00, 0x01, 0x00, 0x02, 0x06, 0x00, 0x0A, 0x01, 0x02, 0x03, 0x04})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUDecodeRejectsWriteMultipleRegistersResponseQuantityOutOfRange(t *testing.T) {
	frame := appendCRC([]byte{0x01, 0x10, 0x00, 0x00, 0x00, 0x7C})
	_, _, err := NewRTU().Decode(frame)
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUDecodeInvalidCRC(t *testing.T) {
	_, _, err := NewRTU().Decode([]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected invalid CRC error")
	}
	if err == gtransport.ErrNeedMoreData {
		t.Fatal("expected invalid CRC error, got ErrNeedMoreData")
	}
}

func TestRTUDecodeUnsupportedFunction(t *testing.T) {
	_, _, err := NewRTU().Decode([]byte{0x01, 0x11, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected unsupported function error")
	}
	if err == gtransport.ErrNeedMoreData {
		t.Fatal("expected unsupported function error, got ErrNeedMoreData")
	}
}

func TestRTUDecodeSkipsNoisePrefixToValidFrame(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	frame := append([]byte{0x99, 0x88}, appendCRC(payload)...)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeSkipsUnsupportedFunctionPrefixToValidFrame(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	frame := append([]byte{0x01, 0x11}, appendCRC(payload)...)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeSkipsInvalidCRCPrefixToValidFrame(t *testing.T) {
	bad := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0x00, 0x00}
	payload := []byte{0x01, 0x03, 0x00, 0x01, 0x00, 0x01}
	frame := append(bad, appendCRC(payload)...)
	got, consumed, err := NewRTU().Decode(frame)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestRTUDecodeReturnsNeedMoreDataForRecoverablePartialFrame(t *testing.T) {
	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	frame := append([]byte{0x99}, appendCRC(payload)...)
	_, _, err := NewRTU().Decode(frame[:len(frame)-1])
	if err != gtransport.ErrNeedMoreData {
		t.Fatalf("expected ErrNeedMoreData, got %v", err)
	}
}

func TestRTUEncodeAppendsCRC(t *testing.T) {
	frame := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A}
	want := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x0A, 0xC5, 0xCD}
	got, err := NewRTU().Encode(frame)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestRTUEncodeUnsupportedFunction(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x11, 0x00})
	if err == nil {
		t.Fatal("expected unsupported function error")
	}
}

func TestRTUEncodeRejectsMalformedExceptionLength(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x83, 0x02, 0x00})
	if err == nil {
		t.Fatal("expected exception length error")
	}
}

func TestRTUEncodeRejectsReadCoilsResponseEmptyByteCount(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x01, 0x00})
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUEncodeRejectsMalformedReadRequest(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x03, 0x00, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected frame length error")
	}
}

func TestRTUEncodeRejectsReadHoldingRegistersQuantityOutOfRange(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x7E})
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUEncodeRejectsReadHoldingRegistersResponseOddByteCount(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x03, 0x03, 0x00, 0x01, 0x00})
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUEncodeRejectsReadInputRegistersAddressRangeOverflow(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x04, 0xFF, 0xFF, 0x00, 0x02})
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestRTUEncodeRejectsWriteSingleCoilInvalidValue(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x05, 0x00, 0xAC, 0x12, 0x34})
	if err == nil {
		t.Fatal("expected coil value error")
	}
}

func TestRTUEncodeWriteSingleRegister(t *testing.T) {
	frame := []byte{0x01, 0x06, 0x00, 0x64, 0x12, 0x34}
	want := appendCRC(frame)
	got, err := NewRTU().Encode(frame)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestRTUEncodeRejectsMalformedWriteMultipleRequest(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x10, 0x00, 0x00, 0x00, 0x02, 0x04})
	if err == nil {
		t.Fatal("expected frame length error")
	}
}

func TestRTUEncodeRejectsWriteMultipleCoilsQuantityOutOfRange(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x0F, 0x00, 0x13, 0x00, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUEncodeRejectsWriteMultipleCoilsByteCountMismatch(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x0F, 0x00, 0x13, 0x00, 0x0A, 0x01, 0xCD})
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUEncodeRejectsWriteMultipleRegistersQuantityOutOfRange(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00})
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUEncodeRejectsWriteMultipleRegistersAddressRangeOverflow(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x10, 0xFF, 0xFE, 0x00, 0x03, 0x06, 0x00, 0x0A, 0x00, 0x0B, 0x00, 0x0C})
	if err == nil {
		t.Fatal("expected address range error")
	}
}

func TestRTUEncodeRejectsWriteMultipleRegistersByteCountMismatch(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x10, 0x00, 0x01, 0x00, 0x02, 0x06, 0x00, 0x0A, 0x01, 0x02, 0x03, 0x04})
	if err == nil {
		t.Fatal("expected byte count error")
	}
}

func TestRTUEncodeRejectsWriteMultipleRegistersResponseQuantityOutOfRange(t *testing.T) {
	_, err := NewRTU().Encode([]byte{0x01, 0x10, 0x00, 0x00, 0x00, 0x7C})
	if err == nil {
		t.Fatal("expected quantity error")
	}
}

func TestRTUEncodeExceptionResponse(t *testing.T) {
	frame := []byte{0x01, 0x83, 0x02}
	want := appendCRC(frame)
	got, err := NewRTU().Encode(frame)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}
