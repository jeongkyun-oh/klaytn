// Copyright 2015 The go-ethereum Authors
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

package state

import (
	"sync"

	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/storage/database"
	"github.com/ground-x/go-gxplatform/storage/statedb"

	"fmt"
	lru "github.com/hashicorp/golang-lru"
)

// Trie cache generation limit after which to evict trie nodes from memory.
var MaxTrieCacheGen = uint16(120)

const (
	// Number of past tries to keep. This value is chosen such that
	// reasonable chain reorg depths will hit an existing trie.
	maxPastTries = 12

	// Number of codehash->size associations to keep
	codeSizeCacheSize = 100000
)

// Database wraps access to tries and contract code.
type Database interface {
	// OpenTrie opens the main account trie.
	OpenTrie(root common.Hash) (Trie, error)

	// OpenStorageTrie opens the storage trie of an account.
	OpenStorageTrie(root common.Hash) (Trie, error)

	// CopyTrie returns an independent copy of the given trie.
	CopyTrie(Trie) Trie

	// ContractCode retrieves a particular contract's code.
	ContractCode(codeHash common.Hash) ([]byte, error)

	// ContractCodeSize retrieves a particular contracts code's size.
	ContractCodeSize(codeHash common.Hash) (int, error)

	// TrieDB retrieves the low level trie database used for data storage.
	TrieDB() *statedb.Database
}

// Trie is a GXPlatform Merkle Trie.
type Trie interface {
	TryGet(key []byte) ([]byte, error)
	TryUpdate(key, value []byte) error
	TryDelete(key []byte) error
	Commit(onleaf statedb.LeafCallback) (common.Hash, error)
	Hash() common.Hash
	NodeIterator(startKey []byte) statedb.NodeIterator
	GetKey([]byte) []byte // TODO(fjl): remove this when SecureTrie is removed
	Prove(key []byte, fromLevel uint, proofDb database.Putter) error
}

func NewDatabase(db database.Database) Database {
	csc, _ := lru.New(codeSizeCacheSize)
	return &cachingDB{
		db:            statedb.NewDatabase(db),
		codeSizeCache: csc,
	}
}

type cachingDB struct {
	db            *statedb.Database
	mu            sync.Mutex
	pastTries     []*statedb.SecureTrie
	codeSizeCache *lru.Cache
}

// OpenTrie opens the main account trie.
func (db *cachingDB) OpenTrie(root common.Hash) (Trie, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	for i := len(db.pastTries) - 1; i >= 0; i-- {
		if db.pastTries[i].Hash() == root {
			return cachedTrie{db.pastTries[i].Copy(), db}, nil
		}
	}
	tr, err := statedb.NewSecureTrie(root, db.db, MaxTrieCacheGen)
	if err != nil {
		return nil, err
	}
	return cachedTrie{tr, db}, nil
}

func (db *cachingDB) pushTrie(t *statedb.SecureTrie) {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(db.pastTries) >= maxPastTries {
		copy(db.pastTries, db.pastTries[1:])
		db.pastTries[len(db.pastTries)-1] = t
	} else {
		db.pastTries = append(db.pastTries, t)
	}
}

// OpenStorageTrie opens the storage trie of an account.
func (db *cachingDB) OpenStorageTrie(root common.Hash) (Trie, error) {
	return statedb.NewSecureTrie(root, db.db, 0)
}

// CopyTrie returns an independent copy of the given trie.
func (db *cachingDB) CopyTrie(t Trie) Trie {
	switch t := t.(type) {
	case cachedTrie:
		return cachedTrie{t.SecureTrie.Copy(), db}
	case *statedb.SecureTrie:
		return t.Copy()
	default:
		panic(fmt.Errorf("unknown trie type %T", t))
	}
}

// ContractCode retrieves a particular contract's code.
func (db *cachingDB) ContractCode(codeHash common.Hash) ([]byte, error) {
	code, err := db.db.Node(codeHash)
	if err == nil {
		db.codeSizeCache.Add(codeHash, len(code))
	}
	return code, err
}

// ContractCodeSize retrieves a particular contracts code's size.
func (db *cachingDB) ContractCodeSize(codeHash common.Hash) (int, error) {
	if cached, ok := db.codeSizeCache.Get(codeHash); ok {
		return cached.(int), nil
	}
	code, err := db.ContractCode(codeHash)
	return len(code), err
}

// TrieDB retrieves the low level trie database used for data storage.
func (db *cachingDB) TrieDB() *statedb.Database {
	return db.db
}

// cachedTrie inserts its trie into a cachingDB on commit.
type cachedTrie struct {
	*statedb.SecureTrie
	db *cachingDB
}

func (m cachedTrie) Commit(onleaf statedb.LeafCallback) (common.Hash, error) {
	root, err := m.SecureTrie.Commit(onleaf)
	if err == nil {
		m.db.pushTrie(m.SecureTrie)
	}
	return root, err
}

func (m cachedTrie) Prove(key []byte, fromLevel uint, proofDb database.Putter) error {
	return m.SecureTrie.Prove(key, fromLevel, proofDb)
}
