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
	"github.com/ground-x/klaytn/common/hexutil"
	"github.com/ground-x/klaytn/common/math"
	"github.com/ground-x/klaytn/kerrors"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataChainDataAnchoring represents the transaction anchoring child chain data.
type TxInternalDataChainDataAnchoring struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	Recipient    common.Address
	Amount       *big.Int
	From         common.Address
	AnchoredData []byte

	TxSignatures

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

func newTxInternalDataChainDataAnchoring() *TxInternalDataChainDataAnchoring {
	h := common.Hash{}

	return &TxInternalDataChainDataAnchoring{
		Price:  new(big.Int),
		Amount: new(big.Int),
		Hash:   &h,
	}
}

func newTxInternalDataChainDataAnchoringWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataChainDataAnchoring, error) {
	d := newTxInternalDataChainDataAnchoring()

	if v, ok := values[TxValueKeyNonce].(uint64); ok {
		d.AccountNonce = v
		delete(values, TxValueKeyNonce)
	} else {
		return nil, errValueKeyNonceMustUint64
	}

	if v, ok := values[TxValueKeyGasPrice].(*big.Int); ok {
		d.Price.Set(v)
		delete(values, TxValueKeyGasPrice)
	} else {
		return nil, errValueKeyGasPriceMustBigInt
	}

	if v, ok := values[TxValueKeyGasLimit].(uint64); ok {
		d.GasLimit = v
		delete(values, TxValueKeyGasLimit)
	} else {
		return nil, errValueKeyGasLimitMustUint64
	}

	if v, ok := values[TxValueKeyTo].(common.Address); ok {
		d.Recipient = v
		delete(values, TxValueKeyTo)
	} else {
		return nil, errValueKeyToMustAddress
	}

	if v, ok := values[TxValueKeyAmount].(*big.Int); ok {
		d.Amount.Set(v)
		delete(values, TxValueKeyAmount)
	} else {
		return nil, errValueKeyAmountMustBigInt
	}

	if v, ok := values[TxValueKeyFrom].(common.Address); ok {
		d.From = v
		delete(values, TxValueKeyFrom)
	} else {
		return nil, errValueKeyFromMustAddress
	}

	if v, ok := values[TxValueKeyAnchoredData].([]byte); ok {
		d.AnchoredData = v
		delete(values, TxValueKeyAnchoredData)
	} else {
		return nil, errValueKeyAnchoredDataMustByteSlice
	}

	if len(values) != 0 {
		for k := range values {
			logger.Warn("unnecessary key", k.String())
		}
		return nil, errUndefinedKeyRemains
	}

	return d, nil
}

func (t *TxInternalDataChainDataAnchoring) Type() TxType {
	return TxTypeChainDataAnchoring
}

func (t *TxInternalDataChainDataAnchoring) GetRoleTypeForValidation() accountkey.RoleType {
	return accountkey.RoleTransaction
}

func (t *TxInternalDataChainDataAnchoring) Equal(b TxInternalData) bool {
	tb, ok := b.(*TxInternalDataChainDataAnchoring)
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
		bytes.Equal(t.AnchoredData, tb.AnchoredData)
}

func (t *TxInternalDataChainDataAnchoring) String() string {
	ser := newTxInternalDataSerializerWithValues(t)
	enc, _ := rlp.EncodeToBytes(ser)
	dataAnchoredRLP, _ := rlp.EncodeToBytes(t.AnchoredData)
	tx := Transaction{data: t}

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
	AnchoredData:  %s
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
		enc,
		common.Bytes2Hex(dataAnchoredRLP))
}

func (t *TxInternalDataChainDataAnchoring) SerializeForSignToBytes() []byte {
	b, _ := rlp.EncodeToBytes(struct {
		Txtype       TxType
		AccountNonce uint64
		Price        *big.Int
		GasLimit     uint64
		Recipient    common.Address
		Amount       *big.Int
		From         common.Address
		AnchoredData []byte
	}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.AnchoredData,
	})

	return b
}

func (t *TxInternalDataChainDataAnchoring) SerializeForSign() []interface{} {
	return []interface{}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.AnchoredData,
	}
}

func (t *TxInternalDataChainDataAnchoring) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataChainDataAnchoring) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataChainDataAnchoring) GetPrice() *big.Int {
	return new(big.Int).Set(t.Price)
}

func (t *TxInternalDataChainDataAnchoring) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataChainDataAnchoring) GetRecipient() *common.Address {
	if t.Recipient == (common.Address{}) {
		return nil
	}

	to := common.Address(t.Recipient)
	return &to
}

func (t *TxInternalDataChainDataAnchoring) GetAmount() *big.Int {
	return new(big.Int).Set(t.Amount)
}

func (t *TxInternalDataChainDataAnchoring) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataChainDataAnchoring) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataChainDataAnchoring) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataChainDataAnchoring) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataChainDataAnchoring) IntrinsicGas(currentBlockNumber uint64) (uint64, error) {
	nByte := (uint64)(len(t.AnchoredData))

	// Make sure we don't exceed uint64 for all data combinations
	if (math.MaxUint64-params.TxChainDataAnchoringGas)/params.ChainDataAnchoringGas < nByte {
		return 0, kerrors.ErrOutOfGas
	}

	return params.TxChainDataAnchoringGas + params.ChainDataAnchoringGas*nByte, nil
}

func (t *TxInternalDataChainDataAnchoring) Validate(stateDB StateDB, currentBlockNumber uint64) error {
	// No validation for TxInternalDataChainDataAnchoring
	return nil
}

func (t *TxInternalDataChainDataAnchoring) Execute(sender ContractRef, vm VM, stateDB StateDB, currentBlockNumber uint64, gas uint64, value *big.Int) (ret []byte, usedGas uint64, err error) {
	stateDB.IncNonce(sender.Address())
	return vm.Call(sender, t.Recipient, nil, gas, value)
}

func (t *TxInternalDataChainDataAnchoring) MakeRPCOutput() map[string]interface{} {
	return map[string]interface{}{
		"type":         t.Type().String(),
		"gas":          hexutil.Uint64(t.GasLimit),
		"gasPrice":     (*hexutil.Big)(t.Price),
		"nonce":        hexutil.Uint64(t.AccountNonce),
		"to":           t.Recipient,
		"value":        (*hexutil.Big)(t.Amount),
		"anchoredData": hexutil.Bytes(t.AnchoredData),
	}
}
