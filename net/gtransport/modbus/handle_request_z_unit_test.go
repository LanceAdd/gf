package modbus

import (
	"bytes"
	"testing"
)

func TestHandleTCPRequestFrameReadHoldingRegisters(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	_ = image.WriteMultipleRegisters(0, []uint16{0x1234, 0x5678})

	reqFrame := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x06, 0x11, 0x03, 0x00, 0x00, 0x00, 0x02}

	got, err := HandleTCPRequestFrame(reqFrame, image)
	if err != nil {
		t.Fatalf("handle tcp request frame: %v", err)
	}

	want := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x07, 0x11, 0x03, 0x04, 0x12, 0x34, 0x56, 0x78}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestHandleTCPRequestFrameWriteSingleRegister(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)

	reqFrame := []byte{0x03, 0x04, 0x00, 0x00, 0x00, 0x06, 0x22, 0x06, 0x00, 0x01, 0x9A, 0xBC}

	got, err := HandleTCPRequestFrame(reqFrame, image)
	if err != nil {
		t.Fatalf("handle tcp request frame: %v", err)
	}

	want := []byte{0x03, 0x04, 0x00, 0x00, 0x00, 0x06, 0x22, 0x06, 0x00, 0x01, 0x9A, 0xBC}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
	if image.holdingRegisters[1] != 0x9ABC {
		t.Fatalf("unexpected holding register value: 0x%04x", image.holdingRegisters[1])
	}
}

func TestHandleRTURequestFrameReadDiscreteInputs(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	image.discreteInputs[0] = true
	image.discreteInputs[2] = true
	image.discreteInputs[3] = true

	reqFrame := appendCRC([]byte{0x33, 0x02, 0x00, 0x00, 0x00, 0x04})

	got, err := HandleRTURequestFrame(reqFrame, image)
	if err != nil {
		t.Fatalf("handle rtu request frame: %v", err)
	}

	want := appendCRC([]byte{0x33, 0x02, 0x01, 0x0D})
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestHandleRTURequestFrameWriteMultipleCoils(t *testing.T) {
	image := NewMemoryProcessImage(6, 4, 4, 4)

	reqFrame := appendCRC([]byte{0x44, 0x0F, 0x00, 0x01, 0x00, 0x03, 0x01, 0x05})

	got, err := HandleRTURequestFrame(reqFrame, image)
	if err != nil {
		t.Fatalf("handle rtu request frame: %v", err)
	}

	want := appendCRC([]byte{0x44, 0x0F, 0x00, 0x01, 0x00, 0x03})
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
	wantValues := []bool{true, false, true}
	for i, value := range wantValues {
		if image.coils[1+i] != value {
			t.Fatalf("unexpected coil value at %d: %v", 1+i, image.coils[1+i])
		}
	}
}

func TestHandleTCPRequestFrameExceptionAddressOutOfRange(t *testing.T) {
	image := NewMemoryProcessImage(2, 2, 1, 2)
	reqFrame := []byte{0x10, 0x20, 0x00, 0x00, 0x00, 0x06, 0x55, 0x03, 0x00, 0x00, 0x00, 0x02}

	got, err := HandleTCPRequestFrame(reqFrame, image)
	if err != nil {
		t.Fatalf("handle tcp request frame: %v", err)
	}

	want := []byte{0x10, 0x20, 0x00, 0x00, 0x00, 0x03, 0x55, 0x83, 0x02}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestHandleRTURequestFrameExceptionAddressOutOfRange(t *testing.T) {
	image := NewMemoryProcessImage(1, 1, 1, 1)
	reqFrame := appendCRC([]byte{0x66, 0x06, 0x00, 0x01, 0x12, 0x34})

	got, err := HandleRTURequestFrame(reqFrame, image)
	if err != nil {
		t.Fatalf("handle rtu request frame: %v", err)
	}

	want := appendCRC([]byte{0x66, 0x86, 0x02})
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
}

func TestHandleTCPRequestFrameNilImage(t *testing.T) {
	reqFrame := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x06, 0x11, 0x03, 0x00, 0x00, 0x00, 0x01}

	if _, err := HandleTCPRequestFrame(reqFrame, nil); err == nil {
		t.Fatal("expected nil image error")
	}
}

func TestHandleRTURequestFrameNilImage(t *testing.T) {
	reqFrame := appendCRC([]byte{0x11, 0x01, 0x00, 0x00, 0x00, 0x01})

	if _, err := HandleRTURequestFrame(reqFrame, nil); err == nil {
		t.Fatal("expected nil image error")
	}
}

func TestHandleTCPRequestFrameInvalidFrame(t *testing.T) {
	reqFrame := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x06, 0x11, 0x03, 0x00}

	if _, err := HandleTCPRequestFrame(reqFrame, NewMemoryProcessImage(1, 1, 1, 1)); err == nil {
		t.Fatal("expected invalid tcp frame error")
	}
}

func TestHandleRTURequestFrameInvalidFrame(t *testing.T) {
	reqFrame := []byte{0x11, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00}

	if _, err := HandleRTURequestFrame(reqFrame, NewMemoryProcessImage(1, 1, 1, 1)); err == nil {
		t.Fatal("expected invalid rtu frame error")
	}
}

func TestHandleTCPRequestFrameInvalidFunction(t *testing.T) {
	reqFrame := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x06, 0x11, 0x11, 0x00, 0x00, 0x00, 0x01}

	if _, err := HandleTCPRequestFrame(reqFrame, NewMemoryProcessImage(1, 1, 1, 1)); err == nil {
		t.Fatal("expected unsupported function parse error")
	}
}
