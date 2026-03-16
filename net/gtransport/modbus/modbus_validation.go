package modbus

import (
	"encoding/binary"
	"fmt"
)

const (
	modbusAddressSpaceSize            = 1 << 16
	readBitsMaxQuantity               = 2000
	readBitsMaxByteCount              = 250
	readRegistersMaxQuantity          = 125
	readRegistersMaxByteCount         = 250
	writeMultipleCoilsMaxQuantity     = 0x07B0
	writeMultipleRegistersMaxQuantity = 0x007B
)

func validateModbusPayload(frame []byte) error {
	if len(frame) < 2 {
		return fmt.Errorf("modbus frame too short: %d", len(frame))
	}
	function := frame[1]
	if function&0x80 != 0 {
		return validateExceptionPayload(frame)
	}
	if !isSupportedFunction(function) {
		return fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
	switch function {
	case 0x01, 0x02:
		return validateReadBitsPayload(function, frame)
	case 0x03, 0x04:
		return validateReadRegistersPayload(function, frame)
	case 0x05:
		return validateWriteSingleCoilPayload(frame)
	case 0x06:
		return validateWriteSingleRegisterPayload(function, frame)
	case 0x0F:
		return validateWriteMultipleCoilsPayload(frame)
	case 0x10:
		return validateWriteMultipleRegistersPayload(frame)
	default:
		return fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
}

func validateExceptionPayload(frame []byte) error {
	function := frame[1]
	if !isSupportedFunction(function & 0x7F) {
		return fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
	if len(frame) != 3 {
		return fmt.Errorf("invalid modbus exception length %d", len(frame))
	}
	return nil
}

func validateReadBitsPayload(function byte, frame []byte) error {
	data := frame[2:]
	if len(data) == 4 {
		startAddress := binary.BigEndian.Uint16(data[0:2])
		quantity := int(binary.BigEndian.Uint16(data[2:4]))
		if quantity >= 1 && quantity <= readBitsMaxQuantity {
			return validateModbusAddressRange(startAddress, uint16(quantity))
		}
	}
	if len(data) >= 1 && len(data) == 1+int(data[0]) {
		byteCount := int(data[0])
		if byteCount >= 1 && byteCount <= readBitsMaxByteCount {
			return nil
		}
	}
	return fmt.Errorf("invalid modbus payload for function 0x%02x", function)
}

func validateReadRegistersPayload(function byte, frame []byte) error {
	data := frame[2:]
	if len(data) == 4 {
		startAddress := binary.BigEndian.Uint16(data[0:2])
		quantity := int(binary.BigEndian.Uint16(data[2:4]))
		if quantity >= 1 && quantity <= readRegistersMaxQuantity {
			return validateModbusAddressRange(startAddress, uint16(quantity))
		}
	}
	if len(data) >= 1 && len(data) == 1+int(data[0]) {
		byteCount := int(data[0])
		if byteCount >= 2 && byteCount <= readRegistersMaxByteCount && byteCount%2 == 0 {
			return nil
		}
	}
	return fmt.Errorf("invalid modbus payload for function 0x%02x", function)
}

func validateWriteSingleCoilPayload(frame []byte) error {
	if len(frame) != 6 {
		return fmt.Errorf("invalid modbus payload length %d for function 0x05", len(frame))
	}
	value := binary.BigEndian.Uint16(frame[4:6])
	if value != 0x0000 && value != 0xFF00 {
		return fmt.Errorf("invalid modbus coil value 0x%04x", value)
	}
	return nil
}

func validateWriteSingleRegisterPayload(function byte, frame []byte) error {
	if len(frame) != 6 {
		return fmt.Errorf("invalid modbus payload length %d for function 0x%02x", len(frame), function)
	}
	return nil
}

func validateWriteMultipleCoilsPayload(frame []byte) error {
	data := frame[2:]
	if len(data) == 4 {
		quantity := int(binary.BigEndian.Uint16(data[2:4]))
		if quantity < 1 || quantity > writeMultipleCoilsMaxQuantity {
			return fmt.Errorf("invalid modbus quantity %d for function 0x0f", quantity)
		}
		return nil
	}
	if len(data) < 5 || len(data) != 5+int(data[4]) {
		return fmt.Errorf("invalid modbus payload length %d for function 0x0f", len(frame))
	}
	quantity := int(binary.BigEndian.Uint16(data[2:4]))
	if quantity < 1 || quantity > writeMultipleCoilsMaxQuantity {
		return fmt.Errorf("invalid modbus quantity %d for function 0x0f", quantity)
	}
	if err := validateModbusAddressRange(binary.BigEndian.Uint16(data[0:2]), uint16(quantity)); err != nil {
		return err
	}
	byteCount := int(data[4])
	wantByteCount := (quantity + 7) / 8
	if byteCount < 1 || byteCount > 246 || byteCount != wantByteCount {
		return fmt.Errorf("invalid modbus byte count %d for function 0x0f, expected %d", byteCount, wantByteCount)
	}
	return nil
}

func validateWriteMultipleRegistersPayload(frame []byte) error {
	data := frame[2:]
	if len(data) == 4 {
		quantity := int(binary.BigEndian.Uint16(data[2:4]))
		if quantity < 1 || quantity > writeMultipleRegistersMaxQuantity {
			return fmt.Errorf("invalid modbus quantity %d for function 0x10", quantity)
		}
		return nil
	}
	if len(data) < 5 || len(data) != 5+int(data[4]) {
		return fmt.Errorf("invalid modbus payload length %d for function 0x10", len(frame))
	}
	quantity := int(binary.BigEndian.Uint16(data[2:4]))
	if quantity < 1 || quantity > writeMultipleRegistersMaxQuantity {
		return fmt.Errorf("invalid modbus quantity %d for function 0x10", quantity)
	}
	if err := validateModbusAddressRange(binary.BigEndian.Uint16(data[0:2]), uint16(quantity)); err != nil {
		return err
	}
	byteCount := int(data[4])
	wantByteCount := quantity * 2
	if byteCount < 2 || byteCount > 246 || byteCount != wantByteCount {
		return fmt.Errorf("invalid modbus byte count %d for function 0x10, expected %d", byteCount, wantByteCount)
	}
	return nil
}

func validateModbusAddressRange(startAddress uint16, quantity uint16) error {
	if int(startAddress)+int(quantity) > modbusAddressSpaceSize {
		return fmt.Errorf("invalid modbus address range start=%d quantity=%d", startAddress, quantity)
	}
	return nil
}

func isSupportedFunction(function byte) bool {
	switch function {
	case 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x0F, 0x10:
		return true
	default:
		return false
	}
}
