package modbus

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/net/gtransport"
)

const (
	rtuCRCLength    = 2
	rtuMaxADULength = 256
)

// rtuCodec implements frame discovery, CRC validation, and encoding for raw
// Modbus RTU ADUs.
type rtuCodec struct{}

// Decode scans the buffer for one complete RTU frame and returns the payload
// without its trailing CRC bytes.
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
		if errors.Is(err, gtransport.ErrNeedMoreData) {
			return nil, offset, err
		}
		// Keep scanning forward so RTU streams can resynchronize after noise or
		// a malformed prefix.
		lastErr = err
	}
	if lastErr != nil {
		return nil, 0, lastErr
	}
	return nil, 0, gtransport.ErrNeedMoreData
}

// decodeCandidate validates a frame candidate starting at the current buffer
// offset.
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

// Encode validates a CRC-less RTU payload and appends a fresh CRC trailer.
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

// frameLength determines the most likely RTU ADU length from the function code
// and payload shape.
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

// writeMultipleFrameLength resolves the shared request/response function codes
// for write-multiple RTU frames.
func (c rtuCodec) writeMultipleFrameLength(in []byte) (int, error) {
	const fixedLen = 8
	if c.hasValidCRC(in, fixedLen) {
		// Write-multiple responses are fixed length and share the same function
		// codes as variable-length requests, so prefer the valid fixed-length
		// interpretation when its CRC matches.
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

// ambiguousFrameLength handles function codes whose request and response forms
// have different lengths.
func (c rtuCodec) ambiguousFrameLength(in []byte, fixedLen int, variableLenFn func([]byte) (int, bool)) (int, error) {
	if c.hasValidCRC(in, fixedLen) {
		// For read functions, exception responses and normal requests share the
		// same function codes. A valid fixed-length CRC means we can safely treat
		// the candidate as the shorter frame.
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

// hasValidCRC checks whether the first frameLen bytes end in a matching RTU
// CRC trailer.
func (c rtuCodec) hasValidCRC(in []byte, frameLen int) bool {
	if frameLen < 4 || len(in) < frameLen {
		return false
	}
	frame := in[:frameLen]
	crcWant := binary.LittleEndian.Uint16(frame[frameLen-rtuCRCLength:])
	crcGot := modbusCRC(frame[:frameLen-rtuCRCLength])
	return crcGot == crcWant
}

// modbusCRC computes the standard Modbus RTU CRC16 checksum.
func modbusCRC(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
