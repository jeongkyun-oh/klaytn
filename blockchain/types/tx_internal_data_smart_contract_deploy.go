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
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
)

// TxInternalDataSmartContractDeploy represents a transaction creating a smart contract.
type TxInternalDataSmartContractDeploy struct {
	*TxInternalDataCommon

	Payload       []byte
	HumanReadable bool

	TxSignatures
}

func newTxInternalDataSmartContractDeploy() *TxInternalDataSmartContractDeploy {
	return &TxInternalDataSmartContractDeploy{
		newTxInternalDataCommon(),
		[]byte{},
		false,
		NewTxSignatures(),
	}
}

func newTxInternalDataSmartContractDeployWithMap(values map[TxValueKeyType]interface{}) (*TxInternalDataSmartContractDeploy, error) {
	c, err := newTxInternalDataCommonWithMap(values)
	if err != nil {
		return nil, err
	}

	t := &TxInternalDataSmartContractDeploy{
		c, []byte{}, false, NewTxSignatures(),
	}

	if v, ok := values[TxValueKeyData].([]byte); ok {
		t.Payload = v
	} else {
		return nil, errValueKeyDataMustByteSlice
	}

	if v, ok := values[TxValueKeyHumanReadable].(bool); ok {
		t.HumanReadable = v
	} else {
		return nil, errValueKeyHumanReadableMustBool
	}

	return t, nil
}

func (t *TxInternalDataSmartContractDeploy) Type() TxType {
	return TxTypeSmartContractDeploy
}

func (t *TxInternalDataSmartContractDeploy) GetPayload() []byte {
	return t.Payload
}

func (t *TxInternalDataSmartContractDeploy) Equal(a TxInternalData) bool {
	ta, ok := a.(*TxInternalDataSmartContractDeploy)
	if !ok {
		return false
	}

	return t.TxInternalDataCommon.equal(ta.TxInternalDataCommon) &&
		bytes.Equal(t.Payload, ta.Payload) &&
		t.HumanReadable == ta.HumanReadable &&
		t.TxSignatures.equal(ta.TxSignatures)
}

func (t *TxInternalDataSmartContractDeploy) SetSignature(s TxSignatures) {
	t.TxSignatures = s
}

func (t *TxInternalDataSmartContractDeploy) String() string {
	ser := newTxInternalDataSerializerWithValues(t)
	tx := Transaction{data: t}
	enc, _ := rlp.EncodeToBytes(ser)
	return fmt.Sprintf(`
	TX(%x)
	Type:          %s%s
	Signature:     %s
	Paylod:        %x
	Hex:           %x
`,
		tx.Hash(),
		t.Type().String(),
		t.TxInternalDataCommon.string(),
		t.TxSignatures.string(),
		common.Bytes2Hex(t.Payload),
		enc)

}

func (t *TxInternalDataSmartContractDeploy) IntrinsicGas() (uint64, error) {
	gas := params.TxGasContractCreation

	gasPayload, err := intrinsicGasPayload(t.Payload)
	if err != nil {
		return 0, err
	}

	return gas + gasPayload, nil
}

func (t *TxInternalDataSmartContractDeploy) SerializeForSign() []interface{} {
	infs := []interface{}{t.Type()}
	infs = append(infs, t.TxInternalDataCommon.serializeForSign()...)

	return append(infs, t.Payload)
}
