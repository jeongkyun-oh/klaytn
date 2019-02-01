// Copyright 2018 The klaytn Authors
// Copyright 2014 The go-ethereum Authors
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
// This file is derived from core/types/block_test.go (2018/06/04).
// Modified and improved for the klaytn development.

package types

import (
	"bytes"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/ser/rlp"
	"math/big"
	"reflect"
	"testing"
)

func TestBlockEncoding(t *testing.T) {
	blockEnc := common.FromHex("0xf902d1f902cca033210b99543e4068b9f9825c052ccc493cf52fd3af9c53d0fefa8676e3548432a01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347940000000000000000000000000000000000000000940000000000000000000000000000000000000000a0f412a15cb6477bd1b0e48e8fc2d101292a5c1bb9c0b78f7a1129fea4f865fb97a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010185e8d4a50fff80845c540a2bb8c0d8820404846b6c617988676f312e31312e328664617277696e00000000000000f89ed594563e21ec3280cb0025be9ee9d7204443835132e2b841a493a8f4bc723d303cb80320b3f0736026fd22765aa15384c5b21479be48f7c6308de35a493181ac79aec32a19c2e299ea4028b73d1dc0f7fd906bfbf977aecd01f843b84143a272dadc1a9b1ce3744c23e2c2c4ac8f1b3e58911c097bc40e75fb93620c583acbf6546ac02baa0ac752051006eab4cdf310a5b6100027b023b15ac2017ee201a063746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365880000000000000000c0c0")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		t.Fatal("decode error: ", err)
	}

	header := block.header
	println(header.String())

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}

	// TODO-Klaytn Update some test code after replacement of Header by HeaderWithoutUncle.
	{
		// Comparing the hash from rlpHash() and HashWithUncle()
		resHashWithUncle := header.HashWithUncle()
		resRlpHash := rlpHash(header)

		check("rlp", resHashWithUncle, resRlpHash)

		// Comparing the hash from  HeaderWithoutUncle and HashNoUncle()
		fHeader := HeaderWithoutUncle{
			header.ParentHash,
			header.Coinbase,
			header.Rewardbase,
			header.Root,
			header.TxHash,
			header.ReceiptHash,
			header.Bloom,
			header.Difficulty,
			header.Number,
			header.GasLimit,
			header.GasUsed,
			header.Time,
			header.Extra,
			header.MixDigest,
			header.Nonce,
		}

		resHashWithoutUncle := rlpHash(fHeader)
		resCopiedBlockHeader := rlpHash(header.ToHeaderWithoutUncle())
		check("Hash", resHashWithoutUncle, block.header.HashNoUncle())
		check("Hash", resHashWithoutUncle, resCopiedBlockHeader)
	}

	// Check the field value of example block.
	check("Difficulty", block.Difficulty(), big.NewInt(1))
	check("GasLimit", block.GasLimit(), uint64(999999999999))
	check("GasUsed", block.GasUsed(), uint64(0))
	check("Coinbase", block.Coinbase(), common.HexToAddress("0000000000000000000000000000000000000000"))
	check("Rewardbase", block.Rewardbase(), common.HexToAddress("0000000000000000000000000000000000000000"))
	check("MixDigest", block.MixDigest(), common.HexToHash("63746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365"))
	check("Root", block.Root(), common.HexToHash("f412a15cb6477bd1b0e48e8fc2d101292a5c1bb9c0b78f7a1129fea4f865fb97"))
	check("Hash", block.Hash(), common.HexToHash("6e3826cd2407f01ceaad3cebc1235102001c0bb9a0f8c915ab2958303bc0972c"))
	check("Nonce", block.Nonce(), uint64(0000000000000000))
	check("Time", block.Time(), big.NewInt(1549011499))
	check("Size", block.Size(), common.StorageSize(len(blockEnc)))

	// TODO-Klaytn Consider to use new block with some transactions
	//tx1 := NewTransaction(0, common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"), big.NewInt(10), 50000, big.NewInt(10), nil)
	//
	//tx1, _ = tx1.WithSignature(HomesteadSigner{}, common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100"))
	//fmt.Println(block.Transactions()[0].Hash())
	//fmt.Println(tx1.data)
	//fmt.Println(tx1.Hash())
	//check("len(Transactions)", len(block.Transactions()), 1)
	//check("Transactions[0].Hash", block.Transactions()[0].Hash(), tx1.Hash())

	ourBlockEnc, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}
	if !bytes.Equal(ourBlockEnc, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", ourBlockEnc, blockEnc)
	}
}

func BenchmarkBlockEncodingHashWithUncle(b *testing.B) {
	blockEnc := common.FromHex("0xf902d1f902cca033210b99543e4068b9f9825c052ccc493cf52fd3af9c53d0fefa8676e3548432a01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347940000000000000000000000000000000000000000940000000000000000000000000000000000000000a0f412a15cb6477bd1b0e48e8fc2d101292a5c1bb9c0b78f7a1129fea4f865fb97a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010185e8d4a50fff80845c540a2bb8c0d8820404846b6c617988676f312e31312e328664617277696e00000000000000f89ed594563e21ec3280cb0025be9ee9d7204443835132e2b841a493a8f4bc723d303cb80320b3f0736026fd22765aa15384c5b21479be48f7c6308de35a493181ac79aec32a19c2e299ea4028b73d1dc0f7fd906bfbf977aecd01f843b84143a272dadc1a9b1ce3744c23e2c2c4ac8f1b3e58911c097bc40e75fb93620c583acbf6546ac02baa0ac752051006eab4cdf310a5b6100027b023b15ac2017ee201a063746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365880000000000000000c0c0")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		b.Fatal("decode error: ", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		block.header.HashWithUncle()
	}
}

func BenchmarkBlockEncodingRlpHash(b *testing.B) {
	blockEnc := common.FromHex("0xf902d1f902cca033210b99543e4068b9f9825c052ccc493cf52fd3af9c53d0fefa8676e3548432a01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347940000000000000000000000000000000000000000940000000000000000000000000000000000000000a0f412a15cb6477bd1b0e48e8fc2d101292a5c1bb9c0b78f7a1129fea4f865fb97a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010185e8d4a50fff80845c540a2bb8c0d8820404846b6c617988676f312e31312e328664617277696e00000000000000f89ed594563e21ec3280cb0025be9ee9d7204443835132e2b841a493a8f4bc723d303cb80320b3f0736026fd22765aa15384c5b21479be48f7c6308de35a493181ac79aec32a19c2e299ea4028b73d1dc0f7fd906bfbf977aecd01f843b84143a272dadc1a9b1ce3744c23e2c2c4ac8f1b3e58911c097bc40e75fb93620c583acbf6546ac02baa0ac752051006eab4cdf310a5b6100027b023b15ac2017ee201a063746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365880000000000000000c0c0")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		b.Fatal("decode error: ", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rlpHash(block.header)
	}
}

func BenchmarkBlockEncodingCopiedBlockHeader(b *testing.B) {
	blockEnc := common.FromHex("0xf902d1f902cca033210b99543e4068b9f9825c052ccc493cf52fd3af9c53d0fefa8676e3548432a01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347940000000000000000000000000000000000000000940000000000000000000000000000000000000000a0f412a15cb6477bd1b0e48e8fc2d101292a5c1bb9c0b78f7a1129fea4f865fb97a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421a056e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421b9010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010185e8d4a50fff80845c540a2bb8c0d8820404846b6c617988676f312e31312e328664617277696e00000000000000f89ed594563e21ec3280cb0025be9ee9d7204443835132e2b841a493a8f4bc723d303cb80320b3f0736026fd22765aa15384c5b21479be48f7c6308de35a493181ac79aec32a19c2e299ea4028b73d1dc0f7fd906bfbf977aecd01f843b84143a272dadc1a9b1ce3744c23e2c2c4ac8f1b3e58911c097bc40e75fb93620c583acbf6546ac02baa0ac752051006eab4cdf310a5b6100027b023b15ac2017ee201a063746963616c2062797a616e74696e65206661756c7420746f6c6572616e6365880000000000000000c0c0")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		b.Fatal("decode error: ", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rlpHash(block.header.ToHeaderWithoutUncle())
	}
}

// TODO-Klaytn-FailedTest Test fails. Analyze and enable it later.
/*
// from bcValidBlockTest.json, "SimpleTx"
func TestBlockEncoding(t *testing.T) {
	blockEnc := common.FromHex("f90260f901f9a083cafc574e1f51ba9dc0568fc617a08ea2429fb384059c972f13b19fa1c8dd55a01dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347948888f1f195afa192cfee860698584c030f4c9db1a0ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017a05fe50b260da6308036625b850b5d6ced6d0a9f814c0688bc91ffb7b7a3a54b67a0bc37d79753ad738a6dac4921e57392f145d8887476de3f783dfa7edae9283e52b90100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008302000001832fefd8825208845506eb0780a0bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff49888a13a5a8c8f2bb1c4f861f85f800a82c35094095e7baea6a6c7c4c2dfeb977efac326af552d870a801ba09bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094fa08a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b1c0")
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	check("Difficulty", block.Difficulty(), big.NewInt(131072))
	check("GasLimit", block.GasLimit(), uint64(3141592))
	check("GasUsed", block.GasUsed(), uint64(21000))
	check("Coinbase", block.Coinbase(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	check("MixDigest", block.MixDigest(), common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"))
	check("Root", block.Root(), common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"))
	check("Hash", block.Hash(), common.HexToHash("0a5843ac1cb04865017cb35a57b50b07084e5fcee39b5acadade33149f4fff9e"))
	check("Nonce", block.Nonce(), uint64(0xa13a5a8c8f2bb1c4))
	check("Time", block.Time(), big.NewInt(1426516743))
	check("Size", block.Size(), common.StorageSize(len(blockEnc)))

	tx1 := NewTransaction(0, common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"), big.NewInt(10), 50000, big.NewInt(10), nil)

	tx1, _ = tx1.WithSignature(HomesteadSigner{}, common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100"))
	fmt.Println(block.Transactions()[0].Hash())
	fmt.Println(tx1.data)
	fmt.Println(tx1.Hash())
	check("len(Transactions)", len(block.Transactions()), 1)
	check("Transactions[0].Hash", block.Transactions()[0].Hash(), tx1.Hash())

	ourBlockEnc, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}
	if !bytes.Equal(ourBlockEnc, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", ourBlockEnc, blockEnc)
	}
}
*/
