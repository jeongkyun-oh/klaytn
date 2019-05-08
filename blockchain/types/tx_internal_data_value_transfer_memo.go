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
	"encoding/json"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/hexutil"
	"github.com/ground-x/klaytn/crypto/sha3"
	"github.com/ground-x/klaytn/kerrors"
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

type TxInternalDataValueTransferMemoJSON struct {
	Type         TxType           `json:"typeInt"`
	TypeStr      string           `json:"type"`
	AccountNonce hexutil.Uint64   `json:"nonce"`
	Price        *hexutil.Big     `json:"gasPrice"`
	GasLimit     hexutil.Uint64   `json:"gas"`
	Recipient    common.Address   `json:"to"`
	Amount       *hexutil.Big     `json:"value"`
	From         common.Address   `json:"from"`
	Payload      hexutil.Bytes    `json:"input"`
	TxSignatures TxSignaturesJSON `json:"signatures"`
	Hash         *common.Hash     `json:"hash"`
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

	if v, ok := values[TxValueKeyData].([]byte); ok {
		d.Payload = v
		delete(values, TxValueKeyData)
	} else {
		return nil, errValueKeyDataMustByteSlice
	}

	if len(values) != 0 {
		for k := range values {
			logger.Warn("unnecessary key", k.String())
		}
		return nil, errUndefinedKeyRemains
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

func (t *TxInternalDataValueTransferMemo) IntrinsicGas(currentBlockNumber uint64) (uint64, error) {
	gas := params.TxGasValueTransfer
	gasPayloadWithGas, err := IntrinsicGasPayload(gas, t.Payload)
	if err != nil {
		return 0, err
	}

	return gasPayloadWithGas, nil
}

func (t *TxInternalDataValueTransferMemo) SerializeForSignToBytes() []byte {
	b, _ := rlp.EncodeToBytes(struct {
		Txtype       TxType
		AccountNonce uint64
		Price        *big.Int
		GasLimit     uint64
		Recipient    common.Address
		Amount       *big.Int
		From         common.Address
		Payload      []byte
	}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.Payload,
	})

	return b
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

func (t *TxInternalDataValueTransferMemo) SenderTxHash() common.Hash {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, t.Type())
	rlp.Encode(hw, []interface{}{
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.Payload,
		t.TxSignatures,
	})

	h := common.Hash{}

	hw.Sum(h[:0])

	return h
}

func (t *TxInternalDataValueTransferMemo) Validate(stateDB StateDB, currentBlockNumber uint64) error {
	if common.IsPrecompiledContractAddress(t.Recipient) {
		return kerrors.ErrPrecompiledContractAddress
	}
	return t.ValidateMutableValue(stateDB, currentBlockNumber)
}

func (t *TxInternalDataValueTransferMemo) ValidateMutableValue(stateDB StateDB, currentBlockNumber uint64) error {
	if stateDB.IsProgramAccount(t.Recipient) {
		return kerrors.ErrNotForProgramAccount
	}
	return nil
}

func (t *TxInternalDataValueTransferMemo) Execute(sender ContractRef, vm VM, stateDB StateDB, currentBlockNumber uint64, gas uint64, value *big.Int) (ret []byte, usedGas uint64, err error) {
	stateDB.IncNonce(sender.Address())
	return vm.Call(sender, t.Recipient, t.Payload, gas, value)
}

func (t *TxInternalDataValueTransferMemo) MakeRPCOutput() map[string]interface{} {
	return map[string]interface{}{
		"typeInt":    t.Type(),
		"type":       t.Type().String(),
		"gas":        hexutil.Uint64(t.GasLimit),
		"gasPrice":   (*hexutil.Big)(t.Price),
		"input":      hexutil.Bytes(t.Payload),
		"nonce":      hexutil.Uint64(t.AccountNonce),
		"to":         t.Recipient,
		"value":      (*hexutil.Big)(t.Amount),
		"signatures": t.TxSignatures.ToJSON(),
	}
}

func (t *TxInternalDataValueTransferMemo) MarshalJSON() ([]byte, error) {
	return json.Marshal(TxInternalDataValueTransferMemoJSON{
		t.Type(),
		t.Type().String(),
		(hexutil.Uint64)(t.AccountNonce),
		(*hexutil.Big)(t.Price),
		(hexutil.Uint64)(t.GasLimit),
		t.Recipient,
		(*hexutil.Big)(t.Amount),
		t.From,
		t.Payload,
		t.TxSignatures.ToJSON(),
		t.Hash,
	})
}

func (t *TxInternalDataValueTransferMemo) UnmarshalJSON(b []byte) error {
	js := &TxInternalDataValueTransferMemoJSON{}
	if err := json.Unmarshal(b, js); err != nil {
		return err
	}

	t.AccountNonce = uint64(js.AccountNonce)
	t.Price = (*big.Int)(js.Price)
	t.GasLimit = uint64(js.GasLimit)
	t.Recipient = js.Recipient
	t.Amount = (*big.Int)(js.Amount)
	t.From = js.From
	t.Payload = js.Payload
	t.TxSignatures = js.TxSignatures.ToTxSignatures()
	t.Hash = js.Hash

	return nil
}
