package modbus

// HandleTCPRequestFrame handles one Modbus TCP request frame and returns the encoded response frame.
func HandleTCPRequestFrame(frame []byte, image ProcessImage) ([]byte, error) {
	return handleRequestFrame(frame, image, ParseTCPRequest, EncodeTCPResponse)
}

// HandleRTURequestFrame handles one Modbus RTU request frame and returns the encoded response frame.
func HandleRTURequestFrame(frame []byte, image ProcessImage) ([]byte, error) {
	return handleRequestFrame(frame, image, ParseRTURequest, EncodeRTUResponse)
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
