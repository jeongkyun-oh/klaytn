// Copyright 2019 The go-ethereum Authors
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

package snapshot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/big"
	"time"

	"github.com/klaytn/klaytn/storage/statedb"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/math"
	"github.com/klaytn/klaytn/crypto"
	"github.com/klaytn/klaytn/rlp"
	"github.com/klaytn/klaytn/storage/database"
)

var (
	// emptyRoot is the known root hash of an empty trie.
	emptyRoot = common.HexToHash("56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")

	// emptyCode is the known hash of the empty EVM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)
)

// generatorStats is a collection of statistics gathered by the snapshot generator
// for logging purposes.
type generatorStats struct {
	wiping   chan struct{}      // Notification channel if wiping is in progress
	origin   uint64             // Origin prefix where generation started
	start    time.Time          // Timestamp when generation started
	accounts uint64             // Number of accounts indexed
	slots    uint64             // Number of storage slots indexed
	storage  common.StorageSize // Account and storage slot size
}

// Log creates an contextual log with the given message and the context pulled
// from the internally maintained statistics.
func (gs *generatorStats) Log(msg string, root common.Hash, marker []byte) {
	var ctx []interface{}
	if root != (common.Hash{}) {
		ctx = append(ctx, []interface{}{"root", root}...)
	}
	// Figure out whether we're after or within an account
	switch len(marker) {
	case common.HashLength:
		ctx = append(ctx, []interface{}{"at", common.BytesToHash(marker)}...)
	case 2 * common.HashLength:
		ctx = append(ctx, []interface{}{
			"in", common.BytesToHash(marker[:common.HashLength]),
			"at", common.BytesToHash(marker[common.HashLength:]),
		}...)
	}
	// Add the usual measurements
	ctx = append(ctx, []interface{}{
		"accounts", gs.accounts,
		"slots", gs.slots,
		"storage", gs.storage,
		"elapsed", common.PrettyDuration(time.Since(gs.start)),
	}...)
	// Calculate the estimated indexing time based on current stats
	if len(marker) > 0 {
		if done := binary.BigEndian.Uint64(marker[:8]) - gs.origin; done > 0 {
			left := math.MaxUint64 - binary.BigEndian.Uint64(marker[:8])

			speed := done/uint64(time.Since(gs.start)/time.Millisecond+1) + 1 // +1s to avoid division by zero
			ctx = append(ctx, []interface{}{
				"eta", common.PrettyDuration(time.Duration(left/speed) * time.Millisecond),
			}...)
		}
	}
	logger.Info(msg, ctx...)
}

// generateSnapshot regenerates a brand new snapshot based on an existing state
// database and head block asynchronously. The snapshot is returned immediately
// and generation is continued in the background until done.
func generateSnapshot(diskdb database.DBManager, triedb *statedb.Database, cache int, root common.Hash, wiper chan struct{}) *diskLayer {
	// Wipe any previously existing snapshot from the database if no wiper is
	// currently in progress.
	if wiper == nil {
		wiper = wipeSnapshot(diskdb, true)
	}
	// Create a new disk layer with an initialized state marker at zero
	diskdb.WriteSnapshotRoot(root)

	base := &diskLayer{
		diskdb:     diskdb,
		triedb:     triedb,
		root:       root,
		cache:      fastcache.New(cache * 1024 * 1024),
		genMarker:  []byte{}, // Initialized but empty!
		genPending: make(chan struct{}),
		genAbort:   make(chan chan *generatorStats),
	}
	go base.generate(&generatorStats{wiping: wiper, start: time.Now()})
	logger.Debug("Start snapshot generation", "root", root)
	return base
}

// journalProgress persists the generator stats into the database to resume later.
func journalProgress(db database.KeyValueWriter, marker []byte, stats *generatorStats) {
	// Write out the generator marker. Note it's a standalone disk layer generator
	// which is not mixed with journal. It's ok if the generator is persisted while
	// journal is not.
	entry := journalGenerator{
		Done:   marker == nil,
		Marker: marker,
	}
	if stats != nil {
		entry.Wiping = (stats.wiping != nil)
		entry.Accounts = stats.accounts
		entry.Slots = stats.slots
		entry.Storage = uint64(stats.storage)
	}
	blob, err := rlp.EncodeToBytes(entry)
	if err != nil {
		panic(err) // Cannot happen, here to catch dev errors
	}
	var logstr string
	switch len(marker) {
	case 0:
		logstr = "done"
	case common.HashLength:
		logstr = fmt.Sprintf("%#x", marker)
	default:
		logstr = fmt.Sprintf("%#x:%#x", marker[:common.HashLength], marker[common.HashLength:])
	}
	logger.Debug("Journalled generator progress", "progress", logstr)
	if err := db.Put(database.SnapshotGeneratorKey, blob); err != nil {
		logger.Crit("Failed to store snapshot generator", "err", err)
	}
}

// generate is a background thread that iterates over the state and storage tries,
// constructing the state snapshot. All the arguments are purely for statistics
// gathering and logging, since the method surfs the blocks as they arrive, often
// being restarted.
func (dl *diskLayer) generate(stats *generatorStats) {
	// If a database wipe is in operation, wait until it's done
	if stats.wiping != nil {
		stats.Log("Wiper running, state snapshotting paused", common.Hash{}, dl.genMarker)
		select {
		// If wiper is done, resume normal mode of operation
		case <-stats.wiping:
			stats.wiping = nil
			stats.start = time.Now()

		// If generator was aborted during wipe, return
		case abort := <-dl.genAbort:
			abort <- stats
			return
		}
	}
	// Create an account and state iterator pointing to the current generator marker
	accTrie, err := statedb.NewSecureTrie(dl.root, dl.triedb)
	if err != nil {
		// The account trie is missing (GC), surf the chain until one becomes available
		stats.Log("Trie missing, state snapshotting paused", dl.root, dl.genMarker)

		abort := <-dl.genAbort
		abort <- stats
		return
	}
	stats.Log("Resuming state snapshot generation", dl.root, dl.genMarker)

	var accMarker []byte
	if len(dl.genMarker) > 0 { // []byte{} is the start, use nil for that
		accMarker = dl.genMarker[:common.HashLength]
	}
	accIt := statedb.NewIterator(accTrie.NodeIterator(accMarker))
	batch := dl.diskdb.NewSnapshotDBBatch()

	// Iterate from the previous marker and continue generating the state snapshot
	logged := time.Now()
	for accIt.Next() {
		// Retrieve the current account and flatten it into the internal format
		accountHash := common.BytesToHash(accIt.Key)

		var acc struct {
			Nonce    uint64
			Balance  *big.Int
			Root     common.Hash
			CodeHash []byte
		}
		if err := rlp.DecodeBytes(accIt.Value, &acc); err != nil {
			logger.Crit("Invalid account encountered during snapshot creation", "err", err)
		}
		data := SlimAccountRLP(acc.Nonce, acc.Balance, acc.Root, acc.CodeHash)

		// If the account is not yet in-progress, write it out
		if accMarker == nil || !bytes.Equal(accountHash[:], accMarker) {
			batch.WriteAccountSnapshot(accountHash, data)
			stats.storage += common.StorageSize(1 + common.HashLength + len(data))
			stats.accounts++
		}
		// If we've exceeded our batch allowance or termination was requested, flush to disk
		var abort chan *generatorStats
		select {
		case abort = <-dl.genAbort:
		default:
		}
		if batch.ValueSize() > database.IdealBatchSize || abort != nil {
			// Only write and set the marker if we actually did something useful
			if batch.ValueSize() > 0 {
				// Ensure the generator entry is in sync with the data
				marker := accountHash[:]
				journalProgress(batch, marker, stats)

				batch.Write()
				batch.Reset()

				dl.lock.Lock()
				dl.genMarker = marker
				dl.lock.Unlock()
			}
			if abort != nil {
				stats.Log("Aborting state snapshot generation", dl.root, accountHash[:])
				abort <- stats
				return
			}
		}
		// If the account is in-progress, continue where we left off (otherwise iterate all)
		if acc.Root != emptyRoot {
			storeTrie, err := statedb.NewSecureTrie(acc.Root, dl.triedb)
			if err != nil {
				logger.Error("Generator failed to access storage trie", "root", dl.root, "account", accountHash, "stroot", acc.Root, "err", err)
				abort := <-dl.genAbort
				abort <- stats
				return
			}
			var storeMarker []byte
			if accMarker != nil && bytes.Equal(accountHash[:], accMarker) && len(dl.genMarker) > common.HashLength {
				storeMarker = dl.genMarker[common.HashLength:]
			}
			storeIt := statedb.NewIterator(storeTrie.NodeIterator(storeMarker))
			for storeIt.Next() {
				batch.WriteStorageSnapshot(accountHash, common.BytesToHash(storeIt.Key), storeIt.Value)
				stats.storage += common.StorageSize(1 + 2*common.HashLength + len(storeIt.Value))
				stats.slots++

				// If we've exceeded our batch allowance or termination was requested, flush to disk
				var abort chan *generatorStats
				select {
				case abort = <-dl.genAbort:
				default:
				}
				if batch.ValueSize() > database.IdealBatchSize || abort != nil {
					// Only write and set the marker if we actually did something useful
					if batch.ValueSize() > 0 {
						// Ensure the generator entry is in sync with the data
						marker := append(accountHash[:], storeIt.Key...)
						journalProgress(batch, marker, stats)

						batch.Write()
						batch.Reset()

						dl.lock.Lock()
						dl.genMarker = marker
						dl.lock.Unlock()
					}
					if abort != nil {
						stats.Log("Aborting state snapshot generation", dl.root, append(accountHash[:], storeIt.Key...))
						abort <- stats
						return
					}
				}
			}
			if err := storeIt.Err; err != nil {
				logger.Error("Generator failed to iterate storage trie", "accroot", dl.root, "acchash", common.BytesToHash(accIt.Key), "stroot", acc.Root, "err", err)
				abort := <-dl.genAbort
				abort <- stats
				return
			}
		}
		if time.Since(logged) > 8*time.Second {
			stats.Log("Generating state snapshot", dl.root, accIt.Key)
			logged = time.Now()
		}
		// Some account processed, unmark the marker
		accMarker = nil
	}
	if err := accIt.Err; err != nil {
		logger.Error("Generator failed to iterate account trie", "root", dl.root, "err", err)
		abort := <-dl.genAbort
		abort <- stats
		return
	}
	// Snapshot fully generated, set the marker to nil
	if batch.ValueSize() > 0 {
		// Ensure the generator entry is in sync with the data
		journalProgress(batch, nil, stats)

		batch.Write()
	}
	logger.Info("Generated state snapshot", "accounts", stats.accounts, "slots", stats.slots,
		"storage", stats.storage, "elapsed", common.PrettyDuration(time.Since(stats.start)))

	dl.lock.Lock()
	dl.genMarker = nil
	close(dl.genPending)
	dl.lock.Unlock()

	// Someone will be looking for us, wait it out
	abort := <-dl.genAbort
	abort <- nil
}
