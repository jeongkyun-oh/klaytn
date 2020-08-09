package kas

import (
	"context"
	"fmt"
	"github.com/klaytn/klaytn/accounts/abi"
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
)

// InterfaceIdentifierABI is the input ABI used to generate the binding from.
const InterfaceIdentifierABI = "[{\"constant\":true,\"inputs\":[{\"name\":\"interfaceID\",\"type\":\"bytes4\"}],\"name\":\"supportsInterface\",\"outputs\":[{\"name\":\"\",\"type\":\"bool\"}],\"payable\":false,\"stateMutability\":\"view\",\"type\":\"function\"}]"

type ChainReader interface {
	GetHeaderByNumber(number uint64) *types.Header
	StateAt(root common.Hash) (*state.StateDB, error)
	Engine() consensus.Engine
	GetHeader(common.Hash, uint64) *types.Header
	Config() *params.ChainConfig
}

type contractCaller2 struct {
	b ChainReader
}

func newContractCaller2(b ChainReader) *contractCaller2 {
	return &contractCaller2{
		b: b,
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

func (c *contractCaller2) doCall(ctx *context.Context, args api.CallArgs, blockNumber *big.Int) ([]byte, error) {
	header, stateDb, err := c.getHeaderAndStateByNumber(blockNumber)
	if stateDb == nil || err != nil {
		return nil, err
	}

	msg := types.NewMessage(common.Address{}, args.To, 0, nil, 0, nil, args.Data, false, 0)
	evmContext := blockchain.NewEVMContext(msg, header, c.b, nil)
	vmenv := vm.NewEVM(evmContext, stateDb, c.b.Config(), &vm.Config{})

	ret, _, kerr := blockchain.NewStateTransition(vmenv, msg).TransitionDb()
	err = kerr.ErrTxInvalid
	if err == nil {
		err = blockchain.GetVMerrFromReceiptStatus(kerr.Status)
	}
	return ret, nil
}

func (c *contractCaller2) supportsInterface(ctx *context.Context, contract common.Address, interfaceID [4]byte, blockNumber *big.Int) (bool, error) {
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

	ret, err := c.doCall(ctx, callArgs, blockNumber)
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
