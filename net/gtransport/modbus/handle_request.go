package modbus

// HandleTCPRequestFrame handles one Modbus TCP request frame and returns the encoded response frame.
func HandleTCPRequestFrame(frame []byte, image ProcessImage) ([]byte, error) {
	return handleRequestFrame(frame, image, ParseTCPRequest, EncodeTCPResponse)
}

// HandleRTURequestFrame handles one raw Modbus RTU request frame and returns the raw encoded response frame.
func HandleRTURequestFrame(frame []byte, image ProcessImage) ([]byte, error) {
	return handleRequestFrame(frame, image, ParseRTURequest, EncodeRTUResponse)
}

// HandleRTURequestPayload handles one decoded Modbus RTU request payload and returns the decoded response payload.
func HandleRTURequestPayload(payload []byte, image ProcessImage) ([]byte, error) {
	return handleRequestFrame(payload, image, parseRTUTransportPayload, encodeRTUTransportPayload)
}

// handleRequestFrame is the shared request pipeline for parse -> execute ->
// encode across TCP frame, RTU frame, and RTU payload entry points.
func handleRequestFrame(
	frame []byte,
	image ProcessImage,
	parse func([]byte) (Request, error),
	encode func(Response) ([]byte, error),
) ([]byte, error) {
	req, err := parse(frame)
	if err != nil {
		return nil, err
	}
	resp, err := ExecuteRequest(req, image)
	if err != nil {
		return nil, err
	}
	return encode(resp)
}

// parseRTUTransportPayload reconstructs a typed request from the CRC-stripped
// payload returned by the RTU transport codec.
func parseRTUTransportPayload(payload []byte) (Request, error) {
	if err := validateModbusPayload(payload); err != nil {
		return nil, err
	}
	// The RTU transport codec already stripped and verified the CRC, so only
	// protocol metadata needs to be reconstructed here.
	meta := ADUMeta{
		Transport: TransportRTU,
		SlaveID:   payload[0],
	}
	return parseRequest(meta, payload)
}

// encodeRTUTransportPayload encodes a typed response back into CRC-stripped RTU
// payload bytes for transport-level writing.
func encodeRTUTransportPayload(resp Response) ([]byte, error) {
	return encodeResponsePayload(resp)
}
