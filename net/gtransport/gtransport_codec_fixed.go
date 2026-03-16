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

// fixedLengthCodec treats every frame as a fixed-size byte block.
type fixedLengthCodec struct {
	length int
}

// NewFixedLength returns a codec for protocols where every frame has the same
// byte length.
func NewFixedLength(length int) Codec {
	return &fixedLengthCodec{length: length}
}

// Decode returns one full fixed-size frame once enough bytes are buffered.
func (c *fixedLengthCodec) Decode(in []byte) ([]byte, int, error) {
	if c.length <= 0 {
		return nil, 0, fmt.Errorf("invalid fixed length %d", c.length)
	}
	if len(in) < c.length {
		return nil, 0, ErrNeedMoreData
	}
	return in[:c.length], c.length, nil
}

// Encode validates that the payload already matches the fixed frame size.
func (c *fixedLengthCodec) Encode(frame []byte) ([]byte, error) {
	if c.length <= 0 {
		return nil, fmt.Errorf("invalid fixed length %d", c.length)
	}
	if len(frame) != c.length {
		return nil, fmt.Errorf("payload length %d does not match fixed length %d", len(frame), c.length)
	}
	return bytes.Clone(frame), nil
}
