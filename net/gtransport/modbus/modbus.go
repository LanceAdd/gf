// Package modbus provides Modbus protocol support for gtransport.
//
// Primary composable APIs:
//   - ParseTCPRequest
//   - ParseRTURequest
//   - ParseRTURequestPayload
//   - EncodeTCPResponse
//   - EncodeRTUResponse
//   - EncodeRTUResponsePayload
//
// Standard execution layer:
//   - ExecuteRequest
//   - ProcessImage
//
// Shared protocol models:
//   - typed Modbus request/response models
//   - ProcessImage
//
// Semantic rules:
//   - TCP frame: full Modbus TCP ADU
//   - RTU frame: raw Modbus RTU ADU with CRC
//   - RTU payload: CRC-stripped content returned by gtransport.Wrap(..., NewRTU())
//
// The recommended flow for bottom-layer integrations is:
// Parse* -> ExecuteRequest (or custom routing) -> Encode*
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
