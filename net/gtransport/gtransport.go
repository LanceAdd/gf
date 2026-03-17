// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package gtransport provides framed I/O over stream connections through one
// Transport model bound to one Codec.
//
// Core package responsibilities:
//   - Codec defines frame discovery and encoding rules
//   - Transport provides synchronous ReadFrame/WriteFrame operations
//   - Dial and Wrap define whether the transport owns connection lifecycle
//
// Built-in generic codecs include delimiter, line, fixed-length, and
// length-field variants.
//
// Protocol-aware codecs live in subpackages, including:
//   - adcp.New(maxFrameLength ...int) for decode-only ADCP payload streams
//   - modbus.NewTCP() and modbus.NewRTU() for Modbus transports
package gtransport

import (
	"bytes"
	"context"
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
	// ErrTransportMissingCodec indicates that no codec was configured.
	ErrTransportMissingCodec = errors.New("transport codec is nil")
	// ErrTransportClosed indicates that the transport has been closed.
	ErrTransportClosed = errors.New("transport is closed")
)

// readDeadliner is the minimal capability needed to apply per-read deadlines
// on compatible stream connections.
type readDeadliner interface {
	SetReadDeadline(t time.Time) error
}

// writeDeadliner is the minimal capability needed to apply per-write deadlines
// on compatible stream connections.
type writeDeadliner interface {
	SetWriteDeadline(t time.Time) error
}

// Codec converts between raw stream bytes and complete frames.
type Codec interface {
	// Decode attempts to extract one frame from in and reports how many bytes
	// were consumed from the front of the buffer.
	Decode(in []byte) (frame []byte, consumed int, err error)
	// Encode converts one logical frame into stream bytes ready to write.
	Encode(frame []byte) ([]byte, error)
}

type transportOption func(*Transport)

// WrapOption configures a wrapped fixed-connection Transport.
type WrapOption = transportOption

// DialOption configures a dial-managed Transport.
type DialOption = transportOption

// State represents the lifecycle state of a Transport.
type State int

const (
	// StateIdle indicates that no active connection is ready.
	StateIdle State = iota
	// StateConnecting indicates that a connect attempt is in progress.
	StateConnecting
	// StateReady indicates that a framed transport is ready for use.
	StateReady
	// StateClosed indicates that the transport is permanently closed.
	StateClosed
)

// String implements fmt.Stringer for stable human-readable state names.
func (s State) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateConnecting:
		return "connecting"
	case StateReady:
		return "ready"
	case StateClosed:
		return "closed"
	default:
		return fmt.Sprintf("state(%d)", s)
	}
}

type transportMode int

const (
	transportModeWrap transportMode = iota
	transportModeDial
)

// Transport performs synchronous framed reads and writes using either a wrapped
// live connection or a dial-managed connection lifecycle.
type Transport struct {
	mode      transportMode
	connector Connector
	codec     Codec

	mu              sync.Mutex
	writeMu         sync.Mutex
	closeOnce       sync.Once
	closedCh        chan struct{}
	state           State
	conn            io.ReadWriteCloser
	connectingCh    chan struct{}
	connectFailures int

	readBuffer     []byte
	maxBufferBytes int
	readTimeout    time.Duration

	connectTimeout   time.Duration
	reconnectBackoff func(attempt int) time.Duration

	lastReadAt  time.Time
	lastWriteAt time.Time
	readyAt     time.Time
}

// WithReadTimeout sets the timeout applied to each underlying read while
// waiting for a complete frame. Set it to 0 to disable read deadlines.
func WithReadTimeout(d time.Duration) WrapOption {
	return func(t *Transport) {
		t.readTimeout = d
	}
}

// WithMaxBufferBytes limits the internal read buffer size. It protects against
// unbounded growth when the peer sends incomplete or invalid frames.
func WithMaxBufferBytes(n int) WrapOption {
	return func(t *Transport) {
		if n > 0 {
			t.maxBufferBytes = n
		}
	}
}

// Wrap creates a Transport bound to an already-open connection.
func Wrap(conn io.ReadWriteCloser, codec Codec, opts ...WrapOption) *Transport {
	t := newTransport(codec)
	t.mode = transportModeWrap
	t.conn = conn
	if conn == nil {
		t.state = StateClosed
	} else {
		t.state = StateReady
		t.readyAt = time.Now()
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func newTransport(codec Codec) *Transport {
	return &Transport{
		codec:          codec,
		closedCh:       make(chan struct{}),
		readBuffer:     make([]byte, 0, defaultReadBufferSize),
		maxBufferBytes: defaultMaxBufferBytes,
		readTimeout:    defaultReadTimeout,
		state:          StateIdle,
	}
}

// ReadFrame blocks until one complete frame is decoded or an error occurs.
func (t *Transport) ReadFrame(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if t.codec == nil {
		return nil, ErrTransportMissingCodec
	}
	conn, err := t.ensureConn(ctx)
	if err != nil {
		return nil, err
	}
	frame, err := t.readFrameFromConn(ctx, conn)
	if err != nil {
		if shouldInvalidateConn(err) {
			t.handleConnectionFailure(conn)
		}
		return nil, err
	}
	t.markRead(time.Now())
	return frame, nil
}

// WriteFrame encodes and writes one complete frame. Concurrent writes are
// serialized internally.
func (t *Transport) WriteFrame(ctx context.Context, frame []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if t.codec == nil {
		return ErrTransportMissingCodec
	}
	conn, err := t.ensureConn(ctx)
	if err != nil {
		return err
	}
	encoded, err := t.codec.Encode(frame)
	if err != nil {
		return err
	}
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if err = t.writeFull(ctx, conn, encoded); err != nil {
		if shouldInvalidateConn(err) {
			t.handleConnectionFailure(conn)
		}
		return err
	}
	t.markWrite(time.Now())
	return nil
}

// Close closes the underlying connection and prevents future reconnects.
func (t *Transport) Close() error {
	var (
		conn     io.ReadWriteCloser
		closeErr error
	)

	t.closeOnce.Do(func() {
		t.mu.Lock()
		conn = t.conn
		t.conn = nil
		t.readBuffer = t.readBuffer[:0]
		t.readyAt = time.Time{}
		t.transitionStateLocked(StateClosed)
		close(t.closedCh)
		t.mu.Unlock()
	})
	if conn != nil {
		closeErr = conn.Close()
	}
	return closeErr
}

// State reports the current lifecycle state.
func (t *Transport) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// LastReadAt reports the last successful read time.
func (t *Transport) LastReadAt() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastReadAt
}

// LastWriteAt reports the last successful write time.
func (t *Transport) LastWriteAt() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastWriteAt
}

// IdleFor reports how long the active connection has been idle for reads.
func (t *Transport) IdleFor(now time.Time) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.conn == nil {
		return 0
	}
	baseline := t.idleBaselineLocked()
	if baseline.IsZero() || now.Before(baseline) {
		return 0
	}
	return now.Sub(baseline)
}

func (t *Transport) readFrameFromConn(ctx context.Context, conn io.ReadWriteCloser) ([]byte, error) {
	for {
		frame, consumed, err := t.codec.Decode(t.readBuffer)
		switch {
		case consumed < 0 || consumed > len(t.readBuffer):
			return nil, fmt.Errorf("invalid codec output consumed bytes %d for input %d", consumed, len(t.readBuffer))
		case err == nil && consumed == 0:
			return nil, errors.New("invalid codec output: consumed bytes cannot be 0 when decoding succeeds")
		case consumed > 0:
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

		restore, setErr := t.prepareReadDeadline(ctx, conn)
		if setErr != nil {
			return nil, setErr
		}
		buffer := make([]byte, defaultReadBufferSize)
		n, readErr := conn.Read(buffer)
		restore()
		if n > 0 {
			t.readBuffer = append(t.readBuffer, buffer[:n]...)
			if len(t.readBuffer) > t.maxBufferBytes {
				return nil, fmt.Errorf("read buffer exceeded max bytes %d", t.maxBufferBytes)
			}
		}
		if readErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			return nil, readErr
		}
	}
}

func (t *Transport) writeFull(ctx context.Context, conn io.ReadWriteCloser, data []byte) error {
	offset := 0
	for offset < len(data) {
		restore, err := t.prepareWriteDeadline(ctx, conn)
		if err != nil {
			return err
		}
		n, writeErr := conn.Write(data[offset:])
		restore()
		if n > 0 {
			offset += n
		}
		if writeErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return writeErr
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	return nil
}

func (t *Transport) prepareReadDeadline(ctx context.Context, conn io.ReadWriteCloser) (func(), error) {
	dl, ok := conn.(readDeadliner)
	if !ok {
		return func() {}, nil
	}
	return prepareDeadline(ctx, t.readTimeout, dl.SetReadDeadline)
}

func (t *Transport) prepareWriteDeadline(ctx context.Context, conn io.ReadWriteCloser) (func(), error) {
	dl, ok := conn.(writeDeadliner)
	if !ok {
		return func() {}, nil
	}
	return prepareDeadline(ctx, 0, dl.SetWriteDeadline)
}

func prepareDeadline(ctx context.Context, fallback time.Duration, setDeadline func(time.Time) error) (func(), error) {
	deadline, hasDeadline := ctx.Deadline()
	if fallback > 0 {
		timeoutDeadline := time.Now().Add(fallback)
		if !hasDeadline || timeoutDeadline.Before(deadline) {
			deadline = timeoutDeadline
			hasDeadline = true
		}
	}
	if hasDeadline {
		if err := setDeadline(deadline); err != nil {
			return nil, fmt.Errorf("set deadline failed: %w", err)
		}
		return func() {
			_ = setDeadline(time.Time{})
		}, nil
	}
	if ctx.Done() == nil {
		return func() {}, nil
	}

	stopCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = setDeadline(time.Now())
		case <-stopCh:
		}
	}()
	return func() {
		close(stopCh)
		_ = setDeadline(time.Time{})
	}, nil
}

func shouldInvalidateConn(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return false
	}
	return true
}

func (t *Transport) handleConnectionFailure(failedConn io.ReadWriteCloser) {
	if failedConn == nil {
		return
	}

	t.mu.Lock()
	if t.conn == failedConn {
		t.conn = nil
		t.readBuffer = t.readBuffer[:0]
		t.readyAt = time.Time{}
		if t.mode == transportModeDial {
			t.transitionStateLocked(StateIdle)
		} else {
			t.transitionStateLocked(StateClosed)
		}
	}
	t.mu.Unlock()

	_ = failedConn.Close()
}

func (t *Transport) markRead(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastReadAt = now
}

func (t *Transport) markWrite(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.lastWriteAt = now
}

func (t *Transport) transitionStateLocked(next State) {
	if t.state == next {
		return
	}
	t.state = next
}

func (t *Transport) idleBaselineLocked() time.Time {
	if !t.lastReadAt.IsZero() && (t.readyAt.IsZero() || !t.lastReadAt.Before(t.readyAt)) {
		return t.lastReadAt
	}
	return t.readyAt
}

// shrinkReadBufferIfNeeded compacts or resets the read buffer after bytes have
// been consumed.
func (t *Transport) shrinkReadBufferIfNeeded() {
	if len(t.readBuffer) == 0 {
		if cap(t.readBuffer) > defaultReadBufferSize*8 {
			t.readBuffer = make([]byte, 0, defaultReadBufferSize)
		}
		return
	}
	if cap(t.readBuffer) > defaultReadBufferSize*8 && len(t.readBuffer)*4 < cap(t.readBuffer) {
		t.readBuffer = bytes.Clone(t.readBuffer)
	}
}
