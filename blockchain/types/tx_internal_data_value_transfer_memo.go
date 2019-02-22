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
	"bytes"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataValueTransferMemo represents a transaction with payload data transferring KLAY.
type TxInternalDataValueTransferMemo struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	Recipient    common.Address
	Amount       *big.Int
	From         common.Address
	Payload      []byte

	TxSignatures

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func newTxInternalDataValueTransferMemo() *TxInternalDataValueTransferMemo {
	h := common.Hash{}

	return &TxInternalDataValueTransferMemo{
		Price:  new(big.Int),
		Amount: new(big.Int),
		Hash:   &h,
	}
}

func newTxInternalDataValueTransferMemoWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataValueTransferMemo, error) {
	d := newTxInternalDataValueTransferMemo()

	if v, ok := values[TxValueKeyNonce].(uint64); ok {
		d.AccountNonce = v
	} else {
		return nil, errValueKeyNonceMustUint64
	}

	if v, ok := values[TxValueKeyGasPrice].(*big.Int); ok {
		d.Price.Set(v)
	} else {
		return nil, errValueKeyGasPriceMustBigInt
	}

	if v, ok := values[TxValueKeyGasLimit].(uint64); ok {
		d.GasLimit = v
	} else {
		return nil, errValueKeyGasLimitMustUint64
	}

	if v, ok := values[TxValueKeyTo].(common.Address); ok {
		d.Recipient = v
	} else {
		return nil, errValueKeyToMustAddress
	}

	if v, ok := values[TxValueKeyAmount].(*big.Int); ok {
		d.Amount.Set(v)
	} else {
		return nil, errValueKeyAmountMustBigInt
	}

	if v, ok := values[TxValueKeyFrom].(common.Address); ok {
		d.From = v
	} else {
		return nil, errValueKeyFromMustAddress
	}

	if v, ok := values[TxValueKeyData].([]byte); ok {
		d.Payload = v
	} else {
		return nil, errValueKeyDataMustByteSlice
	}

	return d, nil
}

func (t *TxInternalDataValueTransferMemo) Type() TxType {
	return TxTypeValueTransferMemo
}

func (t *TxInternalDataValueTransferMemo) GetRoleTypeForValidation() accountkey.RoleType {
	return accountkey.RoleTransaction
}

func (t *TxInternalDataValueTransferMemo) Equal(b TxInternalData) bool {
	tb, ok := b.(*TxInternalDataValueTransferMemo)
	if !ok {
		return false
	}

	return t.AccountNonce == tb.AccountNonce &&
		t.Price.Cmp(tb.Price) == 0 &&
		t.GasLimit == tb.GasLimit &&
		t.Recipient == tb.Recipient &&
		t.Amount.Cmp(tb.Amount) == 0 &&
		t.From == tb.From &&
		bytes.Equal(t.Payload, tb.Payload) &&
		t.TxSignatures.equal(tb.TxSignatures)
}

func (t *TxInternalDataValueTransferMemo) String() string {
	ser := newTxInternalDataSerializerWithValues(t)
	tx := Transaction{data: t}
	enc, _ := rlp.EncodeToBytes(ser)
	return fmt.Sprintf(`
	TX(%x)
	Type:          %s
	From:          %s
	To:            %s
	Nonce:         %v
	GasPrice:      %#x
	GasLimit:      %#x
	Value:         %#x
	Signature:     %s
	Paylod:        %x
	Hex:           %x
`,
		tx.Hash(),
		t.Type().String(),
		t.From.String(),
		t.Recipient.String(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Amount,
		t.TxSignatures.string(),
		common.Bytes2Hex(t.Payload),
		enc)
}

func (t *TxInternalDataValueTransferMemo) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataValueTransferMemo) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataValueTransferMemo) GetPrice() *big.Int {
	return new(big.Int).Set(t.Price)
}

func (t *TxInternalDataValueTransferMemo) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataValueTransferMemo) GetRecipient() *common.Address {
	if t.Recipient == (common.Address{}) {
		return nil
	}

	to := common.Address(t.Recipient)
	return &to
}

func (t *TxInternalDataValueTransferMemo) GetAmount() *big.Int {
	return new(big.Int).Set(t.Amount)
}

func (t *TxInternalDataValueTransferMemo) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataValueTransferMemo) GetPayload() []byte {
	return t.Payload
}

func (t *TxInternalDataValueTransferMemo) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataValueTransferMemo) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataValueTransferMemo) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataValueTransferMemo) IntrinsicGas() (uint64, error) {
	gasPayload, err := intrinsicGasPayload(t.Payload)
	if err != nil {
		return 0, err
	}

	return params.TxGas + gasPayload, nil
}

func (t *TxInternalDataValueTransferMemo) SerializeForSign() []interface{} {
	return []interface{}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.Payload,
	}
}

func (t *TxInternalDataValueTransferMemo) Execute(sender ContractRef, vm VM, stateDB StateDB, gas uint64, value *big.Int) (ret []byte, usedGas uint64, err, vmerr error) {
	stateDB.IncNonce(sender.Address())
	ret, usedGas, vmerr = vm.Call(sender, t.Recipient, t.Payload, gas, value)

	return
}