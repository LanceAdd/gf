package gtransport

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

func TestTransportWrapContextTimeoutDoesNotCloseConnection(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := Wrap(client, NewDelimiter([]byte("|"), 1024, true), WithReadTimeout(0))
	defer tr.Close()

	go func() {
		time.Sleep(40 * time.Millisecond)
		_, _ = server.Write([]byte("ok|"))
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	_, err := tr.ReadFrame(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("expected timeout error, got %v", err)
	}
	if got := tr.State(); got != StateReady {
		t.Fatalf("expected transport to remain ready after timeout, got %v", got)
	}

	frame, err := tr.ReadFrame(context.Background())
	if err != nil {
		t.Fatalf("read after timeout: %v", err)
	}
	if string(frame) != "ok" {
		t.Fatalf("expected ok, got %q", frame)
	}
}

func TestTransportWrapNilCodecReturnsError(t *testing.T) {
	tr := Wrap(nopReadWriteCloser{}, nil)

	err := tr.WriteFrame(context.Background(), []byte("hello"))
	if !errors.Is(err, ErrTransportMissingCodec) {
		t.Fatalf("expected ErrTransportMissingCodec, got %v", err)
	}
}

func TestTransportDialNilCodecReturnsError(t *testing.T) {
	tr := Dial(func(context.Context) (io.ReadWriteCloser, error) {
		return nopReadWriteCloser{}, nil
	}, nil)

	err := tr.WriteFrame(context.Background(), []byte("hello"))
	if !errors.Is(err, ErrTransportMissingCodec) {
		t.Fatalf("expected ErrTransportMissingCodec, got %v", err)
	}
}

type nopReadWriteCloser struct{}

func (nopReadWriteCloser) Read([]byte) (int, error)    { return 0, io.EOF }
func (nopReadWriteCloser) Write(p []byte) (int, error) { return len(p), nil }
func (nopReadWriteCloser) Close() error                { return nil }
