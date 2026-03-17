package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
)

func main() {
	// Start a TCP listener on a random available port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	addr := ln.Addr().String()
	messages := []string{"hello", "world", "gtransport"}
	ctx := context.Background()

	// Server goroutine: accept one connection, echo 3 frames, then exit.
	done := make(chan struct{})
	go func() {
		defer close(done)

		conn, err := ln.Accept()
		if err != nil {
			log.Fatal(err)
		}

		t := gtransport.Wrap(
			conn,
			gtransport.NewLengthPrefixed(2, binary.BigEndian, 1024),
			gtransport.WithReadTimeout(5*time.Second),
		)
		defer t.Close()

		for range messages {
			frame, err := t.ReadFrame(ctx)
			if err != nil {
				log.Fatal(err)
			}
			if err := t.WriteFrame(ctx, frame); err != nil {
				log.Fatal(err)
			}
		}

		fmt.Printf("wrap_echo: server echoed %d frames\n", len(messages))
	}()

	// Client: dial the server, send 3 messages, read back echoes.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	t := gtransport.Wrap(
		conn,
		gtransport.NewLengthPrefixed(2, binary.BigEndian, 1024),
		gtransport.WithReadTimeout(5*time.Second),
	)
	defer t.Close()

	for _, msg := range messages {
		if err := t.WriteFrame(ctx, []byte(msg)); err != nil {
			log.Fatal(err)
		}

		frame, err := t.ReadFrame(ctx)
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("wrap_echo: client received %q\n", string(frame))
	}

	// Wait for the server goroutine to finish.
	<-done
}
