// Copyright GoFrame Author(https://goframe.org). All Rights Reserved.
//
// This Source Code Form is subject to the terms of the MIT License.
// If a copy of the MIT was not distributed with this file,
// You can obtain one at https://github.com/gogf/gf.

// Package gtransport provides framed I/O over stream connections.
//
// Core package responsibilities:
//   - Codec defines frame discovery and encoding rules
//   - WrapTransport provides synchronous ReadFrame/WriteFrame on a fixed connection
//   - DialTransport adds automatic reconnection, backoff, idle detection, and event callbacks
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

// FrameTransport is the common interface for framed transports.
// *WrapTransport and *DialTransport both satisfy it.
type FrameTransport interface {
	ReadFrame(ctx context.Context) ([]byte, error)
	WriteFrame(ctx context.Context, frame []byte) error
	Close() error
	State() State
}

// Observable provides read/write activity timestamps and idle duration.
// *WrapTransport and *DialTransport both satisfy it.
type Observable interface {
	IdleFor(now time.Time) time.Duration
	LastReadAt() time.Time
	LastWriteAt() time.Time
}

// State represents the lifecycle state of a transport.
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

// ---------------------------------------------------------------------------
// frameIO — shared framed I/O logic embedded by WrapTransport and DialTransport
// ---------------------------------------------------------------------------

type frameIO struct {
	codec          Codec
	readBuffer     []byte
	maxBufferBytes int
	readTimeout    time.Duration
}

func newFrameIO(codec Codec) frameIO {
	return frameIO{
		codec:          codec,
		readBuffer:     make([]byte, 0, defaultReadBufferSize),
		maxBufferBytes: defaultMaxBufferBytes,
		readTimeout:    defaultReadTimeout,
	}
}

func (f *frameIO) readFrameFromConn(ctx context.Context, conn io.ReadWriteCloser) ([]byte, error) {
	var pendingReadErr error
	for {
		frame, consumed, err := f.codec.Decode(f.readBuffer)
		switch {
		case consumed < 0 || consumed > len(f.readBuffer):
			return nil, fmt.Errorf("invalid codec output consumed bytes %d for input %d", consumed, len(f.readBuffer))
		case err == nil && consumed == 0:
			return nil, errors.New("invalid codec output: consumed bytes cannot be 0 when decoding succeeds")
		case consumed > 0:
			f.readBuffer = f.readBuffer[consumed:]
			f.shrinkReadBufferIfNeeded()
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
		if pendingReadErr != nil {
			return nil, pendingReadErr
		}

		restore, setErr := f.prepareReadDeadline(ctx, conn)
		if setErr != nil {
			return nil, setErr
		}
		buffer := make([]byte, defaultReadBufferSize)
		n, readErr := conn.Read(buffer)
		restore()
		if n > 0 {
			f.readBuffer = append(f.readBuffer, buffer[:n]...)
			if len(f.readBuffer) > f.maxBufferBytes {
				return nil, fmt.Errorf("read buffer exceeded max bytes %d", f.maxBufferBytes)
			}
		}
		if readErr != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			if n > 0 {
				pendingReadErr = readErr
				continue
			}
			return nil, readErr
		}
	}
}

func (f *frameIO) writeFull(ctx context.Context, conn io.ReadWriteCloser, data []byte) error {
	offset := 0
	for offset < len(data) {
		restore, err := f.prepareWriteDeadline(ctx, conn)
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

func (f *frameIO) prepareReadDeadline(ctx context.Context, conn io.ReadWriteCloser) (func(), error) {
	dl, ok := conn.(readDeadliner)
	if !ok {
		return func() {}, nil
	}
	return prepareDeadline(ctx, f.readTimeout, dl.SetReadDeadline)
}

func (f *frameIO) prepareWriteDeadline(ctx context.Context, conn io.ReadWriteCloser) (func(), error) {
	dl, ok := conn.(writeDeadliner)
	if !ok {
		return func() {}, nil
	}
	return prepareDeadline(ctx, 0, dl.SetWriteDeadline)
}

// shrinkReadBufferIfNeeded compacts or resets the read buffer after bytes have
// been consumed.
func (f *frameIO) shrinkReadBufferIfNeeded() {
	if len(f.readBuffer) == 0 {
		if cap(f.readBuffer) > defaultReadBufferSize*8 {
			f.readBuffer = make([]byte, 0, defaultReadBufferSize)
		}
		return
	}
	if cap(f.readBuffer) > defaultReadBufferSize*8 && len(f.readBuffer)*4 < cap(f.readBuffer) {
		f.readBuffer = bytes.Clone(f.readBuffer)
	}
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// WrapTransport — fixed-connection framed transport
// ---------------------------------------------------------------------------

// Compile-time interface checks.
var (
	_ FrameTransport = (*WrapTransport)(nil)
	_ Observable     = (*WrapTransport)(nil)
)

// WrapOption configures a WrapTransport.
type WrapOption func(*WrapTransport)

// WithReadTimeout sets the timeout applied to each underlying read while
// waiting for a complete frame. Set it to 0 to disable read deadlines.
func WithReadTimeout(d time.Duration) WrapOption {
	return func(w *WrapTransport) {
		w.readTimeout = d
	}
}

// WithMaxBufferBytes limits the internal read buffer size. It protects against
// unbounded growth when the peer sends incomplete or invalid frames.
func WithMaxBufferBytes(n int) WrapOption {
	return func(w *WrapTransport) {
		if n > 0 {
			w.maxBufferBytes = n
		}
	}
}

// WrapTransport performs synchronous framed reads and writes on a fixed,
// externally-managed connection. Connection failures are permanent.
type WrapTransport struct {
	frameIO

	mu        sync.Mutex
	writeMu   sync.Mutex
	closeOnce sync.Once
	state     State
	conn      io.ReadWriteCloser

	lastReadAt  time.Time
	lastWriteAt time.Time
	readyAt     time.Time
}

// Wrap creates a WrapTransport bound to an already-open connection.
func Wrap(conn io.ReadWriteCloser, codec Codec, opts ...WrapOption) *WrapTransport {
	w := &WrapTransport{
		frameIO: newFrameIO(codec),
		state:   StateClosed,
	}
	if conn != nil {
		w.conn = conn
		w.state = StateReady
		w.readyAt = time.Now()
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// ReadFrame blocks until one complete frame is decoded or an error occurs.
func (w *WrapTransport) ReadFrame(ctx context.Context) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if w.codec == nil {
		return nil, ErrTransportMissingCodec
	}
	conn, err := w.getConn()
	if err != nil {
		return nil, err
	}
	frame, err := w.readFrameFromConn(ctx, conn)
	if err != nil {
		if shouldInvalidateConn(err) {
			w.handleConnectionFailure(conn)
		}
		return nil, err
	}
	w.markRead(time.Now())
	return frame, nil
}

// WriteFrame encodes and writes one complete frame. Concurrent writes are
// serialized internally.
func (w *WrapTransport) WriteFrame(ctx context.Context, frame []byte) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if w.codec == nil {
		return ErrTransportMissingCodec
	}
	conn, err := w.getConn()
	if err != nil {
		return err
	}
	encoded, err := w.codec.Encode(frame)
	if err != nil {
		return err
	}
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	if err = w.writeFull(ctx, conn, encoded); err != nil {
		if shouldInvalidateConn(err) {
			w.handleConnectionFailure(conn)
		}
		return err
	}
	w.markWrite(time.Now())
	return nil
}

// Close closes the underlying connection and marks the transport as closed.
func (w *WrapTransport) Close() error {
	var (
		conn     io.ReadWriteCloser
		closeErr error
	)
	w.closeOnce.Do(func() {
		w.mu.Lock()
		conn = w.conn
		w.conn = nil
		w.readBuffer = w.readBuffer[:0]
		w.readyAt = time.Time{}
		w.state = StateClosed
		w.mu.Unlock()
	})
	if conn != nil {
		closeErr = conn.Close()
	}
	return closeErr
}

// State reports the current lifecycle state.
func (w *WrapTransport) State() State {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.state
}

// LastReadAt reports the last successful read time.
func (w *WrapTransport) LastReadAt() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastReadAt
}

// LastWriteAt reports the last successful write time.
func (w *WrapTransport) LastWriteAt() time.Time {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.lastWriteAt
}

// IdleFor reports how long the active connection has been idle for reads.
func (w *WrapTransport) IdleFor(now time.Time) time.Duration {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.conn == nil {
		return 0
	}
	baseline := w.idleBaselineLocked()
	if baseline.IsZero() || now.Before(baseline) {
		return 0
	}
	return now.Sub(baseline)
}

func (w *WrapTransport) getConn() (io.ReadWriteCloser, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.state == StateClosed {
		return nil, ErrTransportClosed
	}
	if w.conn == nil {
		return nil, ErrTransportClosed
	}
	return w.conn, nil
}

func (w *WrapTransport) handleConnectionFailure(failedConn io.ReadWriteCloser) {
	if failedConn == nil {
		return
	}

	w.mu.Lock()
	if w.conn == failedConn {
		w.conn = nil
		w.readBuffer = w.readBuffer[:0]
		w.readyAt = time.Time{}
		w.lastReadAt = time.Time{}
		w.lastWriteAt = time.Time{}
		w.state = StateClosed
	}
	w.mu.Unlock()

	_ = failedConn.Close()
}

func (w *WrapTransport) markRead(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastReadAt = now
}

func (w *WrapTransport) markWrite(now time.Time) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.lastWriteAt = now
}

func (w *WrapTransport) idleBaselineLocked() time.Time {
	if !w.lastReadAt.IsZero() && (w.readyAt.IsZero() || !w.lastReadAt.Before(w.readyAt)) {
		return w.lastReadAt
	}
	return w.readyAt
}
