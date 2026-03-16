package modbus

// MemoryProcessImage stores Modbus data areas in contiguous in-memory slices.
type MemoryProcessImage struct {
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
	return &MemoryProcessImage{
		coils:            make([]bool, coilCount),
		discreteInputs:   make([]bool, discreteInputCount),
		holdingRegisters: make([]uint16, holdingRegisterCount),
		inputRegisters:   make([]uint16, inputRegisterCount),
	}
}

func (m *MemoryProcessImage) ReadCoils(start uint16, quantity uint16) ([]bool, error) {
	return readBoolRange(m.coils, start, quantity)
}

func (m *MemoryProcessImage) ReadDiscreteInputs(start uint16, quantity uint16) ([]bool, error) {
	return readBoolRange(m.discreteInputs, start, quantity)
}

func (m *MemoryProcessImage) ReadHoldingRegisters(start uint16, quantity uint16) ([]uint16, error) {
	return readUint16Range(m.holdingRegisters, start, quantity)
}

func (m *MemoryProcessImage) ReadInputRegisters(start uint16, quantity uint16) ([]uint16, error) {
	return readUint16Range(m.inputRegisters, start, quantity)
}

func (m *MemoryProcessImage) WriteSingleCoil(address uint16, value bool) error {
	index := int(address)
	if index < 0 || index >= len(m.coils) {
		return ErrProcessImageAddressOutOfRange
	}
	m.coils[index] = value
	return nil
}

func (m *MemoryProcessImage) WriteSingleRegister(address uint16, value uint16) error {
	index := int(address)
	if index < 0 || index >= len(m.holdingRegisters) {
		return ErrProcessImageAddressOutOfRange
	}
	m.holdingRegisters[index] = value
	return nil
}

func (m *MemoryProcessImage) WriteMultipleCoils(start uint16, values []bool) error {
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

func (m *MemoryProcessImage) WriteMultipleRegisters(start uint16, values []uint16) error {
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

func readBoolRange(values []bool, start uint16, quantity uint16) ([]bool, error) {
	startIndex, endIndex, err := validateReadRange(len(values), start, quantity)
	if err != nil {
		return nil, err
	}
	out := make([]bool, endIndex-startIndex)
	copy(out, values[startIndex:endIndex])
	return out, nil
}

func readUint16Range(values []uint16, start uint16, quantity uint16) ([]uint16, error) {
	startIndex, endIndex, err := validateReadRange(len(values), start, quantity)
	if err != nil {
		return nil, err
	}
	out := make([]uint16, endIndex-startIndex)
	copy(out, values[startIndex:endIndex])
	return out, nil
}

func validateReadRange(limit int, start uint16, quantity uint16) (int, int, error) {
	if quantity == 0 {
		return 0, 0, ErrProcessImageQuantityOutOfRange
	}
	return validateWriteRange(limit, start, int(quantity))
}

func validateWriteRange(limit int, start uint16, quantity int) (int, int, error) {
	startIndex := int(start)
	endIndex := startIndex + quantity
	if quantity <= 0 {
		return 0, 0, ErrProcessImageQuantityOutOfRange
	}
	if startIndex < 0 || endIndex > limit {
		return 0, 0, ErrProcessImageAddressOutOfRange
	}
	return startIndex, endIndex, nil
}
