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
	"crypto/ecdsa"
	"fmt"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataFeeDelegatedValueTransfer represents a fee-delegated transaction
// transferring KLAY with a fee payer.
// FeePayer should be RLP-encoded after the signature of the sender.
type TxInternalDataFeeDelegatedValueTransfer struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	Recipient    common.Address
	Amount       *big.Int
	From         common.Address

	TxSignatures

	FeePayer          common.Address
	FeePayerSignature TxSignatures

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func newTxInternalDataFeeDelegatedValueTransfer() *TxInternalDataFeeDelegatedValueTransfer {
	h := common.Hash{}
	return &TxInternalDataFeeDelegatedValueTransfer{
		Price:  new(big.Int),
		Amount: new(big.Int),
		Hash:   &h,
	}
}

func newTxInternalDataFeeDelegatedValueTransferWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataFeeDelegatedValueTransfer, error) {
	d := newTxInternalDataFeeDelegatedValueTransfer()

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

	if v, ok := values[TxValueKeyFeePayer].(common.Address); ok {
		d.FeePayer = v
	} else {
		return nil, errValueKeyFeePayerMustAddress
	}

	return d, nil
}

func (t *TxInternalDataFeeDelegatedValueTransfer) Type() TxType {
	return TxTypeFeeDelegatedValueTransfer
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetRoleTypeForValidation() RoleType {
	return RoleTransaction
}

func (t *TxInternalDataFeeDelegatedValueTransfer) Equal(b TxInternalData) bool {
	tb, ok := b.(*TxInternalDataFeeDelegatedValueTransfer)
	if !ok {
		return false
	}

	return t.AccountNonce == tb.AccountNonce &&
		t.Price.Cmp(tb.Price) == 0 &&
		t.GasLimit == tb.GasLimit &&
		t.Recipient == tb.Recipient &&
		t.Amount.Cmp(tb.Amount) == 0 &&
		t.From == tb.From &&
		t.TxSignatures.equal(tb.TxSignatures) &&
		t.FeePayer == tb.FeePayer &&
		t.FeePayerSignature.equal(tb.FeePayerSignature)
}

func (t *TxInternalDataFeeDelegatedValueTransfer) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetPrice() *big.Int {
	return new(big.Int).Set(t.Price)
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetRecipient() *common.Address {
	if t.Recipient == (common.Address{}) {
		return nil
	}

	to := common.Address(t.Recipient)
	return &to
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetAmount() *big.Int {
	return new(big.Int).Set(t.Amount)
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetFeePayer() common.Address {
	return t.FeePayer
}

func (t *TxInternalDataFeeDelegatedValueTransfer) GetFeePayerRawSignatureValues() []*big.Int {
	return t.FeePayerSignature.RawSignatureValues()
}

func (t *TxInternalDataFeeDelegatedValueTransfer) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataFeeDelegatedValueTransfer) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataFeeDelegatedValueTransfer) SetFeePayerSignature(s TxSignatures) {
	t.FeePayerSignature = s
}

func (t *TxInternalDataFeeDelegatedValueTransfer) RecoverFeePayerPubkey(txhash common.Hash, homestead bool, vfunc func(*big.Int) *big.Int) ([]*ecdsa.PublicKey, error) {
	return t.FeePayerSignature.RecoverPubkey(txhash, homestead, vfunc)
}

func (t *TxInternalDataFeeDelegatedValueTransfer) String() string {
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
	FeePayer:      %s
	FeePayerSig:   %s
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
		t.FeePayer.String(),
		t.FeePayerSignature.string(),
		enc)
}

func (t *TxInternalDataFeeDelegatedValueTransfer) IntrinsicGas() (uint64, error) {
	return params.TxGas + params.TxGasFeeDelegated, nil
}

func (t *TxInternalDataFeeDelegatedValueTransfer) SerializeForSign() []interface{} {
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
