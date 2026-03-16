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
func HandleRTURequestPayload(frame []byte, image ProcessImage) ([]byte, error) {
	return handleRequestFrame(frame, image, parseRTUTransportFrame, encodeRTUTransportFrame)
}

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

func parseRTUTransportFrame(frame []byte) (Request, error) {
	if err := validateModbusPayload(frame); err != nil {
		return nil, err
	}
	meta := ADUMeta{
		Transport: TransportRTU,
		SlaveID:   frame[0],
	}
	return parseRequest(meta, frame)
}

func encodeRTUTransportFrame(resp Response) ([]byte, error) {
	return encodeResponsePayload(resp)
}
