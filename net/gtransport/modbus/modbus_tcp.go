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

// tcpCodec validates framed Modbus TCP ADUs and rewrites the MBAP length on
// encode.
type tcpCodec struct{}

// Decode scans the buffered byte stream for the next valid Modbus TCP ADU.
// It may discard malformed candidates as noise so the transport can
// resynchronize and continue reading later frames from the same connection.
func (c tcpCodec) Decode(in []byte) ([]byte, int, error) {
	if len(in) < tcpHeaderLength {
		return nil, 0, gtransport.ErrNeedMoreData
	}
	lastCandidateStart := len(in) - tcpHeaderLength
	partialCandidateStart := -1
	for start := 0; start <= lastCandidateStart; start++ {
		protocolID := binary.BigEndian.Uint16(in[start+2 : start+4])
		if protocolID != tcpProtocolID {
			continue
		}
		length := int(binary.BigEndian.Uint16(in[start+4 : start+6]))
		if length < tcpMinLength || length > tcpMaxLength {
			continue
		}
		// The MBAP length field counts Unit/Slave ID plus PDU bytes, so the full
		// ADU size is the 6-byte prefix before the field plus the declared length.
		frameLength := 6 + length
		if frameLength < tcpHeaderLength {
			continue
		}
		if len(in[start:]) < frameLength {
			if partialCandidateStart < 0 {
				partialCandidateStart = start
			}
			continue
		}
		end := start + frameLength
		if err := validateModbusPayload(in[start+6 : end]); err != nil {
			continue
		}
		return in[start:end], end, nil
	}
	if partialCandidateStart >= 0 {
		return nil, partialCandidateStart, gtransport.ErrNeedMoreData
	}
	// No full header candidate was usable. Preserve the trailing bytes that
	// could still become the start of a future MBAP header once more data arrives.
	consumed := len(in) - (tcpHeaderLength - 1)
	if consumed < 0 {
		consumed = 0
	}
	return nil, consumed, gtransport.ErrNeedMoreData
}

// Encode validates a Modbus TCP ADU and rewrites the MBAP length field from
// the actual payload size.
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
	// Always rewrite MBAP Length from the actual payload size so callers do not
	// need to keep the header field in sync manually.
	binary.BigEndian.PutUint16(out[4:6], uint16(length))
	return out, nil
}
