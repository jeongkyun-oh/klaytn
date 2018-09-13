package backend

import (
	"fmt"
	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/blockchain/types"
	"github.com/ground-x/go-gxplatform/networks/rpc"
	"errors"
	"github.com/ground-x/go-gxplatform/blockchain"
	"github.com/ground-x/go-gxplatform/common/hexutil"
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

// GetValidators retrieves the list of authorized validators at the specified block.
func (api *APIExtension) GetValidators(number *rpc.BlockNumber) ([]common.Address, error) {
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

func (api *APIExtension) getProposerAndValidators(block *types.Block) (common.Address, []common.Address, error) {
	// get the proposer of this block.
	proposer, err := ecrecover(block.Header())
	if err != nil {
		return common.Address{}, []common.Address{}, err
	}

	// get the snapshot of the previous block.
	blockNumber := block.NumberU64()
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
	//		log.Error("validator is different!", "snap", commiteeAddrs, "istanbul", istanbulAddrs)
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

		rpcTransactions[i] = map[string]interface{} {
			"blockHash": hash,
			"blockNumber": (*hexutil.Big)(b.Number()),
			"from":     from,
			"gas":      hexutil.Uint64(tx.Gas()),
			"gasPrice": (*hexutil.Big)(tx.GasPrice()),
			"gasUsed":  hexutil.Uint64(receipts[i].GasUsed),
			"txHash":     tx.Hash(),
			"input":    hexutil.Bytes(tx.Data()),
			"nonce":    hexutil.Uint64(tx.Nonce()),
			"to":       tx.To(),
			"transactionIndex": hexutil.Uint(i),
			"value":    (*hexutil.Big)(tx.Value()),
			"contractAddress": receipts[i].ContractAddress,
			"cumulativeGasUsed": hexutil.Uint64(receipts[i].CumulativeGasUsed),
			"logs": receipts[i].Logs,
			"status": hexutil.Uint(receipts[i].Status),
		}
	}

	return map[string]interface{} {
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
		return nil, errors.New("chain is not blockchain.BlockChain")
	}
	blockNumber := uint64(number.Int64())
	block := b.GetBlockByNumber(blockNumber)
	if block == nil {
		return nil, errors.New(fmt.Sprintf("block %d not found", blockNumber))
	}
	blockHash := block.Hash()

	proposer, committee, err := api.getProposerAndValidators(block)
	if err != nil {
		return nil, err
	}

	receipts, err := b.GetReceiptInCache(blockHash)
	if receipts == nil {
		receipts = b.GetReceiptsByHash(blockHash)
	}
	return api.makeRPCOutput(block, proposer, committee, block.Transactions(), receipts), nil
}

func (api *APIExtension) GetBlockWithConsensusInfoByNumberRange(start *rpc.BlockNumber, end *rpc.BlockNumber) (map[string]interface{}, error){
	blocks := make(map[string]interface{})

	// check error status.
	s := start.Int64()
	e := end.Int64()
	if s < 0 {
		return nil, errors.New("start should be positive")
	}

	eChain := api.chain.CurrentHeader().Number.Int64()
	if e > eChain {
		return nil, errors.New(fmt.Sprintf("end should be smaller than the last block number %d", eChain))
	}

	if s > e {
		return nil, errors.New("start should be smaller than end")
	}

	if (e - s) > 50 {
		return nil, errors.New("number of requested blocks should be smaller than 50")
	}

	// gather s~e blocks
	for i := s; i <= e; i++ {
		strIdx := fmt.Sprintf("0x%x", i)

		blockNum := rpc.BlockNumber(i)
		r, err := api.GetBlockWithConsensusInfoByNumber(&blockNum)
		if err != nil {
			return nil, err
		}

		blocks[strIdx] = r
	}

	return blocks, nil
}

func (api *APIExtension) GetBlockWithConsensusInfoByHash(blockHash common.Hash) (map[string]interface{}, error) {
	b, ok := api.chain.(*blockchain.BlockChain)
	if !ok {
		return nil, errors.New("chain is not blockchain.BlockChain")
	}

	block := b.GetBlockByHash(blockHash)

	proposer, committee, err := api.getProposerAndValidators(block)
	if err != nil {
		return nil, err
	}

	receipts, err := b.GetReceiptInCache(blockHash)
	if receipts == nil {
		receipts = b.GetReceiptsByHash(blockHash)
	}

	return api.makeRPCOutput(block, proposer, committee, block.Transactions(), receipts), nil
}

