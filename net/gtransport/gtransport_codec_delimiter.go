// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gtransport

import (
	"bytes"
	"fmt"
)

type delimiterCodec struct {
	delim           []byte
	maxPayloadBytes int
	stripDelimiter  bool
}

// NewDelimiter returns a codec that splits frames by a fixed delimiter.
// When strip is true, the returned decoded frame excludes the delimiter.
func NewDelimiter(delim []byte, maxPayloadBytes int, strip bool) Codec {
	return &delimiterCodec{
		delim:           append([]byte(nil), delim...),
		maxPayloadBytes: maxPayloadBytes,
		stripDelimiter:  strip,
	}
}

func (c *delimiterCodec) Decode(in []byte) ([]byte, int, error) {
	if len(c.delim) == 0 {
		return nil, 0, fmt.Errorf("delimiter cannot be empty")
	}
	index := bytes.Index(in, c.delim)
	if index < 0 {
		if c.maxPayloadBytes > 0 && len(in) > c.maxPayloadBytes+len(c.delim)-1 {
			return nil, 0, fmt.Errorf("payload length exceeds max payload bytes %d", c.maxPayloadBytes)
		}
		return nil, 0, ErrNeedMoreData
	}
	if c.maxPayloadBytes > 0 && index > c.maxPayloadBytes {
		return nil, 0, fmt.Errorf("payload length exceeds max payload bytes %d", c.maxPayloadBytes)
	}
	end := index + len(c.delim)
	if c.stripDelimiter {
		return in[:index], end, nil
	}
	return in[:end], end, nil
}

func (c *delimiterCodec) Encode(frame []byte) ([]byte, error) {
	if len(c.delim) == 0 {
		return nil, fmt.Errorf("delimiter cannot be empty")
	}
	if c.maxPayloadBytes > 0 && len(frame) > c.maxPayloadBytes {
		return nil, fmt.Errorf("payload length exceeds max payload bytes %d", c.maxPayloadBytes)
	}
	out := make([]byte, 0, len(frame)+len(c.delim))
	out = append(out, frame...)
	out = append(out, c.delim...)
	return out, nil
}
