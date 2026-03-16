package modbus

import (
	"encoding/binary"
	"fmt"

	"github.com/gogf/gf/v2/net/gtransport"
)

const (
	rtuCRCLength    = 2
	rtuMaxADULength = 256
)

type rtuCodec struct{}

func (c rtuCodec) Decode(in []byte) ([]byte, int, error) {
	var lastErr error
	for offset := 0; offset < len(in); offset++ {
		if len(in[offset:]) < 2 {
			break
		}
		frame, frameLength, err := c.decodeCandidate(in[offset:])
		if err == nil {
			return frame, offset + frameLength, nil
		}
		if err == gtransport.ErrNeedMoreData {
			return nil, 0, err
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, gtransport.ErrNeedMoreData
}

func (c rtuCodec) decodeCandidate(in []byte) ([]byte, int, error) {
	if len(in) < 2 {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	frameLength, err := c.frameLength(in)
	if err != nil {
		return nil, 0, err
	}
	if len(in) < frameLength {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	frame := in[:frameLength]
	crcWant := binary.LittleEndian.Uint16(frame[frameLength-rtuCRCLength:])
	crcGot := modbusCRC(frame[:frameLength-rtuCRCLength])
	if crcGot != crcWant {
		return nil, 0, fmt.Errorf("invalid modbus rtu crc %04x, expected %04x", crcWant, crcGot)
	}
	if err := validateModbusPayload(frame[:frameLength-rtuCRCLength]); err != nil {
		return nil, 0, err
	}
	return frame[:frameLength-rtuCRCLength], frameLength, nil
}

func (c rtuCodec) Encode(frame []byte) ([]byte, error) {
	if len(frame)+rtuCRCLength > rtuMaxADULength {
		return nil, fmt.Errorf("modbus rtu frame too long: %d", len(frame)+rtuCRCLength)
	}
	if err := validateModbusPayload(frame); err != nil {
		return nil, err
	}
	out := make([]byte, len(frame)+rtuCRCLength)
	copy(out, frame)
	binary.LittleEndian.PutUint16(out[len(frame):], modbusCRC(frame))
	return out, nil
}

func (c rtuCodec) frameLength(in []byte) (int, error) {
	if len(in) < 2 {
		return 0, gtransport.ErrNeedMoreData
	}
	function := in[1]
	switch function {
	case 0x01, 0x02, 0x03, 0x04:
		return c.ambiguousFrameLength(in, 8, func(in []byte) (int, bool) {
			if len(in) < 3 {
				return 0, false
			}
			return 5 + int(in[2]), true
		})
	case 0x05, 0x06:
		return 8, nil
	case 0x0F, 0x10:
		return c.writeMultipleFrameLength(in)
	default:
		if function&0x80 != 0 {
			if !isSupportedFunction(function & 0x7F) {
				return 0, fmt.Errorf("unsupported modbus function code 0x%02x", function)
			}
			return 5, nil
		}
		return 0, fmt.Errorf("unsupported modbus function code 0x%02x", function)
	}
}

func (c rtuCodec) writeMultipleFrameLength(in []byte) (int, error) {
	const fixedLen = 8
	if c.hasValidCRC(in, fixedLen) {
		return fixedLen, nil
	}
	if len(in) < 7 {
		if len(in) < fixedLen {
			return 0, gtransport.ErrNeedMoreData
		}
		return fixedLen, nil
	}
	frameLen := 9 + int(in[6])
	if frameLen > rtuMaxADULength {
		return 0, fmt.Errorf("modbus rtu frame too long: %d", frameLen)
	}
	if len(in) < frameLen {
		return 0, gtransport.ErrNeedMoreData
	}
	if c.hasValidCRC(in, frameLen) {
		return frameLen, nil
	}
	if len(in) < fixedLen {
		return 0, gtransport.ErrNeedMoreData
	}
	return fixedLen, nil
}

func (c rtuCodec) ambiguousFrameLength(in []byte, fixedLen int, variableLenFn func([]byte) (int, bool)) (int, error) {
	if c.hasValidCRC(in, fixedLen) {
		return fixedLen, nil
	}
	if variableLen, ok := variableLenFn(in); ok {
		if variableLen > rtuMaxADULength {
			return 0, fmt.Errorf("modbus rtu frame too long: %d", variableLen)
		}
		if c.hasValidCRC(in, variableLen) {
			return variableLen, nil
		}
		if len(in) < variableLen {
			return 0, gtransport.ErrNeedMoreData
		}
	}
	if len(in) < fixedLen {
		return 0, gtransport.ErrNeedMoreData
	}
	return fixedLen, nil
}

func (c rtuCodec) hasValidCRC(in []byte, frameLen int) bool {
	if frameLen < 4 || len(in) < frameLen {
		return false
	}
	frame := in[:frameLen]
	crcWant := binary.LittleEndian.Uint16(frame[frameLen-rtuCRCLength:])
	crcGot := modbusCRC(frame[:frameLen-rtuCRCLength])
	return crcGot == crcWant
}

func modbusCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
