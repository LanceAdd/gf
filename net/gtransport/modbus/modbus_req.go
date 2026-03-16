package modbus

import (
	"encoding/binary"
	"fmt"
)

// FunctionCode identifies a Modbus function code.
type FunctionCode byte

// TransportKind identifies the Modbus wire transport.
type TransportKind byte

const (
	TransportTCP TransportKind = iota + 1
	TransportRTU
)

// ADUMeta carries transport-specific metadata preserved across parsing.
type ADUMeta struct {
	Transport     TransportKind
	TransactionID uint16
	UnitID        byte
}

// Request is the common interface implemented by typed Modbus requests.
type Request interface {
	Meta() ADUMeta
	FunctionCode() FunctionCode
}

type ReadCoilsRequest struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

func (r ReadCoilsRequest) Meta() ADUMeta {
	return r.meta
}

func (r ReadCoilsRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x01)
}

type ReadDiscreteInputsRequest struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

func (r ReadDiscreteInputsRequest) Meta() ADUMeta {
	return r.meta
}

func (r ReadDiscreteInputsRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x02)
}

type ReadHoldingRegistersRequest struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

func (r ReadHoldingRegistersRequest) Meta() ADUMeta {
	return r.meta
}

func (r ReadHoldingRegistersRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x03)
}

type ReadInputRegistersRequest struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

func (r ReadInputRegistersRequest) Meta() ADUMeta {
	return r.meta
}

func (r ReadInputRegistersRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x04)
}

// ParseTCPRequest parses a framed Modbus TCP ADU into a typed request.
func ParseTCPRequest(frame []byte) (Request, error) {
	if len(frame) < tcpHeaderLength+tcpMinLength {
		return nil, fmt.Errorf("modbus tcp frame too short: %d", len(frame))
	}
	if protocolID := binary.BigEndian.Uint16(frame[2:4]); protocolID != tcpProtocolID {
		return nil, fmt.Errorf("invalid modbus tcp protocol id %d", protocolID)
	}
	length := int(binary.BigEndian.Uint16(frame[4:6]))
	if length < tcpMinLength || length > tcpMaxLength {
		return nil, fmt.Errorf("invalid modbus tcp length %d", length)
	}
	if len(frame) != 6+length {
		return nil, fmt.Errorf("invalid modbus tcp frame length %d", len(frame))
	}
	payload := frame[6:]
	if err := validateModbusPayload(payload); err != nil {
		return nil, err
	}
	meta := ADUMeta{
		Transport:     TransportTCP,
		TransactionID: binary.BigEndian.Uint16(frame[0:2]),
		UnitID:        payload[0],
	}
	return parseReadRequest(meta, payload)
}

// ParseRTURequest parses a Modbus RTU frame or decoded RTU payload into a typed request.
func ParseRTURequest(frame []byte) (Request, error) {
	payload, err := normalizeRTURequestPayload(frame)
	if err != nil {
		return nil, err
	}
	if err = validateModbusPayload(payload); err != nil {
		return nil, err
	}
	meta := ADUMeta{
		Transport: TransportRTU,
		UnitID:    payload[0],
	}
	return parseReadRequest(meta, payload)
}

func normalizeRTURequestPayload(frame []byte) ([]byte, error) {
	switch {
	case len(frame) < 8:
		return nil, fmt.Errorf("modbus rtu frame missing crc")
	case hasValidRTUCRC(frame):
		return frame[:len(frame)-rtuCRCLength], nil
	default:
		return nil, fmt.Errorf("invalid modbus rtu crc")
	}
}

func hasValidRTUCRC(frame []byte) bool {
	if len(frame) < 4 {
		return false
	}
	crcWant := binary.LittleEndian.Uint16(frame[len(frame)-rtuCRCLength:])
	crcGot := modbusCRC(frame[:len(frame)-rtuCRCLength])
	return crcGot == crcWant
}

func parseReadRequest(meta ADUMeta, payload []byte) (Request, error) {
	if len(payload) != 6 {
		return nil, fmt.Errorf("invalid modbus request payload length %d", len(payload))
	}
	function := payload[1]
	if !isSupportedFunction(function) {
		return nil, fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
	startAddress := binary.BigEndian.Uint16(payload[2:4])
	quantity := binary.BigEndian.Uint16(payload[4:6])

	switch function {
	case 0x01:
		return ReadCoilsRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
	case 0x02:
		return ReadDiscreteInputsRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
	case 0x03:
		return ReadHoldingRegistersRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
	case 0x04:
		return ReadInputRegistersRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
	default:
		return nil, fmt.Errorf("unsupported modbus request function code 0x%02x", function)
	}
}
