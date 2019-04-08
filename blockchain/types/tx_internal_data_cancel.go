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

package types

import (
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/hexutil"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataCancel is a transaction that cancels a transaction previously submitted into txpool by replacement.
// Since Klaytn defines fixed gas price for all transactions, a transaction cannot be replaced with
// another transaction with higher gas price. To provide tx replacement, TxInternalDataCancel is introduced.
// To replace a previously added tx, send a TxInternalCancel transaction with the same nonce.
type TxInternalDataCancel struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	From         common.Address

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`

	TxSignatures
}

func newTxInternalDataCancel() *TxInternalDataCancel {
	return &TxInternalDataCancel{
		Price:        new(big.Int),
		TxSignatures: NewTxSignatures(),
	}
}

func newTxInternalDataCancelWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataCancel, error) {
	d := newTxInternalDataCancel()

	if v, ok := values[TxValueKeyNonce].(uint64); ok {
		d.AccountNonce = v
		delete(values, TxValueKeyNonce)
	} else {
		return nil, errValueKeyNonceMustUint64
	}

	if v, ok := values[TxValueKeyGasLimit].(uint64); ok {
		d.GasLimit = v
		delete(values, TxValueKeyGasLimit)
	} else {
		return nil, errValueKeyGasLimitMustUint64
	}

	if v, ok := values[TxValueKeyGasPrice].(*big.Int); ok {
		d.Price.Set(v)
		delete(values, TxValueKeyGasPrice)
	} else {
		return nil, errValueKeyGasPriceMustBigInt
	}

	if v, ok := values[TxValueKeyFrom].(common.Address); ok {
		d.From = v
		delete(values, TxValueKeyFrom)
	} else {
		return nil, errValueKeyFromMustAddress
	}

	if len(values) != 0 {
		for k := range values {
			fmt.Println("unnecessary key", k.String())
		}
		return nil, errUndefinedKeyRemains
	}

	return d, nil
}

func (t *TxInternalDataCancel) Type() TxType {
	return TxTypeCancel
}

func (t *TxInternalDataCancel) GetRoleTypeForValidation() accountkey.RoleType {
	return accountkey.RoleTransaction
}

func (t *TxInternalDataCancel) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataCancel) GetPrice() *big.Int {
	return t.Price
}

func (t *TxInternalDataCancel) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataCancel) GetRecipient() *common.Address {
	return &common.Address{}
}

func (t *TxInternalDataCancel) GetAmount() *big.Int {
	return common.Big0
}

func (t *TxInternalDataCancel) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataCancel) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataCancel) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataCancel) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataCancel) Equal(b TxInternalData) bool {
	ta, ok := b.(*TxInternalDataCancel)
	if !ok {
		return false
	}

	return t.AccountNonce == ta.AccountNonce &&
		t.Price.Cmp(ta.Price) == 0 &&
		t.GasLimit == ta.GasLimit &&
		t.From == ta.From &&
		t.TxSignatures.equal(ta.TxSignatures)
}

func (t *TxInternalDataCancel) String() string {
	ser := newTxInternalDataSerializerWithValues(t)
	tx := Transaction{data: t}
	enc, _ := rlp.EncodeToBytes(ser)
	return fmt.Sprintf(`
	TX(%x)
	Type:          %s
	From:          %s
	Nonce:         %v
	GasPrice:      %#x
	GasLimit:      %#x
	Signature:     %s
	Hex:           %x
`,
		tx.Hash(),
		t.Type().String(),
		t.From.String(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.TxSignatures.string(),
		enc)
}

func (t *TxInternalDataCancel) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataCancel) IntrinsicGas(currentBlockNumber uint64) (uint64, error) {
	return params.TxGasCancel, nil
}

func (t *TxInternalDataCancel) SerializeForSignToBytes() []byte {
	b, _ := rlp.EncodeToBytes(struct {
		Txtype       TxType
		AccountNonce uint64
		Price        *big.Int
		GasLimit     uint64
		From         common.Address
	}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.From,
	})

	return b
}

func (t *TxInternalDataCancel) SerializeForSign() []interface{} {
	return []interface{}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.From,
	}
}

func (t *TxInternalDataCancel) Validate(stateDB StateDB, currentBlockNumber uint64) error {
	// No more validation required for TxTypeCancel for now.
	return nil
}

func (t *TxInternalDataCancel) Execute(sender ContractRef, vm VM, stateDB StateDB, currentBlockNumber uint64, gas uint64, value *big.Int) (ret []byte, usedGas uint64, err error) {
	stateDB.IncNonce(sender.Address())
	return nil, gas, nil
}

func (t *TxInternalDataCancel) MakeRPCOutput() map[string]interface{} {
	return map[string]interface{}{
		"type":     t.Type().String(),
		"gas":      hexutil.Uint64(t.GasLimit),
		"gasPrice": (*hexutil.Big)(t.Price),
		"nonce":    hexutil.Uint64(t.AccountNonce),
	}
}
