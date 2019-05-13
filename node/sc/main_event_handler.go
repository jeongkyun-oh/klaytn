// Copyright 2018 The klaytn Authors
// This file is part of the klaytn library.
//
// The klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the klaytn library. If not, see <http://www.gnu.org/licenses/>.

package sc

import (
	"errors"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/ser/rlp"
)

var (
	ErrGetServiceChainPHInMCEH = errors.New("ServiceChainPH isn't set in MainChainEventHandler")
)

type MainChainEventHandler struct {
	mainbridge *MainBridge

	handler *MainBridgeHandler
}

func NewMainChainEventHandler(bridge *MainBridge, handler *MainBridgeHandler) (*MainChainEventHandler, error) {
	return &MainChainEventHandler{mainbridge: bridge, handler: handler}, nil
}

func (mce *MainChainEventHandler) HandleChainHeadEvent(block *types.Block) error {
	logger.Debug("bridgeNode block number", "number", block.Number())
	mce.writeChildChainTxHashFromBlock(block)
	return nil
}

func (mce *MainChainEventHandler) HandleTxEvent(tx *types.Transaction) error {
	//@TODO-Klaytn event handle
	return nil
}

func (mce *MainChainEventHandler) HandleTxsEvent(txs []*types.Transaction) error {
	//@TODO-Klaytn event handle
	return nil
}

func (mce *MainChainEventHandler) HandleLogsEvent(logs []*types.Log) error {
	//@TODO-Klaytn event handle
	return nil
}

// GetChildChainIndexingEnabled returns the current child chain indexing configuration.
func (mce *MainChainEventHandler) GetChildChainIndexingEnabled() bool {
	return mce.mainbridge.chainDB.ChildChainIndexingEnabled()
}

// GetLastIndexedBlockNumber returns the last child block number indexed to chain DB.
func (mce *MainChainEventHandler) GetLastIndexedBlockNumber() uint64 {
	return mce.mainbridge.chainDB.GetLastIndexedBlockNumber()
}

// WriteLastIndexedBlockNumber writes the last child block number indexed to chain DB.
func (mce *MainChainEventHandler) WriteLastIndexedBlockNumber(blockNum uint64) {
	mce.mainbridge.chainDB.WriteLastIndexedBlockNumber(blockNum)
}

// ConvertServiceChainBlockHashToMainChainTxHash returns a transaction hash of a transaction which contains
// ChainHashes, with the key made with given service chain block hash.
// Index is built when service chain indexing is enabled.
func (mce *MainChainEventHandler) ConvertServiceChainBlockHashToMainChainTxHash(scBlockHash common.Hash) common.Hash {
	return mce.mainbridge.chainDB.ConvertServiceChainBlockHashToMainChainTxHash(scBlockHash)
}

// WriteChildChainTxHash stores a transaction hash of a transaction which contains
// ChainHashes, with the key made with given child chain block hash.
// Index is built when child chain indexing is enabled.
func (mce *MainChainEventHandler) WriteChildChainTxHash(ccBlockHash common.Hash, ccTxHash common.Hash) {
	mce.mainbridge.chainDB.WriteChildChainTxHash(ccBlockHash, ccTxHash)
}

// writeChildChainTxHashFromBlock writes transaction hashes of transactions which contain
// ChainHashes.
func (mce *MainChainEventHandler) writeChildChainTxHashFromBlock(block *types.Block) {
	if !mce.GetChildChainIndexingEnabled() {
		logger.Trace("ChildChainIndexing is disabled. Skipped to write anchoring data on chainDB", "Head block", block.NumberU64())
		return
	}

	lastIndexedBlkNum := mce.GetLastIndexedBlockNumber()
	chainHeadBlkNum := block.NumberU64()

	for i := lastIndexedBlkNum + 1; i <= chainHeadBlkNum; i++ {
		blk := mce.mainbridge.blockchain.GetBlockByNumber(i)

		txs := blk.Transactions()
		for _, tx := range txs {
			if tx.Type() != types.TxTypeChainDataAnchoring {
				continue
			}

			chainHashes := new(types.ChainHashes)
			data, err := tx.AnchoredData()
			if err != nil {
				logger.Error("writeChildChainTxHashFromBlock : failed to get anchoring data from the tx", "txHash", tx.Hash().String())
				continue
			}
			if err := rlp.DecodeBytes(data, chainHashes); err != nil {
				logger.Error("writeChildChainTxHashFromBlock : failed to decode anchoring data")
				continue
			}
			mce.mainbridge.chainDB.WriteChildChainTxHash(chainHashes.BlockHash, tx.Hash())

			logger.Trace("Write anchoring data on chainDB", "blockHash", chainHashes.BlockHash.String(), "txHash", tx.Hash().String())
		}
	}
	logger.Trace("Done indexing Blocks", "begin", lastIndexedBlkNum+1, "end", chainHeadBlkNum)
	mce.WriteLastIndexedBlockNumber(chainHeadBlkNum)
}
