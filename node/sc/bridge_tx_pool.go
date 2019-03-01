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
// This file is derived from core/tx_pool.go (2018/06/04).
// Modified and improved for the klaytn development.

package sc

import (
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/metrics"
	"math/big"
	"sync"
	"time"
)

var (
	ErrKnownTx           = errors.New("Known Transaction")
	ErrUnknownTx         = errors.New("Unknown Transaction")
	ErrDuplicatedNonceTx = errors.New("Duplicated Nonce Transaction")
)

// TODO-Klaytn-Servicechain Add Metrics
var (
	// Metrics for the pending pool
	refusedTxCounter = metrics.NewRegisteredCounter("bridgeTxpool/refuse", nil)
)

// BridgeTxPoolConfig are the configuration parameters of the transaction pool.
type BridgeTxPoolConfig struct {
	ParentChainID *big.Int
	Journal       string        // Journal of local transactions to survive node restarts
	Rejournal     time.Duration // Time interval to regenerate the local transaction journal

	GlobalQueue uint64 // Maximum number of non-executable transaction slots for all accounts
}

// DefaultBridgeTxPoolConfig contains the default configurations for the transaction
// pool.
var DefaultBridgeTxPoolConfig = BridgeTxPoolConfig{
	ParentChainID: big.NewInt(2018),
	Journal:       "bridge_transactions.rlp",
	Rejournal:     time.Hour,
	GlobalQueue:   8192,
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *BridgeTxPoolConfig) sanitize() BridgeTxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		logger.Error("Sanitizing invalid bridgetxpool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}

	if conf.Journal == "" {
		logger.Error("Sanitizing invalid bridgetxpool journal file name", "updated", DefaultBridgeTxPoolConfig.Journal)
		conf.Journal = DefaultBridgeTxPoolConfig.Journal
	}

	return conf
}

// BridgeTxPool contains all currently known chain transactions.
type BridgeTxPool struct {
	config BridgeTxPoolConfig
	// TODO-Klaytn-Servicechain consider to remove singer. For now, caused of value transfer tx which don't have `from` value, I leave it.
	signer types.Signer
	mu     sync.RWMutex
	txMu   sync.RWMutex

	journal *bridgeTxJournal // Journal of transaction to back up to disk

	queue map[common.Address]*bridgeTxSortedMap // Queued but non-processable transactions
	// TODO-Klaytn-Servicechain refine heartbeat for the tx not for account.
	all map[common.Hash]*types.Transaction // All transactions to allow lookups

	wg     sync.WaitGroup // for shutdown sync
	closed chan struct{}
}

// NewBridgeTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewBridgeTxPool(config BridgeTxPoolConfig) *BridgeTxPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// Create the transaction pool with its initial settings
	pool := &BridgeTxPool{
		config: config,
		queue:  make(map[common.Address]*bridgeTxSortedMap),
		all:    make(map[common.Hash]*types.Transaction),
		closed: make(chan struct{}),
	}

	// load from disk
	pool.journal = newBridgeTxJournal(config.Journal)

	if err := pool.journal.load(pool.AddLocals); err != nil {
		logger.Error("Failed to load chain transaction journal", "err", err)
	}
	if err := pool.journal.rotate(pool.Pending()); err != nil {
		logger.Error("Failed to rotate chain transaction journal", "err", err)
	}

	pool.SetEIP155Signer(config.ParentChainID)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// SetEIP155Signer set signer of txpool.
func (pool *BridgeTxPool) SetEIP155Signer(chainID *big.Int) {
	pool.signer = types.NewEIP155Signer(chainID)
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events.
func (pool *BridgeTxPool) loop() {
	defer pool.wg.Done()

	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle local transaction journal rotation
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.Pending()); err != nil {
					logger.Error("Failed to rotate local tx journal", "err", err)
				}
				pool.mu.Unlock()
			}
		case <-pool.closed:
			// update journal file with txs in pool.
			// if txpool close without the rotate process,
			// when loading the txpool with the journal file again,
			// there is a limit to the size of pool so that not all tx will be
			// loaded and especially the latest tx will not be loaded
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.Pending()); err != nil {
					logger.Error("Failed to rotate local tx journal", "err", err)
				}
				pool.mu.Unlock()
			}
			logger.Info("BridgeTxPool loop is closing")
			return
		}
	}
}

// Stop terminates the transaction pool.
func (pool *BridgeTxPool) Stop() {
	close(pool.closed)
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	logger.Info("Transaction pool stopped")
}

// stats retrieves the current pool stats, namely the number of pending transactions.
func (pool *BridgeTxPool) stats() int {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return queued
}

// Content retrieves the data content of the transaction pool, returning all the
// queued transactions, grouped by account and sorted by nonce.
func (pool *BridgeTxPool) Content() map[common.Address]types.Transactions {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	queued := make(map[common.Address]types.Transactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return queued
}

// GetTx get the tx by tx hash.
func (pool *BridgeTxPool) GetTx(txHash common.Hash) (*types.Transaction, error) {
	tx, ok := pool.all[txHash]

	if ok {
		return tx, nil
	} else {
		return nil, ErrUnknownTx
	}
}

// Pending retrieves all currently known local transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *BridgeTxPool) Pending() map[common.Address]types.Transactions {
	pool.txMu.Lock()
	defer pool.txMu.Unlock()

	pending := make(map[common.Address]types.Transactions)
	for addr, list := range pool.queue {
		pending[addr] = list.Flatten()
	}
	return pending
}

// PendingTxsByAddress retrieves pending transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *BridgeTxPool) PendingTxsByAddress(from *common.Address, limit int) types.Transactions {
	pool.txMu.Lock()
	defer pool.txMu.Unlock()

	var pendingTxs types.Transactions

	if list, exist := pool.queue[*from]; exist {
		pendingTxs = list.Flatten()

		if len(pendingTxs) > limit {
			return pendingTxs[0:limit]
		}
		return pendingTxs
	}
	return nil
}

// PendingTxHashsByAddress retrieves pending transaction hashes, grouped by origin
// account without sorting. The returned hash set is a copy and can be
// freely modified by calling code.
func (pool *BridgeTxPool) PendingTxHashsByAddress(from *common.Address, limit int) []common.Hash {
	pool.txMu.Lock()
	defer pool.txMu.Unlock()

	if list, exist := pool.queue[*from]; exist {
		if len(list.items) > 0 {
			if limit < 0 || len(list.items) <= limit {
				pendingTxHashes := make([]common.Hash, len(list.items))
				var idx = 0
				for _, tx := range list.items {
					pendingTxHashes[idx] = tx.Hash()
					idx++
				}
				return pendingTxHashes
			} else {
				pendingTxHashes := make([]common.Hash, limit)
				var idx = 0
				for _, tx := range list.items {
					pendingTxHashes[idx] = tx.Hash()
					idx++
					if idx >= limit {
						break
					}
				}
				return pendingTxHashes
			}
		}
	}
	return nil
}

// GetMaxTxNonce finds max nonce of the address.
func (pool *BridgeTxPool) GetMaxTxNonce(from *common.Address) uint64 {
	pool.txMu.Lock()
	defer pool.txMu.Unlock()

	maxNonce := uint64(0)
	if list, exist := pool.queue[*from]; exist {
		for _, t := range list.items {
			if maxNonce < t.Nonce() {
				maxNonce = t.Nonce()
			}
		}
	}
	return maxNonce
}

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
func (pool *BridgeTxPool) add(tx *types.Transaction) error {
	// If the transaction is already known, discard it
	hash := tx.Hash()
	if pool.all[hash] != nil {
		logger.Trace("Discarding already known transaction", "hash", hash)
		return ErrKnownTx
	}

	from, err := types.Sender(pool.signer, tx)
	if err != nil {
		return err
	}

	if uint64(len(pool.all)) >= pool.config.GlobalQueue {
		logger.Trace("Rejecting a new Tx, because BridgeTxPool is full and there is no room for the account", "hash", tx.Hash(), "account", from)
		refusedTxCounter.Inc(1)
		return fmt.Errorf("txpool is full: %d", uint64(len(pool.all)))
	}

	if pool.queue[from] == nil {
		pool.queue[from] = newBridgeTxSortedMap()
	} else {
		if pool.queue[from].Get(tx.Nonce()) != nil {
			return ErrDuplicatedNonceTx
		}
	}

	pool.queue[from].Put(tx)

	if pool.all[hash] == nil {
		pool.all[hash] = tx
	}

	// Mark journal transactions
	pool.journalTx(from, tx)

	logger.Trace("Pooled new future transaction", "hash", hash, "from", from, "to", tx.To())
	return nil
}

// journalTx adds the specified transaction to the local disk journal if it is
// deemed to have been sent from a bridgenode account.
func (pool *BridgeTxPool) journalTx(from common.Address, tx *types.Transaction) {
	// Only journal if it's enabled
	if pool.journal == nil {
		return
	}
	if err := pool.journal.insert(tx); err != nil {
		logger.Error("Failed to journal local transaction", "err", err)
	}
}

// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one.
func (pool *BridgeTxPool) AddLocal(tx *types.Transaction) error {
	return pool.addTx(tx)
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones.
func (pool *BridgeTxPool) AddLocals(txs []*types.Transaction) []error {
	return pool.addTxs(txs)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *BridgeTxPool) addTx(tx *types.Transaction) error {
	//senderCacher.recover(pool.signer, []*types.Transaction{tx})

	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to inject the transaction and update any state
	err := pool.add(tx)

	return err
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *BridgeTxPool) addTxs(txs []*types.Transaction) []error {
	//senderCacher.recover(pool.signer, txs)

	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addTxsLocked(txs)
}

// addTxsLocked attempts to queue a batch of transactions if they are valid,
// whilst assuming the transaction pool lock is already held.
func (pool *BridgeTxPool) addTxsLocked(txs []*types.Transaction) []error {
	// Add the batch of transaction, tracking the accepted ones
	dirty := make(map[common.Address]struct{})
	errs := make([]error, len(txs))

	for i, tx := range txs {
		var replace bool
		if errs[i] = pool.add(tx); errs[i] == nil {
			if !replace {
				from, err := types.Sender(pool.signer, tx)
				errs[i] = err

				dirty[from] = struct{}{}
			}
		}
	}

	return errs
}

// Get returns a transaction if it is contained in the pool
// and nil otherwise.
func (pool *BridgeTxPool) Get(hash common.Hash) *types.Transaction {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.all[hash]
}

// removeTx removes a single transaction from the queue.
func (pool *BridgeTxPool) removeTx(hash common.Hash) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Fetch the transaction we wish to delete
	tx, ok := pool.all[hash]
	if !ok {
		return ErrUnknownTx
	}

	addr, err := types.Sender(pool.signer, tx)
	if err != nil {
		return err
	}

	// Remove it from the list of known transactions
	delete(pool.all, hash)

	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx.Nonce())
		if future.Len() == 0 {
			delete(pool.queue, addr)
		}
	}

	return nil
}

// Remove removes transactions from the queue.
func (pool *BridgeTxPool) Remove(txs types.Transactions) []error {
	errs := make([]error, len(txs))
	for i, tx := range txs {
		errs[i] = pool.removeTx(tx.Hash())
	}
	return errs
}

// RemoveTx removes a single transaction from the queue.
func (pool *BridgeTxPool) RemoveTx(tx *types.Transaction) error {
	err := pool.removeTx(tx.Hash())
	if err != nil {
		logger.Error("RemoveTx", "err", err)
		return err
	}
	return nil
}
