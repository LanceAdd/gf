// Package adcp provides ADCP protocol support for gtransport.
//
// The package currently exposes one decode-only transport codec:
//   - New(maxFrameLength ...int)
//
// Decode behavior:
//   - scans the input stream for a 16-byte 0x80 sync preamble
//   - validates ensemble and payload-size redundancy fields
//   - validates a CRC16-CCITT checksum over the payload bytes
//   - returns only the ADCP payload bytes to the caller
//
// Encode is intentionally unsupported in the current version.
//
// DefaultMaxFrameLength exposes the default full-frame size limit used when
// New is called without an override.
package adcp

import (
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/gogf/gf/v2/net/gtransport"
)

const (
	syncByte     = 0x80
	syncLength   = 16
	headerLength = 16
	crcLength    = 4
	// DefaultMaxFrameLength is the default upper bound for one full ADCP frame.
	DefaultMaxFrameLength = 8192
)

// New returns a decode-only codec for ADCP frames.
// The first optional maxFrameLength value overrides the default 8192-byte limit.
func New(maxFrameLength ...int) gtransport.Codec {
	return codec{
		maxFrameLength: resolveMaxFrameLength(maxFrameLength),
	}
}

type codec struct {
	maxFrameLength int
}

// Decode finds one valid ADCP frame and returns only its payload bytes.
func (c codec) Decode(in []byte) ([]byte, int, error) {
	if len(in) == 0 {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	for offset := 0; offset < len(in); {
		index := findSyncStart(in[offset:])
		if index < 0 {
			return nil, consumeUntilPossibleSync(in), gtransport.ErrNeedMoreData
		}
		offset += index
		frame, consumed, err := c.decodeAt(in[offset:])
		if err == nil {
			return frame, offset + consumed, nil
		}
		if errors.Is(err, gtransport.ErrNeedMoreData) {
			return nil, offset, err
		}
		offset++
	}
	return nil, consumeUntilPossibleSync(in), gtransport.ErrNeedMoreData
}

// Encode reports that ADCP transport encoding is not implemented.
func (c codec) Encode(frame []byte) ([]byte, error) {
	return nil, errors.New("adcp codec does not support encoding")
}

func (c codec) decodeAt(in []byte) ([]byte, int, error) {
	if len(in) < syncLength+headerLength {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	ensemble := binary.LittleEndian.Uint32(in[syncLength:])
	ensembleCopy := binary.LittleEndian.Uint32(in[syncLength+4:])
	if ensemble&ensembleCopy != 0 {
		return nil, 0, errors.New("invalid adcp ensemble redundancy")
	}
	payloadSize := binary.LittleEndian.Uint32(in[syncLength+8:])
	payloadSizeCopy := binary.LittleEndian.Uint32(in[syncLength+12:])
	if payloadSize&payloadSizeCopy != 0 {
		return nil, 0, errors.New("invalid adcp payload size redundancy")
	}
	frameLength := syncLength + headerLength + int(payloadSize) + crcLength
	if frameLength > c.maxFrameLength {
		return nil, 0, fmt.Errorf("adcp frame too long: %d", frameLength)
	}
	if len(in) < frameLength {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	payloadStart := syncLength + headerLength
	payloadEnd := payloadStart + int(payloadSize)
	payload := in[payloadStart:payloadEnd]
	crcWant := binary.LittleEndian.Uint32(in[payloadEnd : payloadEnd+crcLength])
	crcGot := uint32(crc16CCITT(payload))
	if crcWant != crcGot {
		return nil, 0, fmt.Errorf("invalid adcp crc %08x, expected %08x", crcWant, crcGot)
	}
	return payload, frameLength, nil
}

func findSyncStart(in []byte) int {
	if len(in) < syncLength {
		return -1
	}
outer:
	for i := 0; i <= len(in)-syncLength; i++ {
		for j := range syncLength {
			if in[i+j] != syncByte {
				continue outer
			}
		}
		return i
	}
	return -1
}

func consumeUntilPossibleSync(in []byte) int {
	if len(in) < syncLength {
		return 0
	}
	return len(in) - (syncLength - 1)
}

func crc16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func resolveMaxFrameLength(maxFrameLength []int) int {
	if len(maxFrameLength) > 0 && maxFrameLength[0] > 0 {
		return maxFrameLength[0]
	}
	return DefaultMaxFrameLength
}
