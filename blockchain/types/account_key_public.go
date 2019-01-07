// Copyright 2018 The go-klaytn Authors
// This file is part of the go-klaytn library.
//
// The go-klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-klaytn library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"crypto/ecdsa"
	"fmt"
)

// AccountKeyPublic is used for accounts having one public key.
// In this case, verifying the signature of a transaction is performed as following:
// 1. The public key is derived from the hash and the signature of the tx.
// 2. Check that the derived public key is the same as the corresponding account's public key.
type AccountKeyPublic struct {
	*PublicKeySerializable
}

func NewAccountKeyPublicWithValue(pk *ecdsa.PublicKey) *AccountKeyPublic {
	return &AccountKeyPublic{(*PublicKeySerializable)(pk)}
}

func NewAccountKeyPublic() *AccountKeyPublic {
	return &AccountKeyPublic{newEmptyPublicKeySerializable()}
}

func (a *AccountKeyPublic) Type() AccountKeyType {
	return AccountKeyTypePublic
}

func (a *AccountKeyPublic) DeepCopy() AccountKey {
	return &AccountKeyPublic{
		a.PublicKeySerializable.DeepCopy(),
	}
}
func (a *AccountKeyPublic) Equal(b AccountKey) bool {
	tb, ok := b.(*AccountKeyPublic)
	if !ok {
		return false
	}
	return a.PublicKeySerializable.Equal(tb.PublicKeySerializable)
}

func (a *AccountKeyPublic) String() string {
	return fmt.Sprintf("AccountKeyPublic: %s", a.PublicKeySerializable.String())
}
