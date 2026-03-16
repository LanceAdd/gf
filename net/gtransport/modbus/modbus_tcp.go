package modbus

import (
	"encoding/binary"
	"fmt"

	"github.com/gogf/gf/v2/net/gtransport"
)

const (
	tcpHeaderLength = 7
	tcpProtocolID   = 0
	tcpMinLength    = 2
	tcpMaxLength    = 254
)

type tcpCodec struct{}

func (c tcpCodec) Decode(in []byte) ([]byte, int, error) {
	if len(in) < tcpHeaderLength {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	if protocolID := binary.BigEndian.Uint16(in[2:4]); protocolID != tcpProtocolID {
		return nil, 0, fmt.Errorf("invalid modbus tcp protocol id %d", protocolID)
	}
	length := int(binary.BigEndian.Uint16(in[4:6]))
	if length < tcpMinLength || length > tcpMaxLength {
		return nil, 0, fmt.Errorf("invalid modbus tcp length %d", length)
	}
	frameLength := 6 + length
	if frameLength < tcpHeaderLength {
		return nil, 0, fmt.Errorf("invalid modbus tcp frame length %d", frameLength)
	}
	if len(in) < frameLength {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	if err := validateModbusPayload(in[6:frameLength]); err != nil {
		return nil, 0, err
	}
	return in[:frameLength], frameLength, nil
}

func (c tcpCodec) Encode(frame []byte) ([]byte, error) {
	if len(frame) < 6+tcpMinLength {
		return nil, fmt.Errorf("modbus tcp frame too short: %d", len(frame))
	}
	if protocolID := binary.BigEndian.Uint16(frame[2:4]); protocolID != tcpProtocolID {
		return nil, fmt.Errorf("invalid modbus tcp protocol id %d", protocolID)
	}
	out := append([]byte(nil), frame...)
	length := len(out) - 6
	if length < tcpMinLength || length > tcpMaxLength {
		return nil, fmt.Errorf("invalid modbus tcp length %d", length)
	}
	if err := validateModbusPayload(out[6:]); err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint16(out[4:6], uint16(length))
	return out, nil
}
