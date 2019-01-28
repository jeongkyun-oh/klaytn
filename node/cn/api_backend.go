// Copyright 2018 The klaytn Authors
// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
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
// This file is derived from eth/api_backend.go (2018/06/04).
// Modified and improved for the klaytn development.

package cn

import (
	"context"
	"fmt"
	"github.com/ground-x/klaytn/accounts"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/bloombits"
	"github.com/ground-x/klaytn/blockchain/state"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/blockchain/vm"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/math"
	"github.com/ground-x/klaytn/datasync/downloader"
	"github.com/ground-x/klaytn/event"
	"github.com/ground-x/klaytn/networks/rpc"
	"github.com/ground-x/klaytn/node/cn/gasprice"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/storage/database"
	"math/big"
)

// CNAPIBackend implements api.Backend for full nodes
type CNAPIBackend struct {
	cn  *CN
	gpo *gasprice.Oracle
}

func (b *CNAPIBackend) GetTransactionInCache(hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	return b.cn.blockchain.GetTransactionInCache(hash)
}

func (b *CNAPIBackend) GetReceiptsInCache(blockHash common.Hash) types.Receipts {
	return b.cn.blockchain.GetReceiptsInCache(blockHash)
}

func (b *CNAPIBackend) ChainConfig() *params.ChainConfig {
	return b.cn.chainConfig
}

func (b *CNAPIBackend) CurrentBlock() *types.Block {
	return b.cn.blockchain.CurrentBlock()
}

func (b *CNAPIBackend) SetHead(number uint64) {
	//b.cn.protocolManager.downloader.Cancel()
	b.cn.blockchain.SetHead(number)
}

func (b *CNAPIBackend) HeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.cn.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.cn.blockchain.CurrentBlock().Header(), nil
	}
	header := b.cn.blockchain.GetHeaderByNumber(uint64(blockNr))
	if header == nil {
		return nil, fmt.Errorf("the block does not exist (block number: %d)", blockNr)
	}
	return header, nil
}

func (b *CNAPIBackend) BlockByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block := b.cn.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if blockNr == rpc.LatestBlockNumber {
		return b.cn.blockchain.CurrentBlock(), nil
	}
	block := b.cn.blockchain.GetBlockByNumber(uint64(blockNr))
	if block == nil {
		return nil, fmt.Errorf("the block does not exist (block number: %d)", blockNr)
	}
	return block, nil
}

func (b *CNAPIBackend) StateAndHeaderByNumber(ctx context.Context, blockNr rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if blockNr == rpc.PendingBlockNumber {
		block, state := b.cn.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, blockNr)
	if header == nil || err != nil {
		return nil, nil, err
	}
	stateDb, err := b.cn.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *CNAPIBackend) GetBlock(ctx context.Context, hash common.Hash) (*types.Block, error) {
	block := b.cn.blockchain.GetBlockByHash(hash)
	if block == nil {
		return nil, fmt.Errorf("the block does not exist (block hash: %s)", hash.String())
	}
	return block, nil
}

func (b *CNAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) types.Receipts {
	return b.cn.blockchain.GetReceiptsByBlockHash(hash)
}

func (b *CNAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	return b.cn.blockchain.GetLogsByHash(hash), nil
}

func (b *CNAPIBackend) GetTd(blockHash common.Hash) *big.Int {
	return b.cn.blockchain.GetTdByHash(blockHash)
}

func (b *CNAPIBackend) GetEVM(ctx context.Context, msg blockchain.Message, state *state.StateDB, header *types.Header, vmCfg vm.Config) (*vm.EVM, func() error, error) {
	state.SetBalance(msg.From(), math.MaxBig256)
	vmError := func() error { return nil }

	context := blockchain.NewEVMContext(msg, header, b.cn.BlockChain(), nil)
	return vm.NewEVM(context, state, b.cn.chainConfig, &vmCfg), vmError, nil
}

func (b *CNAPIBackend) SubscribeRemovedLogsEvent(ch chan<- blockchain.RemovedLogsEvent) event.Subscription {
	return b.cn.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *CNAPIBackend) SubscribeChainEvent(ch chan<- blockchain.ChainEvent) event.Subscription {
	return b.cn.BlockChain().SubscribeChainEvent(ch)
}

func (b *CNAPIBackend) SubscribeChainHeadEvent(ch chan<- blockchain.ChainHeadEvent) event.Subscription {
	return b.cn.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *CNAPIBackend) SubscribeChainSideEvent(ch chan<- blockchain.ChainSideEvent) event.Subscription {
	return b.cn.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *CNAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.cn.BlockChain().SubscribeLogsEvent(ch)
}

func (b *CNAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.cn.txPool.AddLocal(signedTx)
}

func (b *CNAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.cn.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *CNAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.cn.txPool.Get(hash)
}

func (b *CNAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) uint64 {
	return b.cn.txPool.State().GetNonce(addr)
}

func (b *CNAPIBackend) Stats() (pending int, queued int) {
	return b.cn.txPool.Stats()
}

func (b *CNAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.cn.TxPool().Content()
}

func (b *CNAPIBackend) SubscribeNewTxsEvent(ch chan<- blockchain.NewTxsEvent) event.Subscription {
	return b.cn.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *CNAPIBackend) Downloader() *downloader.Downloader {
	return b.cn.Downloader()
}

func (b *CNAPIBackend) ProtocolVersion() int {
	return b.cn.ProtocolVersion()
}

func (b *CNAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) { // TODO-Klaytn-Issue136 gasPrice
	return b.gpo.SuggestPrice(ctx)
}

func (b *CNAPIBackend) ChainDB() database.DBManager {
	return b.cn.ChainDB()
}

func (b *CNAPIBackend) EventMux() *event.TypeMux {
	return b.cn.EventMux()
}

func (b *CNAPIBackend) AccountManager() *accounts.Manager {
	return b.cn.AccountManager()
}

func (b *CNAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.cn.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *CNAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.cn.bloomRequests)
	}
}

func (b *CNAPIBackend) GetChildChainIndexingEnabled() bool {
	return b.cn.blockchain.GetChildChainIndexingEnabled()
}

func (b *CNAPIBackend) ConvertChildChainBlockHashToParentChainTxHash(ccBlockHash common.Hash) common.Hash {
	return b.cn.blockchain.ConvertChildChainBlockHashToParentChainTxHash(ccBlockHash)
}

func (b *CNAPIBackend) GetLatestAnchoredBlockNumber() uint64 {
	return b.cn.blockchain.GetLatestAnchoredBlockNumber()
}

func (b *CNAPIBackend) GetReceiptFromParentChain(blockHash common.Hash) *types.Receipt {
	return b.cn.blockchain.GetReceiptFromParentChain(blockHash)
}

func (b *CNAPIBackend) GetChainAddr() string {
	return b.cn.protocolManager.GetChainAddr()
}

func (b *CNAPIBackend) GetChainTxPeriod() uint64 {
	return b.cn.protocolManager.GetChainTxPeriod()
}

func (b *CNAPIBackend) GetSentChainTxsLimit() uint64 {
	return b.cn.protocolManager.GetSentChainTxsLimit()
}
