// Copyright 2018 The go-klaytn Authors
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
// This file is derived from core/state/state_test.go (2018/06/04).
// Modified and improved for the go-klaytn development.

package state

import (
	"bytes"
	"math/big"
	"testing"

	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/storage/database"
	checker "gopkg.in/check.v1"
)

type StateSuite struct {
	db    database.DBManager
	state *StateDB
}

var _ = checker.Suite(&StateSuite{})

var toAddr = common.BytesToAddress

func (s *StateSuite) TestDump(c *checker.C) {
	// generate a few entries
	obj1 := s.state.GetOrNewStateObject(toAddr([]byte{0x01}))
	obj1.AddBalance(big.NewInt(22))
	obj2 := s.state.GetOrNewStateObject(toAddr([]byte{0x01, 0x02}))
	obj2.SetCode(crypto.Keccak256Hash([]byte{3, 3, 3, 3, 3, 3, 3}), []byte{3, 3, 3, 3, 3, 3, 3})
	obj3 := s.state.GetOrNewStateObject(toAddr([]byte{0x02}))
	obj3.SetBalance(big.NewInt(44))

	// write some of them to the trie
	s.state.updateStateObject(obj1)
	s.state.updateStateObject(obj2)
	s.state.Commit(false)

	// check that dump contains the state objects that are in trie
	got := string(s.state.Dump())
	want := `{
    "root": "71edff0130dd2385947095001c73d9e28d862fc286fca2b922ca6f6f3cddfdd2",
    "accounts": {
        "0000000000000000000000000000000000000001": {
            "balance": "22",
            "nonce": 0,
            "root": "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "codeHash": "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
            "code": "",
            "storage": {}
        },
        "0000000000000000000000000000000000000002": {
            "balance": "44",
            "nonce": 0,
            "root": "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "codeHash": "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470",
            "code": "",
            "storage": {}
        },
        "0000000000000000000000000000000000000102": {
            "balance": "0",
            "nonce": 0,
            "root": "56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421",
            "codeHash": "87874902497a5bb968da31a2998d8f22e949d1ef6214bcdedd8bae24cca4b9e3",
            "code": "03030303030303",
            "storage": {}
        }
    }
}`
	if got != want {
		c.Errorf("dump mismatch:\ngot: %s\nwant: %s\n", got, want)
	}
}

func (s *StateSuite) SetUpTest(c *checker.C) {
	s.db = database.NewMemoryDBManager()
	s.state, _ = New(common.Hash{}, NewDatabase(s.db))
}

func (s *StateSuite) TestNull(c *checker.C) {
	address := common.HexToAddress("0x823140710bf13990e4500136726d8b55")
	s.state.CreateAccount(address)
	//value := common.FromHex("0x823140710bf13990e4500136726d8b55")
	var value common.Hash
	s.state.SetState(address, common.Hash{}, value)
	s.state.Commit(false)
	value = s.state.GetState(address, common.Hash{})
	if value != (common.Hash{}) {
		c.Errorf("expected empty hash. got %x", value)
	}
}

// This test is to check if stateObject is restored back to the original
// one after reverting to the state snapshot.
func (s *StateSuite) TestSnapshot(c *checker.C) {
	stateobjaddr := toAddr([]byte("aa"))
	var storageaddr common.Hash
	data1 := common.BytesToHash([]byte{42})
	data2 := common.BytesToHash([]byte{43})

	// set initial state object value
	s.state.SetState(stateobjaddr, storageaddr, data1)
	// get snapshot of current state
	snapshot := s.state.Snapshot()

	// set new state object value
	s.state.SetState(stateobjaddr, storageaddr, data2)
	// restore snapshot
	s.state.RevertToSnapshot(snapshot)

	// get state storage value
	res := s.state.GetState(stateobjaddr, storageaddr)

	// should be equal before/after restoring.
	c.Assert(data1, checker.DeepEquals, res)
}

// This test is to check reverting to the empty snapshot.
func (s *StateSuite) TestSnapshotEmpty(c *checker.C) {
	s.state.RevertToSnapshot(s.state.Snapshot())
}

// use testing instead of checker because checker does not support
// printing/logging in tests (-check.vv does not work)
// This test is to compare deleted/non-deleted stateObject after restoring.
func TestSnapshotForDeletedObject(t *testing.T) {
	memDB := database.NewMemoryDBManager()
	state, _ := New(common.Hash{}, NewDatabase(memDB))

	stateObjAddr0 := toAddr([]byte("so0"))
	stateObjAddr1 := toAddr([]byte("so1"))
	var storageAddr common.Hash

	data0 := common.BytesToHash([]byte{17})
	data1 := common.BytesToHash([]byte{18})

	state.SetState(stateObjAddr0, storageAddr, data0)
	state.SetState(stateObjAddr1, storageAddr, data1)

	// db, trie are already non-empty values
	so0 := state.getStateObject(stateObjAddr0)
	so0.SetBalance(big.NewInt(42))
	so0.SetNonce(43)
	so0.SetCode(crypto.Keccak256Hash([]byte{'c', 'a', 'f', 'e'}), []byte{'c', 'a', 'f', 'e'})
	so0.suicided = false
	so0.deleted = false
	state.setStateObject(so0)

	root, _ := state.Commit(false)
	state.Reset(root)

	// and one with deleted == true
	so1 := state.getStateObject(stateObjAddr1)
	so1.SetBalance(big.NewInt(52))
	so1.SetNonce(53)
	so1.SetCode(crypto.Keccak256Hash([]byte{'c', 'a', 'f', 'e', '2'}), []byte{'c', 'a', 'f', 'e', '2'})
	so1.suicided = true
	so1.deleted = true
	state.setStateObject(so1)

	// try to get deleted object before reverting to state snapshot = should be nil
	so1 = state.getStateObject(stateObjAddr1)
	if so1 != nil {
		t.Fatalf("deleted object not nil when getting")
	}

	snapshot := state.Snapshot()
	state.RevertToSnapshot(snapshot)

	so0Restored := state.getStateObject(stateObjAddr0)
	so0Restored.GetState(state.db, storageAddr) // Update lazily-loaded values before comparing.
	so0Restored.Code(state.db)                  // Update lazily-loaded values before comparing.

	// non-deleted stateObject should be equal before/after restoring.
	compareStateObjects(so0Restored, so0, t)

	// deleted should be nil, both before and after restore of state copy
	// try to get deleted object after reverting to state snapshot = should be nil
	so1Restored := state.getStateObject(stateObjAddr1)
	if so1Restored != nil {
		t.Fatalf("deleted object not nil after restoring snapshot: %+v", so1Restored)
	}
}

func compareStateObjects(so0, so1 *stateObject, t *testing.T) {
	if so0.Address() != so1.Address() {
		t.Fatalf("Address mismatch: have %v, want %v", so0.address, so1.address)
	}
	if so0.Balance().Cmp(so1.Balance()) != 0 {
		t.Fatalf("Balance mismatch: have %v, want %v", so0.Balance(), so1.Balance())
	}
	if so0.Nonce() != so1.Nonce() {
		t.Fatalf("Nonce mismatch: have %v, want %v", so0.Nonce(), so1.Nonce())
	}
	so0ac, so0ok := so0.account.(ProgramAccount)
	so1ac, so1ok := so1.account.(ProgramAccount)
	if so0ok != so1ok {
		t.Errorf("so0ok(%v) != so1ok(%v). Both should be smart contracts or not!", so0ok, so1ok)
	}
	if so0ok && so0ac.GetStorageRoot() != so1ac.GetStorageRoot() {
		// check the equivalence of storage root only if both are smart contract accounts.
		t.Errorf("Root mismatch: have %x, want %x", so0ac.GetStorageRoot().Bytes(), so1ac.GetStorageRoot().Bytes())
	}
	if !bytes.Equal(so0.CodeHash(), so1.CodeHash()) {
		t.Fatalf("CodeHash mismatch: have %v, want %v", so0.CodeHash(), so1.CodeHash())
	}
	if !bytes.Equal(so0.code, so1.code) {
		t.Fatalf("Code mismatch: have %v, want %v", so0.code, so1.code)
	}

	if len(so1.cachedStorage) != len(so0.cachedStorage) {
		t.Errorf("Storage size mismatch: have %d, want %d", len(so1.cachedStorage), len(so0.cachedStorage))
	}
	for k, v := range so1.cachedStorage {
		if so0.cachedStorage[k] != v {
			t.Errorf("Storage key %x mismatch: have %v, want %v", k, so0.cachedStorage[k], v)
		}
	}
	for k, v := range so0.cachedStorage {
		if so1.cachedStorage[k] != v {
			t.Errorf("Storage key %x mismatch: have %v, want none.", k, v)
		}
	}

	if so0.suicided != so1.suicided {
		t.Fatalf("suicided mismatch: have %v, want %v", so0.suicided, so1.suicided)
	}
	if so0.deleted != so1.deleted {
		t.Fatalf("Deleted mismatch: have %v, want %v", so0.deleted, so1.deleted)
	}
}
