package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
)

func main() {
	const numRounds = 3

	// Start a TCP server on a random port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	// Server: accept numRounds connections, echo one frame each, then close
	// the connection — forcing the client to reconnect on the next operation.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range numRounds {
			conn, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			tr := gtransport.Wrap(conn, gtransport.NewLengthPrefixed(2, binary.BigEndian, 1024),
				gtransport.WithReadTimeout(5*time.Second))
			frame, readErr := tr.ReadFrame(context.Background())
			if readErr != nil {
				tr.Close()
				continue
			}
			_ = tr.WriteFrame(context.Background(), frame)
			tr.Close()
		}
	}()

	// Client: dial with reconnect backoff.
	connector := func(ctx context.Context) (io.ReadWriteCloser, error) {
		fmt.Println("dial_reconnect: connector dialing...")
		return net.DialTimeout("tcp", addr, 3*time.Second)
	}
	codec := gtransport.NewLengthPrefixed(2, binary.BigEndian, 1024)

	tr := gtransport.Dial(
		connector,
		codec,
		gtransport.WithConnectTimeout(3*time.Second),
		gtransport.WithReconnectBackoff(func(attempt int) time.Duration {
			d := time.Duration(attempt) * 100 * time.Millisecond
			fmt.Printf("dial_reconnect: backoff attempt %d, waiting %s\n", attempt, d)
			return d
		}),
	)
	defer tr.Close()

	ctx := context.Background()

	fmt.Printf("dial_reconnect: initial state: %s\n", tr.State())

	for i := 1; i <= numRounds; i++ {
		msg := fmt.Sprintf("message-%d", i)

		// With Dial-mode transport, a failed operation invalidates the dead
		// connection and transitions to idle. The next call to WriteFrame or
		// ReadFrame will transparently reconnect via the connector.
		for {
			if err := tr.WriteFrame(ctx, []byte(msg)); err != nil {
				fmt.Printf("dial_reconnect: write failed (will reconnect): %v\n", err)
				continue
			}
			resp, readErr := tr.ReadFrame(ctx)
			if readErr != nil {
				fmt.Printf("dial_reconnect: read failed (will reconnect): %v\n", readErr)
				continue
			}
			fmt.Printf("dial_reconnect: sent %q, received echo %q\n", msg, string(resp))
			break
		}
		fmt.Printf("dial_reconnect: state: %s\n", tr.State())
	}

	fmt.Printf("dial_reconnect: last read at: %s\n", tr.LastReadAt().Format(time.RFC3339Nano))
	fmt.Printf("dial_reconnect: idle for: %s\n", tr.IdleFor(time.Now()))

	wg.Wait()
}
