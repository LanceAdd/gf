package modbus

import (
	"encoding/binary"
	"fmt"
)

// Response is the common interface implemented by typed Modbus responses.
type Response interface {
	// Meta returns the transport metadata to preserve in the encoded response.
	Meta() ADUMeta
	// FunctionCode returns the base Modbus function code for the response.
	FunctionCode() FunctionCode
	// IsException reports whether the response should be encoded as a Modbus
	// exception frame.
	IsException() bool
}

// ReadBitsResponse represents a successful bit-oriented read response.
type ReadBitsResponse struct {
	meta     ADUMeta
	function FunctionCode
	Values   []bool
}

// Meta implements Response.Meta.
func (r ReadBitsResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r ReadBitsResponse) FunctionCode() FunctionCode {
	return r.function
}

// IsException implements Response.IsException.
func (r ReadBitsResponse) IsException() bool {
	return false
}

// ReadRegistersResponse represents a successful register-oriented read response.
type ReadRegistersResponse struct {
	meta     ADUMeta
	function FunctionCode
	Values   []uint16
}

// Meta implements Response.Meta.
func (r ReadRegistersResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r ReadRegistersResponse) FunctionCode() FunctionCode {
	return r.function
}

// IsException implements Response.IsException.
func (r ReadRegistersResponse) IsException() bool {
	return false
}

// WriteSingleCoilResponse represents a successful single-coil write response.
type WriteSingleCoilResponse struct {
	meta    ADUMeta
	Address uint16
	Value   bool
}

// Meta implements Response.Meta.
func (r WriteSingleCoilResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r WriteSingleCoilResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x05)
}

// IsException implements Response.IsException.
func (r WriteSingleCoilResponse) IsException() bool {
	return false
}

// WriteSingleRegisterResponse represents a successful single-register write response.
type WriteSingleRegisterResponse struct {
	meta    ADUMeta
	Address uint16
	Value   uint16
}

// Meta implements Response.Meta.
func (r WriteSingleRegisterResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r WriteSingleRegisterResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x06)
}

// IsException implements Response.IsException.
func (r WriteSingleRegisterResponse) IsException() bool {
	return false
}

// WriteMultipleCoilsResponse represents a successful multi-coil write response.
type WriteMultipleCoilsResponse struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

// Meta implements Response.Meta.
func (r WriteMultipleCoilsResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r WriteMultipleCoilsResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x0F)
}

// IsException implements Response.IsException.
func (r WriteMultipleCoilsResponse) IsException() bool {
	return false
}

// WriteMultipleRegistersResponse represents a successful multi-register write response.
type WriteMultipleRegistersResponse struct {
	meta         ADUMeta
	StartAddress uint16
	Quantity     uint16
}

// Meta implements Response.Meta.
func (r WriteMultipleRegistersResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r WriteMultipleRegistersResponse) FunctionCode() FunctionCode {
	return FunctionCode(0x10)
}

// IsException implements Response.IsException.
func (r WriteMultipleRegistersResponse) IsException() bool {
	return false
}

// ExceptionResponse represents a Modbus exception response frame.
type ExceptionResponse struct {
	meta          ADUMeta
	function      FunctionCode
	ExceptionCode byte
}

// Meta implements Response.Meta.
func (r ExceptionResponse) Meta() ADUMeta {
	return r.meta
}

// FunctionCode implements Response.FunctionCode.
func (r ExceptionResponse) FunctionCode() FunctionCode {
	return r.function
}

// IsException implements Response.IsException.
func (r ExceptionResponse) IsException() bool {
	return true
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

// encodeResponsePayload turns a typed response into the shared Modbus payload
// body before transport-specific wrapping.
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
	case ExceptionResponse:
		var exceptionErr error
		payload, exceptionErr = encodeExceptionResponsePayload(typed)
		if exceptionErr != nil {
			return nil, exceptionErr
		}
	default:
		return nil, fmt.Errorf("unsupported modbus response type %T", resp)
	}
	if err := validateModbusPayload(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

// encodeReadBitsPayload encodes a successful bit-read response body.
func encodeReadBitsPayload(resp ReadBitsResponse) []byte {
	byteCount := (len(resp.Values) + 7) / 8
	payload := make([]byte, 3+byteCount)
	payload[0] = resp.meta.SlaveID
	payload[1] = byte(resp.function)
	payload[2] = byte(byteCount)
	for i, value := range resp.Values {
		if value {
			// Read-bit responses use the same LSB-first packing as Modbus coil
			// write requests.
			payload[3+i/8] |= 1 << uint(i%8)
		}
	}
	return payload
}

// encodeReadRegistersPayload encodes a successful register-read response body.
func encodeReadRegistersPayload(resp ReadRegistersResponse) []byte {
	payload := make([]byte, 3+len(resp.Values)*2)
	payload[0] = resp.meta.SlaveID
	payload[1] = byte(resp.function)
	payload[2] = byte(len(resp.Values) * 2)
	offset := 3
	for _, value := range resp.Values {
		binary.BigEndian.PutUint16(payload[offset:offset+2], value)
		offset += 2
	}
	return payload
}

// encodeWriteSingleCoilPayload encodes a successful single-coil write response body.
func encodeWriteSingleCoilPayload(resp WriteSingleCoilResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.SlaveID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.Address)
	if resp.Value {
		binary.BigEndian.PutUint16(payload[4:6], 0xFF00)
	}
	return payload
}

// encodeWriteSingleRegisterPayload encodes a successful single-register write response body.
func encodeWriteSingleRegisterPayload(resp WriteSingleRegisterResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.SlaveID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.Address)
	binary.BigEndian.PutUint16(payload[4:6], resp.Value)
	return payload
}

// encodeWriteMultipleCoilsPayload encodes a successful multi-coil write response body.
func encodeWriteMultipleCoilsPayload(resp WriteMultipleCoilsResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.SlaveID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.StartAddress)
	binary.BigEndian.PutUint16(payload[4:6], resp.Quantity)
	return payload
}

// encodeWriteMultipleRegistersPayload encodes a successful multi-register write response body.
func encodeWriteMultipleRegistersPayload(resp WriteMultipleRegistersResponse) []byte {
	payload := make([]byte, 6)
	payload[0] = resp.meta.SlaveID
	payload[1] = byte(resp.FunctionCode())
	binary.BigEndian.PutUint16(payload[2:4], resp.StartAddress)
	binary.BigEndian.PutUint16(payload[4:6], resp.Quantity)
	return payload
}

// encodeExceptionResponsePayload encodes a Modbus exception response body using
// the base function code plus the exception flag bit.
func encodeExceptionResponsePayload(resp ExceptionResponse) ([]byte, error) {
	function := byte(resp.function)
	if !isSupportedFunction(function) {
		return nil, fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
	return []byte{resp.meta.SlaveID, function | 0x80, resp.ExceptionCode}, nil
}
