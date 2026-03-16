package modbus

import (
	"encoding/binary"
	"fmt"
)

// Response is the common interface implemented by typed Modbus responses.
type Response interface {
	Meta() ADUMeta
	FunctionCode() FunctionCode
	IsException() bool
}

type ReadBitsResponse struct {
	meta     ADUMeta
	function FunctionCode
	Values   []bool
}

func (r ReadBitsResponse) Meta() ADUMeta {
	return r.meta
}

func (r ReadBitsResponse) FunctionCode() FunctionCode {
	return r.function
}

func (r ReadBitsResponse) IsException() bool {
	return false
}

type ReadRegistersResponse struct {
	meta     ADUMeta
	function FunctionCode
	Values   []uint16
}

func (r ReadRegistersResponse) Meta() ADUMeta {
	return r.meta
}

func (r ReadRegistersResponse) FunctionCode() FunctionCode {
	return r.function
}

func (r ReadRegistersResponse) IsException() bool {
	return false
}

type WriteSingleCoilResponse struct {
	meta    ADUMeta
	Address uint16
	Value   bool
}

func (r WriteSingleCoilResponse) Meta() ADUMeta {
	return r.meta
}

func (r WriteSingleCoilResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x05)
}

func (r WriteSingleCoilResponse) IsException() bool {
	return false
}

type WriteSingleRegisterResponse struct {
	meta    ADUMeta
	Address uint16
	Value   uint16
}

func (r WriteSingleRegisterResponse) Meta() ADUMeta {
	return r.meta
}

func (r WriteSingleRegisterResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x06)
}

func (r WriteSingleRegisterResponse) IsException() bool {
	return false
}

type WriteMultipleCoilsResponse struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

func (r WriteMultipleCoilsResponse) Meta() ADUMeta {
	return r.meta
}

func (r WriteMultipleCoilsResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x0F)
}

func (r WriteMultipleCoilsResponse) IsException() bool {
	return false
}

type WriteMultipleRegistersResponse struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

func (r WriteMultipleRegistersResponse) Meta() ADUMeta {
	return r.meta
}

func (r WriteMultipleRegistersResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x10)
}

func (r WriteMultipleRegistersResponse) IsException() bool {
	return false
}

// EncodeTCPResponse encodes a typed Modbus response as a TCP ADU.
func EncodeTCPResponse(resp Response) ([]byte, error) {
	payload, err := encodeResponsePayload(resp)
	if err != nil {
		return nil, err
	}
	out := make([]byte, 6+len(payload))
	meta := resp.Meta()
	binary.BigEndian.PutUint16(out[0:2], meta.TransactionID)
	binary.BigEndian.PutUint16(out[2:4], tcpProtocolID)
	binary.BigEndian.PutUint16(out[4:6], uint16(len(payload)))
	copy(out[6:], payload)
	return out, nil
}

// EncodeRTUResponse encodes a typed Modbus response as an RTU ADU.
func EncodeRTUResponse(resp Response) ([]byte, error) {
	payload, err := encodeResponsePayload(resp)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(payload)+rtuCRCLength)
	copy(out, payload)
	binary.LittleEndian.PutUint16(out[len(payload):], modbusCRC(payload))
	return out, nil
}

func encodeResponsePayload(resp Response) ([]byte, error) {
	var payload []byte

	switch typed := resp.(type) {
	case ReadBitsResponse:
		payload = encodeReadBitsPayload(typed)
	case ReadRegistersResponse:
		payload = encodeReadRegistersPayload(typed)
	case WriteSingleCoilResponse:
		payload = encodeWriteSingleCoilPayload(typed)
	case WriteSingleRegisterResponse:
		payload = encodeWriteSingleRegisterPayload(typed)
	case WriteMultipleCoilsResponse:
		payload = encodeWriteMultipleCoilsPayload(typed)
	case WriteMultipleRegistersResponse:
		payload = encodeWriteMultipleRegistersPayload(typed)
	default:
		return nil, fmt.Errorf("unsupported modbus response type %T", resp)
	}
	if err := validateModbusPayload(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func encodeReadBitsPayload(resp ReadBitsResponse) []byte {
	byteCount := (len(resp.Values) + 7) / 8
	payload := make([]byte, 3+byteCount)
	payload[0] = resp.meta.UnitID
	payload[1] = byte(resp.function)
	payload[2] = byte(byteCount)
	for i, value := range resp.Values {
		if value {
			payload[3+i/8] |= 1 << uint(i%8)
		}
	}
	return payload
}

func encodeReadRegistersPayload(resp ReadRegistersResponse) []byte {
	payload := make([]byte, 3+len(resp.Values)*2)
	payload[0] = resp.meta.UnitID
	payload[1] = byte(resp.function)
	payload[2] = byte(len(resp.Values) * 2)
	offset := 3
	for _, value := range resp.Values {
		binary.BigEndian.PutUint16(payload[offset:offset+2], value)
		offset += 2
	}
	return payload
}

func encodeWriteSingleCoilPayload(resp WriteSingleCoilResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.UnitID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.Address)
	if resp.Value {
		binary.BigEndian.PutUint16(payload[4:6], 0xFF00)
	}
	return payload
}

func encodeWriteSingleRegisterPayload(resp WriteSingleRegisterResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.UnitID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.Address)
	binary.BigEndian.PutUint16(payload[4:6], resp.Value)
	return payload
}

func encodeWriteMultipleCoilsPayload(resp WriteMultipleCoilsResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.UnitID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.StartAddress)
	binary.BigEndian.PutUint16(payload[4:6], resp.Quantity)
	return payload
}

func encodeWriteMultipleRegistersPayload(resp WriteMultipleRegistersResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.UnitID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.StartAddress)
	binary.BigEndian.PutUint16(payload[4:6], resp.Quantity)
	return payload
}
