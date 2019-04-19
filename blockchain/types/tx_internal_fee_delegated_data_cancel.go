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
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/hexutil"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataFeeDelegatedCancel is a fee-delegated transaction that cancels a transaction previously submitted into txpool by replacement.
// Since Klaytn defines fixed gas price for all transactions, a transaction cannot be replaced with
// another transaction with higher gas price. To provide tx replacement, TxInternalDataFeeDelegatedCancel is introduced.
// To replace a previously added tx, send a TxInternalFeeDelegatedCancel transaction with the same nonce.
type TxInternalDataFeeDelegatedCancel struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	From         common.Address

	TxSignatures

	FeePayer          common.Address
	FeePayerSignature TxSignatures

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func newTxInternalDataFeeDelegatedCancel() *TxInternalDataFeeDelegatedCancel {
	return &TxInternalDataFeeDelegatedCancel{
		Price:        new(big.Int),
		TxSignatures: NewTxSignatures(),
	}
}

func newTxInternalDataFeeDelegatedCancelWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataFeeDelegatedCancel, error) {
	d := newTxInternalDataFeeDelegatedCancel()

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

	if v, ok := values[TxValueKeyFeePayer].(common.Address); ok {
		d.FeePayer = v
		delete(values, TxValueKeyFeePayer)
	} else {
		return nil, errValueKeyFeePayerMustAddress
	}

	if len(values) != 0 {
		for k := range values {
			logger.Warn("unnecessary key", k.String())
		}
		return nil, errUndefinedKeyRemains
	}

	return d, nil
}

func (t *TxInternalDataFeeDelegatedCancel) Type() TxType {
	return TxTypeFeeDelegatedCancel
}

func (t *TxInternalDataFeeDelegatedCancel) GetRoleTypeForValidation() accountkey.RoleType {
	return accountkey.RoleTransaction
}

func (t *TxInternalDataFeeDelegatedCancel) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataFeeDelegatedCancel) GetPrice() *big.Int {
	return t.Price
}

func (t *TxInternalDataFeeDelegatedCancel) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataFeeDelegatedCancel) GetRecipient() *common.Address {
	return &common.Address{}
}

func (t *TxInternalDataFeeDelegatedCancel) GetAmount() *big.Int {
	return common.Big0
}

func (t *TxInternalDataFeeDelegatedCancel) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataFeeDelegatedCancel) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataFeeDelegatedCancel) GetFeePayer() common.Address {
	return t.FeePayer
}

func (t *TxInternalDataFeeDelegatedCancel) GetFeePayerRawSignatureValues() []*big.Int {
	return t.FeePayerSignature.RawSignatureValues()
}

func (t *TxInternalDataFeeDelegatedCancel) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataFeeDelegatedCancel) SetFeePayerSignature(s TxSignatures) {
	t.FeePayerSignature = s
}

func (t *TxInternalDataFeeDelegatedCancel) RecoverFeePayerPubkey(txhash common.Hash, homestead bool, vfunc func(*big.Int) *big.Int) ([]*ecdsa.PublicKey, error) {
	return t.FeePayerSignature.RecoverPubkey(txhash, homestead, vfunc)
}

func (t *TxInternalDataFeeDelegatedCancel) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataFeeDelegatedCancel) Equal(b TxInternalData) bool {
	ta, ok := b.(*TxInternalDataFeeDelegatedCancel)
	if !ok {
		return false
	}

	return t.AccountNonce == ta.AccountNonce &&
		t.Price.Cmp(ta.Price) == 0 &&
		t.GasLimit == ta.GasLimit &&
		t.From == ta.From &&
		t.TxSignatures.equal(ta.TxSignatures) &&
		t.FeePayer == ta.FeePayer &&
		t.FeePayerSignature.equal(ta.FeePayerSignature)
}

func (t *TxInternalDataFeeDelegatedCancel) String() string {
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
	FeePayer:      %s
	FeePayerSig:   %s
	Hex:           %x
`,
		tx.Hash(),
		t.Type().String(),
		t.From.String(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.TxSignatures.string(),
		t.FeePayer.String(),
		t.FeePayerSignature.string(),
		enc)
}

func (t *TxInternalDataFeeDelegatedCancel) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataFeeDelegatedCancel) IntrinsicGas(currentBlockNumber uint64) (uint64, error) {
	return params.TxGasCancel + params.TxGasFeeDelegated, nil
}

func (t *TxInternalDataFeeDelegatedCancel) SerializeForSignToBytes() []byte {
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

func (t *TxInternalDataFeeDelegatedCancel) SerializeForSign() []interface{} {
	return []interface{}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.From,
	}
}

func (t *TxInternalDataFeeDelegatedCancel) Validate(stateDB StateDB, currentBlockNumber uint64) error {
	// Fail if the sender does not exist.
	if !stateDB.Exist(t.From) {
		return errValueKeySenderUnknown
	}
	return nil
}

func (t *TxInternalDataFeeDelegatedCancel) ValidateMutableValue(stateDB StateDB) bool {
	return true
}

func (t *TxInternalDataFeeDelegatedCancel) Execute(sender ContractRef, vm VM, stateDB StateDB, currentBlockNumber uint64, gas uint64, value *big.Int) (ret []byte, usedGas uint64, err error) {
	stateDB.IncNonce(sender.Address())
	return nil, gas, nil
}

func (t *TxInternalDataFeeDelegatedCancel) MakeRPCOutput() map[string]interface{} {
	return map[string]interface{}{
		"type":     t.Type().String(),
		"gas":      hexutil.Uint64(t.GasLimit),
		"gasPrice": (*hexutil.Big)(t.Price),
		"nonce":    hexutil.Uint64(t.AccountNonce),
		"feePayer": t.FeePayer,
	}
}
