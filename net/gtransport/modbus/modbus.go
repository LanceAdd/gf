// Package modbus provides Modbus protocol support for gtransport.
//
// Recommended APIs:
//   - HandleTCPRequestFrame
//   - HandleRTURequestFrame
//   - HandleRTURequestPayload
//   - ExecuteRequest
//
// Advanced APIs:
//   - ParseTCPRequest
//   - ParseRTURequest
//   - EncodeTCPResponse
//   - EncodeRTUResponse
//   - typed Modbus request/response models
//   - ProcessImage
//
// Semantic rules:
//   - TCP frame: full Modbus TCP ADU
//   - RTU frame: raw Modbus RTU ADU with CRC
//   - RTU payload: CRC-stripped content returned by gtransport.New(..., NewRTU())
package modbus

import "github.com/gogf/gf/v2/net/gtransport"

// NewTCP returns a codec for Modbus TCP ADUs.
func NewTCP() gtransport.Codec {
	return tcpCodec{}
}

// NewRTU returns a codec for Modbus RTU frames.
func NewRTU() gtransport.Codec {
	return rtuCodec{}
}
