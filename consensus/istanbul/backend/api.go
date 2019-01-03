// Copyright 2018 The go-klaytn Authors
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
// This file is derived from quorum/consensus/istanbul/backend/api.go (2018/06/04).
// Modified and improved for the go-klaytn development.

package backend

import (
	"errors"
	"fmt"
	"github.com/ground-x/go-gxplatform/blockchain"
	"github.com/ground-x/go-gxplatform/blockchain/types"
	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/common/hexutil"
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/networks/rpc"
	"reflect"
)

// API is a user facing RPC API to dump Istanbul state
type API struct {
	chain    consensus.ChainReader
	istanbul *backend
}

// GetSnapshot retrieves the state snapshot at a given block.
func (api *API) GetSnapshot(number *rpc.BlockNumber) (*Snapshot, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.istanbul.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
}

// GetSnapshotAtHash retrieves the state snapshot at a given block.
func (api *API) GetSnapshotAtHash(hash common.Hash) (*Snapshot, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	return api.istanbul.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
}

// GetValidators retrieves the list of authorized validators at the specified block.
func (api *API) GetValidators(number *rpc.BlockNumber) ([]common.Address, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return the validators from its snapshot
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.istanbul.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.validators(), nil
}

// GetValidatorsAtHash retrieves the state snapshot at a given block.
func (api *API) GetValidatorsAtHash(hash common.Hash) ([]common.Address, error) {
	header := api.chain.GetHeaderByHash(hash)
	if header == nil {
		return nil, errUnknownBlock
	}
	snap, err := api.istanbul.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		return nil, err
	}
	return snap.validators(), nil
}

// Candidates returns the current candidates the node tries to uphold and vote on.
func (api *API) Candidates() map[common.Address]bool {
	api.istanbul.candidatesLock.RLock()
	defer api.istanbul.candidatesLock.RUnlock()

	proposals := make(map[common.Address]bool)
	for address, auth := range api.istanbul.candidates {
		proposals[address] = auth
	}
	return proposals
}

// Propose injects a new authorization candidate that the validator will attempt to
// push through.
func (api *API) Propose(address common.Address, auth bool) {
	api.istanbul.candidatesLock.Lock()
	defer api.istanbul.candidatesLock.Unlock()

	api.istanbul.candidates[address] = auth
}

// Discard drops a currently running candidate, stopping the validator from casting
// further votes (either for or against).
func (api *API) Discard(address common.Address) {
	api.istanbul.candidatesLock.Lock()
	defer api.istanbul.candidatesLock.Unlock()

	delete(api.istanbul.candidates, address)
}

// API extended by Klaytn developers
type APIExtension struct {
	chain    consensus.ChainReader
	istanbul *backend
}

var (
	errPendingNotAllowed       = errors.New("pending is not allowed")
	errInternalError           = errors.New("internal error")
	errStartNotPositive        = errors.New("start block number should be positive")
	errEndLargetThanLatest     = errors.New("end block number should be smaller than the latest block number")
	errStartLargerThanEnd      = errors.New("start should be smaller than end")
	errRequestedBlocksTooLarge = errors.New("number of requested blocks should be smaller than 50")
)

// GetValidators retrieves the list of authorized validators at the specified block.
func (api *APIExtension) GetValidators(number *rpc.BlockNumber) ([]common.Address, error) {
	// Retrieve the requested block number (or current if none requested)
	var header *types.Header
	if number == nil || *number == rpc.LatestBlockNumber {
		header = api.chain.CurrentHeader()
	} else if *number == rpc.PendingBlockNumber {
		logger.Error("Cannot get validators of the pending block.", "number", number)
		return nil, errPendingNotAllowed
	} else {
		header = api.chain.GetHeaderByNumber(uint64(number.Int64()))
	}
	// Ensure we have an actually valid block and return the validators from its snapshot
	if header == nil {
		logger.Error("Failed to find the requested block", "number", number)
		return nil, nil // return nil if block is not found.
	}
	snap, err := api.istanbul.snapshot(api.chain, header.Number.Uint64(), header.Hash(), nil)
	if err != nil {
		logger.Error("Failed to get snapshot.", "hash", header.Hash(), "err", err)
		return nil, errInternalError
	}
	return snap.validators(), nil
}

func (api *APIExtension) getProposerAndValidators(block *types.Block) (common.Address, []common.Address, error) {
	blockNumber := block.NumberU64()
	if blockNumber == 0 {
		return common.Address{}, []common.Address{}, nil
	}

	// get the proposer of this block.
	proposer, err := ecrecover(block.Header())
	if err != nil {
		return common.Address{}, []common.Address{}, err
	}

	// get the snapshot of the previous block.
	parentHash := block.ParentHash()
	snap, err := api.istanbul.snapshot(api.chain, blockNumber-1, parentHash, nil)
	if err != nil {
		return proposer, []common.Address{}, err
	}

	// get the committee list of this block.
	committee := snap.ValSet.SubListWithProposer(parentHash, proposer)
	commiteeAddrs := make([]common.Address, len(committee))
	for i, v := range committee {
		commiteeAddrs[i] = v.Address()
	}

	// verify the committee list of the block using istanbul
	//proposalSeal := istanbulCore.PrepareCommittedSeal(block.Hash())
	//extra, err := types.ExtractIstanbulExtra(block.Header())
	//istanbulAddrs := make([]common.Address, len(commiteeAddrs))
	//for i, seal := range extra.CommittedSeal {
	//	addr, err := istanbul.GetSignatureAddress(proposalSeal, seal)
	//	istanbulAddrs[i] = addr
	//	if err != nil {
	//		return proposer, []common.Address{}, err
	//	}
	//
	//	var found bool = false
	//	for _, v := range commiteeAddrs {
	//		if addr == v {
	//			found = true
	//			break
	//		}
	//	}
	//	if found == false {
	//		logger.Error("validator is different!", "snap", commiteeAddrs, "istanbul", istanbulAddrs)
	//		return proposer, commiteeAddrs, errors.New("validator set is different from Istanbul engine!!")
	//	}
	//}

	return proposer, commiteeAddrs, nil
}

func (api *APIExtension) makeRPCOutput(b *types.Block, proposer common.Address, committee []common.Address,
	transactions types.Transactions, receipts types.Receipts) map[string]interface{} {
	head := b.Header() // copies the header once
	hash := head.Hash()

	// make transactions
	numTxs := len(transactions)
	rpcTransactions := make([]map[string]interface{}, numTxs)
	for i, tx := range transactions {
		var signer types.Signer = types.FrontierSigner{}
		if tx.Protected() {
			signer = types.NewEIP155Signer(tx.ChainId())
		}
		from, _ := types.Sender(signer, tx)

		rpcTransactions[i] = map[string]interface{}{
			"blockHash":         hash,
			"blockNumber":       (*hexutil.Big)(b.Number()),
			"from":              from,
			"gas":               hexutil.Uint64(tx.Gas()),
			"gasPrice":          (*hexutil.Big)(tx.GasPrice()),
			"gasUsed":           hexutil.Uint64(receipts[i].GasUsed),
			"txHash":            tx.Hash(),
			"input":             hexutil.Bytes(tx.Data()),
			"nonce":             hexutil.Uint64(tx.Nonce()),
			"to":                tx.To(),
			"transactionIndex":  hexutil.Uint(i),
			"value":             (*hexutil.Big)(tx.Value()),
			"contractAddress":   receipts[i].ContractAddress,
			"cumulativeGasUsed": hexutil.Uint64(receipts[i].CumulativeGasUsed),
			"logs":              receipts[i].Logs,
			"status":            hexutil.Uint(receipts[i].Status),
		}
	}

	return map[string]interface{}{
		"number":           (*hexutil.Big)(head.Number),
		"hash":             b.Hash(),
		"parentHash":       head.ParentHash,
		"nonce":            head.Nonce,
		"stateRoot":        head.Root,
		"miner":            head.Coinbase,
		"size":             hexutil.Uint64(b.Size()),
		"gasLimit":         hexutil.Uint64(head.GasLimit),
		"gasUsed":          hexutil.Uint64(head.GasUsed),
		"timestamp":        (*hexutil.Big)(head.Time),
		"transactionsRoot": head.TxHash,
		"receiptsRoot":     head.ReceiptHash,
		"committee":        committee,
		"proposer":         proposer,
		"transactions":     rpcTransactions,
	}
}

// TODO-GX: This API functions should be managed with API functions with namespace "klay"
func (api *APIExtension) GetBlockWithConsensusInfoByNumber(number *rpc.BlockNumber) (map[string]interface{}, error) {
	b, ok := api.chain.(*blockchain.BlockChain)
	if !ok {
		logger.Error("chain is not a type of blockchain.BlockChain", "type", reflect.TypeOf(api.chain))
		return nil, errInternalError
	}
	var block *types.Block
	var blockNumber uint64

	if number == nil {
		number = new(rpc.BlockNumber)
		*number = rpc.LatestBlockNumber
	}

	if *number == rpc.PendingBlockNumber {
		logger.Error("Cannot get consensus information of the PendingBlock.")
		return nil, errPendingNotAllowed
	}

	if *number == rpc.LatestBlockNumber {
		block = b.CurrentBlock()
		blockNumber = block.NumberU64()
	} else {
		// rpc.EarliestBlockNumber == 0, no need to treat it as a special case.
		blockNumber = uint64(number.Int64())
		block = b.GetBlockByNumber(blockNumber)
	}

	if block == nil {
		logger.Error("Finding a block by number failed.", "blockNum", blockNumber)
		return nil, fmt.Errorf("the block does not exist (block number: %d)", blockNumber)
	}
	blockHash := block.Hash()

	proposer, committee, err := api.getProposerAndValidators(block)
	if err != nil {
		logger.Error("Getting the proposer and validators failed.", "blockHash", blockHash, "err", err)
		return nil, errInternalError
	}

	receipts, err := b.GetReceiptsInCache(blockHash)
	if receipts == nil {
		receipts = b.GetReceiptsByBlockHash(blockHash)
	}
	return api.makeRPCOutput(block, proposer, committee, block.Transactions(), receipts), nil
}

func (api *APIExtension) GetBlockWithConsensusInfoByNumberRange(start *rpc.BlockNumber, end *rpc.BlockNumber) (map[string]interface{}, error) {
	blocks := make(map[string]interface{})

	// check error status.
	s := start.Int64()
	e := end.Int64()
	if s < 0 {
		logger.Error("start should be positive", "start", s)
		return nil, errStartNotPositive
	}

	eChain := api.chain.CurrentHeader().Number.Int64()
	if e > eChain {
		logger.Error("end should be smaller than the lastest block number", "end", end, "eChain", eChain)
		return nil, errEndLargetThanLatest
	}

	if s > e {
		logger.Error("start should be smaller than end", "start", s, "end", e)
		return nil, errStartLargerThanEnd
	}

	if (e - s) > 50 {
		logger.Error("number of requested blocks should be smaller than 50", "start", s, "end", e)
		return nil, errRequestedBlocksTooLarge
	}

	// gather s~e blocks
	for i := s; i <= e; i++ {
		strIdx := fmt.Sprintf("0x%x", i)

		blockNum := rpc.BlockNumber(i)
		b, err := api.GetBlockWithConsensusInfoByNumber(&blockNum)
		if err != nil {
			logger.Error("error on GetBlockWithConsensusInfoByNumber", "err", err)
			blocks[strIdx] = nil
		} else {
			blocks[strIdx] = b
		}
	}

	return blocks, nil
}

func (api *APIExtension) GetBlockWithConsensusInfoByHash(blockHash common.Hash) (map[string]interface{}, error) {
	b, ok := api.chain.(*blockchain.BlockChain)
	if !ok {
		logger.Error("chain is not a type of blockchain.Blockchain, returning...", "type", reflect.TypeOf(api.chain))
		return nil, errInternalError
	}

	block := b.GetBlockByHash(blockHash)
	if block == nil {
		logger.Error("Finding a block failed.", "blockHash", blockHash)
		return nil, fmt.Errorf("the block does not exist (block hash: %s)", blockHash.String())
	}

	proposer, committee, err := api.getProposerAndValidators(block)
	if err != nil {
		logger.Error("Getting the proposer and validators failed.", "blockHash", blockHash, "err", err)
		return nil, errInternalError
	}

	receipts, _ := b.GetReceiptsInCache(blockHash)
	if receipts == nil {
		receipts = b.GetReceiptsByBlockHash(blockHash)
	}

	return api.makeRPCOutput(block, proposer, committee, block.Transactions(), receipts), nil
}
