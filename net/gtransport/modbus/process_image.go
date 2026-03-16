package modbus

import "errors"

var (
	ErrProcessImageAddressOutOfRange  = errors.New("modbus process image address out of range")
	ErrProcessImageQuantityOutOfRange = errors.New("modbus process image quantity out of range")
	ErrProcessImageWriteValuesEmpty   = errors.New("modbus process image write values empty")
)

// ProcessImage stores the four Modbus data areas.
type ProcessImage interface {
	ReadCoils(start uint16, quantity uint16) ([]bool, error)
	ReadDiscreteInputs(start uint16, quantity uint16) ([]bool, error)
	ReadHoldingRegisters(start uint16, quantity uint16) ([]uint16, error)
	ReadInputRegisters(start uint16, quantity uint16) ([]uint16, error)

	WriteSingleCoil(address uint16, value bool) error
	WriteSingleRegister(address uint16, value uint16) error
	WriteMultipleCoils(start uint16, values []bool) error
	WriteMultipleRegisters(start uint16, values []uint16) error
}
