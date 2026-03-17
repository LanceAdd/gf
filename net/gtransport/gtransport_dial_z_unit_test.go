package gtransport

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestTransportDialPublicAPIShape(t *testing.T) {
	var _ = Dial

	type ctor func(Connector, Codec, ...DialOption) *Transport

	var _ ctor = Dial
	_ = (&Transport{}).ReadFrame
	_ = (&Transport{}).WriteFrame
	_ = (&Transport{}).Close
	_ = (&Transport{}).LastReadAt
	_ = (&Transport{}).LastWriteAt
	_ = (&Transport{}).IdleFor
	_ = (&Transport{}).State

	var connector Connector = func(context.Context) (io.ReadWriteCloser, error) {
		return nil, nil
	}
	_ = connector
	_ = State(0)
	_ = DialOption(nil)
	_ = time.Time{}
}

func TestTransportDialConnectOnFirstRead(t *testing.T) {
	var calls int

	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		calls++
		return &scriptedManagedConn{
			reads: []scriptedManagedRead{
				{data: []byte("hello|")},
			},
		}, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	frame, err := tr.ReadFrame(context.Background())
	if err != nil {
		t.Fatalf("read first frame: %v", err)
	}
	if string(frame) != "hello" {
		t.Fatalf("expected hello, got %q", frame)
	}
	if calls != 1 {
		t.Fatalf("expected one connect call, got %d", calls)
	}
}

func TestTransportDialConnectOnFirstWrite(t *testing.T) {
	var (
		calls int
		conn  = &scriptedManagedConn{}
	)

	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		calls++
		return conn, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	if err := tr.WriteFrame(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write first frame: %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected one connect call, got %d", calls)
	}
	if got := conn.writePayloads(); len(got) != 1 || !bytes.Equal(got[0], []byte("hello|")) {
		t.Fatalf("expected one encoded write hello|, got %q", got)
	}
}

func TestTransportDialReconnectAfterReadFailure(t *testing.T) {
	var (
		readErr = errors.New("read failed")
		calls   int
	)

	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		calls++
		if calls == 1 {
			return &scriptedManagedConn{
				reads: []scriptedManagedRead{
					{err: readErr},
				},
			}, nil
		}
		return &scriptedManagedConn{
			reads: []scriptedManagedRead{
				{data: []byte("world|")},
			},
		}, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	_, err := tr.ReadFrame(context.Background())
	if !errors.Is(err, readErr) {
		t.Fatalf("expected read error %v, got %v", readErr, err)
	}

	frame, err := tr.ReadFrame(context.Background())
	if err != nil {
		t.Fatalf("read after reconnect: %v", err)
	}
	if string(frame) != "world" {
		t.Fatalf("expected world, got %q", frame)
	}
	if calls != 2 {
		t.Fatalf("expected two connect calls, got %d", calls)
	}
}

func TestTransportDialReconnectAfterWriteFailure(t *testing.T) {
	var (
		writeErr = errors.New("write failed")
		calls    int
		conn1    = &scriptedManagedConn{
			writeSteps: []scriptedManagedWrite{
				{n: len("first|"), err: writeErr},
			},
		}
		conn2 = &scriptedManagedConn{}
	)

	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		calls++
		if calls == 1 {
			return conn1, nil
		}
		return conn2, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	err := tr.WriteFrame(context.Background(), []byte("first"))
	if !errors.Is(err, writeErr) {
		t.Fatalf("expected write error %v, got %v", writeErr, err)
	}

	if err = tr.WriteFrame(context.Background(), []byte("second")); err != nil {
		t.Fatalf("write after reconnect: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected two connect calls, got %d", calls)
	}
	if got := conn2.writePayloads(); len(got) != 1 || !bytes.Equal(got[0], []byte("second|")) {
		t.Fatalf("expected second connection to receive only second frame, got %q", got)
	}
}

func TestTransportDialClosePreventsReconnect(t *testing.T) {
	var calls int

	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		calls++
		return &scriptedManagedConn{}, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	if err := tr.Close(); err != nil {
		t.Fatalf("close managed transport: %v", err)
	}

	_, err := tr.ReadFrame(context.Background())
	if !errors.Is(err, ErrTransportClosed) {
		t.Fatalf("expected ErrTransportClosed, got %v", err)
	}
	if calls != 0 {
		t.Fatalf("expected no connect attempt after close, got %d", calls)
	}
}

func TestTransportDialActivityReadUpdatesLastReadAt(t *testing.T) {
	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		return &scriptedManagedConn{
			reads: []scriptedManagedRead{
				{data: []byte("hello|")},
			},
		}, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	if !tr.LastReadAt().IsZero() {
		t.Fatal("expected zero LastReadAt before reads")
	}

	frame, err := tr.ReadFrame(context.Background())
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if string(frame) != "hello" {
		t.Fatalf("expected hello, got %q", frame)
	}
	if tr.LastReadAt().IsZero() {
		t.Fatal("expected LastReadAt after successful read")
	}
}

func TestTransportDialActivityWriteUpdatesLastWriteAt(t *testing.T) {
	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		return &scriptedManagedConn{}, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	if !tr.LastWriteAt().IsZero() {
		t.Fatal("expected zero LastWriteAt before writes")
	}

	if err := tr.WriteFrame(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	if tr.LastWriteAt().IsZero() {
		t.Fatal("expected LastWriteAt after successful write")
	}
}

func TestTransportDialIdleUsesReadyAtBeforeFirstRead(t *testing.T) {
	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		return &scriptedManagedConn{}, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	if err := tr.WriteFrame(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write frame: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if idle := tr.IdleFor(time.Now()); idle <= 0 {
		t.Fatalf("expected idle duration from readyAt, got %v", idle)
	}
}

func TestTransportDialIdleUsesLastReadAtAfterSuccessfulRead(t *testing.T) {
	conn := &scriptedManagedConn{
		reads: []scriptedManagedRead{
			{data: []byte("world|")},
		},
	}
	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		return conn, nil
	}, NewDelimiter([]byte("|"), 1024, true))

	if err := tr.WriteFrame(context.Background(), []byte("hello")); err != nil {
		t.Fatalf("write frame: %v", err)
	}

	time.Sleep(25 * time.Millisecond)
	if _, err := tr.ReadFrame(context.Background()); err != nil {
		t.Fatalf("read frame: %v", err)
	}

	readAt := tr.LastReadAt()
	idle := tr.IdleFor(readAt.Add(5 * time.Millisecond))
	if idle < 4*time.Millisecond || idle > 6*time.Millisecond {
		t.Fatalf("expected idle duration near 5ms from LastReadAt, got %v", idle)
	}
}
