// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gtransport

import (
	"encoding/binary"
	"fmt"
	"math"
)

// LengthFieldOption configures the advanced length-field codec used by
// NewLengthField.
type LengthFieldOption struct {
	// ByteOrder is the byte order used for the embedded length field.
	ByteOrder binary.ByteOrder
	// MaxFrameBytes limits the total encoded frame size, including header bytes.
	MaxFrameBytes int
	// LengthFieldOffset is the zero-based offset of the length field.
	LengthFieldOffset int
	// LengthFieldLength is the size of the length field in bytes.
	LengthFieldLength int
	// LengthAdjustment adjusts the decoded length-field value before computing
	// the full frame length. For encoding, it is subtracted from len(frame)
	// before writing the field value.
	LengthAdjustment int
	// InitialBytesToStrip controls how many leading bytes are omitted from the
	// decoded frame returned by Decode.
	InitialBytesToStrip int
}

// lengthFieldCodec implements decoding and encoding around an embedded length
// field inside the frame header.
type lengthFieldCodec struct {
	opt                  LengthFieldOption
	lengthFieldEndOffset int
	initErr              error
}

// NewLengthPrefixed returns a codec for the common case where a frame begins
// with a 1/2/3/4/8-byte length field followed immediately by payload bytes.
func NewLengthPrefixed(fieldBytes int, order binary.ByteOrder, maxPayloadBytes int) Codec {
	maxFrameBytes := 0
	if maxPayloadBytes > 0 {
		maxFrameBytes = maxPayloadBytes + fieldBytes
	}
	return NewLengthField(LengthFieldOption{
		ByteOrder:           order,
		MaxFrameBytes:       maxFrameBytes,
		LengthFieldOffset:   0,
		LengthFieldLength:   fieldBytes,
		LengthAdjustment:    0,
		InitialBytesToStrip: fieldBytes,
	})
}

// NewLengthField returns the advanced length-field codec for protocols with
// custom embedded length-field layouts.
func NewLengthField(opt LengthFieldOption) Codec {
	if opt.MaxFrameBytes <= 0 {
		opt.MaxFrameBytes = defaultMaxBufferBytes
	}
	if opt.ByteOrder == nil {
		opt.ByteOrder = binary.BigEndian
	}
	c := &lengthFieldCodec{
		opt:                  opt,
		lengthFieldEndOffset: opt.LengthFieldOffset + opt.LengthFieldLength,
	}
	c.initErr = c.validate()
	return c
}

// Decode reads the embedded length field and returns one complete frame once
// enough bytes are buffered.
func (c *lengthFieldCodec) Decode(in []byte) ([]byte, int, error) {
	if c.initErr != nil {
		return nil, 0, c.initErr
	}
	if len(in) < c.lengthFieldEndOffset {
		return nil, 0, ErrNeedMoreData
	}
	unadjusted, err := c.getUnadjustedFrameLength(in)
	if err != nil {
		return nil, 0, err
	}
	// The configured length field usually excludes at least the bytes before
	// the field itself, so rebuild the full frame length from the raw field
	// value plus the caller's adjustment and the header bytes already skipped.
	frameLength := unadjusted + int64(c.opt.LengthAdjustment) + int64(c.lengthFieldEndOffset)
	if frameLength < int64(c.lengthFieldEndOffset) {
		return nil, 0, fmt.Errorf("invalid frame length %d less than length field end offset %d", frameLength, c.lengthFieldEndOffset)
	}
	if frameLength > int64(c.opt.MaxFrameBytes) {
		return nil, 0, fmt.Errorf("frame length %d exceeds max frame bytes %d", frameLength, c.opt.MaxFrameBytes)
	}
	if int64(len(in)) < frameLength {
		return nil, 0, ErrNeedMoreData
	}
	if c.opt.InitialBytesToStrip > int(frameLength) {
		return nil, 0, fmt.Errorf("initial bytes to strip %d exceeds frame length %d", c.opt.InitialBytesToStrip, frameLength)
	}
	consumed := int(frameLength)
	return in[c.opt.InitialBytesToStrip:consumed], consumed, nil
}

// Encode writes a fresh embedded length field based on the payload size.
func (c *lengthFieldCodec) Encode(frame []byte) ([]byte, error) {
	if c.initErr != nil {
		return nil, c.initErr
	}
	headerLen := c.opt.LengthFieldOffset + c.opt.LengthFieldLength
	lengthValue := len(frame) - c.opt.LengthAdjustment
	if lengthValue < 0 {
		return nil, fmt.Errorf("negative length value %d after adjustment", lengthValue)
	}
	if err := c.checkLengthOverflow(lengthValue); err != nil {
		return nil, err
	}
	out := make([]byte, headerLen+len(frame))
	c.putLengthField(out[c.opt.LengthFieldOffset:], lengthValue)
	copy(out[headerLen:], frame)
	if len(out) > c.opt.MaxFrameBytes {
		return nil, fmt.Errorf("encoded frame length %d exceeds max frame bytes %d", len(out), c.opt.MaxFrameBytes)
	}
	return out, nil
}

// validate checks constructor options once and stores any reusable setup error.
func (c *lengthFieldCodec) validate() error {
	if c.opt.LengthFieldOffset < 0 {
		return fmt.Errorf("invalid length field offset %d", c.opt.LengthFieldOffset)
	}
	switch c.opt.LengthFieldLength {
	case 1, 2, 3, 4, 8:
	default:
		return fmt.Errorf("invalid length field length %d", c.opt.LengthFieldLength)
	}
	if c.opt.InitialBytesToStrip < 0 {
		return fmt.Errorf("invalid initial bytes to strip %d", c.opt.InitialBytesToStrip)
	}
	return nil
}

// getUnadjustedFrameLength reads the raw length value from the configured
// length-field location without applying framing adjustments.
func (c *lengthFieldCodec) getUnadjustedFrameLength(in []byte) (int64, error) {
	offset := c.opt.LengthFieldOffset
	length := c.opt.LengthFieldLength
	field := in[offset : offset+length]
	switch length {
	case 1:
		return int64(uint8(field[0])), nil
	case 2:
		return int64(c.opt.ByteOrder.Uint16(field)), nil
	case 3:
		// Three-byte length fields are common in binary protocols but are not
		// supported by encoding/binary helpers, so decode them manually.
		if c.opt.ByteOrder == binary.LittleEndian {
			return int64(uint32(field[0]) | uint32(field[1])<<8 | uint32(field[2])<<16), nil
		}
		return int64(uint32(field[2]) | uint32(field[1])<<8 | uint32(field[0])<<16), nil
	case 4:
		return int64(c.opt.ByteOrder.Uint32(field)), nil
	case 8:
		v := c.opt.ByteOrder.Uint64(field)
		if v > math.MaxInt64 {
			return 0, fmt.Errorf("frame length %d exceeds int64 max", v)
		}
		return int64(v), nil
	default:
		return 0, fmt.Errorf("unsupported length field length %d", length)
	}
}

// putLengthField writes the computed length value back into the configured
// field width and byte order.
func (c *lengthFieldCodec) putLengthField(buf []byte, length int) {
	switch c.opt.LengthFieldLength {
	case 1:
		buf[0] = byte(length)
	case 2:
		c.opt.ByteOrder.PutUint16(buf, uint16(length))
	case 3:
		if c.opt.ByteOrder == binary.LittleEndian {
			buf[0] = byte(length)
			buf[1] = byte(length >> 8)
			buf[2] = byte(length >> 16)
		} else {
			buf[0] = byte(length >> 16)
			buf[1] = byte(length >> 8)
			buf[2] = byte(length)
		}
	case 4:
		c.opt.ByteOrder.PutUint32(buf, uint32(length))
	case 8:
		c.opt.ByteOrder.PutUint64(buf, uint64(length))
	}
}

// checkLengthOverflow rejects values that cannot fit into the configured field
// width during encoding.
func (c *lengthFieldCodec) checkLengthOverflow(length int) error {
	lengthValue := uint64(length)
	var maxVal uint64
	switch c.opt.LengthFieldLength {
	case 1:
		maxVal = 0xFF
	case 2:
		maxVal = 0xFFFF
	case 3:
		maxVal = 0xFFFFFF
	case 4:
		maxVal = 0xFFFFFFFF
	case 8:
		return nil
	}
	if lengthValue > maxVal {
		return fmt.Errorf("length value %d overflows %d-byte field (max %d)", length, c.opt.LengthFieldLength, maxVal)
	}
	return nil
}
