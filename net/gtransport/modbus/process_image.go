package modbus

import "errors"

var (
	// ErrProcessImageAddressOutOfRange indicates that the requested Modbus data
	// address range does not exist in the process image.
	ErrProcessImageAddressOutOfRange = errors.New("modbus process image address out of range")
	// ErrProcessImageQuantityOutOfRange indicates that the requested quantity is
	// zero or otherwise invalid for the target operation.
	ErrProcessImageQuantityOutOfRange = errors.New("modbus process image quantity out of range")
	// ErrProcessImageWriteValuesEmpty indicates that a multi-write call did not
	// provide any values to write.
	ErrProcessImageWriteValuesEmpty = errors.New("modbus process image write values empty")
)

// ProcessImage stores the four Modbus data areas.
type ProcessImage interface {
	// ReadCoils returns coil values starting at start for quantity items.
	ReadCoils(start uint16, quantity uint16) ([]bool, error)
	// ReadDiscreteInputs returns discrete input values starting at start for
	// quantity items.
	ReadDiscreteInputs(start uint16, quantity uint16) ([]bool, error)
	// ReadHoldingRegisters returns holding register values starting at start for
	// quantity items.
	ReadHoldingRegisters(start uint16, quantity uint16) ([]uint16, error)
	// ReadInputRegisters returns input register values starting at start for
	// quantity items.
	ReadInputRegisters(start uint16, quantity uint16) ([]uint16, error)

	// WriteSingleCoil writes one coil value at address.
	WriteSingleCoil(address uint16, value bool) error
	// WriteSingleRegister writes one holding register value at address.
	WriteSingleRegister(address uint16, value uint16) error
	// WriteMultipleCoils writes consecutive coil values starting at start.
	WriteMultipleCoils(start uint16, values []bool) error
	// WriteMultipleRegisters writes consecutive holding register values starting
	// at start.
	WriteMultipleRegisters(start uint16, values []uint16) error
}
