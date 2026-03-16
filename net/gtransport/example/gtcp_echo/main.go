// Package main demonstrates using gtransport with gtcp.Server and gtcp.Conn.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
	"github.com/gogf/gf/v2/net/gtcp"
)

func main() {
	codec := gtransport.NewLengthPrefixed(2, nil, 1024)

	// --- Server side ---
	s := gtcp.NewServer(gtcp.FreePortAddress, func(conn *gtcp.Conn) {
		defer conn.Close()

		tr := gtransport.New(conn, codec, gtransport.WithReadTimeout(5*time.Second))
		defer tr.Close()

		fmt.Println("server: transport started")
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
	})
	go s.Run()
	defer s.Close()
	time.Sleep(50 * time.Millisecond)

	addr := s.GetListenedAddress()
	fmt.Printf("server listening on %s\n", addr)

	conn, err := gtcp.NewConn(addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	tr := gtransport.New(conn, codec, gtransport.WithReadTimeout(5*time.Second))
	defer tr.Close()

	// Send messages.
	messages := []string{"hello", "gtransport", "with gtcp"}
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
		fmt.Printf("client: received %q\n", frame)
	}

	tr.Close()
	time.Sleep(50 * time.Millisecond)
}
