package modbus

import "testing"

func TestExecuteRequestReadCoils(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	_ = image.WriteMultipleCoils(0, []bool{true, false, true, true})

	resp, err := ExecuteRequest(ReadCoilsRequest{
		meta:         ADUMeta{Transport: TransportTCP, TransactionID: 0x0102, SlaveID: 0x11},
		StartAddress: 0,
		Quantity:     4,
	}, image)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	readResp, ok := resp.(ReadBitsResponse)
	if !ok {
		t.Fatalf("expected ReadBitsResponse, got %T", resp)
	}
	if readResp.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x0102, SlaveID: 0x11}) {
		t.Fatalf("unexpected response meta: %+v", readResp.Meta())
	}
	if readResp.FunctionCode() != FunctionCode(0x01) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readResp.FunctionCode()))
	}
	want := []bool{true, false, true, true}
	if len(readResp.Values) != len(want) {
		t.Fatalf("unexpected values length: %d", len(readResp.Values))
	}
	for i, v := range want {
		if readResp.Values[i] != v {
			t.Fatalf("unexpected bit at %d: %v", i, readResp.Values[i])
		}
	}
}

func TestExecuteRequestReadDiscreteInputs(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	image.discreteInputs[1] = true
	image.discreteInputs[3] = true

	resp, err := ExecuteRequest(ReadDiscreteInputsRequest{
		meta:         ADUMeta{Transport: TransportRTU, SlaveID: 0x22},
		StartAddress: 0,
		Quantity:     4,
	}, image)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	readResp, ok := resp.(ReadBitsResponse)
	if !ok {
		t.Fatalf("expected ReadBitsResponse, got %T", resp)
	}
	if readResp.Meta() != (ADUMeta{Transport: TransportRTU, SlaveID: 0x22}) {
		t.Fatalf("unexpected response meta: %+v", readResp.Meta())
	}
	if readResp.FunctionCode() != FunctionCode(0x02) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readResp.FunctionCode()))
	}
	want := []bool{false, true, false, true}
	for i, v := range want {
		if readResp.Values[i] != v {
			t.Fatalf("unexpected bit at %d: %v", i, readResp.Values[i])
		}
	}
}

func TestExecuteRequestReadHoldingRegisters(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	_ = image.WriteMultipleRegisters(0, []uint16{0x1234, 0x5678, 0x9ABC})

	resp, err := ExecuteRequest(ReadHoldingRegistersRequest{
		meta:         ADUMeta{Transport: TransportTCP, TransactionID: 0x0304, SlaveID: 0x33},
		StartAddress: 0,
		Quantity:     3,
	}, image)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	readResp, ok := resp.(ReadRegistersResponse)
	if !ok {
		t.Fatalf("expected ReadRegistersResponse, got %T", resp)
	}
	if readResp.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x0304, SlaveID: 0x33}) {
		t.Fatalf("unexpected response meta: %+v", readResp.Meta())
	}
	if readResp.FunctionCode() != FunctionCode(0x03) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readResp.FunctionCode()))
	}
	want := []uint16{0x1234, 0x5678, 0x9ABC}
	for i, v := range want {
		if readResp.Values[i] != v {
			t.Fatalf("unexpected register at %d: 0x%04x", i, readResp.Values[i])
		}
	}
}

func TestExecuteRequestReadInputRegisters(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	image.inputRegisters[0] = 0x1111
	image.inputRegisters[1] = 0x2222

	resp, err := ExecuteRequest(ReadInputRegistersRequest{
		meta:         ADUMeta{Transport: TransportRTU, SlaveID: 0x44},
		StartAddress: 0,
		Quantity:     2,
	}, image)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}

	readResp, ok := resp.(ReadRegistersResponse)
	if !ok {
		t.Fatalf("expected ReadRegistersResponse, got %T", resp)
	}
	if readResp.Meta() != (ADUMeta{Transport: TransportRTU, SlaveID: 0x44}) {
		t.Fatalf("unexpected response meta: %+v", readResp.Meta())
	}
	if readResp.FunctionCode() != FunctionCode(0x04) {
		t.Fatalf("unexpected function code: 0x%02x", byte(readResp.FunctionCode()))
	}
	want := []uint16{0x1111, 0x2222}
	for i, v := range want {
		if readResp.Values[i] != v {
			t.Fatalf("unexpected register at %d: 0x%04x", i, readResp.Values[i])
		}
	}
}

func TestExecuteRequestMetadataPreservation(t *testing.T) {
	image := NewMemoryProcessImage(4, 4, 4, 4)
	_ = image.WriteMultipleRegisters(0, []uint16{0xAAAA})

	resp, err := ExecuteRequest(ReadHoldingRegistersRequest{
		meta:         ADUMeta{Transport: TransportTCP, TransactionID: 0x0A0B, SlaveID: 0x55},
		StartAddress: 0,
		Quantity:     1,
	}, image)
	if err != nil {
		t.Fatalf("execute request: %v", err)
	}
	if resp.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x0A0B, SlaveID: 0x55}) {
		t.Fatalf("unexpected meta: %+v", resp.Meta())
	}
}

func TestExecuteRequestUnsupportedFunctionReturnsException(t *testing.T) {
	image := NewMemoryProcessImage(1, 1, 1, 1)

	resp, err := ExecuteRequest(unsupportedRequest{
		meta:     ADUMeta{Transport: TransportTCP, TransactionID: 0x0C0D, SlaveID: 0x66},
		function: FunctionCode(0x11),
	}, image)
	if err != nil {
		t.Fatalf("expected exception response, got error: %v", err)
	}

	exceptionResp, ok := resp.(ExceptionResponse)
	if !ok {
		t.Fatalf("expected ExceptionResponse, got %T", resp)
	}
	if exceptionResp.Meta() != (ADUMeta{Transport: TransportTCP, TransactionID: 0x0C0D, SlaveID: 0x66}) {
		t.Fatalf("unexpected exception meta: %+v", exceptionResp.Meta())
	}
	if exceptionResp.FunctionCode() != FunctionCode(0x11) {
		t.Fatalf("unexpected exception function code: 0x%02x", byte(exceptionResp.FunctionCode()))
	}
	if exceptionResp.ExceptionCode != 0x01 {
		t.Fatalf("unexpected exception code: 0x%02x", exceptionResp.ExceptionCode)
	}
}

func TestExecuteRequestNilPreconditionsReturnError(t *testing.T) {
	image := NewMemoryProcessImage(1, 1, 1, 1)

	if _, err := ExecuteRequest(nil, image); err == nil {
		t.Fatal("expected nil request error")
	}
	if _, err := ExecuteRequest(ReadCoilsRequest{}, nil); err == nil {
		t.Fatal("expected nil image error")
	}
}

type unsupportedRequest struct {
	meta     ADUMeta
	function FunctionCode
}

func (r unsupportedRequest) Meta() ADUMeta {
	return r.meta
}

func (r unsupportedRequest) FunctionCode() FunctionCode {
	return r.function
}

var _ Request = unsupportedRequest{}
