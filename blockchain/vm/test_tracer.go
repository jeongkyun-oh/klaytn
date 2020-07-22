// Modifications Copyright 2020 The klaytn Authors
// Copyright 2017 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// TestTracer is a full blown transaction tracer that extracts and reports all
// the internal calls made by a transaction, along with any useful information.
//
// This file is derived from eth/tracers/internal/tracers/call_tracer.js (2018/06/04).
// Modified and improved for the klaytn development.

package vm

import (
	"encoding/json"
	"errors"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/hexutil"
	"math/big"
	"reflect"
	"time"
)

type TestTracer struct {
	internalTxTracer *InternalTxTracer
	jsCallTracer     *Tracer
}

// NewInternalTxTracer returns a new NewTestTracer.
func NewTestTracer() *TestTracer {
	jsCallTracer, err := New("callTracer")
	if err != nil {
		panic(err)
	}
	return &TestTracer{
		internalTxTracer: NewInternalTxTracer(),
		jsCallTracer:     jsCallTracer,
	}
}

// CaptureStart implements the Tracer interface to initialize the tracing operation.
func (this *TestTracer) CaptureStart(from common.Address, to common.Address, create bool, input []byte, gas uint64, value *big.Int) error {
	if err := this.internalTxTracer.CaptureStart(from, to, create, input, gas, value); err != nil {
		return err
	}

	if err := this.jsCallTracer.CaptureStart(from, to, create, input, gas, value); err != nil {
		return err
	}
	return nil
}

// CaptureState implements the Tracer interface to trace a single step of VM execution.
func (this *TestTracer) CaptureState(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, logStack *Stack, contract *Contract, depth int, err error) error {
	if err := this.internalTxTracer.CaptureState(env, pc, op, gas, cost, memory, logStack, contract, depth, err); err != nil {
		return err
	}

	if err := this.jsCallTracer.CaptureState(env, pc, op, gas, cost, memory, logStack, contract, depth, err); err != nil {
		return err
	}

	return nil
}

// CaptureFault implements the Tracer interface to trace an execution fault
// while running an opcode.
func (this *TestTracer) CaptureFault(env *EVM, pc uint64, op OpCode, gas, cost uint64, memory *Memory, s *Stack, contract *Contract, depth int, err error) error {
	if err := this.internalTxTracer.CaptureFault(env, pc, op, gas, cost, memory, s, contract, depth, err); err != nil {
		return err
	}

	if err := this.jsCallTracer.CaptureFault(env, pc, op, gas, cost, memory, s, contract, depth, err); err != nil {
		return err
	}

	return nil
}

// CaptureEnd is called after the call finishes to finalize the tracing.
func (this *TestTracer) CaptureEnd(output []byte, gasUsed uint64, t time.Duration, err error) error {
	if err := this.internalTxTracer.CaptureEnd(output, gasUsed, t, err); err != nil {
		return err
	}

	if err := this.jsCallTracer.CaptureEnd(output, gasUsed, t, err); err != nil {
		return err
	}

	return nil
}

var valueTransferResult = json.RawMessage("{\"type\":0,\"from\":\"0x\",\"to\":\"0x\",\"value\":\"0x0\",\"gas\":\"0x0\",\"gasUsed\":\"0x0\",\"input\":\"0x\",\"output\":\"0x\",\"time\":0}")

// CallTrace is the result of a callTracer run.
type CallTrace struct {
	Type     string         `json:"type"`
	From     common.Address `json:"from"`
	To       common.Address `json:"to"`
	Input    hexutil.Bytes  `json:"input"`
	Output   hexutil.Bytes  `json:"output"`
	Gas      hexutil.Uint64 `json:"gas,omitempty"`
	GasUsed  hexutil.Uint64 `json:"gasUsed,omitempty"`
	Value    hexutil.Uint64 `json:"value,omitempty"`
	Error    string         `json:"error,omitempty"`
	Calls    []CallTrace    `json:"calls,omitempty"`
	Reverted Reverted       `json:"reverted,omitempty"`
}

type Reverted struct {
	Contract common.Address `json:"contract"`
	Message  string         `json:"message"`
}

func convertToCallTrace(internalTx *InternalTxTrace) (*CallTrace, error) {
	// coverts nested InternalTxTraces
	var nestedCalls []CallTrace
	for _, call := range internalTx.Calls {
		nestedCall, err := convertToCallTrace(call)
		if err != nil {
			return nil, err
		}
		nestedCalls = append(nestedCalls, *nestedCall)
	}

	// decodes input and output if they are not an empty string
	decodedInput := []byte{}
	var decodedOutput []byte
	var err error
	if internalTx.Input != "" {
		decodedInput, err = hexutil.Decode(internalTx.Input)
		if err != nil {
			logger.Error("failed to decode input of an internal transaction", "err", err)
			return nil, err
		}
	}
	if internalTx.Output != "" {
		decodedOutput, err = hexutil.Decode(internalTx.Output)
		if err != nil {
			logger.Error("failed to decode output of an internal transaction", "err", err)
			return nil, err
		}
	}

	// decodes value into *big.Int if it is not an empty string
	var value *big.Int
	if internalTx.Value != "" {
		value, err = hexutil.DecodeBig(internalTx.Value)
		if err != nil {
			logger.Error("failed to decode value of an internal transaction", "err", err)
			return nil, err
		}
	}
	var val hexutil.Uint64
	if value != nil {
		val = hexutil.Uint64(value.Uint64())
	}

	errStr := ""
	if internalTx.Error != nil {
		errStr = internalTx.Error.Error()
	}

	ct := &CallTrace{
		Type:    internalTx.Type,
		From:    internalTx.From,
		To:      internalTx.To,
		Input:   decodedInput,
		Output:  decodedOutput,
		Gas:     hexutil.Uint64(internalTx.Gas),
		GasUsed: hexutil.Uint64(internalTx.GasUsed),
		Value:   val,
		Error:   errStr,
		Calls:   nestedCalls,
		Reverted: Reverted{
			Contract: internalTx.Reverted.Contract,
			Message:  internalTx.Reverted.Message,
		},
	}

	return ct, nil
}

func (this *TestTracer) CompareResults() error {
	r1, err := this.internalTxTracer.GetResult()
	if err != nil {
		logger.Error("failed to get result for internalTxTracer", "result", r1, "err", err)
		return err
	}

	r2, err := this.jsCallTracer.GetResult()
	if err != nil {
		logger.Error("failed to get result for jsCallTracer", "result", string(r2), "err", err)
		return err
	}

	// skip when the transaction does not include internal transaction
	if reflect.DeepEqual(r2, valueTransferResult) {
		logger.Info("value transfer")
		return nil
	}

	ret1, err := convertToCallTrace(r1)
	if err != nil {
		logger.Error("failed to convertToCallTrace", "err", err)
		return err
	}

	ret2 := new(CallTrace)
	if err := json.Unmarshal(r2, ret2); err != nil {
		logger.Error("failed to unmarshal trace result", "err", err)
		return err
	}

	if !reflect.DeepEqual(ret1, ret2) {
		logger.Error("trace mismatch:", "have", ret1, "want", ret2)
		return errors.New("trace mismatch")
	}

	return nil
}
