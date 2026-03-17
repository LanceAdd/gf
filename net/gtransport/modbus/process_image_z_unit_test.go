package modbus

import "testing"

func TestMemoryProcessImageReadWithinBounds(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	image.coils[1] = true
	image.discreteInputs[2] = true
	image.holdingRegisters[1] = 0x1234
	image.inputRegisters[2] = 0x5678

	coils, err := image.ReadCoils(0, 4)
	if err != nil {
		t.Fatalf("read coils: %v", err)
	}
	discreteInputs, err := image.ReadDiscreteInputs(0, 4)
	if err != nil {
		t.Fatalf("read discrete inputs: %v", err)
	}
	holdingRegisters, err := image.ReadHoldingRegisters(0, 4)
	if err != nil {
		t.Fatalf("read holding registers: %v", err)
	}
	inputRegisters, err := image.ReadInputRegisters(0, 4)
	if err != nil {
		t.Fatalf("read input registers: %v", err)
	}

	if !coils[1] {
		t.Fatalf("expected coil 1 to be true, got %v", coils)
	}
	if !discreteInputs[2] {
		t.Fatalf("expected discrete input 2 to be true, got %v", discreteInputs)
	}
	if holdingRegisters[1] != 0x1234 {
		t.Fatalf("expected holding register 1 to be 0x1234, got 0x%04x", holdingRegisters[1])
	}
	if inputRegisters[2] != 0x5678 {
		t.Fatalf("expected input register 2 to be 0x5678, got 0x%04x", inputRegisters[2])
	}
}

func TestMemoryProcessImageWriteWithinBounds(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)

	if err := image.WriteSingleCoil(1, true); err != nil {
		t.Fatalf("write single coil: %v", err)
	}
	if err := image.WriteSingleRegister(2, 0x1234); err != nil {
		t.Fatalf("write single register: %v", err)
	}
	if err := image.WriteMultipleCoils(0, []bool{true, false, true}); err != nil {
		t.Fatalf("write multiple coils: %v", err)
	}
	if err := image.WriteMultipleRegisters(1, []uint16{0x2222, 0x3333}); err != nil {
		t.Fatalf("write multiple registers: %v", err)
	}

	if !image.coils[0] || image.coils[1] || !image.coils[2] {
		t.Fatalf("unexpected coil state: %v", image.coils)
	}
	if image.holdingRegisters[1] != 0x2222 || image.holdingRegisters[2] != 0x3333 {
		t.Fatalf("unexpected holding registers: %v", image.holdingRegisters)
	}
}

func TestMemoryProcessImageRejectsOutOfRangeAccess(t *testing.T) {
	image := NewMemoryProcessImage(2, 2, 2, 2)

	if _, err := image.ReadCoils(1, 2); err == nil {
		t.Fatal("expected read coils out-of-range error")
	}
	if _, err := image.ReadDiscreteInputs(2, 1); err == nil {
		t.Fatal("expected read discrete inputs out-of-range error")
	}
	if _, err := image.ReadHoldingRegisters(1, 2); err == nil {
		t.Fatal("expected read holding registers out-of-range error")
	}
	if _, err := image.ReadInputRegisters(2, 1); err == nil {
		t.Fatal("expected read input registers out-of-range error")
	}
	if err := image.WriteSingleCoil(2, true); err == nil {
		t.Fatal("expected write single coil out-of-range error")
	}
	if err := image.WriteSingleRegister(2, 0x1234); err == nil {
		t.Fatal("expected write single register out-of-range error")
	}
	if err := image.WriteMultipleCoils(1, []bool{true, false}); err == nil {
		t.Fatal("expected write multiple coils out-of-range error")
	}
	if err := image.WriteMultipleRegisters(1, []uint16{0x1111, 0x2222}); err == nil {
		t.Fatal("expected write multiple registers out-of-range error")
	}
}

func TestMemoryProcessImageReadReturnsCopies(t *testing.T) {
	image := NewMemoryProcessImage(2, 2, 2, 2)
	image.coils[0] = true
	image.discreteInputs[0] = true
	image.holdingRegisters[0] = 0x1111
	image.inputRegisters[0] = 0x2222

	coils, err := image.ReadCoils(0, 2)
	if err != nil {
		t.Fatalf("read coils: %v", err)
	}
	discreteInputs, err := image.ReadDiscreteInputs(0, 2)
	if err != nil {
		t.Fatalf("read discrete inputs: %v", err)
	}
	holdingRegisters, err := image.ReadHoldingRegisters(0, 2)
	if err != nil {
		t.Fatalf("read holding registers: %v", err)
	}
	inputRegisters, err := image.ReadInputRegisters(0, 2)
	if err != nil {
		t.Fatalf("read input registers: %v", err)
	}

	coils[0] = false
	discreteInputs[0] = false
	holdingRegisters[0] = 0
	inputRegisters[0] = 0

	if !image.coils[0] {
		t.Fatal("expected coils read to return a copy")
	}
	if !image.discreteInputs[0] {
		t.Fatal("expected discrete inputs read to return a copy")
	}
	if image.holdingRegisters[0] != 0x1111 {
		t.Fatal("expected holding registers read to return a copy")
	}
	if image.inputRegisters[0] != 0x2222 {
		t.Fatal("expected input registers read to return a copy")
	}
}

func TestMemoryProcessImageAtomicBoundsForMultipleCoils(t *testing.T) {
	image := NewMemoryProcessImage(3, 1, 1, 1)
	image.coils[0] = true
	image.coils[1] = false
	image.coils[2] = true

	err := image.WriteMultipleCoils(2, []bool{false, false})
	if err == nil {
		t.Fatal("expected write multiple coils out-of-range error")
	}
	if !image.coils[0] || image.coils[1] || !image.coils[2] {
		t.Fatalf("expected original coil values to remain unchanged, got %v", image.coils)
	}
}

func TestMemoryProcessImageAtomicBoundsForMultipleRegisters(t *testing.T) {
	image := NewMemoryProcessImage(1, 1, 3, 1)
	image.holdingRegisters[0] = 0x1111
	image.holdingRegisters[1] = 0x2222
	image.holdingRegisters[2] = 0x3333

	err := image.WriteMultipleRegisters(2, []uint16{0xAAAA, 0xBBBB})
	if err == nil {
		t.Fatal("expected write multiple registers out-of-range error")
	}
	if image.holdingRegisters[0] != 0x1111 || image.holdingRegisters[1] != 0x2222 || image.holdingRegisters[2] != 0x3333 {
		t.Fatalf("expected original register values to remain unchanged, got %v", image.holdingRegisters)
	}
}

func TestMemoryProcessImageZeroLengthWrites(t *testing.T) {
	image := NewMemoryProcessImage(2, 2, 2, 2)

	if err := image.WriteMultipleCoils(0, nil); err == nil {
		t.Fatal("expected empty coils write error")
	}
	if err := image.WriteMultipleRegisters(0, nil); err == nil {
		t.Fatal("expected empty registers write error")
	}
}

func TestMemoryProcessImageNegativeCountsClampToZero(t *testing.T) {
	image := NewMemoryProcessImage(-1, -2, -3, -4)

	if len(image.coils) != 0 || len(image.discreteInputs) != 0 || len(image.holdingRegisters) != 0 || len(image.inputRegisters) != 0 {
		t.Fatalf("expected negative counts to clamp to zero, got %d/%d/%d/%d", len(image.coils), len(image.discreteInputs), len(image.holdingRegisters), len(image.inputRegisters))
	}
	if err := image.WriteSingleCoil(0, true); err == nil {
		t.Fatal("expected out-of-range error for zero-length coils")
	}
	if err := image.WriteSingleRegister(0, 1); err == nil {
		t.Fatal("expected out-of-range error for zero-length holding registers")
	}
}
