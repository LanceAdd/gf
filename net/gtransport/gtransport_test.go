package gtransport

import (
	"bytes"
	"encoding/binary"
	"errors"
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
		_ = New
	)
	type ctor func(io.ReadWriteCloser, Codec, ...Option) *Transport
	var _ ctor = New
	_ = (&Transport{}).ReadFrame
	_ = (&Transport{}).WriteFrame
	_ = (&Transport{}).Close
}

func TestTransportReadFrameDelimiter(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		_, _ = server.Write([]byte("hello|world|"))
	}()

	tr := New(client, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))
	defer tr.Close()

	frame1, err := tr.ReadFrame()
	if err != nil {
		t.Fatalf("read first frame: %v", err)
	}
	frame2, err := tr.ReadFrame()
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

	tr := New(client, NewLengthPrefixed(2, binary.BigEndian, 1024), WithReadTimeout(0))
	defer tr.Close()

	done := make(chan []byte, 1)
	go func() {
		buf := make([]byte, 7)
		_, _ = io.ReadFull(server, buf)
		done <- buf
	}()

	if err := tr.WriteFrame([]byte("hello")); err != nil {
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
	conn := &testConn{
		writeFunc: func(p []byte) (int, error) {
			n := 2
			if len(p) < n {
				n = len(p)
			}
			written.Write(p[:n])
			return n, nil
		},
	}
	tr := New(conn, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))

	if err := tr.WriteFrame([]byte("hello")); err != nil {
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
	conn := &testConn{
		writeFunc: func(p []byte) (int, error) {
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
		},
	}
	tr := New(conn, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		if err := tr.WriteFrame([]byte("a")); err != nil {
			t.Errorf("write a: %v", err)
		}
	}()
	go func() {
		defer wg.Done()
		if err := tr.WriteFrame([]byte("b")); err != nil {
			t.Errorf("write b: %v", err)
		}
	}()
	wg.Wait()

	if maxConcurrent != 1 {
		t.Fatalf("expected serialized writes, max concurrent=%d", maxConcurrent)
	}
}

func TestTransportCloseUnblocksReadFrame(t *testing.T) {
	conn := &testConn{}
	tr := New(conn, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))

	errCh := make(chan error, 1)
	go func() {
		_, err := tr.ReadFrame()
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

func (c *testConn) Read(_ []byte) (int, error) {
	c.readCount.Add(1)
	if c.closed == nil {
		c.closed = make(chan struct{})
	}
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
	if c.closed == nil {
		c.closed = make(chan struct{})
	}
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
