package modbus

func appendCRC(payload []byte) []byte {
	frame := make([]byte, len(payload)+2)
	copy(frame, payload)
	crc := modbusCRC(payload)
	frame[len(payload)] = byte(crc)
	frame[len(payload)+1] = byte(crc >> 8)
	return frame
}
