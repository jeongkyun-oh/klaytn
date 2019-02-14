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
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataValueTransfer represents a transaction transferring KLAY.
// No more attributes required than attributes in TxInternalDataCommon.
type TxInternalDataValueTransfer struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	Recipient    common.Address
	Amount       *big.Int
	From         common.Address

	TxSignatures

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func newTxInternalDataValueTransfer() *TxInternalDataValueTransfer {
	h := common.Hash{}

	return &TxInternalDataValueTransfer{
		Price:  new(big.Int),
		Amount: new(big.Int),
		Hash:   &h,
	}
}

func newTxInternalDataValueTransferWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataValueTransfer, error) {
	d := newTxInternalDataValueTransfer()

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

	return d, nil
}

func (t *TxInternalDataValueTransfer) Type() TxType {
	return TxTypeValueTransfer
}

func (t *TxInternalDataValueTransfer) Equal(b TxInternalData) bool {
	tb, ok := b.(*TxInternalDataValueTransfer)
	if !ok {
		return false
	}

	return t.AccountNonce == tb.AccountNonce &&
		t.Price.Cmp(tb.Price) == 0 &&
		t.GasLimit == tb.GasLimit &&
		t.Recipient == tb.Recipient &&
		t.Amount.Cmp(tb.Amount) == 0 &&
		t.From == tb.From &&
		t.TxSignatures.equal(tb.TxSignatures)
}

func (t *TxInternalDataValueTransfer) String() string {
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
		enc)
}

func (t *TxInternalDataValueTransfer) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataValueTransfer) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataValueTransfer) GetPrice() *big.Int {
	return new(big.Int).Set(t.Price)
}

func (t *TxInternalDataValueTransfer) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataValueTransfer) GetRecipient() *common.Address {
	if t.Recipient == (common.Address{}) {
		return nil
	}

	to := common.Address(t.Recipient)
	return &to
}

func (t *TxInternalDataValueTransfer) GetAmount() *big.Int {
	return new(big.Int).Set(t.Amount)
}

func (t *TxInternalDataValueTransfer) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataValueTransfer) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataValueTransfer) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataValueTransfer) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataValueTransfer) IntrinsicGas() (uint64, error) {
	// TxInternalDataValueTransfer does not have payload, and it
	// is not account creation. Hence, its intrinsic gas is determined by
	// params.TxGas. Refer to types.IntrinsicGas().
	return params.TxGas, nil
}

func (t *TxInternalDataValueTransfer) SerializeForSign() []interface{} {
	return []interface{}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
	}
}
