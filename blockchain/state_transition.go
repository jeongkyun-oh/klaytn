// Copyright 2018 The klaytn Authors
// Copyright 2014 The go-ethereum Authors
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
// This file is derived from core/state_transition.go (2018/06/04).
// Modified and improved for the klaytn development.

package blockchain

import (
	"errors"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/blockchain/vm"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/kerrors"
	"math/big"
)

var (
	errInsufficientBalanceForGas         = errors.New("insufficient balance of the sender to pay for gas")
	errInsufficientBalanceForGasFeePayer = errors.New("insufficient balance of the fee payer to pay for gas")
	errNotProgramAccount                 = errors.New("not a program account")
	errAccountAlreadyExists              = errors.New("account already exists")
	errMsgToNil                          = errors.New("msg.To() is nil")
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay gas
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *GasPool
	msg        Message
	gas        uint64
	gasPrice   *big.Int
	initialGas uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}

// Message represents a message sent to a contract.
type Message interface {
	// ValidatedSender returns the sender of the transaction.
	// The returned sender should be derived by calling AsMessageAccountKeyPicker().
	ValidatedSender() common.Address

	// ValidatedFeePayer returns the fee payer of the transaction.
	// The returned fee payer should be derived by calling AsMessageAccountKeyPicker().
	ValidatedFeePayer() common.Address

	// ValidatedIntrinsicGas returns the intrinsic gas of the transaction.
	// The returned intrinsic gas should be derived by calling AsMessageAccountKeyPicker().
	ValidatedIntrinsicGas() uint64

	// FeeRatio returns a ratio of tx fee paid by the fee payer in percentage.
	// For example, if it is 30, 30% of tx fee will be paid by the fee payer.
	// 70% will be paid by the sender.
	FeeRatio() uint8

	//FromFrontier() (common.Address, error)
	To() *common.Address

	GasPrice() *big.Int
	Gas() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte

	// IntrinsicGas returns `intrinsic gas` based on the tx type.
	// This value is used to differentiate tx fee based on the tx type.
	IntrinsicGas() (uint64, error)

	// Type returns the transaction type of the message.
	Type() types.TxType

	// Execute performs execution of the transaction according to the transaction type.
	Execute(vm types.VM, stateDB types.StateDB, gas uint64, value *big.Int) ([]byte, uint64, error, error)
}

// TODO-Klaytn Later we can merge Err and Status into one uniform error.
//         This might require changing overall error handling mechanism in Klaytn.
// Klaytn error type
// - Status: Indicate status of transaction after execution.
//           This value will be stored in Receipt if Receipt is available.
//           Please see getReceiptStatusFromVMerr() how this value is calculated.
type kerror struct {
	Err    error
	Status uint
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *vm.EVM, msg Message, gp *GasPool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		gasPrice: msg.GasPrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the gas used (which includes gas refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm *vm.EVM, msg Message, gp *GasPool) ([]byte, uint64, kerror) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) useGas(amount uint64) error {
	if st.gas < amount {
		return kerrors.ErrOutOfGas
	}
	st.gas -= amount

	return nil
}

func (st *StateTransition) buyGas() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Gas()), st.gasPrice) // TODO-Klaytn-Issue136 gasPrice gasLimit
	if err := st.gp.SubGas(st.msg.Gas()); err != nil {
		return err
	}

	feeRatio := st.msg.FeeRatio()
	validatedFeePayer := st.msg.ValidatedFeePayer()
	validatedSender := st.msg.ValidatedSender()
	switch {
	case feeRatio == types.MaxFeeRatio:
		// to make a short circuit, process the special case feeRatio == MaxFeeRatio
		if st.state.GetBalance(validatedFeePayer).Cmp(mgval) < 0 {
			return errInsufficientBalanceForGas
		}

		st.state.SubBalance(validatedFeePayer, mgval)

	case feeRatio < types.MaxFeeRatio:
		feePayer, feeSender := types.CalcFeeWithRatio(feeRatio, mgval)

		if st.state.GetBalance(validatedFeePayer).Cmp(feePayer) < 0 {
			return errInsufficientBalanceForGasFeePayer
		}

		if st.state.GetBalance(validatedSender).Cmp(feeSender) < 0 {
			return errInsufficientBalanceForGas
		}

		st.state.SubBalance(validatedFeePayer, feePayer)
		st.state.SubBalance(validatedSender, feeSender)

	default:
		// feeRatio > types.MaxFeeRatio
		return kerrors.ErrMaxFeeRatioExceeded
	}
	st.gas += st.msg.Gas()

	st.initialGas = st.msg.Gas()
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		nonce := st.state.GetNonce(st.msg.ValidatedSender())
		if nonce < st.msg.Nonce() {
			return ErrNonceTooHigh
		} else if nonce > st.msg.Nonce() {
			return ErrNonceTooLow
		}
	}
	// TODO-Klaytn-Issue136
	return st.buyGas()
}

// TransitionDb will transition the state by applying the current message and
// returning the result including the used gas. It returns an error if failed.
// An error indicates a consensus issue.
func (st *StateTransition) TransitionDb() (ret []byte, usedGas uint64, kerr kerror) {
	// TODO-Klaytn-Issue136
	if kerr.Err = st.preCheck(); kerr.Err != nil {
		return
	}
	msg := st.msg

	// TODO-Klaytn-Issue136
	// Pay intrinsic gas.
	if kerr.Err = st.useGas(msg.ValidatedIntrinsicGas()); kerr.Err != nil {
		kerr.Status = getReceiptStatusFromVMerr(nil)
		return nil, 0, kerr
	}

	var (
		// vm errors do not effect consensus and are therefor
		// not assigned to err, except for insufficient balance
		// error and total time limit reached error.
		vmerr error
		err   error
	)

	ret, st.gas, err, vmerr = msg.Execute(st.evm, st.state, st.gas, st.value)

	// TODO-Klaytn-Issue136
	if vmerr != nil {
		logger.Debug("VM returned with error", "err", vmerr)
		// The only possible consensus-error would be if there wasn't
		// sufficient balance to make the transfer happen. The first
		// balance transfer may never fail.
		// Another possible vmerr could be a time-limit error that happens
		// when the EVM is still running while the block proposer's total
		// execution time of txs for a candidate block reached the predefined
		// limit.
		if vmerr == vm.ErrInsufficientBalance || vmerr == vm.ErrTotalTimeLimitReached {
			kerr.Err = vmerr
			kerr.Status = getReceiptStatusFromVMerr(nil)
			return nil, 0, kerr
		}
	}
	// TODO-Klaytn-Issue136
	st.refundGas()

	// Defer transfering Tx fee when DeferredTxFee is true
	if st.evm.ChainConfig().Governance == nil || !st.evm.ChainConfig().Governance.DeferredTxFee() {
		st.state.AddBalance(st.evm.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.gasUsed()), st.gasPrice)) // TODO-Klaytn-Issue136 gasPrice
	}

	kerr.Err = err
	kerr.Status = getReceiptStatusFromVMerr(vmerr)
	return ret, st.gasUsed(), kerr
}

var vmerr2receiptstatus = map[error]uint{
	nil:                            types.ReceiptStatusSuccessful,
	vm.ErrDepth:                    types.ReceiptStatusErrDepth,
	vm.ErrContractAddressCollision: types.ReceiptStatusErrContractAddressCollision,
	vm.ErrCodeStoreOutOfGas:        types.ReceiptStatusErrCodeStoreOutOfGas,
	vm.ErrMaxCodeSizeExceeded:      types.ReceiptStatuserrMaxCodeSizeExceed,
	kerrors.ErrOutOfGas:            types.ReceiptStatusErrOutOfGas,
	vm.ErrWriteProtection:          types.ReceiptStatusErrWriteProtection,
	vm.ErrExecutionReverted:        types.ReceiptStatusErrExecutionReverted,
	vm.ErrOpcodeCntLimitReached:    types.ReceiptStatusErrOpcodeCntLimitReached,
}

var receiptstatus2vmerr = map[uint]error{
	types.ReceiptStatusSuccessful:                  nil,
	types.ReceiptStatusErrDefault:                  ErrVMDefault,
	types.ReceiptStatusErrDepth:                    vm.ErrDepth,
	types.ReceiptStatusErrContractAddressCollision: vm.ErrContractAddressCollision,
	types.ReceiptStatusErrCodeStoreOutOfGas:        vm.ErrCodeStoreOutOfGas,
	types.ReceiptStatuserrMaxCodeSizeExceed:        vm.ErrMaxCodeSizeExceeded,
	types.ReceiptStatusErrOutOfGas:                 kerrors.ErrOutOfGas,
	types.ReceiptStatusErrWriteProtection:          vm.ErrWriteProtection,
	types.ReceiptStatusErrExecutionReverted:        vm.ErrExecutionReverted,
	types.ReceiptStatusErrOpcodeCntLimitReached:    vm.ErrOpcodeCntLimitReached,
}

// getReceiptStatusFromVMerr returns corresponding ReceiptStatus for VM error.
func getReceiptStatusFromVMerr(vmerr error) (status uint) {
	// TODO-Klaytn Add more VM error to ReceiptStatus
	status, ok := vmerr2receiptstatus[vmerr]
	if !ok {
		// No corresponding receiptStatus available for vmerr
		status = types.ReceiptStatusErrDefault
	}

	return
}

// GetVMerrFromReceiptStatus returns VM error according to status of receipt.
func GetVMerrFromReceiptStatus(status uint) (vmerr error) {
	vmerr, ok := receiptstatus2vmerr[status]
	if !ok {
		return ErrInvalidReceiptStatus
	}

	return
}

func (st *StateTransition) refundGas() {
	// Apply refund counter, capped to half of the used gas.
	refund := st.gasUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.gas += refund

	// Return KLAY for remaining gas, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.gas), st.gasPrice) // TODO-Klaytn-Issue136 gasPrice

	feeRatio := st.msg.FeeRatio()
	validatedFeePayer := st.msg.ValidatedFeePayer()
	validatedSender := st.msg.ValidatedSender()
	switch {
	case feeRatio == types.MaxFeeRatio:
		// To make a short circuit, the below routine processes when feeRatio == 100.
		st.state.AddBalance(validatedFeePayer, remaining)

	case feeRatio < types.MaxFeeRatio:
		feePayer, feeSender := types.CalcFeeWithRatio(feeRatio, remaining)

		st.state.AddBalance(validatedFeePayer, feePayer)
		st.state.AddBalance(validatedSender, feeSender)

	default:
		// feeRatio > types.MaxFeeRatio
		// This will not happen because it is already checked in buyGas(), but to make sure, we add a log here.
		logger.Error("FeeRatio exceeds the maximum", "feeRatio", feeRatio, "msg", st.msg)
	}

	// Also return remaining gas to the block gas counter so it is
	// available for the next transaction.
	st.gp.AddGas(st.gas)
}

// gasUsed returns the amount of gas used up by the state transition.
func (st *StateTransition) gasUsed() uint64 {
	return st.initialGas - st.gas
}
