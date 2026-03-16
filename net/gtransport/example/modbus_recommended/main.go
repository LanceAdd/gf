package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
	"github.com/gogf/gf/v2/net/gtransport/modbus"
)

func main() {
	if err := runRecommendedExample(); err != nil {
		log.Fatal(err)
	}
}

func runRecommendedExample() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	defer ln.Close()

	image := modbus.NewMemoryProcessImage(1, 1, 1, 1)
	if err = image.WriteSingleRegister(0, 0x1234); err != nil {
		return err
	}

	serverErrCh := make(chan error, 1)
	go func() {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			serverErrCh <- acceptErr
			return
		}
		defer conn.Close()

		tr := gtransport.New(conn, modbus.NewTCP(), gtransport.WithReadTimeout(5*time.Second))
		defer tr.Close()

		frame, readErr := tr.ReadFrame()
		if readErr != nil {
			serverErrCh <- readErr
			return
		}
		respFrame, handleErr := modbus.HandleTCPRequestFrame(frame, image)
		if handleErr != nil {
			serverErrCh <- handleErr
			return
		}
		serverErrCh <- tr.WriteFrame(respFrame)
	}()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		return err
	}
	defer conn.Close()

	tr := gtransport.New(conn, modbus.NewTCP(), gtransport.WithReadTimeout(5*time.Second))
	defer tr.Close()

	reqFrame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x11, 0x03, 0x00, 0x00, 0x00, 0x01}
	if err = tr.WriteFrame(reqFrame); err != nil {
		return err
	}

	respFrame, err := tr.ReadFrame()
	if err != nil {
		return err
	}
	value, err := parseSingleRegisterTCPResponse(respFrame)
	if err != nil {
		return err
	}
	fmt.Printf("recommended example: read holding register value 0x%04X\n", value)

	if err = <-serverErrCh; err != nil {
		return err
	}
	return nil
}

func parseSingleRegisterTCPResponse(frame []byte) (uint16, error) {
	if len(frame) == 9 && frame[7]&0x80 != 0 {
		return 0, fmt.Errorf("modbus exception response function=0x%02x code=0x%02x", frame[7], frame[8])
	}
	if len(frame) != 11 {
		return 0, fmt.Errorf("unexpected tcp response length %d", len(frame))
	}
	if frame[7] != 0x03 {
		return 0, fmt.Errorf("unexpected function code 0x%02x", frame[7])
	}
	if frame[8] != 0x02 {
		return 0, fmt.Errorf("unexpected byte count %d", frame[8])
	}
	return binary.BigEndian.Uint16(frame[9:11]), nil
}
