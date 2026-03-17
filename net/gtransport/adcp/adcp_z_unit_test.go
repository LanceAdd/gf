package adcp

import (
	"bytes"
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"

	"github.com/gogf/gf/v2/net/gtransport"
)

func TestADCPPublicAPIShape(t *testing.T) {
	var _ = New
	_ = DefaultMaxFrameLength

	type ctor func(...int) gtransport.Codec

	var _ ctor = New
}

func TestADCPDefaultMaxFrameLengthValue(t *testing.T) {
	if DefaultMaxFrameLength != 8192 {
		t.Fatalf("expected DefaultMaxFrameLength=8192, got %d", DefaultMaxFrameLength)
	}
}

func TestADCPNewUsesDefaultMaxFrameLength(t *testing.T) {
	codec := New()
	payload := bytes.Repeat([]byte{0x5A}, DefaultMaxFrameLength-syncLength-headerLength-crcLength+1)
	frame := testADCPFrame(0x12345678, payload)

	_, _, err := codec.Decode(frame)
	if err == nil {
		t.Fatal("expected frame too long error with default max frame length")
	}
}

func TestADCPNewAcceptsCustomMaxFrameLength(t *testing.T) {
	codec := New(DefaultMaxFrameLength + 1)
	payload := bytes.Repeat([]byte{0x5A}, DefaultMaxFrameLength-syncLength-headerLength-crcLength+1)
	frame := testADCPFrame(0x12345678, payload)

	got, consumed, err := codec.Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("expected large payload to decode with custom max frame length")
	}
}

func TestADCPNewFallsBackToDefaultOnNonPositiveMaxFrameLength(t *testing.T) {
	codec := New(0)
	payload := bytes.Repeat([]byte{0x5A}, DefaultMaxFrameLength-syncLength-headerLength-crcLength+1)
	frame := testADCPFrame(0x12345678, payload)

	_, _, err := codec.Decode(frame)
	if err == nil {
		t.Fatal("expected frame too long error when max frame length is non-positive")
	}
}

func TestADCPDecodeCompleteFrame(t *testing.T) {
	codec := New()
	payload := []byte{0x10, 0x20, 0x30, 0x40}
	frame := testADCPFrame(0x12345678, payload)

	got, consumed, err := codec.Decode(frame)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(frame) {
		t.Fatalf("expected consumed=%d, got %d", len(frame), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestADCPDecodePartialFrameNeedsMoreData(t *testing.T) {
	codec := New()
	payload := []byte{0x10, 0x20, 0x30, 0x40}
	frame := testADCPFrame(0x12345678, payload)

	_, _, err := codec.Decode(frame[:len(frame)-1])
	if err != gtransport.ErrNeedMoreData {
		t.Fatalf("expected ErrNeedMoreData, got %v", err)
	}
}

func TestADCPDecodeSkipsNoisePrefix(t *testing.T) {
	codec := New()
	payload := []byte{0xAA, 0xBB, 0xCC}
	frame := testADCPFrame(0x01020304, payload)
	stream := append([]byte{0x01, 0x02, 0x03, 0x04}, frame...)

	got, consumed, err := codec.Decode(stream)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(stream) {
		t.Fatalf("expected consumed=%d, got %d", len(stream), consumed)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %x, got %x", payload, got)
	}
}

func TestADCPDecodeRecoversFromInvalidHeaderField(t *testing.T) {
	codec := New()
	bad := testADCPFrameWithSizeCopy(0x01020304, []byte{0x01, 0x02}, 0x00000002)
	goodPayload := []byte{0x09, 0x08, 0x07}
	good := testADCPFrame(0x11223344, goodPayload)
	stream := append(bad, good...)

	got, consumed, err := codec.Decode(stream)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(stream) {
		t.Fatalf("expected consumed=%d, got %d", len(stream), consumed)
	}
	if !bytes.Equal(got, goodPayload) {
		t.Fatalf("expected %x, got %x", goodPayload, got)
	}
}

func TestADCPDecodeRecoversFromInvalidCRC(t *testing.T) {
	codec := New()
	bad := testADCPFrame(0x01020304, []byte{0x01, 0x02, 0x03})
	bad[len(bad)-1] ^= 0xFF
	goodPayload := []byte{0x11, 0x22, 0x33, 0x44}
	good := testADCPFrame(0x55667788, goodPayload)
	stream := append(bad, good...)

	got, consumed, err := codec.Decode(stream)
	if err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if consumed != len(stream) {
		t.Fatalf("expected consumed=%d, got %d", len(stream), consumed)
	}
	if !bytes.Equal(got, goodPayload) {
		t.Fatalf("expected %x, got %x", goodPayload, got)
	}
}

func TestADCPEncodeReturnsUnsupported(t *testing.T) {
	_, err := New().Encode([]byte{0x01, 0x02, 0x03})
	if err == nil {
		t.Fatal("expected unsupported encode error")
	}
}

func TestTransportADCPReadsPartialFrameAfterCompletion(t *testing.T) {
	server, client := net.Pipe()
	defer server.Close()
	defer client.Close()

	tr := gtransport.Wrap(client, New(), gtransport.WithReadTimeout(time.Second))
	defer tr.Close()

	payload := []byte{0x21, 0x22, 0x23, 0x24}
	frame := testADCPFrame(0x11111111, payload)
	resultCh := make(chan []byte, 1)
	errCh := make(chan error, 1)

	go func() {
		got, err := tr.ReadFrame(context.Background())
		if err != nil {
			errCh <- err
			return
		}
		resultCh <- got
	}()

	if _, err := server.Write(frame[:20]); err != nil {
		t.Fatalf("write partial frame: %v", err)
	}

	select {
	case <-resultCh:
		t.Fatal("ReadFrame returned before ADCP frame was complete")
	case err := <-errCh:
		t.Fatalf("ReadFrame returned error before completion: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	if _, err := server.Write(frame[20:]); err != nil {
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
		t.Fatal("timed out waiting for completed ADCP frame")
	}
}

func testADCPFrame(ensemble uint32, payload []byte) []byte {
	return testADCPFrameWithSizeCopy(ensemble, payload, ^uint32(len(payload)))
}

func testADCPFrameWithSizeCopy(ensemble uint32, payload []byte, payloadSizeCopy uint32) []byte {
	const syncLen = 16

	frame := make([]byte, syncLen+16+len(payload)+4)
	for i := range syncLen {
		frame[i] = 0x80
	}
	offset := syncLen
	binary.LittleEndian.PutUint32(frame[offset:], ensemble)
	offset += 4
	binary.LittleEndian.PutUint32(frame[offset:], ^ensemble)
	offset += 4
	binary.LittleEndian.PutUint32(frame[offset:], uint32(len(payload)))
	offset += 4
	binary.LittleEndian.PutUint32(frame[offset:], payloadSizeCopy)
	offset += 4
	copy(frame[offset:], payload)
	offset += len(payload)
	binary.LittleEndian.PutUint32(frame[offset:], uint32(testCRC16CCITT(payload)))
	return frame
}

func testCRC16CCITT(data []byte) uint16 {
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
