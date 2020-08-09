package kas

import (
	"context"
	"fmt"
	"github.com/klaytn/klaytn/accounts/abi"
	"github.com/klaytn/klaytn/accounts/abi/bind"
	"github.com/klaytn/klaytn/api"
	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/state"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/vm"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/hexutil"
	"github.com/klaytn/klaytn/consensus"
	"github.com/klaytn/klaytn/params"
	"math/big"
	"strings"
	"time"
)

// InterfaceIdentifierABI is the input ABI used to generate the binding from.
const InterfaceIdentifierABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"interfaceID\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

// TODO-ChainDataFetcher extract the call timeout c as a configuration
const callTimeout = 300 * time.Millisecond

type ChainReader interface {
	GetHeaderByNumber(number uint64) *types.Header
	StateAt(root common.Hash) (*state.StateDB, error)
	Engine() consensus.Engine
	GetHeader(common.Hash, uint64) *types.Header
	Config() *params.ChainConfig
}

type contractCaller2 struct {
	b           ChainReader
	callTimeout time.Duration
}

func newContractCaller2(b ChainReader) *contractCaller2 {
	return &contractCaller2{
		b:           b,
		callTimeout: 300 * time.Millisecond,
	}
}

func (c *contractCaller2) getHeaderAndStateByNumber(blockNumber *big.Int) (*types.Header, *state.StateDB, error) {
	header := c.b.GetHeaderByNumber(blockNumber.Uint64())
	if header == nil {
		return nil, nil, fmt.Errorf("the block does not exist (block number: %d)", blockNumber.Uint64())
	}

	stateDb, err := c.b.StateAt(header.Root)
	return header, stateDb, err
}

// IsContractAccount returns true if the account associated with addr has a non-empty codeHash.
// It returns false otherwise.
func (c *contractCaller2) IsContractAccount(contract common.Address, blockNumber *big.Int) (bool, error) {
	_, stateDb, err := c.getHeaderAndStateByNumber(blockNumber)
	if err != nil {
		return false, err
	}
	return stateDb.IsContractAccount(contract), stateDb.Error()
}

func (c *contractCaller2) doCall(ctx context.Context, args api.CallArgs, blockNumber *big.Int) ([]byte, error) {
	header, stateDb, err := c.getHeaderAndStateByNumber(blockNumber)
	if stateDb == nil || err != nil {
		return nil, err
	}

	msg := types.NewMessage(common.Address{}, args.To, 0, nil, 0, nil, args.Data, false, 0)
	// Setup context so it may be cancelled the call has completed
	// or, in case of unmetered gas, setup a context with a timeout.
	var cancel context.CancelFunc
	if c.callTimeout > 0 {
		ctx, cancel = context.WithTimeout(ctx, c.callTimeout)
	} else {
		ctx, cancel = context.WithCancel(ctx)
	}
	// Make sure the context is cancelled when the call has completed
	// this makes sure resources are cleaned up.
	defer cancel()
	evmContext := blockchain.NewEVMContext(msg, header, c.b, nil)
	vmenv := vm.NewEVM(evmContext, stateDb, c.b.Config(), &vm.Config{})

	// Wait for the context to be done and cancel the evm. Even if the
	// EVM has finished, cancelling may be done (repeatedly)
	go func() {
		<-ctx.Done()
		vmenv.Cancel(vm.CancelByCtxDone)
	}()

	ret, _, kerr := blockchain.NewStateTransition(vmenv, msg).TransitionDb()
	err = kerr.ErrTxInvalid
	if err == nil {
		err = blockchain.GetVMerrFromReceiptStatus(kerr.Status)
	}
	return ret, nil
}

func (c *contractCaller2) supportsInterface(contract common.Address, opts *bind.CallOpts, interfaceID [4]byte) (bool, error) {
	method := "supportsInterface"
	parsed, err := abi.JSON(strings.NewReader(InterfaceIdentifierABI))
	if err != nil {
		return false, err
	}

	data, err := parsed.Pack(method, interfaceID)
	if err != nil {
		return false, err
	}

	callArgs := api.CallArgs{
		To:   &contract,
		Data: hexutil.Bytes(data),
	}

	ret, err := c.doCall(opts.Context, callArgs, opts.BlockNumber)
	if err != nil {
		return false, err
	}

	output := new(bool)
	err = parsed.Unpack(output, method, ret)
	if err != nil {
		return false, err
	}

	return *output, nil
}

// the `SupportsInterface` method error must be handled with the following cases.
// case 1: the contract implements fallback function
// - the call can be reverted within fallback function: returns "evm: execution reverted"
// - the call can be done successfully, but it outputs empty: returns "abi: unmarshalling empty output"
// case 2: the contract does not implements fallback function
// - the call can be reverted: returns "evm: execution reverted"
// handleSupportsInterfaceErr handles the given error according to the above explanation.
func handleSupportsInterfaceErr(err error) error {
	if err != nil && (strings.Contains(err.Error(), errMsgEmptyOutput) || strings.Contains(err.Error(), errMsgEvmReverted)) {
		return nil
	}
	return err
}

// isKIP13 checks if the given contract implements KIP13 interface or not at the given block.
func (c *contractCaller2) isKIP13(contract common.Address, blockNumber *big.Int) (bool, error) {
	var (
		opts   *bind.CallOpts
		cancel context.CancelFunc
	)
	opts, cancel = getCallOpts(blockNumber, c.callTimeout)
	defer cancel()
	if isKIP13, err := c.supportsInterface(contract, opts, IKIP13Id); err != nil {
		logger.Error("supportsInterface is failed", "contract", contract.String(), "blockNumber", blockNumber, "interfaceID", hexutil.Encode(IKIP13Id[:]))
		return false, err
	} else if !isKIP13 {
		return false, nil
	}

	opts, cancel = getCallOpts(blockNumber, c.callTimeout)
	defer cancel()
	if isInvalid, err := c.supportsInterface(contract, opts, InvalidId); err != nil {
		logger.Error("supportsInterface is failed", "contract", contract.String(), "blockNumber", blockNumber, "interfaceID", hexutil.Encode(InvalidId[:]))
		return false, err
	} else if isInvalid {
		return false, nil
	}

	return true, nil
}

// isKIP7 checks if the given contract implements IKIP7 and IKIP7Metadata interface or not at the given block.
func (c *contractCaller2) isKIP7(contract common.Address, blockNumber *big.Int) (bool, error) {
	var (
		opts   *bind.CallOpts
		cancel context.CancelFunc
	)
	opts, cancel = getCallOpts(blockNumber, c.callTimeout)
	defer cancel()
	if isIKIP7, err := c.supportsInterface(contract, opts, IKIP7Id); err != nil {
		logger.Error("supportsInterface is failed", "contract", contract.String(), "blockNumber", blockNumber, "interfaceID", hexutil.Encode(IKIP7Id[:]))
		return false, err
	} else if !isIKIP7 {
		return false, nil
	}

	opts, cancel = getCallOpts(blockNumber, c.callTimeout)
	defer cancel()
	if isIKIP7Metadata, err := c.supportsInterface(contract, opts, IKIP7MetadataId); err != nil {
		logger.Error("supportsInterface is failed", "contract", contract.String(), "blockNumber", blockNumber, "interfaceID", hexutil.Encode(IKIP7MetadataId[:]))
		return false, err
	} else if !isIKIP7Metadata {
		return false, nil
	}

	return true, nil
}

// isKIP17 checks if the given contract implements IKIP17 and IKIP17Metadata interface or not at the given block.
func (c *contractCaller2) isKIP17(contract common.Address, blockNumber *big.Int) (bool, error) {
	var (
		opts   *bind.CallOpts
		cancel context.CancelFunc
	)
	opts, cancel = getCallOpts(blockNumber, c.callTimeout)
	defer cancel()
	if isIKIP17, err := c.supportsInterface(contract, opts, IKIP17Id); err != nil {
		logger.Error("supportsInterface is failed", "contract", contract.String(), "blockNumber", blockNumber, "interfaceID", hexutil.Encode(IKIP17Id[:]))
		return false, err
	} else if !isIKIP17 {
		return false, nil
	}

	opts, cancel = getCallOpts(blockNumber, c.callTimeout)
	defer cancel()
	if isIKIP17Metadata, err := c.supportsInterface(contract, opts, IKIP17MetadataId); err != nil {
		logger.Error("supportsInterface is failed", "contract", contract.String(), "blockNumber", blockNumber, "interfaceID", hexutil.Encode(IKIP17MetadataId[:]))
		return false, err
	} else if !isIKIP17Metadata {
		return false, nil
	}

	return true, nil
}
