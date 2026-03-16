// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gtransport

import "fmt"

type lineCodec struct {
	maxPayloadBytes int
	stripDelimiter  bool
}

// NewLine returns a line codec that decodes both LF and CRLF terminated
// frames, and encodes frames using LF. When strip is true, decoded frames do
// not include the line ending.
func NewLine(maxPayloadBytes int, strip bool) Codec {
	return &lineCodec{
		maxPayloadBytes: maxPayloadBytes,
		stripDelimiter:  strip,
	}
}

func (c *lineCodec) Decode(in []byte) ([]byte, int, error) {
	index, delimLen := c.findLineEnd(in)
	if index < 0 {
		if c.maxPayloadBytes > 0 && len(in) > c.maxPayloadBytes+1 {
			return nil, 0, fmt.Errorf("payload length exceeds max payload bytes %d", c.maxPayloadBytes)
		}
		return nil, 0, ErrNeedMoreData
	}
	if c.maxPayloadBytes > 0 && index > c.maxPayloadBytes {
		return nil, 0, fmt.Errorf("payload length exceeds max payload bytes %d", c.maxPayloadBytes)
	}
	consumed := index + delimLen
	if c.stripDelimiter {
		return in[:index], consumed, nil
	}
	return in[:consumed], consumed, nil
}

func (c *lineCodec) Encode(frame []byte) ([]byte, error) {
	if c.maxPayloadBytes > 0 && len(frame) > c.maxPayloadBytes {
		return nil, fmt.Errorf("payload length exceeds max payload bytes %d", c.maxPayloadBytes)
	}
	out := make([]byte, 0, len(frame)+1)
	out = append(out, frame...)
	out = append(out, '\n')
	return out, nil
}

func (c *lineCodec) findLineEnd(in []byte) (int, int) {
	for i := 0; i < len(in); i++ {
		if in[i] == '\n' {
			if i > 0 && in[i-1] == '\r' {
				return i - 1, 2
			}
			return i, 1
		}
	}
	return -1, 0
}
