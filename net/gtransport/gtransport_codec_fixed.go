// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

package gtransport

import "fmt"

type fixedLengthCodec struct {
	length int
}

// NewFixedLength returns a codec for protocols where every frame has the same
// byte length.
func NewFixedLength(length int) Codec {
	return &fixedLengthCodec{length: length}
}

func (c *fixedLengthCodec) Decode(in []byte) ([]byte, int, error) {
	if c.length <= 0 {
		return nil, 0, fmt.Errorf("invalid fixed length %d", c.length)
	}
	if len(in) < c.length {
		return nil, 0, ErrNeedMoreData
	}
	return in[:c.length], c.length, nil
}

func (c *fixedLengthCodec) Encode(frame []byte) ([]byte, error) {
	if c.length <= 0 {
		return nil, fmt.Errorf("invalid fixed length %d", c.length)
	}
	if len(frame) != c.length {
		return nil, fmt.Errorf("payload length %d does not match fixed length %d", len(frame), c.length)
	}
	return append([]byte(nil), frame...), nil
}
