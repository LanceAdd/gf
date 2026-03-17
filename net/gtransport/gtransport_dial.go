package gtransport

import (
	"context"
	"errors"
	"io"
	"time"
)

var (
	// ErrTransportMissingConnector indicates that no connector was configured.
	ErrTransportMissingConnector = errors.New("transport connector is nil")
	// ErrTransportNilConnection indicates that the connector returned nil.
	ErrTransportNilConnection = errors.New("transport connector returned nil connection")
)

// Connector creates a stream connection for a dial-managed transport instance.
type Connector func(ctx context.Context) (io.ReadWriteCloser, error)

// WithConnectTimeout limits how long one connect attempt may take.
func WithConnectTimeout(d time.Duration) DialOption {
	return func(t *Transport) {
		if d > 0 {
			t.connectTimeout = d
		}
	}
}

// WithReconnectBackoff sets a backoff strategy before each connect attempt.
func WithReconnectBackoff(backoff func(attempt int) time.Duration) DialOption {
	return func(t *Transport) {
		t.reconnectBackoff = backoff
	}
}

// Dial creates a transport with lazy connection ownership and reconnectable
// future availability.
func Dial(connector Connector, codec Codec, opts ...DialOption) *Transport {
	t := newTransport(codec)
	t.mode = transportModeDial
	t.connector = connector
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Transport) ensureConn(ctx context.Context) (io.ReadWriteCloser, error) {
	if t.mode != transportModeDial {
		return t.ensureWrappedConn()
	}

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

func (t *Transport) ensureWrappedConn() (io.ReadWriteCloser, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.state == StateClosed {
		return nil, ErrTransportClosed
	}
	if t.conn == nil {
		return nil, ErrTransportClosed
	}
	return t.conn, nil
}

func (t *Transport) prepareConnect() (io.ReadWriteCloser, chan struct{}, Connector, int, error) {
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
	t.transitionStateLocked(StateConnecting)
	return nil, nil, t.connector, t.connectFailures + 1, nil
}

func (t *Transport) connectOnce(ctx context.Context, connector Connector, attempt int) (io.ReadWriteCloser, error) {
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

func (t *Transport) finishConnectSuccess(conn io.ReadWriteCloser) error {
	t.mu.Lock()
	connectingCh := t.connectingCh
	t.connectingCh = nil
	defer func() {
		t.mu.Unlock()
		if connectingCh != nil {
			close(connectingCh)
		}
	}()

	if t.state == StateClosed {
		return ErrTransportClosed
	}
	t.conn = conn
	t.readBuffer = t.readBuffer[:0]
	t.connectFailures = 0
	t.readyAt = time.Now()
	t.transitionStateLocked(StateReady)
	return nil
}

func (t *Transport) finishConnectFailure(err error) {
	t.mu.Lock()
	connectingCh := t.connectingCh
	t.connectingCh = nil
	if err != nil && !errors.Is(err, ErrTransportClosed) {
		t.connectFailures++
	}
	if t.state != StateClosed {
		t.transitionStateLocked(StateIdle)
	}
	t.mu.Unlock()

	if connectingCh != nil {
		close(connectingCh)
	}
}

func (t *Transport) connectDelay(attempt int) time.Duration {
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
