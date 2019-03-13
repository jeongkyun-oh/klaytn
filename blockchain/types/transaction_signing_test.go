// Copyright 2018 The klaytn Authors
// Copyright 2016 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.
//
// This file is derived from core/types/transaction_signing_test.go (2018/06/04).
// Modified and improved for the klaytn development.

package types

import (
	"crypto/ecdsa"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/stretchr/testify/assert"
	"math/big"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/ser/rlp"
)

func TestEIP155Signing(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	signer := NewEIP155Signer(big.NewInt(18))
	tx, err := SignTx(NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil), signer, key)
	if err != nil {
		t.Fatal(err)
	}

	from, err := Sender(signer, tx)
	if err != nil {
		t.Fatal(err)
	}
	if from != addr {
		t.Errorf("exected from and address to be equal. Got %x want %x", from, addr)
	}
}

func TestEIP155RawSignatureValues(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	signer := NewEIP155Signer(big.NewInt(18))
	tx, err := SignTx(NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil), signer, key)
	if err != nil {
		t.Fatal(err)
	}

	h := signer.Hash(tx)
	sig, err := crypto.Sign(h[:], key)
	if err != nil {
		t.Fatal(err)
	}
	r, s, v, err := signer.SignatureValues(sig)

	sigs := tx.RawSignatureValues()
	txV, txR, txS := sigs[0], sigs[1], sigs[2]

	assert.Equal(t, r, txR)
	assert.Equal(t, s, txS)
	assert.Equal(t, v, txV)
}

func TestEIP155ChainId(t *testing.T) {
	key, _ := crypto.GenerateKey()
	addr := crypto.PubkeyToAddress(key.PublicKey)

	signer := NewEIP155Signer(big.NewInt(18))
	tx, err := SignTx(NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil), signer, key)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.Protected() {
		t.Fatal("expected tx to be protected")
	}

	if tx.ChainId().Cmp(signer.chainId) != 0 {
		t.Error("expected chainId to be", signer.chainId, "got", tx.ChainId())
	}

	tx = NewTransaction(0, addr, new(big.Int), 0, new(big.Int), nil)
	tx, err = SignTx(tx, HomesteadSigner{}, key)
	if err != nil {
		t.Fatal(err)
	}

	if tx.Protected() {
		t.Error("didn't expect tx to be protected")
	}

	if tx.ChainId().Sign() != 0 {
		t.Error("expected chain id to be 0 got", tx.ChainId())
	}
}

func TestEIP155SigningVitalik(t *testing.T) {
	// Test vectors come from http://vitalik.ca/files/eip155_testvec.txt
	for i, test := range []struct {
		txRlp, addr string
	}{
		{"f864808504a817c800825208943535353535353535353535353535353535353535808025a0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116da0044852b2a670ade5407e78fb2863c51de9fcb96542a07186fe3aeda6bb8a116d", "0xf0f6f18bca1b28cd68e4357452947e021241e9ce"},
		{"f864018504a817c80182a410943535353535353535353535353535353535353535018025a0489efdaa54c0f20c7adf612882df0950f5a951637e0307cdcb4c672f298b8bcaa0489efdaa54c0f20c7adf612882df0950f5a951637e0307cdcb4c672f298b8bc6", "0x23ef145a395ea3fa3deb533b8a9e1b4c6c25d112"},
		{"f864028504a817c80282f618943535353535353535353535353535353535353535088025a02d7c5bef027816a800da1736444fb58a807ef4c9603b7848673f7e3a68eb14a5a02d7c5bef027816a800da1736444fb58a807ef4c9603b7848673f7e3a68eb14a5", "0x2e485e0c23b4c3c542628a5f672eeab0ad4888be"},
		{"f865038504a817c803830148209435353535353535353535353535353535353535351b8025a02a80e1ef1d7842f27f2e6be0972bb708b9a135c38860dbe73c27c3486c34f4e0a02a80e1ef1d7842f27f2e6be0972bb708b9a135c38860dbe73c27c3486c34f4de", "0x82a88539669a3fd524d669e858935de5e5410cf0"},
		{"f865048504a817c80483019a28943535353535353535353535353535353535353535408025a013600b294191fc92924bb3ce4b969c1e7e2bab8f4c93c3fc6d0a51733df3c063a013600b294191fc92924bb3ce4b969c1e7e2bab8f4c93c3fc6d0a51733df3c060", "0xf9358f2538fd5ccfeb848b64a96b743fcc930554"},
		{"f865058504a817c8058301ec309435353535353535353535353535353535353535357d8025a04eebf77a833b30520287ddd9478ff51abbdffa30aa90a8d655dba0e8a79ce0c1a04eebf77a833b30520287ddd9478ff51abbdffa30aa90a8d655dba0e8a79ce0c1", "0xa8f7aba377317440bc5b26198a363ad22af1f3a4"},
		{"f866068504a817c80683023e3894353535353535353535353535353535353535353581d88025a06455bf8ea6e7463a1046a0b52804526e119b4bf5136279614e0b1e8e296a4e2fa06455bf8ea6e7463a1046a0b52804526e119b4bf5136279614e0b1e8e296a4e2d", "0xf1f571dc362a0e5b2696b8e775f8491d3e50de35"},
		{"f867078504a817c807830290409435353535353535353535353535353535353535358201578025a052f1a9b320cab38e5da8a8f97989383aab0a49165fc91c737310e4f7e9821021a052f1a9b320cab38e5da8a8f97989383aab0a49165fc91c737310e4f7e9821021", "0xd37922162ab7cea97c97a87551ed02c9a38b7332"},
		{"f867088504a817c8088302e2489435353535353535353535353535353535353535358202008025a064b1702d9298fee62dfeccc57d322a463ad55ca201256d01f62b45b2e1c21c12a064b1702d9298fee62dfeccc57d322a463ad55ca201256d01f62b45b2e1c21c10", "0x9bddad43f934d313c2b79ca28a432dd2b7281029"},
		{"f867098504a817c809830334509435353535353535353535353535353535353535358202d98025a052f8f61201b2b11a78d6e866abc9c3db2ae8631fa656bfe5cb53668255367afba052f8f61201b2b11a78d6e866abc9c3db2ae8631fa656bfe5cb53668255367afb", "0x3c24d7329e92f84f08556ceb6df1cdb0104ca49f"},
	} {
		signer := NewEIP155Signer(big.NewInt(1))

		var tx *Transaction
		err := rlp.DecodeBytes(common.Hex2Bytes(test.txRlp), &tx)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}

		from, err := Sender(signer, tx)
		if err != nil {
			t.Errorf("%d: %v", i, err)
			continue
		}

		addr := common.HexToAddress(test.addr)
		if from != addr {
			t.Errorf("%d: expected %x got %x", i, addr, from)
		}

	}
}

func TestChainId(t *testing.T) {
	key, _ := defaultTestKey()

	tx := NewTransaction(0, common.Address{}, new(big.Int), 0, new(big.Int), nil)

	var err error
	tx, err = SignTx(tx, NewEIP155Signer(big.NewInt(1)), key)
	if err != nil {
		t.Fatal(err)
	}

	_, err = Sender(NewEIP155Signer(big.NewInt(2)), tx)
	if err != ErrInvalidChainId {
		t.Error("expected error:", ErrInvalidChainId)
	}

	_, err = Sender(NewEIP155Signer(big.NewInt(1)), tx)
	if err != nil {
		t.Error("expected no error")
	}
}

// AccountKeyPickerForTest is an temporary object for testing.
// It simulates GetKey() instead of using `StateDB` directly in order to avoid cycle imports.
type AccountKeyPickerForTest struct {
	AddrKeyMap map[common.Address]accountkey.AccountKey
}

func (a *AccountKeyPickerForTest) GetKey(addr common.Address) accountkey.AccountKey {
	return a.AddrKeyMap[addr]
}

func (a *AccountKeyPickerForTest) SetKey(addr common.Address, key accountkey.AccountKey) {
	a.AddrKeyMap[addr] = key
}

type testTx func(t *testing.T)

// TestValidateTransaction tests validation process of transactions.
// To check that each tx makes a msg for the signature well,
// this test generates the msg manually and then performs validation process.
func TestValidateTransaction(t *testing.T) {
	testfns := []testTx{
		testValidateValueTransfer,
		testValidateFeeDelegatedValueTransfer,
	}

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		t.Run(fnname, func(t *testing.T) {
			t.Parallel()
			fn(t)
		})
	}
}

func testValidateValueTransfer(t *testing.T) {
	// Transaction generation
	internalTx := genValueTransferTransaction().(*TxInternalDataValueTransfer)
	tx := &Transaction{data: internalTx}

	chainid := big.NewInt(1)
	signer := NewEIP155Signer(chainid)

	prv, from := defaultTestKey()
	internalTx.From = from

	// Sign
	// encode([ encode([type, nonce, gasPrice, gas, to, value, from]), chainid, 0, 0 ])
	b, err := rlp.EncodeToBytes([]interface{}{
		internalTx.Type(),
		internalTx.AccountNonce,
		internalTx.Price,
		internalTx.GasLimit,
		internalTx.Recipient,
		internalTx.Amount,
		internalTx.From,
	})
	assert.Equal(t, nil, err)

	h := rlpHash([]interface{}{
		b,
		chainid,
		uint(0),
		uint(0),
	})

	sig, err := NewTxSignaturesWithValues(signer, h, []*ecdsa.PrivateKey{prv})
	assert.Equal(t, nil, err)

	tx.SetSignature(sig)

	// AccountKeyPicker initialization
	p := &AccountKeyPickerForTest{
		AddrKeyMap: make(map[common.Address]accountkey.AccountKey),
	}
	key := accountkey.NewAccountKeyPublicWithValue(&prv.PublicKey)
	p.SetKey(from, key)

	// Validate
	validatedFrom, _, err := ValidateSender(signer, tx, p)
	assert.Equal(t, nil, err)
	assert.Equal(t, from, validatedFrom)
}

func testValidateFeeDelegatedValueTransfer(t *testing.T) {
	// Transaction generation
	internalTx := genFeeDelegatedValueTransferTransaction().(*TxInternalDataFeeDelegatedValueTransfer)
	tx := &Transaction{data: internalTx}

	chainid := big.NewInt(1)
	signer := NewEIP155Signer(chainid)

	prv, from := defaultTestKey()
	internalTx.From = from

	feePayerPrv, err := crypto.HexToECDSA("b9d5558443585bca6f225b935950e3f6e69f9da8a5809a83f51c3365dff53936")
	assert.Equal(t, nil, err)
	feePayer := crypto.PubkeyToAddress(feePayerPrv.PublicKey)
	internalTx.FeePayer = feePayer

	// Sign
	// encode([ encode([type, nonce, gasPrice, gas, to, value, from]), chainid, 0, 0 ])
	b, err := rlp.EncodeToBytes([]interface{}{
		internalTx.Type(),
		internalTx.AccountNonce,
		internalTx.Price,
		internalTx.GasLimit,
		internalTx.Recipient,
		internalTx.Amount,
		internalTx.From,
	})
	assert.Equal(t, nil, err)

	h := rlpHash([]interface{}{
		b,
		chainid,
		uint(0),
		uint(0),
	})

	sig, err := NewTxSignaturesWithValues(signer, h, []*ecdsa.PrivateKey{prv})
	assert.Equal(t, nil, err)

	tx.SetSignature(sig)

	// Sign fee payer
	// encode([ encode([type, nonce, gasPrice, gas, to, value, from]), feePayer, chainid, 0, 0 ])
	b, err = rlp.EncodeToBytes([]interface{}{
		internalTx.Type(),
		internalTx.AccountNonce,
		internalTx.Price,
		internalTx.GasLimit,
		internalTx.Recipient,
		internalTx.Amount,
		internalTx.From,
	})
	assert.Equal(t, nil, err)

	h = rlpHash([]interface{}{
		b,
		feePayer,
		chainid,
		uint(0),
		uint(0),
	})

	feePayerSig, err := NewTxSignaturesWithValues(signer, h, []*ecdsa.PrivateKey{feePayerPrv})
	assert.Equal(t, nil, err)

	tx.SetFeePayerSignature(feePayerSig)

	// AccountKeyPicker initialization
	p := &AccountKeyPickerForTest{
		AddrKeyMap: make(map[common.Address]accountkey.AccountKey),
	}
	key := accountkey.NewAccountKeyPublicWithValue(&prv.PublicKey)
	p.SetKey(from, key)
	feePayerKey := accountkey.NewAccountKeyPublicWithValue(&feePayerPrv.PublicKey)
	p.SetKey(feePayer, feePayerKey)

	// Validate
	validatedFrom, _, err := ValidateSender(signer, tx, p)
	assert.Equal(t, nil, err)
	assert.Equal(t, from, validatedFrom)

	// Validate fee payer
	validatedFeePayer, _, err := ValidateFeePayer(signer, tx, p)
	assert.Equal(t, nil, err)
	assert.Equal(t, feePayer, validatedFeePayer)
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}
