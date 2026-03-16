package gtransport_test

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
	"github.com/gogf/gf/v2/net/gtransport/modbus"
)

func TestTransportModbusTCPReadsStickyFrames(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := gtransport.New(client, modbus.NewTCP(), gtransport.WithReadTimeout(time.Second))
	defer tr.Close()

	frame1 := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	frame2 := []byte{0x00, 0x02, 0x00, 0x00, 0x00, 0x06, 0x01, 0x06, 0x00, 0x64, 0x12, 0x34}

	go func() {
		_, _ = server.Write(append(frame1, frame2...))
	}()

	got1, err := tr.ReadFrame()
	if err != nil {
		t.Fatalf("read first frame: %v", err)
	}
	got2, err := tr.ReadFrame()
	if err != nil {
		t.Fatalf("read second frame: %v", err)
	}
	if !bytes.Equal(got1, frame1) {
		t.Fatalf("expected first frame %x, got %x", frame1, got1)
	}
	if !bytes.Equal(got2, frame2) {
		t.Fatalf("expected second frame %x, got %x", frame2, got2)
	}
}

func TestTransportModbusTCPReadsPartialFrameAfterCompletion(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := gtransport.New(client, modbus.NewTCP(), gtransport.WithReadTimeout(time.Second))
	defer tr.Close()

	frame := []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x06, 0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	resultCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		got, err := tr.ReadFrame()
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- got
	}()

	if _, err := server.Write(frame[:5]); err != nil {
		t.Fatalf("write partial frame: %v", err)
	}

	select {
	case <-resultCh:
		t.Fatal("ReadFrame returned before TCP frame was complete")
	case err := <-errCh:
		t.Fatalf("ReadFrame returned error before completion: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	if _, err := server.Write(frame[5:]); err != nil {
		t.Fatalf("write frame remainder: %v", err)
	}

	select {
	case got := <-resultCh:
		if !bytes.Equal(got, frame) {
			t.Fatalf("expected %x, got %x", frame, got)
		}
	case err := <-errCh:
		t.Fatalf("ReadFrame returned error after completion: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completed TCP frame")
	}
}

func TestTransportModbusRTUReadsFrameAfterNoisePrefix(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := gtransport.New(client, modbus.NewRTU(), gtransport.WithReadTimeout(time.Second))
	defer tr.Close()

	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	frame := append([]byte{0x99, 0x88}, appendModbusCRC(payload)...)

	go func() {
		_, _ = server.Write(frame)
	}()

	got, err := tr.ReadFrame()
	if err != nil {
		t.Fatalf("read frame: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestTransportModbusRTUReadsPartialFrameAfterCompletion(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := gtransport.New(client, modbus.NewRTU(), gtransport.WithReadTimeout(time.Second))
	defer tr.Close()

	payload := []byte{0x01, 0x03, 0x00, 0x00, 0x00, 0x01}
	frame := appendModbusCRC(payload)
	resultCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		got, err := tr.ReadFrame()
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- got
	}()

	if _, err := server.Write(frame[:4]); err != nil {
		t.Fatalf("write partial frame: %v", err)
	}

	select {
	case <-resultCh:
		t.Fatal("ReadFrame returned before RTU frame was complete")
	case err := <-errCh:
		t.Fatalf("ReadFrame returned error before completion: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	if _, err := server.Write(frame[4:]); err != nil {
		t.Fatalf("write frame remainder: %v", err)
	}

	select {
	case got := <-resultCh:
		if !bytes.Equal(got, payload) {
			t.Fatalf("expected %x, got %x", payload, got)
		}
	case err := <-errCh:
		t.Fatalf("ReadFrame returned error after completion: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for completed RTU frame")
	}
}

func TestTransportModbusTCPHandlesRequestFrameEndToEnd(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverTr := gtransport.New(serverConn, modbus.NewTCP(), gtransport.WithReadTimeout(time.Second))
	clientTr := gtransport.New(clientConn, modbus.NewTCP(), gtransport.WithReadTimeout(time.Second))
	defer serverTr.Close()
	defer clientTr.Close()

	image := modbus.NewMemoryProcessImage(4, 4, 4, 4)
	_ = image.WriteMultipleRegisters(0, []uint16{0x1234})

	serverErrCh := make(chan error, 1)
	go func() {
		frame, err := serverTr.ReadFrame()
		if err != nil {
			serverErrCh <- err
			return
		}
		resp, err := modbus.HandleTCPRequestFrame(frame, image)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverErrCh <- serverTr.WriteFrame(resp)
	}()

	reqFrame := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x06, 0x11, 0x03, 0x00, 0x00, 0x00, 0x01}
	if err := clientTr.WriteFrame(reqFrame); err != nil {
		t.Fatalf("client write request frame: %v", err)
	}

	got, err := clientTr.ReadFrame()
	if err != nil {
		t.Fatalf("client read response frame: %v", err)
	}
	want := []byte{0x01, 0x02, 0x00, 0x00, 0x00, 0x05, 0x11, 0x03, 0x02, 0x12, 0x34}
	if !bytes.Equal(got, want) {
		t.Fatalf("expected %x, got %x", want, got)
	}
	if err = <-serverErrCh; err != nil {
		t.Fatalf("server loop failed: %v", err)
	}
}

func TestTransportModbusRTUHandlesRequestFrameEndToEnd(t *testing.T) {
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()

	serverTr := gtransport.New(serverConn, modbus.NewRTU(), gtransport.WithReadTimeout(time.Second))
	clientTr := gtransport.New(clientConn, modbus.NewRTU(), gtransport.WithReadTimeout(time.Second))
	defer serverTr.Close()
	defer clientTr.Close()

	image := modbus.NewMemoryProcessImage(4, 4, 4, 4)
	_ = image.WriteMultipleRegisters(0, []uint16{0x1234})

	serverErrCh := make(chan error, 1)
	go func() {
		frame, err := serverTr.ReadFrame()
		if err != nil {
			serverErrCh <- err
			return
		}
		resp, err := modbus.HandleRTURequestPayload(frame, image)
		if err != nil {
			serverErrCh <- err
			return
		}
		serverErrCh <- serverTr.WriteFrame(resp)
	}()

	reqPayload := []byte{0x11, 0x03, 0x00, 0x00, 0x00, 0x01}
	if err := clientTr.WriteFrame(reqPayload); err != nil {
		t.Fatalf("client write request payload: %v", err)
	}

	resultCh := make(chan []byte, 1)
	clientErrCh := make(chan error, 1)
	go func() {
		got, err := clientTr.ReadFrame()
		if err != nil {
			clientErrCh <- err
			return
		}
		resultCh <- got
	}()

	select {
	case err := <-serverErrCh:
		if err != nil {
			t.Fatalf("server loop failed: %v", err)
		}
	case got := <-resultCh:
		want := []byte{0x11, 0x03, 0x02, 0x12, 0x34}
		if !bytes.Equal(got, want) {
			t.Fatalf("expected %x, got %x", want, got)
		}
	case err := <-clientErrCh:
		t.Fatalf("client read response payload: %v", err)
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for RTU end-to-end response")
	}
}

func appendModbusCRC(payload []byte) []byte {
	frame := make([]byte, len(payload)+2)
	copy(frame, payload)
	crc := modbusCRCForTest(payload)
	binary.LittleEndian.PutUint16(frame[len(payload):], crc)
	return frame
}

func modbusCRCForTest(data []byte) uint16 {
	crc := uint16(0xFFFF)
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
