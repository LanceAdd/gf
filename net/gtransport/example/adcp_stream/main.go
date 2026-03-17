package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
	"github.com/gogf/gf/v2/net/gtransport/adcp"
)

func main() {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	go func() {
		defer server.Close()

		payload := []byte{0x10, 0x20, 0x30, 0x40, 0x50}
		frame := buildADCPFrame(0x12345678, payload)

		_, _ = server.Write(frame[:18])
		time.Sleep(50 * time.Millisecond)
		_, _ = server.Write(frame[18:])
	}()

	tr := gtransport.Wrap(
		client,
		adcp.New(adcp.DefaultMaxFrameLength*2),
		gtransport.WithReadTimeout(5*time.Second),
	)
	defer tr.Close()

	payload, err := tr.ReadFrame(context.Background())
	if err != nil {
		panic(fmt.Sprintf("read ADCP payload: %v", err))
	}

	fmt.Printf("adcp_stream: decoded payload=% X\n", payload)
}

func buildADCPFrame(ensemble uint32, payload []byte) []byte {
	const (
		syncByte   = 0x80
		syncLength = 16
	)

	frame := make([]byte, syncLength+16+len(payload)+4)
	for i := range syncLength {
		frame[i] = syncByte
	}
	offset := syncLength
	binary.LittleEndian.PutUint32(frame[offset:], ensemble)
	offset += 4
	binary.LittleEndian.PutUint32(frame[offset:], ^ensemble)
	offset += 4
	binary.LittleEndian.PutUint32(frame[offset:], uint32(len(payload)))
	offset += 4
	binary.LittleEndian.PutUint32(frame[offset:], ^uint32(len(payload)))
	offset += 4
	copy(frame[offset:], payload)
	offset += len(payload)
	binary.LittleEndian.PutUint32(frame[offset:], uint32(crc16CCITT(payload)))
	return frame
}

func crc16CCITT(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			if crc&0x8000 != 0 {
				crc = (crc << 1) ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
