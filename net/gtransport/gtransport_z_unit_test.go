package gtransport

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublicAPIShape(t *testing.T) {
	var (
		_ Codec = NewDelimiter([]byte("|"), 1024, true)
		_       = Dial
		_       = Wrap
	)
	type dialCtor func(Connector, Codec, ...DialOption) *Transport
	type wrapCtor func(io.ReadWriteCloser, Codec, ...WrapOption) *Transport
	type readFrameFunc func(context.Context) ([]byte, error)
	type writeFrameFunc func(context.Context, []byte) error
	var _ dialCtor = Dial
	var _ wrapCtor = Wrap
	var _ readFrameFunc = (&Transport{}).ReadFrame
	var _ writeFrameFunc = (&Transport{}).WriteFrame
	var _ fmt.Stringer = State(0)
	_ = (&Transport{}).Close
	_ = (&Transport{}).State
	_ = (&Transport{}).LastReadAt
	_ = (&Transport{}).LastWriteAt
	_ = (&Transport{}).IdleFor
}

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{state: StateIdle, want: "idle"},
		{state: StateConnecting, want: "connecting"},
		{state: StateReady, want: "ready"},
		{state: StateClosed, want: "closed"},
		{state: State(99), want: "state(99)"},
	}

	for _, tc := range tests {
		if got := tc.state.String(); got != tc.want {
			t.Fatalf("state %d: expected %q, got %q", tc.state, tc.want, got)
		}
	}
}

func TestTransportReadFrameDelimiter(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		_, _ = server.Write([]byte("hello|world|"))
	}()

	tr := Wrap(client, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))
	defer tr.Close()

	frame1, err := tr.ReadFrame(context.Background())
	if err != nil {
		t.Fatalf("read first frame: %v", err)
	}
	frame2, err := tr.ReadFrame(context.Background())
	if err != nil {
		t.Fatalf("read second frame: %v", err)
	}
	if string(frame1) != "hello" || string(frame2) != "world" {
		t.Fatalf("expected [hello world], got [%s %s]", frame1, frame2)
	}
}

func TestTransportWriteFrameLengthPrefixed(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := Wrap(client, NewLengthPrefixed(2, binary.BigEndian, 1024), WithReadTimeout(0))
	defer tr.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 7)
		_, _ = io.ReadFull(server, buf)
		done <- buf
	}()

	if err := tr.WriteFrame(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	got := <-done
	want := []byte{0, 5, 'h', 'e', 'l', 'l', 'o'}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestTransportWriteFrameHandlesShortWrite(t *testing.T) {
	var written bytes.Buffer
	conn := newTestConn()
	conn.writeFunc = func(p []byte) (int, error) {
		n := 2
		if len(p) < n {
			n = len(p)
		}
		written.Write(p[:n])
		return n, nil
	}
	tr := Wrap(conn, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))

	if err := tr.WriteFrame(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	if got := written.String(); got != "hello|" {
		t.Fatalf("expected full payload, got %q", got)
	}
}

func TestTransportWriteFrameSerializesConcurrentWrites(t *testing.T) {
	var (
		mu            sync.Mutex
		activeWrites  int
		maxConcurrent int
	)
	conn := newTestConn()
	conn.writeFunc = func(p []byte) (int, error) {
		mu.Lock()
		activeWrites++
		if activeWrites > maxConcurrent {
			maxConcurrent = activeWrites
		}
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)

		mu.Lock()
		activeWrites--
		mu.Unlock()
		return len(p), nil
	}
	tr := Wrap(conn, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := tr.WriteFrame(context.Background(), []byte("a")); err != nil {
			t.Errorf("write a: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := tr.WriteFrame(context.Background(), []byte("b")); err != nil {
			t.Errorf("write b: %v", err)
		}
	}()
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("expected serialized writes, max concurrent=%d", maxConcurrent)
	}
}

func TestTransportCloseUnblocksReadFrame(t *testing.T) {
	conn := newTestConn()
	tr := Wrap(conn, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))

	errCh := make(chan error, 1)
	go func() {
		_, err := tr.ReadFrame(context.Background())
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	if err := tr.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected read error after close")
		}
	case <-time.After(300 * time.Millisecond):
		t.Fatal("ReadFrame did not unblock after close")
	}
}

func TestNewLengthFieldAdvancedOption(t *testing.T) {
	codec := NewLengthField(LengthFieldOption{
		ByteOrder:           binary.BigEndian,
		MaxFrameBytes:       16,
		LengthFieldOffset:   0,
		LengthFieldLength:   2,
		InitialBytesToStrip: 2,
	})

	encoded, err := codec.Encode([]byte("ok"))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	frame, consumed, err := codec.Decode(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if consumed != len(encoded) {
		t.Fatalf("expected consumed=%d, got %d", len(encoded), consumed)
	}
	if string(frame) != "ok" {
		t.Fatalf("expected ok, got %q", string(frame))
	}
}

type testConn struct {
	readCount atomic.Int32
	closed    chan struct{}
	closeOnce sync.Once
	writeFunc func([]byte) (int, error)
}

func newTestConn() *testConn {
	return &testConn{closed: make(chan struct{})}
}

func (c *testConn) Read(_ []byte) (int, error) {
	c.readCount.Add(1)
	<-c.closed
	return 0, io.EOF
}

func (c *testConn) Write(p []byte) (int, error) {
	if c.writeFunc != nil {
		return c.writeFunc(p)
	}
	return len(p), nil
}

func (c *testConn) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
	return nil
}

func TestErrNeedMoreData(t *testing.T) {
	if !errors.Is(ErrNeedMoreData, ErrNeedMoreData) {
		t.Fatal("ErrNeedMoreData should be stable")
	}
}
