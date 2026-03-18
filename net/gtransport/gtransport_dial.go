package gtransport

import (
	"context"
	"errors"
	"io"
	"sync"
	"time"
)

const defaultPollInterval = 1 * time.Second

var (
	// ErrTransportMissingConnector indicates that no connector was configured.
	ErrTransportMissingConnector = errors.New("transport connector is nil")
	// ErrTransportNilConnection indicates that the connector returned nil.
	ErrTransportNilConnection = errors.New("transport connector returned nil connection")
)

// Connector creates a stream connection for a dial-managed transport instance.
type Connector func(ctx context.Context) (io.ReadWriteCloser, error)

// Compile-time interface checks.
var (
	_ FrameTransport = (*DialTransport)(nil)
	_ Observable     = (*DialTransport)(nil)
)

// DialOption configures a DialTransport.
type DialOption func(*DialTransport)

// WithDialReadTimeout sets the timeout applied to each underlying read while
// waiting for a complete frame. Set it to 0 to disable read deadlines.
func WithDialReadTimeout(d time.Duration) DialOption {
	return func(t *DialTransport) {
		t.readTimeout = d
	}
}

// WithDialMaxBufferBytes limits the internal read buffer size. It protects
// against unbounded growth when the peer sends incomplete or invalid frames.
func WithDialMaxBufferBytes(n int) DialOption {
	return func(t *DialTransport) {
		if n > 0 {
			t.maxBufferBytes = n
		}
	}
}

// WithConnectTimeout limits how long one connect attempt may take.
func WithConnectTimeout(d time.Duration) DialOption {
	return func(t *DialTransport) {
		if d > 0 {
			t.connectTimeout = d
		}
	}
}

// WithReconnectBackoff sets a backoff strategy before each connect attempt.
func WithReconnectBackoff(backoff func(attempt int) time.Duration) DialOption {
	return func(t *DialTransport) {
		t.reconnectBackoff = backoff
	}
}

// WithMinStableDuration sets the minimum time a connection must remain open
// to be considered stable. If a connection fails before this duration,
// connectFailures is incremented instead of reset, preserving backoff.
func WithMinStableDuration(d time.Duration) DialOption {
	return func(t *DialTransport) {
		t.minStableDuration = d
	}
}

// WithIdleTimeout sets the idle duration after which the onIdle callback
// fires. Unlike IdleFor (which resets on reconnect), idle detection is based on
// the last successfully received frame, so it remains accurate across
// disconnections. Set to 0 to disable idle detection.
func WithIdleTimeout(d time.Duration) DialOption {
	return func(t *DialTransport) {
		t.idleTimeout = d
	}
}

// WithOnIdle sets the callback invoked when no complete frame has been
// received for longer than the configured idle timeout.
// Called from the watchdog goroutine; must not block.
func WithOnIdle(fn func()) DialOption {
	return func(t *DialTransport) {
		t.onIdle = fn
	}
}

// WithOnConnect sets the callback invoked when a connection is established.
// The first parameter is true for the initial connection and false for
// subsequent reconnections. Called from the watchdog goroutine; must not block.
func WithOnConnect(fn func(first bool)) DialOption {
	return func(t *DialTransport) {
		t.onConnect = fn
	}
}

// WithOnConnectionLost sets the callback invoked when an active connection
// fails. Called from the watchdog goroutine; must not block.
func WithOnConnectionLost(fn func(err error)) DialOption {
	return func(t *DialTransport) {
		t.onConnectionLost = fn
	}
}

// WithOnConnectFail sets the callback invoked when a connection attempt
// fails. Called from the watchdog goroutine; must not block.
func WithOnConnectFail(fn func(err error)) DialOption {
	return func(t *DialTransport) {
		t.onConnectFail = fn
	}
}

// WithPollInterval sets how frequently the watchdog checks idle duration
// and triggers reconnection. Defaults to 1 second.
func WithPollInterval(d time.Duration) DialOption {
	return func(t *DialTransport) {
		if d > 0 {
			t.pollInterval = d
		}
	}
}

// ---------------------------------------------------------------------------
// Connection events
// ---------------------------------------------------------------------------

type dialEventKind int

const (
	dialEventConnected      dialEventKind = iota // connection established
	dialEventConnectionLost                      // active connection failed
	dialEventConnectFailed                       // connect attempt failed
)

type dialEvent struct {
	kind dialEventKind
	err  error
}

// DialTransport performs synchronous framed reads and writes with automatic
// reconnection on failure. A background watchdog goroutine monitors connection
// state and fires event callbacks.
type DialTransport struct {
	frameIO
	connector Connector

	mu              sync.Mutex
	writeMu         sync.Mutex
	closeOnce       sync.Once
	closedCh        chan struct{}
	state           State
	conn            io.ReadWriteCloser
	connectingCh    chan struct{}
	connectFailures int

	connectTimeout    time.Duration
	reconnectBackoff  func(attempt int) time.Duration
	minStableDuration time.Duration

	lastReadAt     time.Time
	lastWriteAt    time.Time
	readyAt        time.Time
	firstConnectAt time.Time
	lastFrameAt    time.Time // survives reconnects; never cleared by disconnect

	// Watchdog and event configuration.
	idleTimeout      time.Duration
	onIdle           func()
	onConnect        func(first bool)
	onConnectionLost func(err error)
	onConnectFail    func(err error)
	pollInterval     time.Duration

	// Watchdog state.
	lastIdleFired time.Time
	everConnected bool
	events        chan dialEvent
	watchCancel   context.CancelFunc
	stopCh        chan struct{}
	done          chan struct{}
}

// Dial creates a DialTransport with automatic reconnection. A background
// watchdog goroutine is started immediately; it monitors connection state,
// triggers reconnection, and fires configured event callbacks. The provided
// ctx controls the lifetime of the watchdog.
func Dial(ctx context.Context, connector Connector, codec Codec, opts ...DialOption) *DialTransport {
	t := &DialTransport{
		frameIO:           newFrameIO(codec),
		connector:         connector,
		closedCh:          make(chan struct{}),
		state:             StateIdle,
		minStableDuration: 10 * time.Second,
		pollInterval:      defaultPollInterval,
		events:            make(chan dialEvent, 8),
		stopCh:            make(chan struct{}),
		done:              make(chan struct{}),
	}
	for _, opt := range opts {
		opt(t)
	}
	watchCtx, watchCancel := context.WithCancel(ctx)
	t.watchCancel = watchCancel
	go t.watchLoop(watchCtx)
	return t
}

// ReadFrame blocks until one complete frame is decoded or an error occurs.
//
// ReadFrame is NOT safe for concurrent use by multiple goroutines. Callers
// must serialize ReadFrame calls externally (e.g., a single read-loop
// goroutine). WriteFrame, by contrast, is internally serialized.
func (t *DialTransport) ReadFrame(ctx context.Context) ([]byte, error) {
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
			t.handleConnectionFailure(conn, err)
		}
		return nil, err
	}
	t.markRead(time.Now())
	return frame, nil
}

// WriteFrame encodes and writes one complete frame. Concurrent writes are
// serialized internally.
func (t *DialTransport) WriteFrame(ctx context.Context, frame []byte) error {
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
			t.handleConnectionFailure(conn, err)
		}
		return err
	}
	t.markWrite(time.Now())
	return nil
}

// Close stops the watchdog goroutine and closes the underlying connection.
func (t *DialTransport) Close() error {
	var closeErr error
	t.closeOnce.Do(func() {
		t.watchCancel() // unblock any in-flight connection attempt
		close(t.stopCh)
		<-t.done // wait for watchdog to exit

		t.mu.Lock()
		conn := t.conn
		t.conn = nil
		t.readBuffer = t.readBuffer[:0]
		t.readyAt = time.Time{}
		t.state = StateClosed
		close(t.closedCh)
		t.mu.Unlock()

		if conn != nil {
			closeErr = conn.Close()
		}
	})
	return closeErr
}

// State reports the current lifecycle state.
func (t *DialTransport) State() State {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.state
}

// LastReadAt reports the last successful read time.
func (t *DialTransport) LastReadAt() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastReadAt
}

// LastWriteAt reports the last successful write time.
func (t *DialTransport) LastWriteAt() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastWriteAt
}

// LastFrameAt reports the last time a complete frame was successfully read.
// Unlike LastReadAt, this timestamp survives reconnects and is never cleared,
// making it suitable for business-level "time since last data" monitoring.
func (t *DialTransport) LastFrameAt() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastFrameAt
}

// IdleFor reports how long the active connection has been idle for reads.
func (t *DialTransport) IdleFor(now time.Time) time.Duration {
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

// EnsureConnected triggers a connection attempt if the transport is idle.
// It is a no-op when already connected.
func (t *DialTransport) EnsureConnected(ctx context.Context) error {
	_, err := t.ensureConn(ctx)
	return err
}

// ---------------------------------------------------------------------------
// Watchdog
// ---------------------------------------------------------------------------

func (t *DialTransport) watchLoop(ctx context.Context) {
	defer close(t.done)

	ticker := time.NewTicker(t.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.stopCh:
			return
		case ev := <-t.events:
			t.dispatchEvent(ev)
		case <-ticker.C:
			t.watchTick(ctx)
		}
	}
}

func (t *DialTransport) dispatchEvent(ev dialEvent) {
	switch ev.kind {
	case dialEventConnected:
		first := !t.everConnected
		t.everConnected = true
		if t.onConnect != nil {
			t.onConnect(first)
		}
	case dialEventConnectionLost:
		if t.onConnectionLost != nil {
			t.onConnectionLost(ev.err)
		}
	case dialEventConnectFailed:
		if t.onConnectFail != nil {
			t.onConnectFail(ev.err)
		}
	}
}

func (t *DialTransport) watchTick(ctx context.Context) {
	// Auto-reconnect when idle.
	if t.State() == StateIdle {
		_ = t.EnsureConnected(ctx)
		// Events (success/failure) are dispatched via the events channel,
		// not inferred here.
	}

	t.checkIdle()
}

func (t *DialTransport) checkIdle() {
	if t.idleTimeout <= 0 || t.onIdle == nil {
		return
	}

	// Use lastFrameAt — survives reconnects, never cleared by disconnect.
	lastFrame := t.LastFrameAt()
	now := time.Now()

	// No frame ever received: measure from the first successful connection and
	// keep that baseline across later disconnects/reconnects.
	if lastFrame.IsZero() {
		t.mu.Lock()
		firstConnectAt := t.firstConnectAt
		t.mu.Unlock()
		if firstConnectAt.IsZero() {
			return
		}
		if now.Sub(firstConnectAt) < t.idleTimeout {
			return
		}
	} else if now.Sub(lastFrame) < t.idleTimeout {
		return
	}

	// Deduplicate: only fire once per idle period.
	// A new frame must arrive before we fire again.
	t.mu.Lock()
	if !t.lastIdleFired.IsZero() && (lastFrame.IsZero() || !lastFrame.After(t.lastIdleFired)) {
		t.mu.Unlock()
		return
	}
	t.lastIdleFired = now
	t.mu.Unlock()

	t.onIdle()
}

// sendEvent sends a connection event to the watchdog non-blocking.
// Must be called after releasing mu.
func (t *DialTransport) sendEvent(ev dialEvent) {
	select {
	case t.events <- ev:
	default:
		// Channel full — watchdog is behind; drop oldest to make room.
		select {
		case <-t.events:
		default:
		}
		select {
		case t.events <- ev:
		default:
		}
	}
}

// ---------------------------------------------------------------------------
// Connection management internals
// ---------------------------------------------------------------------------

func (t *DialTransport) ensureConn(ctx context.Context) (io.ReadWriteCloser, error) {
	for {
		conn, waitCh, connector, attempt, err := t.prepareConnect()
		switch {
		case err != nil:
			return nil, err
		case conn != nil:
			return conn, nil
		case waitCh != nil:
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-t.closedCh:
				return nil, ErrTransportClosed
			case <-waitCh:
				continue
			}
		default:
			return t.connectOnce(ctx, connector, attempt)
		}
	}
}

func (t *DialTransport) prepareConnect() (io.ReadWriteCloser, chan struct{}, Connector, int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == StateClosed {
		return nil, nil, nil, 0, ErrTransportClosed
	}
	if t.conn != nil {
		return t.conn, nil, nil, 0, nil
	}
	if t.connectingCh != nil {
		return nil, t.connectingCh, nil, 0, nil
	}
	if t.connector == nil {
		return nil, nil, nil, 0, ErrTransportMissingConnector
	}

	t.connectingCh = make(chan struct{})
	t.state = StateConnecting
	return nil, nil, t.connector, t.connectFailures + 1, nil
}

func (t *DialTransport) connectOnce(ctx context.Context, connector Connector, attempt int) (io.ReadWriteCloser, error) {
	if delay := t.connectDelay(attempt); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			t.finishConnectFailure(ctx.Err())
			return nil, ctx.Err()
		case <-t.closedCh:
			t.finishConnectFailure(ErrTransportClosed)
			return nil, ErrTransportClosed
		case <-timer.C:
		}
	}

	connectCtx := ctx
	cancel := func() {}
	if t.connectTimeout > 0 {
		connectCtx, cancel = context.WithTimeout(ctx, t.connectTimeout)
	}
	defer cancel()

	conn, err := connector(connectCtx)
	if err == nil && conn == nil {
		err = ErrTransportNilConnection
	}
	if err != nil {
		t.finishConnectFailure(err)
		return nil, err
	}
	if err = t.finishConnectSuccess(conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return conn, nil
}

func (t *DialTransport) finishConnectSuccess(conn io.ReadWriteCloser) error {
	t.mu.Lock()
	connectingCh := t.connectingCh
	t.connectingCh = nil

	if t.state == StateClosed {
		t.mu.Unlock()
		if connectingCh != nil {
			close(connectingCh)
		}
		return ErrTransportClosed
	}
	t.conn = conn
	t.readBuffer = t.readBuffer[:0]
	t.readyAt = time.Now()
	if t.firstConnectAt.IsZero() {
		t.firstConnectAt = t.readyAt
	}
	t.lastReadAt = time.Time{}
	t.lastWriteAt = time.Time{}
	t.state = StateReady
	t.mu.Unlock()

	if connectingCh != nil {
		close(connectingCh)
	}
	t.sendEvent(dialEvent{kind: dialEventConnected})
	return nil
}

func (t *DialTransport) finishConnectFailure(err error) {
	t.mu.Lock()
	connectingCh := t.connectingCh
	t.connectingCh = nil
	if shouldCountConnectFailure(err) {
		t.connectFailures++
	}
	if t.state != StateClosed {
		t.state = StateIdle
	}
	t.mu.Unlock()

	if connectingCh != nil {
		close(connectingCh)
	}
	if shouldEmitConnectFailEvent(err) {
		t.sendEvent(dialEvent{kind: dialEventConnectFailed, err: err})
	}
}

func (t *DialTransport) connectDelay(attempt int) time.Duration {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.reconnectBackoff == nil {
		return 0
	}
	d := t.reconnectBackoff(attempt)
	if d < 0 {
		return 0
	}
	return d
}

func (t *DialTransport) handleConnectionFailure(failedConn io.ReadWriteCloser, cause error) {
	if failedConn == nil {
		return
	}

	t.mu.Lock()
	matched := t.conn == failedConn
	if matched {
		if !t.readyAt.IsZero() && time.Since(t.readyAt) >= t.minStableDuration {
			t.connectFailures = 0
		} else {
			t.connectFailures++
		}
		t.conn = nil
		t.readBuffer = t.readBuffer[:0]
		t.readyAt = time.Time{}
		t.lastReadAt = time.Time{}
		t.lastWriteAt = time.Time{}
		t.state = StateIdle
	}
	t.mu.Unlock()

	_ = failedConn.Close()
	if matched {
		t.sendEvent(dialEvent{kind: dialEventConnectionLost, err: cause})
	}
}

func shouldCountConnectFailure(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTransportClosed) || errors.Is(err, context.Canceled) {
		return false
	}
	return true
}

func shouldEmitConnectFailEvent(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrTransportClosed) || errors.Is(err, context.Canceled) {
		return false
	}
	return true
}

func (t *DialTransport) markRead(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastReadAt = now
	t.lastFrameAt = now
}

func (t *DialTransport) markWrite(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.lastWriteAt = now
}

func (t *DialTransport) idleBaselineLocked() time.Time {
	if !t.lastReadAt.IsZero() && (t.readyAt.IsZero() || !t.lastReadAt.Before(t.readyAt)) {
		return t.lastReadAt
	}
	return t.readyAt
}
