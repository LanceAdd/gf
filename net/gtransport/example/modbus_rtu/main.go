package main

import (
	"context"
	"fmt"
	"net"

	"github.com/gogf/gf/v2/net/gtransport"
	"github.com/gogf/gf/v2/net/gtransport/modbus"
)

func main() {
	ctx := context.Background()

	// net.Pipe provides a synchronous in-memory connection pair.
	serverConn, clientConn := net.Pipe()

	// Create process image with 10 coils.
	image := modbus.NewMemoryProcessImage(10, 0, 0, 0)

	// Wrap both sides with the RTU codec.
	// The RTU codec handles CRC transparently:
	//   Encode: appends CRC to outgoing payload
	//   Decode: validates and strips CRC from incoming data
	// Disable read timeout because net.Pipe does not support deadlines.
	serverTr := gtransport.Wrap(serverConn, modbus.NewRTU(), gtransport.WithReadTimeout(0))
	clientTr := gtransport.Wrap(clientConn, modbus.NewRTU(), gtransport.WithReadTimeout(0))

	// Channel to collect server errors.
	errCh := make(chan error, 3)

	// Server goroutine: handle 3 requests.
	go func() {
		defer serverTr.Close()
		for i := range 3 {
			// ReadFrame returns CRC-stripped RTU payload.
			payload, err := serverTr.ReadFrame(ctx)
			if err != nil {
				errCh <- fmt.Errorf("server read %d: %w", i, err)
				return
			}
			req, err := modbus.ParseRTURequestPayload(payload)
			if err != nil {
				errCh <- fmt.Errorf("server parse %d: %w", i, err)
				return
			}
			resp, err := modbus.ExecuteRequest(req, image)
			if err != nil {
				errCh <- fmt.Errorf("server execute %d: %w", i, err)
				return
			}
			respPayload, err := modbus.EncodeRTUResponsePayload(resp)
			if err != nil {
				errCh <- fmt.Errorf("server encode %d: %w", i, err)
				return
			}
			// WriteFrame passes the payload through the RTU codec which adds CRC.
			if err = serverTr.WriteFrame(ctx, respPayload); err != nil {
				errCh <- fmt.Errorf("server write %d: %w", i, err)
				return
			}
		}
		errCh <- nil
	}()

	// --- Request 1: WriteSingleCoil, coil 3 = ON ---
	// RTU payload (no CRC): slaveID=0x01, FC=0x05, addr=0x0003, value=0xFF00
	req1 := []byte{0x01, 0x05, 0x00, 0x03, 0xFF, 0x00}
	if err := clientTr.WriteFrame(ctx, req1); err != nil {
		panic(err)
	}
	resp1, err := clientTr.ReadFrame(ctx)
	if err != nil {
		panic(err)
	}
	// WriteSingleCoil echo response: same as request payload.
	if resp1[1] == 0x05 && resp1[4] == 0xFF {
		fmt.Println("modbus_rtu: wrote coil 3 = ON")
	}

	// --- Request 2: ReadCoils, start=0x0000, quantity=10 ---
	// RTU payload (no CRC): slaveID=0x01, FC=0x01, startAddr=0x0000, qty=0x000A
	req2 := []byte{0x01, 0x01, 0x00, 0x00, 0x00, 0x0A}
	if err := clientTr.WriteFrame(ctx, req2); err != nil {
		panic(err)
	}
	resp2, err := clientTr.ReadFrame(ctx)
	if err != nil {
		panic(err)
	}
	// ReadCoils response: slaveID, FC, byteCount, data...
	// For 10 coils we get 2 bytes of coil data starting at index 3.
	byteCount := int(resp2[2])
	coils := make([]bool, 10)
	for i := range 10 {
		byteIdx := i / 8
		bitIdx := uint(i % 8)
		if byteIdx < byteCount {
			coils[i] = (resp2[3+byteIdx]>>bitIdx)&1 == 1
		}
	}
	fmt.Printf("modbus_rtu: read 10 coils: %v\n", coils)

	// --- Request 3: WriteSingleCoil, coil 7 = ON ---
	// RTU payload (no CRC): slaveID=0x01, FC=0x05, addr=0x0007, value=0xFF00
	req3 := []byte{0x01, 0x05, 0x00, 0x07, 0xFF, 0x00}
	if err := clientTr.WriteFrame(ctx, req3); err != nil {
		panic(err)
	}
	resp3, err := clientTr.ReadFrame(ctx)
	if err != nil {
		panic(err)
	}
	if resp3[1] == 0x05 && resp3[4] == 0xFF {
		fmt.Println("modbus_rtu: wrote coil 7 = ON")
	}

	// Wait for server to finish and check for errors.
	if srvErr := <-errCh; srvErr != nil {
		panic(srvErr)
	}

	clientTr.Close()
}
