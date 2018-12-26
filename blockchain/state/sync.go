// Copyright 2018 The go-klaytn Authors
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
//
// This file is derived from core/state/sync.go (2018/06/04).
// Modified and improved for the go-klaytn development.

package state

import (
	"bytes"
	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/ser/rlp"
	"github.com/ground-x/go-gxplatform/storage/database"
	"github.com/ground-x/go-gxplatform/storage/statedb"
)

// NewStateSync create a new state trie download scheduler.
func NewStateSync(root common.Hash, database database.DBManager) *statedb.TrieSync {
	var syncer *statedb.TrieSync
	callback := func(leaf []byte, parent common.Hash) error {
		var obj Account
		if err := rlp.Decode(bytes.NewReader(leaf), &obj); err != nil {
			return err
		}
		syncer.AddSubTrie(obj.Root, 64, parent, nil)
		syncer.AddRawEntry(common.BytesToHash(obj.CodeHash), 64, parent)
		return nil
	}
	syncer = statedb.NewTrieSync(root, database, callback)
	return syncer
}
