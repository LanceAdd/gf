package modbus

import "fmt"

// ExecuteRequest runs a typed Modbus request against a process image.
func ExecuteRequest(req Request, image ProcessImage) (Response, error) {
	if req == nil {
		return nil, fmt.Errorf("modbus request is nil")
	}
	if image == nil {
		return nil, fmt.Errorf("modbus process image is nil")
	}

	switch typed := req.(type) {
	case ReadCoilsRequest:
		values, err := image.ReadCoils(typed.StartAddress, typed.Quantity)
		if err != nil {
			return nil, err
		}
		return ReadBitsResponse{
			meta:     typed.Meta(),
			function: typed.FunctionCode(),
			Values:   values,
		}, nil

	case ReadDiscreteInputsRequest:
		values, err := image.ReadDiscreteInputs(typed.StartAddress, typed.Quantity)
		if err != nil {
			return nil, err
		}
		return ReadBitsResponse{
			meta:     typed.Meta(),
			function: typed.FunctionCode(),
			Values:   values,
		}, nil

	case ReadHoldingRegistersRequest:
		values, err := image.ReadHoldingRegisters(typed.StartAddress, typed.Quantity)
		if err != nil {
			return nil, err
		}
		return ReadRegistersResponse{
			meta:     typed.Meta(),
			function: typed.FunctionCode(),
			Values:   values,
		}, nil

	case ReadInputRegistersRequest:
		values, err := image.ReadInputRegisters(typed.StartAddress, typed.Quantity)
		if err != nil {
			return nil, err
		}
		return ReadRegistersResponse{
			meta:     typed.Meta(),
			function: typed.FunctionCode(),
			Values:   values,
		}, nil

	case WriteSingleCoilRequest:
		if err := image.WriteSingleCoil(typed.Address, typed.Value); err != nil {
			return nil, err
		}
		return WriteSingleCoilResponse{
			meta:    typed.Meta(),
			Address: typed.Address,
			Value:   typed.Value,
		}, nil

	case WriteSingleRegisterRequest:
		if err := image.WriteSingleRegister(typed.Address, typed.Value); err != nil {
			return nil, err
		}
		return WriteSingleRegisterResponse{
			meta:    typed.Meta(),
			Address: typed.Address,
			Value:   typed.Value,
		}, nil

	case WriteMultipleCoilsRequest:
		if err := image.WriteMultipleCoils(typed.StartAddress, typed.Values); err != nil {
			return nil, err
		}
		return WriteMultipleCoilsResponse{
			meta:         typed.Meta(),
			StartAddress: typed.StartAddress,
			Quantity:     uint16(len(typed.Values)),
		}, nil

	case WriteMultipleRegistersRequest:
		if err := image.WriteMultipleRegisters(typed.StartAddress, typed.Values); err != nil {
			return nil, err
		}
		return WriteMultipleRegistersResponse{
			meta:         typed.Meta(),
			StartAddress: typed.StartAddress,
			Quantity:     uint16(len(typed.Values)),
		}, nil
	}

	if !isSupportedFunction(byte(req.FunctionCode())) {
		return ExceptionResponse{
			meta:          req.Meta(),
			function:      req.FunctionCode(),
			ExceptionCode: 0x01,
		}, nil
	}

	return nil, fmt.Errorf("modbus request type %T is not executable", req)
}
