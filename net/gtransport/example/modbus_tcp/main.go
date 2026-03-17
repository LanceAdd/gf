package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
	"github.com/gogf/gf/v2/net/gtransport/modbus"
)

func main() {
	ctx := context.Background()

	// Listen on a random available port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	// Channel to collect server errors.
	serverErr := make(chan error, 1)

	// --- Server goroutine ---
	go func() {
		defer close(serverErr)

		conn, err := ln.Accept()
		if err != nil {
			serverErr <- fmt.Errorf("accept: %w", err)
			return
		}
		defer conn.Close()

		tr := gtransport.Wrap(conn, modbus.NewTCP(), gtransport.WithReadTimeout(5*time.Second))

		// Create a process image with 10 holding registers.
		image := modbus.NewMemoryProcessImage(0, 0, 10, 0)

		// Pre-populate holding register 0 with 0x1234.
		_ = image.WriteSingleRegister(0, 0x1234)

		// Handle 3 requests.
		for i := range 3 {
			frame, err := tr.ReadFrame(ctx)
			if err != nil {
				serverErr <- fmt.Errorf("server read frame %d: %w", i, err)
				return
			}

			req, err := modbus.ParseTCPRequest(frame)
			if err != nil {
				serverErr <- fmt.Errorf("server parse request %d: %w", i, err)
				return
			}

			// Typed request inspection.
			switch req.(type) {
			case modbus.ReadHoldingRegistersRequest:
				fmt.Println("modbus_tcp: server received modbus.ReadHoldingRegistersRequest")
			case modbus.WriteSingleRegisterRequest:
				fmt.Println("modbus_tcp: server received modbus.WriteSingleRegisterRequest")
			default:
				fmt.Printf("modbus_tcp: server received %T\n", req)
			}

			resp, err := modbus.ExecuteRequest(req, image)
			if err != nil {
				serverErr <- fmt.Errorf("server execute request %d: %w", i, err)
				return
			}

			respFrame, err := modbus.EncodeTCPResponse(resp)
			if err != nil {
				serverErr <- fmt.Errorf("server encode response %d: %w", i, err)
				return
			}

			if err := tr.WriteFrame(ctx, respFrame); err != nil {
				serverErr <- fmt.Errorf("server write frame %d: %w", i, err)
				return
			}
		}
	}()

	// --- Client ---
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	tr := gtransport.Wrap(conn, modbus.NewTCP(), gtransport.WithReadTimeout(5*time.Second))

	// Three raw Modbus TCP request frames.
	requests := []struct {
		frame []byte
		desc  string
	}{
		{
			// ReadHoldingRegisters: txID=0x0001, slaveID=0x01, FC=0x03, startAddr=0x0000, qty=0x0001
			frame: []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x01},
			desc:  "read register 0",
		},
		{
			// WriteSingleRegister: txID=0x0002, slaveID=0x01, FC=0x06, addr=0x0001, value=0xABCD
			frame: []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x06, 0x00, 0x01, 0xAB, 0xCD},
			desc:  "write register 1",
		},
		{
			// ReadHoldingRegisters: txID=0x0003, slaveID=0x01, FC=0x03, startAddr=0x0001, qty=0x0001
			frame: []byte{0x00, 0x03, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x01, 0x00, 0x01},
			desc:  "read register 1",
		},
	}

	for i, r := range requests {
		if err := tr.WriteFrame(ctx, r.frame); err != nil {
			panic(fmt.Sprintf("client write frame %d: %v", i, err))
		}

		resp, err := tr.ReadFrame(ctx)
		if err != nil {
			panic(fmt.Sprintf("client read frame %d: %v", i, err))
		}

		// Parse and print human-readable results.
		switch i {
		case 0:
			// ReadHoldingRegisters response: MBAP(7) + FC(1) + byteCount(1) + 2 bytes per register.
			// Data starts at offset 9 (7 MBAP + 1 FC + 1 byteCount).
			value := binary.BigEndian.Uint16(resp[9:11])
			fmt.Printf("modbus_tcp: client read register 0: value=0x%04X\n", value)
		case 1:
			// WriteSingleRegister response: MBAP(7) + FC(1) + addr(2) + value(2).
			value := binary.BigEndian.Uint16(resp[10:12])
			fmt.Printf("modbus_tcp: client wrote register 1: value=0x%04X\n", value)
		case 2:
			// ReadHoldingRegisters response for register 1.
			value := binary.BigEndian.Uint16(resp[9:11])
			fmt.Printf("modbus_tcp: client read register 1: value=0x%04X\n", value)
		}
	}

	// Check for server errors.
	if sErr, ok := <-serverErr; ok && sErr != nil {
		panic(fmt.Sprintf("server error: %v", sErr))
	}
}
