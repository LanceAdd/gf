// Package main demonstrates using gtransport with standard library net.Conn
// to build a TCP echo server and client.
package main

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
)

func main() {
	// Start echo server.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()
	fmt.Printf("server listening on %s\n", addr)

	codec := gtransport.NewLengthPrefixed(2, nil, 1024)

	// Server goroutine: accept one connection and echo frames.
	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			return
		}
		defer conn.Close()

		tr := gtransport.New(conn, codec, gtransport.WithReadTimeout(5*time.Second))
		defer tr.Close()

		fmt.Println("server: transport started, waiting for frames...")
		for {
			frame, err := tr.ReadFrame()
			if err != nil {
				fmt.Println("server: client disconnected")
				return
			}
			fmt.Printf("server: echoing %q\n", frame)
			if err := tr.WriteFrame(frame); err != nil {
				log.Printf("server send error: %v", err)
				return
			}
		}
	}()

	// Give server a moment to start accepting.
	time.Sleep(50 * time.Millisecond)

	// Client: connect, send messages, receive echoes.
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	tr := gtransport.New(conn, codec, gtransport.WithReadTimeout(5*time.Second))
	defer tr.Close()

	// Send three messages.
	messages := []string{"hello", "gtransport", "works!"}
	for _, msg := range messages {
		fmt.Printf("client: sending %q\n", msg)
		if err := tr.WriteFrame([]byte(msg)); err != nil {
			log.Fatalf("client send error: %v", err)
		}
	}

	// Receive echoed responses.
	for i := 0; i < len(messages); i++ {
		frame, err := tr.ReadFrame()
		if err != nil {
			log.Fatalf("client read error: %v", err)
		}
		fmt.Printf("client: received echo %q\n", frame)
	}

	// Close client transport (triggers server side EOF).
	tr.Close()

	// Wait for server to finish.
	<-serverDone
}
