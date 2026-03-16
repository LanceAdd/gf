// Package modbus provides Modbus protocol codecs for gtransport.
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
