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

type WriteSingleCoilRequest struct {
	meta    ADUMeta
	Address uint16
	Value   bool
}

func (r WriteSingleCoilRequest) Meta() ADUMeta {
	return r.meta
}

func (r WriteSingleCoilRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x05)
}

type WriteSingleRegisterRequest struct {
	meta    ADUMeta
	Address uint16
	Value   uint16
}

func (r WriteSingleRegisterRequest) Meta() ADUMeta {
	return r.meta
}

func (r WriteSingleRegisterRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x06)
}

type WriteMultipleCoilsRequest struct {
	meta         ADUMeta
	StartAddress uint16
	Values       []bool
}

func (r WriteMultipleCoilsRequest) Meta() ADUMeta {
	return r.meta
}

func (r WriteMultipleCoilsRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x0F)
}

type WriteMultipleRegistersRequest struct {
	meta         ADUMeta
	StartAddress uint16
	Values       []uint16
}

func (r WriteMultipleRegistersRequest) Meta() ADUMeta {
	return r.meta
}

func (r WriteMultipleRegistersRequest) FunctionCode() FunctionCode {
	return FunctionCode(0x10)
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
	return parseRequest(meta, payload)
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
	return parseRequest(meta, payload)
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

func parseRequest(meta ADUMeta, payload []byte) (Request, error) {
	function := payload[1]
	switch function {
	case 0x01:
		return parseReadCoilsRequest(meta, payload)
	case 0x02:
		return parseReadDiscreteInputsRequest(meta, payload)
	case 0x03:
		return parseReadHoldingRegistersRequest(meta, payload)
	case 0x04:
		return parseReadInputRegistersRequest(meta, payload)
	case 0x05:
		return parseWriteSingleCoilRequest(meta, payload)
	case 0x06:
		return parseWriteSingleRegisterRequest(meta, payload)
	case 0x0F:
		return parseWriteMultipleCoilsRequest(meta, payload)
	case 0x10:
		return parseWriteMultipleRegistersRequest(meta, payload)
	default:
		return nil, fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
}

func parseReadCoilsRequest(meta ADUMeta, payload []byte) (Request, error) {
	startAddress, quantity, err := parseReadRequestRange(payload)
	if err != nil {
		return nil, err
	}
	return ReadCoilsRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
}

func parseReadDiscreteInputsRequest(meta ADUMeta, payload []byte) (Request, error) {
	startAddress, quantity, err := parseReadRequestRange(payload)
	if err != nil {
		return nil, err
	}
	return ReadDiscreteInputsRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
}

func parseReadHoldingRegistersRequest(meta ADUMeta, payload []byte) (Request, error) {
	startAddress, quantity, err := parseReadRequestRange(payload)
	if err != nil {
		return nil, err
	}
	return ReadHoldingRegistersRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
}

func parseReadInputRegistersRequest(meta ADUMeta, payload []byte) (Request, error) {
	startAddress, quantity, err := parseReadRequestRange(payload)
	if err != nil {
		return nil, err
	}
	return ReadInputRegistersRequest{meta: meta, StartAddress: startAddress, Quantity: quantity}, nil
}

func parseReadRequestRange(payload []byte) (uint16, uint16, error) {
	if len(payload) != 6 {
		return 0, 0, fmt.Errorf("invalid modbus request payload length %d", len(payload))
	}
	return binary.BigEndian.Uint16(payload[2:4]), binary.BigEndian.Uint16(payload[4:6]), nil
}

func parseWriteSingleCoilRequest(meta ADUMeta, payload []byte) (Request, error) {
	if len(payload) != 6 {
		return nil, fmt.Errorf("invalid modbus request payload length %d", len(payload))
	}
	return WriteSingleCoilRequest{
		meta:    meta,
		Address: binary.BigEndian.Uint16(payload[2:4]),
		Value:   binary.BigEndian.Uint16(payload[4:6]) == 0xFF00,
	}, nil
}

func parseWriteSingleRegisterRequest(meta ADUMeta, payload []byte) (Request, error) {
	if len(payload) != 6 {
		return nil, fmt.Errorf("invalid modbus request payload length %d", len(payload))
	}
	return WriteSingleRegisterRequest{
		meta:    meta,
		Address: binary.BigEndian.Uint16(payload[2:4]),
		Value:   binary.BigEndian.Uint16(payload[4:6]),
	}, nil
}

func parseWriteMultipleCoilsRequest(meta ADUMeta, payload []byte) (Request, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("invalid modbus request payload length %d", len(payload))
	}
	startAddress := binary.BigEndian.Uint16(payload[2:4])
	quantity := int(binary.BigEndian.Uint16(payload[4:6]))
	byteCount := int(payload[6])
	values := make([]bool, quantity)
	data := payload[7 : 7+byteCount]
	for i := 0; i < quantity; i++ {
		values[i] = data[i/8]&(1<<uint(i%8)) != 0
	}
	return WriteMultipleCoilsRequest{
		meta:         meta,
		StartAddress: startAddress,
		Values:       values,
	}, nil
}

func parseWriteMultipleRegistersRequest(meta ADUMeta, payload []byte) (Request, error) {
	if len(payload) < 8 {
		return nil, fmt.Errorf("invalid modbus request payload length %d", len(payload))
	}
	startAddress := binary.BigEndian.Uint16(payload[2:4])
	quantity := int(binary.BigEndian.Uint16(payload[4:6]))
	values := make([]uint16, quantity)
	data := payload[7:]
	for i := 0; i < quantity; i++ {
		values[i] = binary.BigEndian.Uint16(data[i*2 : i*2+2])
	}
	return WriteMultipleRegistersRequest{
		meta:         meta,
		StartAddress: startAddress,
		Values:       values,
	}, nil
}
