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

package accountkey

import (
	"crypto/ecdsa"
	"encoding/json"
	"github.com/ground-x/klaytn/fork"
	"github.com/ground-x/klaytn/kerrors"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
)

const (
	// TODO-Klaytn-MultiSig: Need to fix the maximum number of keys allowed for an account.
	// NOTE-Klaytn-MultiSig: This value should not be reduced. If it is reduced, there is a case:
	// - the tx validation will be failed if the sender has larger keys.
	MaxNumKeysForMultiSig = uint64(10)
)

// AccountKeyWeightedMultiSig is an account key type containing a threshold and `WeightedPublicKeys`.
// `WeightedPublicKeys` contains a slice of {weight and key}.
// To be a valid tx for an account associated with `AccountKeyWeightedMultiSig`,
// the weighted sum of signed public keys should be larger than the threshold.
// Refer to AccountKeyWeightedMultiSig.Validate().
type AccountKeyWeightedMultiSig struct {
	Threshold uint
	Keys      WeightedPublicKeys
}

func NewAccountKeyWeightedMultiSig() *AccountKeyWeightedMultiSig {
	return &AccountKeyWeightedMultiSig{}
}

func NewAccountKeyWeightedMultiSigWithValues(threshold uint, keys WeightedPublicKeys) *AccountKeyWeightedMultiSig {
	return &AccountKeyWeightedMultiSig{threshold, keys}
}

func (a *AccountKeyWeightedMultiSig) Type() AccountKeyType {
	return AccountKeyTypeWeightedMultiSig
}

func (a *AccountKeyWeightedMultiSig) IsCompositeType() bool {
	return false
}

func (a *AccountKeyWeightedMultiSig) DeepCopy() AccountKey {
	return &AccountKeyWeightedMultiSig{
		a.Threshold, a.Keys.DeepCopy(),
	}
}

func (a *AccountKeyWeightedMultiSig) Equal(b AccountKey) bool {
	tb, ok := b.(*AccountKeyWeightedMultiSig)
	if !ok {
		return false
	}

	return a.Threshold == tb.Threshold &&
		a.Keys.Equal(tb.Keys)
}

func (a *AccountKeyWeightedMultiSig) Validate(r RoleType, pubkeys []*ecdsa.PublicKey) bool {
	weightedSum := uint(0)

	// To prohibit making a signature with the same key, make a map.
	// TODO-Klaytn: find another way for better performance
	pMap := make(map[string]*ecdsa.PublicKey)
	for _, bk := range pubkeys {
		b, err := rlp.EncodeToBytes((*PublicKeySerializable)(bk))
		if err != nil {
			logger.Warn("Failed to encode public keys in the tx", pubkeys)
			continue
		}
		pMap[string(b)] = bk
	}

	for _, k := range a.Keys {
		b, err := rlp.EncodeToBytes(k.Key)
		if err != nil {
			logger.Warn("Failed to encode public keys in the account", "AccountKey", a.String())
			continue
		}

		if _, ok := pMap[string(b)]; ok {
			weightedSum += k.Weight
		}
	}

	if weightedSum >= a.Threshold {
		return true
	}

	logger.Debug("AccountKeyWeightedMultiSig validation is failed", "pubkeys", pubkeys,
		"accountKeys", a.String(), "threshold", a.Threshold, "weighted sum", weightedSum)

	return false
}

func (a *AccountKeyWeightedMultiSig) String() string {
	serializer := NewAccountKeySerializerWithAccountKey(a)
	b, _ := json.Marshal(serializer)
	return string(b)
}

func (a *AccountKeyWeightedMultiSig) AccountCreationGas(currentBlockNumber uint64) (uint64, error) {
	numKeys := uint64(len(a.Keys))
	if numKeys > MaxNumKeysForMultiSig {
		return 0, kerrors.ErrMaxKeysExceed
	}
	// TODO-Klaytn-HF After GasFormulaFixBlockNumber, different accountCreationGas logic will be operated.
	if fork.IsGasFormulaFixEnabled(currentBlockNumber) {
		return numKeys * params.TxAccountCreationGasPerKey, nil
	}
	return params.TxAccountCreationGasDefault + numKeys*params.TxAccountCreationGasPerKey, nil
}

func (a *AccountKeyWeightedMultiSig) SigValidationGas(currentBlockNumber uint64, r RoleType) (uint64, error) {
	numKeys := uint64(len(a.Keys))
	if numKeys > MaxNumKeysForMultiSig {
		logger.Warn("validation failed due to the number of keys in the account is larger than the limit.",
			"account", a.String())
		return 0, kerrors.ErrMaxKeysExceedInValidation
	}
	// TODO-Klaytn-HF After GasFormulaFixBlockNumber, different sigValidationGas logic will be operated.
	if fork.IsGasFormulaFixEnabled(currentBlockNumber) {
		if numKeys == 0 {
			logger.Error("should not happen! numKeys is equal to zero!")
			return 0, kerrors.ErrZeroLength
		}
		return (numKeys - 1) * params.TxValidationGasPerKey, nil
	}
	return params.TxValidationGasDefault + numKeys*params.TxValidationGasPerKey, nil
}

func (a *AccountKeyWeightedMultiSig) Init(currentBlockNumber uint64) error {
	sum := uint(0)
	prevSum := uint(0)

	if len(a.Keys) == 0 {
		return kerrors.ErrZeroLength
	}

	if uint64(len(a.Keys)) > MaxNumKeysForMultiSig {
		return kerrors.ErrMaxKeysExceed
	}

	keyMap := make(map[string]bool)
	for _, k := range a.Keys {
		// Do not allow zero weight.
		if k.Weight == 0 {
			return kerrors.ErrZeroKeyWeight
		}
		sum += k.Weight

		b, err := rlp.EncodeToBytes(k.Key)
		if err != nil {
			// Do not allow unserializable keys.
			return kerrors.ErrUnserializableKey
		}
		if _, ok := keyMap[string(b)]; ok {
			// Do not allow duplicated keys.
			return kerrors.ErrDuplicatedKey
		}
		keyMap[string(b)] = true

		// Do not allow overflow of weighted sum.
		if prevSum > sum {
			return kerrors.ErrWeightedSumOverflow
		}
		prevSum = sum
	}
	// The weighted sum should be larger than the threshold.
	if sum < a.Threshold {
		return kerrors.ErrUnsatisfiableThreshold
	}

	return nil
}

func (a *AccountKeyWeightedMultiSig) Update(key AccountKey, currentBlockNumber uint64) error {
	if ak, ok := key.(*AccountKeyWeightedMultiSig); ok {
		if err := ak.Init(currentBlockNumber); err != nil {
			return err
		}
		a.Threshold = ak.Threshold
		copy(a.Keys, ak.Keys)
		return nil
	}

	// Update is not possible if the type is different.
	return kerrors.ErrDifferentAccountKeyType
}

// WeightedPublicKey contains a public key and its weight.
// The weight is used to check whether the weighted sum of public keys are larger than
// the threshold of the AccountKeyWeightedMultiSig object.
type WeightedPublicKey struct {
	Weight uint
	Key    *PublicKeySerializable
}

func (w *WeightedPublicKey) Equal(b *WeightedPublicKey) bool {
	return w.Weight == b.Weight &&
		w.Key.Equal(b.Key)
}

func NewWeightedPublicKey(weight uint, key *PublicKeySerializable) *WeightedPublicKey {
	return &WeightedPublicKey{weight, key}
}

// WeightedPublicKeys is a slice of WeightedPublicKey objects.
type WeightedPublicKeys []*WeightedPublicKey

func (w WeightedPublicKeys) DeepCopy() WeightedPublicKeys {
	keys := make(WeightedPublicKeys, len(w))

	for i, v := range w {
		keys[i] = NewWeightedPublicKey(v.Weight, v.Key.DeepCopy())
	}

	return keys
}

func (w WeightedPublicKeys) Equal(b WeightedPublicKeys) bool {
	if len(w) != len(b) {
		return false
	}

	for i, wv := range w {
		if !wv.Equal(b[i]) {
			return false
		}
	}

	return true
}
