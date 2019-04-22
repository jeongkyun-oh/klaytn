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
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/hexutil"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/kerrors"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
)

// TxInternalDataFeeDelegatedSmartContractDeployWithRatio represents a fee-delegated transaction creating a smart contract
// with a specified fee ratio between the sender and the fee payer.
// The ratio is a fee payer's ratio in percentage.
// For example, if it is 20, 20% of tx fee will be paid by the fee payer.
// 80% of tx fee will be paid by the sender.
type TxInternalDataFeeDelegatedSmartContractDeployWithRatio struct {
	AccountNonce  uint64
	Price         *big.Int
	GasLimit      uint64
	Recipient     *common.Address `rlp:"nil"`
	Amount        *big.Int
	From          common.Address
	Payload       []byte
	HumanReadable bool
	FeeRatio      FeeRatio

	TxSignatures

	FeePayer           common.Address
	FeePayerSignatures TxSignatures

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

type TxInternalDataFeeDelegatedSmartContractDeployWithRatioJSON struct {
	Type               TxType           `json:"typeInt"`
	TypeStr            string           `json:"type"`
	AccountNonce       hexutil.Uint64   `json:"nonce"`
	Price              *hexutil.Big     `json:"gasPrice"`
	GasLimit           hexutil.Uint64   `json:"gas"`
	Recipient          *common.Address  `json:"to"`
	Amount             *hexutil.Big     `json:"value"`
	From               common.Address   `json:"from"`
	Payload            hexutil.Bytes    `json:"input"`
	HumanReadable      bool             `json:"humanReadable"`
	FeeRatio           hexutil.Uint     `json:"feeRatio"`
	TxSignatures       TxSignaturesJSON `json:"signatures"`
	FeePayer           common.Address   `json:"feePayer"`
	FeePayerSignatures TxSignaturesJSON `json:"feePayerSignatures"`
	Hash               *common.Hash     `json:"hash"`
}

func newTxInternalDataFeeDelegatedSmartContractDeployWithRatio() *TxInternalDataFeeDelegatedSmartContractDeployWithRatio {
	h := common.Hash{}
	return &TxInternalDataFeeDelegatedSmartContractDeployWithRatio{
		Price:  new(big.Int),
		Amount: new(big.Int),
		Hash:   &h,
	}
}

func newTxInternalDataFeeDelegatedSmartContractDeployWithRatioWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataFeeDelegatedSmartContractDeployWithRatio, error) {
	t := newTxInternalDataFeeDelegatedSmartContractDeployWithRatio()

	if v, ok := values[TxValueKeyNonce].(uint64); ok {
		t.AccountNonce = v
		delete(values, TxValueKeyNonce)
	} else {
		return nil, errValueKeyNonceMustUint64
	}

	if v, ok := values[TxValueKeyTo].(*common.Address); ok {
		t.Recipient = v
		delete(values, TxValueKeyTo)
	} else {
		return nil, errValueKeyToMustAddressPointer
	}

	if v, ok := values[TxValueKeyAmount].(*big.Int); ok {
		t.Amount.Set(v)
		delete(values, TxValueKeyAmount)
	} else {
		return nil, errValueKeyAmountMustBigInt
	}

	if v, ok := values[TxValueKeyGasLimit].(uint64); ok {
		t.GasLimit = v
		delete(values, TxValueKeyGasLimit)
	} else {
		return nil, errValueKeyGasLimitMustUint64
	}

	if v, ok := values[TxValueKeyGasPrice].(*big.Int); ok {
		t.Price.Set(v)
		delete(values, TxValueKeyGasPrice)
	} else {
		return nil, errValueKeyGasPriceMustBigInt
	}

	if v, ok := values[TxValueKeyFrom].(common.Address); ok {
		t.From = v
		delete(values, TxValueKeyFrom)
	} else {
		return nil, errValueKeyFromMustAddress
	}

	if v, ok := values[TxValueKeyData].([]byte); ok {
		t.Payload = v
		delete(values, TxValueKeyData)
	} else {
		return nil, errValueKeyDataMustByteSlice
	}

	if v, ok := values[TxValueKeyHumanReadable].(bool); ok {
		t.HumanReadable = v
		delete(values, TxValueKeyHumanReadable)
	} else {
		return nil, errValueKeyHumanReadableMustBool
	}

	if v, ok := values[TxValueKeyFeePayer].(common.Address); ok {
		t.FeePayer = v
		delete(values, TxValueKeyFeePayer)
	} else {
		return nil, errValueKeyFeePayerMustAddress
	}

	if v, ok := values[TxValueKeyFeeRatioOfFeePayer].(FeeRatio); ok {
		t.FeeRatio = v
		delete(values, TxValueKeyFeeRatioOfFeePayer)
	} else {
		return nil, errValueKeyFeeRatioMustUint8
	}

	if len(values) != 0 {
		for k := range values {
			logger.Warn("unnecessary key", k.String())
		}
		return nil, errUndefinedKeyRemains
	}

	return t, nil
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) Type() TxType {
	return TxTypeFeeDelegatedSmartContractDeployWithRatio
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetRoleTypeForValidation() accountkey.RoleType {
	return accountkey.RoleTransaction
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetPayload() []byte {
	return t.Payload
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) Equal(a TxInternalData) bool {
	ta, ok := a.(*TxInternalDataFeeDelegatedSmartContractDeployWithRatio)
	if !ok {
		return false
	}

	return t.AccountNonce == ta.AccountNonce &&
		t.Price.Cmp(ta.Price) == 0 &&
		t.GasLimit == ta.GasLimit &&
		equalRecipient(t.Recipient, ta.Recipient) &&
		t.Amount.Cmp(ta.Amount) == 0 &&
		t.From == ta.From &&
		t.FeeRatio == ta.FeeRatio &&
		bytes.Equal(t.Payload, ta.Payload) &&
		t.HumanReadable == ta.HumanReadable &&
		t.TxSignatures.equal(ta.TxSignatures) &&
		t.FeePayer == ta.FeePayer &&
		t.FeePayerSignatures.equal(ta.FeePayerSignatures)
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) IsLegacyTransaction() bool {
	return false
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetAccountNonce() uint64 {
	return t.AccountNonce
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetPrice() *big.Int {
	return new(big.Int).Set(t.Price)
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetGasLimit() uint64 {
	return t.GasLimit
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetRecipient() *common.Address {
	return t.Recipient
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetAmount() *big.Int {
	return new(big.Int).Set(t.Amount)
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetFrom() common.Address {
	return t.From
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetHash() *common.Hash {
	return t.Hash
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetFeePayer() common.Address {
	return t.FeePayer
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetFeePayerRawSignatureValues() TxSignatures {
	return t.FeePayerSignatures.RawSignatureValues()
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) GetFeeRatio() FeeRatio {
	return t.FeeRatio
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) SetHash(h *common.Hash) {
	t.Hash = h
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) SetFeePayerSignatures(s TxSignatures) {
	t.FeePayerSignatures = s
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) RecoverFeePayerPubkey(txhash common.Hash, homestead bool, vfunc func(*big.Int) *big.Int) ([]*ecdsa.PublicKey, error) {
	return t.FeePayerSignatures.RecoverPubkey(txhash, homestead, vfunc)
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) String() string {
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
	Paylod:        %x
	HumanReadable: %v
	Signature:     %s
	FeePayer:      %s
	FeeRatio:      %d
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
		common.Bytes2Hex(t.Payload),
		t.HumanReadable,
		t.TxSignatures.string(),
		t.FeePayer.String(),
		t.FeeRatio,
		t.FeePayerSignatures.string(),
		enc)

}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) IntrinsicGas(currentBlockNumber uint64) (uint64, error) {
	gas := params.TxGasContractCreation + params.TxGasFeeDelegatedWithRatio

	gasPayloadWithGas, err := IntrinsicGasPayload(gas, t.Payload)
	if err != nil {
		return 0, err
	}

	return gasPayloadWithGas, nil
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) SerializeForSignToBytes() []byte {
	b, _ := rlp.EncodeToBytes(struct {
		Txtype        TxType
		AccountNonce  uint64
		Price         *big.Int
		GasLimit      uint64
		Recipient     *common.Address
		Amount        *big.Int
		From          common.Address
		Payload       []byte
		HumanReadable bool
		FeeRatio      FeeRatio
	}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.Payload,
		t.HumanReadable,
		t.FeeRatio,
	})

	return b
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) SerializeForSign() []interface{} {
	return []interface{}{
		t.Type(),
		t.AccountNonce,
		t.Price,
		t.GasLimit,
		t.Recipient,
		t.Amount,
		t.From,
		t.Payload,
		t.HumanReadable,
		t.FeeRatio,
	}
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) Validate(stateDB StateDB, currentBlockNumber uint64) error {
	if t.Recipient != nil {
		to := t.Recipient
		if t.HumanReadable {
			addrString := string(bytes.TrimRightFunc(to.Bytes(), func(r rune) bool {
				if r == rune(0x0) {
					return true
				}
				return false
			}))
			if err := common.IsHumanReadableAddress(addrString); err != nil {
				return kerrors.ErrNotHumanReadableAddress
			}
		}
		// Fail if the address is already created.
		if stateDB.Exist(*to) {
			return kerrors.ErrAccountAlreadyExists
		}
	}
	// Fail if the sender does not exist.
	if !stateDB.Exist(t.From) {
		return errValueKeySenderUnknown
	}

	return nil
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) ValidateMutableValue(stateDB StateDB) bool {
	if t.Recipient != nil && stateDB.Exist(*t.Recipient) {
		return false
	}
	return true
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) FillContractAddress(from common.Address, r *Receipt) {
	if t.Recipient == nil {
		codeHash := crypto.Keccak256Hash(t.Payload)
		r.ContractAddress = crypto.CreateAddress(from, t.AccountNonce, codeHash)
	} else {
		r.ContractAddress = *t.Recipient
	}
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) Execute(sender ContractRef, vm VM, stateDB StateDB, currentBlockNumber uint64, gas uint64, value *big.Int) (ret []byte, usedGas uint64, err error) {
	if err := t.Validate(stateDB, currentBlockNumber); err != nil {
		stateDB.IncNonce(sender.Address())
		return nil, 0, err
	}

	if t.Recipient == nil {
		ret, _, usedGas, err = vm.Create(sender, t.Payload, gas, value)
	} else {
		ret, _, usedGas, err = vm.CreateWithAddress(sender, t.Payload, gas, value, *t.Recipient, t.HumanReadable)
	}
	return
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) MakeRPCOutput() map[string]interface{} {
	return map[string]interface{}{
		"typeInt":            t.Type(),
		"type":               t.Type().String(),
		"gas":                hexutil.Uint64(t.GasLimit),
		"gasPrice":           (*hexutil.Big)(t.Price),
		"input":              hexutil.Bytes(t.Payload),
		"nonce":              hexutil.Uint64(t.AccountNonce),
		"to":                 t.Recipient,
		"value":              (*hexutil.Big)(t.Amount),
		"humanReadable":      t.HumanReadable,
		"feeRatio":           hexutil.Uint(t.FeeRatio),
		"signatures":         t.TxSignatures.ToJSON(),
		"feePayer":           t.FeePayer,
		"feePayerSignatures": t.FeePayerSignatures.ToJSON(),
	}
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) MarshalJSON() ([]byte, error) {
	return json.Marshal(TxInternalDataFeeDelegatedSmartContractDeployWithRatioJSON{
		t.Type(),
		t.Type().String(),
		(hexutil.Uint64)(t.AccountNonce),
		(*hexutil.Big)(t.Price),
		(hexutil.Uint64)(t.GasLimit),
		t.Recipient,
		(*hexutil.Big)(t.Amount),
		t.From,
		t.Payload,
		t.HumanReadable,
		(hexutil.Uint)(t.FeeRatio),
		t.TxSignatures.ToJSON(),
		t.FeePayer,
		t.FeePayerSignatures.ToJSON(),
		t.Hash,
	})
}

func (t *TxInternalDataFeeDelegatedSmartContractDeployWithRatio) UnmarshalJSON(b []byte) error {
	js := &TxInternalDataFeeDelegatedSmartContractDeployWithRatioJSON{}
	if err := json.Unmarshal(b, js); err != nil {
		return err
	}

	t.AccountNonce = uint64(js.AccountNonce)
	t.Price = (*big.Int)(js.Price)
	t.GasLimit = (uint64)(js.GasLimit)
	t.Recipient = js.Recipient
	t.Amount = (*big.Int)(js.Amount)
	t.From = js.From
	t.Payload = js.Payload
	t.HumanReadable = js.HumanReadable
	t.FeeRatio = FeeRatio(js.FeeRatio)
	t.TxSignatures = js.TxSignatures.ToTxSignatures()
	t.FeePayer = js.FeePayer
	t.FeePayerSignatures = js.FeePayerSignatures.ToTxSignatures()
	t.Hash = js.Hash

	return nil
}
