package modbus

import (
	"slices"
	"sync"
)

// MemoryProcessImage stores Modbus data areas in contiguous in-memory slices.
//
// MemoryProcessImage is safe for concurrent use at single-call granularity.
type MemoryProcessImage struct {
	mu               sync.RWMutex
	coils            []bool
	discreteInputs   []bool
	holdingRegisters []uint16
	inputRegisters   []uint16
}

// NewMemoryProcessImage creates a memory-backed ProcessImage with fixed capacities.
func NewMemoryProcessImage(
	coilCount int,
	discreteInputCount int,
	holdingRegisterCount int,
	inputRegisterCount int,
) *MemoryProcessImage {
	coilCount = max(coilCount, 0)
	discreteInputCount = max(discreteInputCount, 0)
	holdingRegisterCount = max(holdingRegisterCount, 0)
	inputRegisterCount = max(inputRegisterCount, 0)
	return &MemoryProcessImage{
		coils:            make([]bool, coilCount),
		discreteInputs:   make([]bool, discreteInputCount),
		holdingRegisters: make([]uint16, holdingRegisterCount),
		inputRegisters:   make([]uint16, inputRegisterCount),
	}
}

// ReadCoils implements ProcessImage.ReadCoils.
func (m *MemoryProcessImage) ReadCoils(start uint16, quantity uint16) ([]bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return readBoolRange(m.coils, start, quantity)
}

// ReadDiscreteInputs implements ProcessImage.ReadDiscreteInputs.
func (m *MemoryProcessImage) ReadDiscreteInputs(start uint16, quantity uint16) ([]bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return readBoolRange(m.discreteInputs, start, quantity)
}

// ReadHoldingRegisters implements ProcessImage.ReadHoldingRegisters.
func (m *MemoryProcessImage) ReadHoldingRegisters(start uint16, quantity uint16) ([]uint16, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return readUint16Range(m.holdingRegisters, start, quantity)
}

// ReadInputRegisters implements ProcessImage.ReadInputRegisters.
func (m *MemoryProcessImage) ReadInputRegisters(start uint16, quantity uint16) ([]uint16, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return readUint16Range(m.inputRegisters, start, quantity)
}

// WriteSingleCoil implements ProcessImage.WriteSingleCoil.
func (m *MemoryProcessImage) WriteSingleCoil(address uint16, value bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := int(address)
	if index >= len(m.coils) {
		return ErrProcessImageAddressOutOfRange
	}
	m.coils[index] = value
	return nil
}

// WriteSingleRegister implements ProcessImage.WriteSingleRegister.
func (m *MemoryProcessImage) WriteSingleRegister(address uint16, value uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := int(address)
	if index >= len(m.holdingRegisters) {
		return ErrProcessImageAddressOutOfRange
	}
	m.holdingRegisters[index] = value
	return nil
}

// WriteMultipleCoils implements ProcessImage.WriteMultipleCoils.
func (m *MemoryProcessImage) WriteMultipleCoils(start uint16, values []bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(values) == 0 {
		return ErrProcessImageWriteValuesEmpty
	}
	startIndex, endIndex, err := validateWriteRange(len(m.coils), start, len(values))
	if err != nil {
		return err
	}
	copy(m.coils[startIndex:endIndex], values)
	return nil
}

// WriteMultipleRegisters implements ProcessImage.WriteMultipleRegisters.
func (m *MemoryProcessImage) WriteMultipleRegisters(start uint16, values []uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(values) == 0 {
		return ErrProcessImageWriteValuesEmpty
	}
	startIndex, endIndex, err := validateWriteRange(len(m.holdingRegisters), start, len(values))
	if err != nil {
		return err
	}
	copy(m.holdingRegisters[startIndex:endIndex], values)
	return nil
}

// readBoolRange copies a contiguous boolean range out of the backing slice.
func readBoolRange(values []bool, start uint16, quantity uint16) ([]bool, error) {
	startIndex, endIndex, err := validateReadRange(len(values), start, quantity)
	if err != nil {
		return nil, err
	}
	return slices.Clone(values[startIndex:endIndex]), nil
}

// readUint16Range copies a contiguous register range out of the backing slice.
func readUint16Range(values []uint16, start uint16, quantity uint16) ([]uint16, error) {
	startIndex, endIndex, err := validateReadRange(len(values), start, quantity)
	if err != nil {
		return nil, err
	}
	return slices.Clone(values[startIndex:endIndex]), nil
}

// validateReadRange validates a read request and returns normalized slice
// indexes.
func validateReadRange(limit int, start uint16, quantity uint16) (int, int, error) {
	if quantity == 0 {
		return 0, 0, ErrProcessImageQuantityOutOfRange
	}
	return validateWriteRange(limit, start, int(quantity))
}

// validateWriteRange validates a write range and returns normalized slice
// indexes.
func validateWriteRange(limit int, start uint16, quantity int) (int, int, error) {
	startIndex := int(start)
	endIndex := startIndex + quantity
	if quantity <= 0 {
		return 0, 0, ErrProcessImageQuantityOutOfRange
	}
	if endIndex > limit {
		return 0, 0, ErrProcessImageAddressOutOfRange
	}
	return startIndex, endIndex, nil
}
