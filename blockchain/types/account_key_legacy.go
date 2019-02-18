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
	"github.com/ground-x/klaytn/params"
	"runtime"
)

// AccountKeyLegacy is used for accounts having no keys.
// In this case, verifying the signature of a transaction uses the legacy scheme.
// 1. The address comes from the public key which is derived from txhash and the tx's signature.
// 2. Check that the address is the same as the address in the tx.
// It is implemented to support LegacyAccounts.
type AccountKeyLegacy struct {
}

var globalLegacyKey = &AccountKeyLegacy{}

// NewAccountKeyLegacy creates a new AccountKeyLegacy object.
// Since AccountKeyLegacy has no attributes, use one global variable for all allocations.
func NewAccountKeyLegacy() *AccountKeyLegacy { return globalLegacyKey }

func (a *AccountKeyLegacy) Type() AccountKeyType {
	return AccountKeyTypeLegacy
}

func (a *AccountKeyLegacy) Equal(b AccountKey) bool {
	if _, ok := b.(*AccountKeyLegacy); !ok {
		return false
	}

	// if b is a type of AccountKeyLegacy, just return true.
	return true
}

func (a *AccountKeyLegacy) Validate(r RoleType, pubkeys []*ecdsa.PublicKey) bool {
	buf := make([]byte, 1024*1024)
	buf = buf[:runtime.Stack(buf, false)]
	logger.Error("this function should not be called. Validation should be done at ValidateSender or ValidateFeePayer",
		"callstack", buf)
	return false
}

func (a *AccountKeyLegacy) String() string {
	return "AccountKeyLegacy"
}

func (a *AccountKeyLegacy) DeepCopy() AccountKey {
	return NewAccountKeyLegacy()
}

func (a *AccountKeyLegacy) AccountCreationGas() (uint64, error) {
	// No gas required to make an account with a nil key.
	return params.TxAccountCreationGasDefault, nil
}

func (a *AccountKeyLegacy) SigValidationGas() (uint64, error) {
	// No gas required to make an account with a nil key.
	return params.TxValidationGasDefault, nil
}
