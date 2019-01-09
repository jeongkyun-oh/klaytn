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
	"encoding/json"
	"github.com/ground-x/go-gxplatform/ser/rlp"
	"io"
)

type AccountKeySerializer struct {
	keyType AccountKeyType
	key     AccountKey
}

type accountKeyJSON struct {
	KeyType AccountKeyType
	Key     json.RawMessage
}

func NewAccountKeySerializerWithAccountKey(k AccountKey) *AccountKeySerializer {
	return &AccountKeySerializer{k.Type(), k}
}

func NewAccountKeySerializer() *AccountKeySerializer {
	return &AccountKeySerializer{}
}

func (serializer *AccountKeySerializer) GetKey() AccountKey {
	return serializer.key
}

func (serializer *AccountKeySerializer) EncodeRLP(w io.Writer) error {
	if err := rlp.Encode(w, serializer.keyType); err != nil {
		return err
	}

	return rlp.Encode(w, serializer.key)
}

func (serializer *AccountKeySerializer) DecodeRLP(s *rlp.Stream) error {
	if err := s.Decode(&serializer.keyType); err != nil {
		return err
	}

	var err error
	serializer.key, err = NewAccountKey(serializer.keyType)
	if err != nil {
		return err
	}

	return s.Decode(serializer.key)
}

func (serializer *AccountKeySerializer) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(serializer.key)
	if err != nil {
		return nil, err
	}

	return json.Marshal(&accountKeyJSON{serializer.keyType, b})
}

func (serializer *AccountKeySerializer) UnmarshalJSON(b []byte) error {
	var keyJSON accountKeyJSON

	if err := json.Unmarshal(b, &keyJSON); err != nil {
		return err
	}

	serializer.keyType = keyJSON.KeyType

	var err error
	serializer.key, err = NewAccountKey(serializer.keyType)
	if err != nil {
		return err
	}

	return json.Unmarshal(keyJSON.Key, serializer.key)
}
