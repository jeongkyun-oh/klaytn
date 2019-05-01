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

package account

import (
	"encoding/json"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/ser/rlp"
	"io"
	"math/big"
)

// AccountCommon represents the common data structure of a Klaytn account.
type AccountCommon struct {
	nonce         uint64
	balance       *big.Int
	humanReadable bool
	key           accountkey.AccountKey
}

// accountCommonSerializable is an internal data structure for RLP serialization.
// This object is required due to AccountKey.
// AccountKey is an interface and it requires to use AccountKeySerializer for serialization.
type accountCommonSerializable struct {
	Nonce         uint64
	Balance       *big.Int
	HumanReadable bool
	Key           *accountkey.AccountKeySerializer
}

// newAccountCommon creates an AccountCommon object with default values.
func newAccountCommon() *AccountCommon {
	return &AccountCommon{
		nonce:         0,
		balance:       new(big.Int),
		humanReadable: false,
		key:           accountkey.NewAccountKeyLegacy(),
	}
}

// newAccountCommonWithMap creates an AccountCommon object initialized with the given values.
func newAccountCommonWithMap(values map[AccountValueKeyType]interface{}) *AccountCommon {
	acc := newAccountCommon()

	if v, ok := values[AccountValueKeyNonce].(uint64); ok {
		acc.nonce = v
	}

	if v, ok := values[AccountValueKeyBalance].(*big.Int); ok {
		acc.balance.Set(v)
	}

	if v, ok := values[AccountValueKeyHumanReadable].(bool); ok {
		acc.humanReadable = v
	}

	if v, ok := values[AccountValueKeyAccountKey].(accountkey.AccountKey); ok {
		acc.key = v
	}

	return acc
}

func newAccountCommonSerializable() *accountCommonSerializable {
	return &accountCommonSerializable{
		Balance: new(big.Int),
		Key:     accountkey.NewAccountKeySerializer(),
	}
}

// toSerializable converts an AccountCommon object to an accountCommonSerializable object.
func (e *AccountCommon) toSerializable() *accountCommonSerializable {
	return &accountCommonSerializable{
		e.nonce,
		e.balance,
		e.humanReadable,
		accountkey.NewAccountKeySerializerWithAccountKey(e.key),
	}
}

// fromSerializable updates its values from the given accountCommonSerializable object.
func (e *AccountCommon) fromSerializable(o *accountCommonSerializable) {
	e.nonce = o.Nonce
	e.balance = o.Balance
	e.humanReadable = o.HumanReadable
	e.key = o.Key.GetKey()
}

func (e *AccountCommon) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, e.toSerializable())
}

func (e *AccountCommon) DecodeRLP(s *rlp.Stream) error {
	serialized := newAccountCommonSerializable()

	if err := s.Decode(serialized); err != nil {
		return err
	}
	e.fromSerializable(serialized)
	return nil
}

func (e *AccountCommon) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.toSerializable())
}

func (e *AccountCommon) UnmarshalJSON(b []byte) error {
	serialized := newAccountCommonSerializable()

	if err := json.Unmarshal(b, serialized); err != nil {
		return err
	}

	e.fromSerializable(serialized)

	return nil
}

func (e *AccountCommon) GetNonce() uint64 {
	return e.nonce
}

func (e *AccountCommon) GetBalance() *big.Int {
	return e.balance
}

func (e *AccountCommon) GetHumanReadable() bool {
	return e.humanReadable
}

func (e *AccountCommon) GetKey() accountkey.AccountKey {
	return e.key
}

func (e *AccountCommon) SetNonce(n uint64) {
	e.nonce = n
}

func (e *AccountCommon) SetBalance(b *big.Int) {
	e.balance = b
}

func (e *AccountCommon) SetHumanReadable(h bool) {
	e.humanReadable = h
}

func (e *AccountCommon) SetKey(k accountkey.AccountKey) {
	e.key = k
}

func (e *AccountCommon) Empty() bool {
	return e.nonce == 0 && e.balance.Sign() == 0 && e.key == accountkey.NewAccountKeyLegacy()
}

func (e *AccountCommon) DeepCopy() *AccountCommon {
	return &AccountCommon{
		nonce:         e.nonce,
		balance:       new(big.Int).Set(e.balance),
		humanReadable: e.humanReadable,
		key:           e.key.DeepCopy()}
}

func (e *AccountCommon) UpdateKey(key accountkey.AccountKey, currentBlockNumber uint64) error {
	if e.key.Type() == key.Type() {
		// If the type is same, call Update() to set fields in key into e.key.
		if err := e.key.Update(key, currentBlockNumber); err != nil {
			return err
		}
	} else {
		// If the type is different, check it can be initialized and assigned.
		if err := key.Init(currentBlockNumber); err != nil {
			return err
		}
		// If assignable, assign it.
		e.key = key
	}
	return nil
}

func (e *AccountCommon) Equal(ta *AccountCommon) bool {
	return e.nonce == ta.nonce &&
		e.balance.Cmp(ta.balance) == 0 &&
		e.humanReadable == ta.humanReadable &&
		e.key.Equal(ta.key)

}

func (e *AccountCommon) String() string {
	return fmt.Sprintf("{Nonce:%d, Balance:%s, HumanReadable:%t key:%s}\n", e.nonce, e.balance.String(), e.humanReadable,
		e.key.String())
}
