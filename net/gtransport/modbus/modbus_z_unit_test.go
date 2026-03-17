package modbus

import "testing"

func TestPublicAPIShape(t *testing.T) {
	var (
		_ func([]byte) (Request, error)  = ParseTCPRequest
		_ func([]byte) (Request, error)  = ParseRTURequest
		_ func([]byte) (Request, error)  = ParseRTURequestPayload
		_ func(Response) ([]byte, error) = EncodeTCPResponse
		_ func(Response) ([]byte, error) = EncodeRTUResponse
		_ func(Response) ([]byte, error) = EncodeRTUResponsePayload
		_ ProcessImage                   = NewMemoryProcessImage(1, 1, 1, 1)
	)
	_ = ADUMeta{}.SlaveID
}

func appendCRC(payload []byte) []byte {
	frame := make([]byte, len(payload)+2)
	copy(frame, payload)
	crc := modbusCRC(payload)
	frame[len(payload)] = byte(crc)
	frame[len(payload)+1] = byte(crc >> 8)
	return frame
}
