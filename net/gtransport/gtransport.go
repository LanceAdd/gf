// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package gtransport provides framed read/write over stream connections using
// a fixed codec per transport.
//
// The package is intentionally small:
//   - Codec defines how bytes are decoded into frames and encoded back out.
//   - Transport performs synchronous framed reads and writes on an
//     io.ReadWriteCloser.
//
// Common protocols should prefer the simple codec constructors such as
// NewDelimiter, NewLine, NewFixedLength, and NewLengthPrefixed. NewLengthField
// is the advanced constructor for protocols with custom embedded length-field
// layouts.
package gtransport

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"
)

const (
	defaultReadBufferSize = 128
	defaultMaxBufferBytes = 4 * 1024 * 1024
	defaultReadTimeout    = 30 * time.Second
)

var (
	// ErrNeedMoreData indicates that the current input bytes are insufficient to
	// decode a full frame.
	ErrNeedMoreData = errors.New("need more data")
)

// readDeadliner is the minimal capability needed to apply per-read deadlines
// on compatible stream connections.
type readDeadliner interface {
	SetReadDeadline(t time.Time) error
}

// Codec converts between raw stream bytes and complete frames.
type Codec interface {
	// Decode attempts to extract one frame from in and reports how many bytes
	// were consumed from the front of the buffer.
	Decode(in []byte) (frame []byte, consumed int, err error)
	// Encode converts one logical frame into stream bytes ready to write.
	Encode(frame []byte) ([]byte, error)
}

// Option configures a Transport.
type Option func(*Transport)

// Transport performs synchronous framed reads and writes on a stream
// connection using a fixed Codec.
type Transport struct {
	conn           io.ReadWriteCloser
	codec          Codec
	writeMu        sync.Mutex
	closeOnce      sync.Once
	readBuffer     []byte
	maxBufferBytes int
	readTimeout    time.Duration
}

// WithReadTimeout sets the timeout applied to each underlying read while
// waiting for a complete frame. Set it to 0 to disable read deadlines.
func WithReadTimeout(d time.Duration) Option {
	return func(t *Transport) {
		t.readTimeout = d
	}
}

// WithMaxBufferBytes limits the internal read buffer size. It protects against
// unbounded growth when the peer sends incomplete or invalid frames.
func WithMaxBufferBytes(n int) Option {
	return func(t *Transport) {
		if n > 0 {
			t.maxBufferBytes = n
		}
	}
}

// New creates a Transport bound to a fixed Codec.
func New(conn io.ReadWriteCloser, codec Codec, opts ...Option) *Transport {
	t := &Transport{
		conn:           conn,
		codec:          codec,
		readBuffer:     make([]byte, 0, defaultReadBufferSize),
		maxBufferBytes: defaultMaxBufferBytes,
		readTimeout:    defaultReadTimeout,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

// Close closes the underlying connection.
func (t *Transport) Close() error {
	var err error
	t.closeOnce.Do(func() {
		err = t.conn.Close()
	})
	return err
}

// ReadFrame blocks until one complete frame is decoded or an error occurs.
func (t *Transport) ReadFrame() ([]byte, error) {
	for {
		frame, consumed, err := t.codec.Decode(t.readBuffer)
		switch {
		case consumed < 0 || consumed > len(t.readBuffer):
			return nil, fmt.Errorf("invalid codec output consumed bytes %d for input %d", consumed, len(t.readBuffer))
		case err == nil && consumed == 0:
			return nil, errors.New("invalid codec output: consumed bytes cannot be 0 when decoding succeeds")
		case consumed > 0:
			// Always discard bytes the codec explicitly consumed, even when it
			// still asks for more input, so sticky-packet and resync scenarios can
			// continue from the remaining unread tail.
			t.readBuffer = t.readBuffer[consumed:]
			t.shrinkReadBufferIfNeeded()
		}
		if err == nil {
			if frame == nil {
				continue
			}
			return bytes.Clone(frame), nil
		}
		if !errors.Is(err, ErrNeedMoreData) {
			return nil, err
		}
		if t.readTimeout > 0 {
			if dl, ok := t.conn.(readDeadliner); ok {
				if err = dl.SetReadDeadline(time.Now().Add(t.readTimeout)); err != nil {
					return nil, fmt.Errorf("SetReadDeadline failed: %w", err)
				}
			}
		}
		buffer := make([]byte, defaultReadBufferSize)
		n, readErr := t.conn.Read(buffer)
		if n > 0 {
			t.readBuffer = append(t.readBuffer, buffer[:n]...)
			if len(t.readBuffer) > t.maxBufferBytes {
				return nil, fmt.Errorf("read buffer exceeded max bytes %d", t.maxBufferBytes)
			}
		}
		if readErr != nil {
			return nil, readErr
		}
	}
}

// WriteFrame encodes and writes one complete frame. Concurrent writes are
// serialized internally.
func (t *Transport) WriteFrame(frame []byte) error {
	encoded, err := t.codec.Encode(frame)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	return t.writeFull(encoded)
}

// writeFull keeps writing until all encoded bytes are sent or an error occurs.
func (t *Transport) writeFull(data []byte) error {
	offset := 0
	for offset < len(data) {
		n, err := t.conn.Write(data[offset:])
		if n > 0 {
			offset += n
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

// shrinkReadBufferIfNeeded compacts or resets the read buffer after bytes have
// been consumed.
func (t *Transport) shrinkReadBufferIfNeeded() {
	if len(t.readBuffer) == 0 {
		if cap(t.readBuffer) > defaultReadBufferSize*8 {
			// Drop oversized backing arrays after large bursts so the transport
			// returns to a small steady-state footprint.
			t.readBuffer = make([]byte, 0, defaultReadBufferSize)
		}
		return
	}
	if cap(t.readBuffer) > defaultReadBufferSize*8 &&
		len(t.readBuffer)*4 < cap(t.readBuffer) {
		// Re-pack the unread tail when the spare capacity becomes much larger
		// than the live data to avoid retaining large buffers indefinitely.
		t.readBuffer = bytes.Clone(t.readBuffer)
	}
}
