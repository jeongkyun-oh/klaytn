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
	"encoding/json"
	"errors"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/kerrors"
	"math/big"
)

var (
	ErrShouldBeSingleSignature = errors.New("the number of signatures should be one")
)

// TxSignatures is a slice of TxSignature. It is created to support multi-sig accounts.
// Note that this structure also processes txs having a single signature.
// TODO-Klaytn-Accounts: replace TxSignature with TxSignatures to all newly implemented tx types.
type TxSignatures []*TxSignature

func NewTxSignatures() TxSignatures {
	return TxSignatures{NewTxSignature()}
}

func NewTxSignaturesWithValues(signer Signer, txhash common.Hash, prv []*ecdsa.PrivateKey) (TxSignatures, error) {
	if len(prv) == 0 {
		return nil, kerrors.ErrEmptySlice
	}
	if uint64(len(prv)) > accountkey.MaxNumKeysForMultiSig {
		return nil, kerrors.ErrMaxKeysExceed
	}
	txsigs := make(TxSignatures, len(prv))

	for i, p := range prv {
		t, err := NewTxSignatureWithValues(signer, txhash, p)
		if err != nil {
			return nil, err
		}
		txsigs[i] = t
	}

	return txsigs, nil
}

func (t TxSignatures) getDefaultSig() *TxSignature {
	return t[0]
}

func (t TxSignatures) empty() bool {
	return len(t) == 0
}

func (t TxSignatures) ChainId() *big.Int {
	if t.empty() {
		// This path should not be executed. This is written only for debugging.
		logger.CritWithStack("should not be called if no entries exist")
	}

	// TODO-Klaytn-Multisig: Find a way to handle multiple V values here.
	return t.getDefaultSig().ChainId()
}

func (t TxSignatures) Protected() bool {
	if t.empty() {
		// This path should not be executed. This is written only for debugging.
		logger.CritWithStack("should not be called if no entries exist")
	}

	// TODO-Klaytn-Multisig: Find a way to handle multiple V values here.
	return t.getDefaultSig().Protected()
}

func (t TxSignatures) RawSignatureValues() []*big.Int {
	values := make([]*big.Int, 0, 3*len(t))
	for _, s := range t {
		values = append(values, s.RawSignatureValues()...)
	}

	return values
}

func (t TxSignatures) ValidateSignature() bool {
	if t.empty() {
		return false
	}

	cid := t.getDefaultSig().ChainId()
	for _, s := range t {
		if s.ValidateSignature() == false {
			return false
		}
		if cid.Cmp(s.ChainId()) != 0 {
			return false
		}
	}

	return true
}

func (t TxSignatures) equal(tb TxSignatures) bool {
	if len(t) != len(tb) {
		return false
	}

	for i, s := range t {
		if s.equal(tb[i]) == false {
			return false
		}
	}

	return true
}

func (t TxSignatures) RecoverAddress(txhash common.Hash, homestead bool, vfunc func(*big.Int) *big.Int) (common.Address, error) {
	if len(t) != 1 {
		return common.Address{}, ErrShouldBeSingleSignature
	}

	return t.getDefaultSig().RecoverAddress(txhash, homestead, vfunc)
}

func (t TxSignatures) RecoverPubkey(txhash common.Hash, homestead bool, vfunc func(*big.Int) *big.Int) ([]*ecdsa.PublicKey, error) {
	var err error

	pubkeys := make([]*ecdsa.PublicKey, len(t))
	for i, s := range t {
		pubkeys[i], err = s.RecoverPubkey(txhash, homestead, vfunc)
		if err != nil {
			return nil, err
		}
	}

	return pubkeys, nil
}

func (t TxSignatures) string() string {
	b, _ := json.Marshal(t)

	return string(b)
}
