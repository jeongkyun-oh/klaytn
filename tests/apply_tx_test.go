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

package tests

import (
	"crypto/ecdsa"
	"fmt"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/blockchain/types/accountkey"
	"github.com/ground-x/klaytn/blockchain/vm"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/profile"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/params"
	"github.com/stretchr/testify/assert"
	"math/big"
	"os"
	"reflect"
	"runtime"
	"runtime/pprof"
	"strings"
	"testing"
	"time"
)

var benchName string

type genTx func(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction

// BenchmarkTxPerformanceCompatible compares performance between a legacy transaction and new transaction types.
// It compares the following:
// - legacy value transfer vs. new value transfer
// - legacy smart contract deploy vs. new smart contract deploy
func BenchmarkTxPerformanceCompatible(b *testing.B) {
	testfns := []genTx{
		genLegacyValueTransfer,
		genNewValueTransfer,
		genLegacySmartContractDeploy,
		genNewSmartContractDeploy,
	}

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		if strings.Contains(fnname, "New") {
			benchName = "New/" + strings.Split(fnname, "New")[1]
		} else {
			benchName = "Legacy/" + strings.Split(fnname, "Legacy")[1]
		}
		b.Run(benchName, func(b *testing.B) {
			benchmarkTxPerformanceCompatible(b, fn)
		})
	}
}

// BenchmarkTxPerformanceSmartContractExecution compares performance between a legacy transaction and new transaction types.
// This needs one more step "deploying a smart contract" compared to BenchmarkTxPerformanceCompatible.
// It compares the following:
// - legacy smart contract execution vs. new smart contract execution.
func BenchmarkTxPerformanceSmartContractExecution(b *testing.B) {
	testfns := []genTx{
		genLegacySmartContractExecution,
		genNewSmartContractExecution,
	}

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		if strings.Contains(fnname, "New") {
			benchName = "New/" + strings.Split(fnname, "New")[1]
		} else {
			benchName = "Legacy/" + strings.Split(fnname, "Legacy")[1]
		}
		b.Run(benchName, func(b *testing.B) {
			benchmarkTxPerformanceSmartContractExecution(b, fn)
		})
	}
}

// BenchmarkTxPerformanceNew measures performance of newly introduced transaction types.
// This requires one more step "account creation of a Klaytn account" compared to BenchmarkTxPerformanceCompatible.
func BenchmarkTxPerformanceNew(b *testing.B) {
	testfns := []genTx{
		genNewAccountCreation,
		genNewAccountCreationMultisig3,
		genNewAccountCreationRoleBasedSingle,
		genNewAccountCreationRoleBasedMultisig3,
		genNewAccountUpdateAccountKeyPublic,
		genNewFeeDelegatedValueTransfer,
		genNewFeeDelegatedValueTransferWithRatio,
		genNewCancel,
	}

	// sender account
	sender, err := createDecoupledAccount("c64f2cd1196e2a1791365b00c4bc07ab8f047b73152e4617c6ed06ac221a4b0c",
		common.HexToAddress("0x75c3098be5e4b63fbac05838daaee378dd48098d"))
	assert.Equal(b, nil, err)

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		if strings.Contains(fnname, "New") {
			benchName = "New/" + strings.Split(fnname, "New")[1]
		} else {
			benchName = "Legacy/" + strings.Split(fnname, "Legacy")[1]
		}
		b.Run(benchName, func(b *testing.B) {
			sender.Nonce = 0
			benchmarkTxPerformanceNew(b, fn, sender)
		})
	}
}

func BenchmarkTxPerformanceNewMultisig(b *testing.B) {
	testfns := []genTx{
		genNewAccountCreation,
		genNewAccountCreationMultisig3,
		genNewAccountCreationRoleBasedSingle,
		genNewAccountCreationRoleBasedMultisig3,
		genNewAccountUpdateAccountKeyPublic,
		genNewFeeDelegatedValueTransfer,
		genNewFeeDelegatedValueTransferWithRatio,
		genNewCancel,
	}

	// sender account
	sender, err := createMultisigAccount(uint(2),
		[]uint{1, 1, 1},
		[]string{"bb113e82881499a7a361e8354a5b68f6c6885c7bcba09ea2b0891480396c322e",
			"a5c9a50938a089618167c9d67dbebc0deaffc3c76ddc6b40c2777ae59438e989",
			"c32c471b732e2f56103e2f8e8cfd52792ef548f05f326e546a7d1fbf9d0419ec"},
		common.HexToAddress("0xbbfa38050bf3167c887c086758f448ce067ea8ea"))
	assert.Equal(b, nil, err)

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		if strings.Contains(fnname, "New") {
			benchName = "New/" + strings.Split(fnname, "New")[1]
		} else {
			benchName = "Legacy/" + strings.Split(fnname, "Legacy")[1]
		}
		b.Run(benchName, func(b *testing.B) {
			sender.Nonce = 0
			benchmarkTxPerformanceNew(b, fn, sender)
		})
	}
}

func BenchmarkTxPerformanceNewRoleBasedSingle(b *testing.B) {
	testfns := []genTx{
		genNewAccountCreation,
		genNewAccountCreationMultisig3,
		genNewAccountCreationRoleBasedSingle,
		genNewAccountCreationRoleBasedMultisig3,
		genNewAccountUpdateAccountKeyPublic,
		genNewFeeDelegatedValueTransfer,
		genNewFeeDelegatedValueTransferWithRatio,
		genNewCancel,
	}

	// sender account
	k1, err := crypto.HexToECDSA("98275a145bc1726eb0445433088f5f882f8a4a9499135239cfb4040e78991dab")
	if err != nil {
		panic(err)
	}
	pubkey := accountkey.NewAccountKeyPublicWithValue(&k1.PublicKey)
	sender := &TestAccountType{
		Addr:   common.HexToAddress("0x75c3098be5e4b63fbac05838daaee378dd48098d"),
		Keys:   []*ecdsa.PrivateKey{k1},
		Nonce:  uint64(0),
		AccKey: accountkey.NewAccountKeyRoleBasedWithValues(accountkey.AccountKeyRoleBased{pubkey, pubkey, pubkey}),
	}

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		if strings.Contains(fnname, "New") {
			benchName = "New/" + strings.Split(fnname, "New")[1]
		} else {
			benchName = "Legacy/" + strings.Split(fnname, "Legacy")[1]
		}
		b.Run(benchName, func(b *testing.B) {
			sender.Nonce = 0
			benchmarkTxPerformanceNew(b, fn, sender)
		})
	}
}

func BenchmarkTxPerformanceNewRoleBasedMultisig3(b *testing.B) {
	testfns := []genTx{
		genNewAccountCreation,
		genNewAccountCreationMultisig3,
		genNewAccountCreationRoleBasedSingle,
		genNewAccountCreationRoleBasedMultisig3,
		genNewAccountUpdateAccountKeyPublic,
		genNewFeeDelegatedValueTransfer,
		genNewFeeDelegatedValueTransferWithRatio,
		genNewCancel,
	}

	// sender account
	k1, err := crypto.HexToECDSA("98275a145bc1726eb0445433088f5f882f8a4a9499135239cfb4040e78991dab")
	if err != nil {
		panic(err)
	}
	k2, err := crypto.HexToECDSA("c64f2cd1196e2a1791365b00c4bc07ab8f047b73152e4617c6ed06ac221a4b0c")
	if err != nil {
		panic(err)
	}
	k3, err := crypto.HexToECDSA("ed580f5bd71a2ee4dae5cb43e331b7d0318596e561e6add7844271ed94156b20")
	if err != nil {
		panic(err)
	}

	keys := accountkey.WeightedPublicKeys{
		accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k1.PublicKey)),
		accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k2.PublicKey)),
		accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k3.PublicKey)),
	}
	threshold := uint(2)
	pubkey := accountkey.NewAccountKeyWeightedMultiSigWithValues(threshold, keys)
	sender := &TestAccountType{
		Addr:   common.HexToAddress("0x75c3098be5e4b63fbac05838daaee378dd48098d"),
		Keys:   []*ecdsa.PrivateKey{k1, k2, k3},
		Nonce:  uint64(0),
		AccKey: accountkey.NewAccountKeyRoleBasedWithValues(accountkey.AccountKeyRoleBased{pubkey, pubkey, pubkey}),
	}

	for _, fn := range testfns {
		fnname := getFunctionName(fn)
		fnname = fnname[strings.LastIndex(fnname, ".")+1:]
		if strings.Contains(fnname, "New") {
			benchName = "New/" + strings.Split(fnname, "New")[1]
		} else {
			benchName = "Legacy/" + strings.Split(fnname, "Legacy")[1]
		}
		b.Run(benchName, func(b *testing.B) {
			sender.Nonce = 0
			benchmarkTxPerformanceNew(b, fn, sender)
		})
	}
}

func benchmarkTxPerformanceCompatible(b *testing.B, genTx genTx) {
	if testing.Verbose() {
		enableLog()
	}

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		b.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		b.Fatal(err)
	}

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	// decoupled account
	decoupled, err := createDecoupledAccount("c64f2cd1196e2a1791365b00c4bc07ab8f047b73152e4617c6ed06ac221a4b0c",
		common.HexToAddress("0x75c3098be5e4b63fbac05838daaee378dd48098d"))
	assert.Equal(b, nil, err)

	colin, err := createHumanReadableAccount("ed580f5bd71a2ee4dae5cb43e331b7d0318596e561e6add7844271ed94156b20", "colin")
	assert.Equal(b, nil, err)

	if testing.Verbose() {
		fmt.Println("reservoirAddr = ", reservoir.Addr.String())
		fmt.Println("decoupledAddr = ", decoupled.Addr.String())
		fmt.Println("colinAddr = ", colin.Addr.String())
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// Prepare a next block header.
	author := bcdata.addrs[0]
	vmConfig := &vm.Config{
		JumpTable: vm.ConstantinopleInstructionSet,
	}
	parent := bcdata.bc.CurrentBlock()
	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		Extra:      parent.Extra(),
		Time:       new(big.Int).Add(parent.Time(), common.Big1),
	}
	if err := bcdata.engine.Prepare(bcdata.bc, header); err != nil {
		b.Fatal(err)
	}

	state, err := bcdata.bc.State()
	assert.Equal(b, nil, err)

	txs := make([]*types.Transaction, b.N)

	// Generate transactions.
	for i := 0; i < b.N; i++ {
		tx := genTx(signer, reservoir, colin)

		txs[i] = tx

		reservoir.Nonce += 1

		// execute this to cache ecrecover.
		tx.AsMessageWithAccountKeyPicker(signer, state, header.Number.Uint64())
	}

	if isProfileEnabled() {
		fname := strings.Replace(benchName, "/", ".", -1)
		f, err := os.Create(fname + ".cpu.out")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	b.ResetTimer()
	// Execute ApplyTransaction to measure performance of the given transaction type.
	for i := 0; i < b.N; i++ {
		usedGas := uint64(0)
		_, _, err = blockchain.ApplyTransaction(bcdata.bc.Config(), bcdata.bc, author, state, header, txs[i], &usedGas, vmConfig)
		assert.Equal(b, nil, err)
	}
	b.StopTimer()
}

func benchmarkTxPerformanceSmartContractExecution(b *testing.B, genTx genTx) {
	if testing.Verbose() {
		enableLog()
	}
	prof := profile.NewProfiler()

	// Initialize blockchain
	start := time.Now()
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		b.Fatal(err)
	}
	prof.Profile("main_init_blockchain", time.Now().Sub(start))
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	start = time.Now()
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		b.Fatal(err)
	}
	prof.Profile("main_init_accountMap", time.Now().Sub(start))

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	if testing.Verbose() {
		fmt.Println("reservoirAddr = ", reservoir.Addr.String())
	}

	contract, err := createHumanReadableAccount("ed34b0cf47a0021e9897760f0a904a69260c2f638e0bcc805facb745ec3ff9ab",
		"contract")
	assert.Equal(b, nil, err)

	gasPrice := new(big.Int).SetUint64(0)
	gasLimit := uint64(100000000000)

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)

	// Deploy smart contract (reservoir -> contract)
	{
		var txs types.Transactions

		code := "0x608060405234801561001057600080fd5b506101de806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631a39d8ef81146100805780636353586b146100a757806370a08231146100ca578063fd6b7ef8146100f8575b3360009081526001602052604081208054349081019091558154019055005b34801561008c57600080fd5b5061009561010d565b60408051918252519081900360200190f35b6100c873ffffffffffffffffffffffffffffffffffffffff60043516610113565b005b3480156100d657600080fd5b5061009573ffffffffffffffffffffffffffffffffffffffff60043516610147565b34801561010457600080fd5b506100c8610159565b60005481565b73ffffffffffffffffffffffffffffffffffffffff1660009081526001602052604081208054349081019091558154019055565b60016020526000908152604090205481565b336000908152600160205260408120805490829055908111156101af57604051339082156108fc029083906000818181858888f193505050501561019c576101af565b3360009081526001602052604090208190555b505600a165627a7a72305820627ca46bb09478a015762806cc00c431230501118c7c26c30ac58c4e09e51c4f0029"
		amount := new(big.Int).SetUint64(0)

		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.Nonce,
			types.TxValueKeyFrom:          reservoir.Addr,
			types.TxValueKeyTo:            contract.Addr,
			types.TxValueKeyAmount:        amount,
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      gasPrice,
			types.TxValueKeyHumanReadable: true,
			types.TxValueKeyData:          common.FromHex(code),
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, values)
		assert.Equal(b, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(b, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			b.Fatal(err)
		}
		reservoir.Nonce += 1
	}

	// Prepare a next block header.
	author := bcdata.addrs[0]
	vmConfig := &vm.Config{
		JumpTable: vm.ConstantinopleInstructionSet,
	}
	parent := bcdata.bc.CurrentBlock()
	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		Extra:      parent.Extra(),
		Time:       new(big.Int).Add(parent.Time(), common.Big1),
	}
	if err := bcdata.engine.Prepare(bcdata.bc, header); err != nil {
		b.Fatal(err)
	}

	state, err := bcdata.bc.State()
	assert.Equal(b, nil, err)

	txs := make([]*types.Transaction, b.N)

	// Generate transactions.
	for i := 0; i < b.N; i++ {
		tx := genTx(signer, reservoir, contract)

		txs[i] = tx

		reservoir.Nonce += 1

		tx.AsMessageWithAccountKeyPicker(signer, state, header.Number.Uint64())
	}

	if isProfileEnabled() {
		fname := strings.Replace(benchName, "/", ".", -1)
		f, err := os.Create(fname + ".cpu.out")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	b.ResetTimer()
	// Execute ApplyTransaction to measure performance of the given transaction type.
	for i := 0; i < b.N; i++ {
		usedGas := uint64(0)
		_, _, err = blockchain.ApplyTransaction(bcdata.bc.Config(), bcdata.bc, author, state, header, txs[i], &usedGas, vmConfig)
		assert.Equal(b, nil, err)
	}
	b.StopTimer()

	if testing.Verbose() {
		prof.PrintProfileInfo()
	}
}

func benchmarkTxPerformanceNew(b *testing.B, genTx genTx, sender *TestAccountType) {
	if testing.Verbose() {
		enableLog()
	}
	prof := profile.NewProfiler()

	// Initialize blockchain
	bcdata, err := NewBCData(6, 4)
	if err != nil {
		b.Fatal(err)
	}
	defer bcdata.Shutdown()

	// Initialize address-balance map for verification
	accountMap := NewAccountMap()
	if err := accountMap.Initialize(bcdata); err != nil {
		b.Fatal(err)
	}

	// reservoir account
	reservoir := &TestAccountType{
		Addr:  *bcdata.addrs[0],
		Keys:  []*ecdsa.PrivateKey{bcdata.privKeys[0]},
		Nonce: uint64(0),
	}

	colin, err := createHumanReadableAccount("ed580f5bd71a2ee4dae5cb43e331b7d0318596e561e6add7844271ed94156b20", "colin")
	assert.Equal(b, nil, err)

	if testing.Verbose() {
		fmt.Println("reservoirAddr = ", reservoir.Addr.String())
		fmt.Println("decoupledAddr = ", sender.Addr.String())
		fmt.Println("colinAddr = ", colin.Addr.String())
	}

	signer := types.NewEIP155Signer(bcdata.bc.Config().ChainID)
	gasPrice := new(big.Int).SetUint64(bcdata.bc.Config().UnitPrice)

	// Create an account sender using TxTypeAccountCreation.
	{
		var txs types.Transactions

		amount := new(big.Int).SetUint64(1000000000000)
		values := map[types.TxValueKeyType]interface{}{
			types.TxValueKeyNonce:         reservoir.Nonce,
			types.TxValueKeyFrom:          reservoir.Addr,
			types.TxValueKeyTo:            sender.Addr,
			types.TxValueKeyAmount:        amount,
			types.TxValueKeyGasLimit:      gasLimit,
			types.TxValueKeyGasPrice:      gasPrice,
			types.TxValueKeyHumanReadable: false,
			types.TxValueKeyAccountKey:    sender.AccKey,
		}
		tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, values)
		assert.Equal(b, nil, err)

		err = tx.SignWithKeys(signer, reservoir.Keys)
		assert.Equal(b, nil, err)

		txs = append(txs, tx)

		if err := bcdata.GenABlockWithTransactions(accountMap, txs, prof); err != nil {
			b.Fatal(err)
		}
		reservoir.Nonce += 1
	}

	// Prepare a next block header.
	author := bcdata.addrs[0]
	vmConfig := &vm.Config{
		JumpTable: vm.ConstantinopleInstructionSet,
	}
	parent := bcdata.bc.CurrentBlock()
	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		Extra:      parent.Extra(),
		Time:       new(big.Int).Add(parent.Time(), common.Big1),
	}
	if err := bcdata.engine.Prepare(bcdata.bc, header); err != nil {
		b.Fatal(err)
	}

	state, err := bcdata.bc.State()
	assert.Equal(b, nil, err)

	txs := make([]*types.Transaction, b.N)

	// Generate transactions.
	for i := 0; i < b.N; i++ {
		tx := genTx(signer, sender, colin)

		txs[i] = tx

		sender.Nonce += 1

		tx.AsMessageWithAccountKeyPicker(signer, state, header.Number.Uint64())
	}

	if isProfileEnabled() {
		fname := strings.Replace(benchName, "/", ".", -1)
		f, err := os.Create(fname + ".cpu.out")
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	b.ResetTimer()
	// Execute ApplyTransaction to measure performance of the given transaction type.
	for i := 0; i < b.N; i++ {
		usedGas := uint64(0)
		_, _, err = blockchain.ApplyTransaction(bcdata.bc.Config(), bcdata.bc, author, state, header, txs[i], &usedGas, vmConfig)
		assert.Equal(b, nil, err)
	}
	b.StopTimer()

	if testing.Verbose() {
		prof.PrintProfileInfo()
	}
}

func genLegacyValueTransfer(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx := types.NewTransaction(from.Nonce, to.Addr, amount, gasLimit, gasPrice, []byte{})
	err := tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genNewValueTransfer(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx, err := types.NewTransactionWithMap(types.TxTypeValueTransfer, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    from.Nonce,
		types.TxValueKeyTo:       to.Addr,
		types.TxValueKeyAmount:   amount,
		types.TxValueKeyGasLimit: gasLimit,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     from.Addr,
	})
	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genLegacySmartContractDeploy(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	data := common.Hex2Bytes("608060405234801561001057600080fd5b506101de806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631a39d8ef81146100805780636353586b146100a757806370a08231146100ca578063fd6b7ef8146100f8575b3360009081526001602052604081208054349081019091558154019055005b34801561008c57600080fd5b5061009561010d565b60408051918252519081900360200190f35b6100c873ffffffffffffffffffffffffffffffffffffffff60043516610113565b005b3480156100d657600080fd5b5061009573ffffffffffffffffffffffffffffffffffffffff60043516610147565b34801561010457600080fd5b506100c8610159565b60005481565b73ffffffffffffffffffffffffffffffffffffffff1660009081526001602052604081208054349081019091558154019055565b60016020526000908152604090205481565b336000908152600160205260408120805490829055908111156101af57604051339082156108fc029083906000818181858888f193505050501561019c576101af565b3360009081526001602052604090208190555b505600a165627a7a72305820627ca46bb09478a015762806cc00c431230501118c7c26c30ac58c4e09e51c4f0029")
	tx := types.NewContractCreation(from.Nonce, amount, gasLimit, gasPrice, data)
	err := tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genNewSmartContractDeploy(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractDeploy, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         from.Nonce,
		types.TxValueKeyAmount:        amount,
		types.TxValueKeyGasLimit:      gasLimit,
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyHumanReadable: true,
		types.TxValueKeyTo:            to.Addr,
		types.TxValueKeyFrom:          from.Addr,
		// The binary below is a compiled binary of contracts/reward/contract/KlaytnReward.sol.
		types.TxValueKeyData: common.Hex2Bytes("608060405234801561001057600080fd5b506101de806100206000396000f3006080604052600436106100615763ffffffff7c01000000000000000000000000000000000000000000000000000000006000350416631a39d8ef81146100805780636353586b146100a757806370a08231146100ca578063fd6b7ef8146100f8575b3360009081526001602052604081208054349081019091558154019055005b34801561008c57600080fd5b5061009561010d565b60408051918252519081900360200190f35b6100c873ffffffffffffffffffffffffffffffffffffffff60043516610113565b005b3480156100d657600080fd5b5061009573ffffffffffffffffffffffffffffffffffffffff60043516610147565b34801561010457600080fd5b506100c8610159565b60005481565b73ffffffffffffffffffffffffffffffffffffffff1660009081526001602052604081208054349081019091558154019055565b60016020526000908152604090205481565b336000908152600160205260408120805490829055908111156101af57604051339082156108fc029083906000818181858888f193505050501561019c576101af565b3360009081526001602052604090208190555b505600a165627a7a72305820627ca46bb09478a015762806cc00c431230501118c7c26c30ac58c4e09e51c4f0029"),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genNewAccountCreation(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	k, _ := crypto.GenerateKey()
	addr := common.BytesToAddress(genRandomHash().Bytes())
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         from.Nonce,
		types.TxValueKeyTo:            addr,
		types.TxValueKeyAmount:        amount,
		types.TxValueKeyGasLimit:      gasLimit,
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyFrom:          from.Addr,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyAccountKey:    accountkey.NewAccountKeyPublicWithValue(&k.PublicKey),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genAccountKeyWeightedMultisig() accountkey.AccountKey {
	threshold := uint(2)
	numKeys := 3
	keys := make(accountkey.WeightedPublicKeys, numKeys)

	for i := 0; i < numKeys; i++ {
		k, _ := crypto.GenerateKey()
		keys[i] = accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k.PublicKey))
	}

	return accountkey.NewAccountKeyWeightedMultiSigWithValues(threshold, keys)
}

func genNewAccountCreationMultisig3(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	addr := common.BytesToAddress(genRandomHash().Bytes())
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         from.Nonce,
		types.TxValueKeyTo:            addr,
		types.TxValueKeyAmount:        amount,
		types.TxValueKeyGasLimit:      gasLimit,
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyFrom:          from.Addr,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyAccountKey:    genAccountKeyWeightedMultisig(),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genAccountKeyRoleBasedSingle() accountkey.AccountKey {
	k1, err := crypto.HexToECDSA("98275a145bc1726eb0445433088f5f882f8a4a9499135239cfb4040e78991dab")
	if err != nil {
		panic(err)
	}
	txKey := accountkey.NewAccountKeyPublicWithValue(&k1.PublicKey)

	return accountkey.NewAccountKeyRoleBasedWithValues(accountkey.AccountKeyRoleBased{txKey, txKey, txKey})
}

func genNewAccountCreationRoleBasedSingle(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	addr := common.BytesToAddress(genRandomHash().Bytes())
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         from.Nonce,
		types.TxValueKeyTo:            addr,
		types.TxValueKeyAmount:        amount,
		types.TxValueKeyGasLimit:      gasLimit,
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyFrom:          from.Addr,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyAccountKey:    genAccountKeyRoleBasedSingle(),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genAccountKeyRoleBasedMultisig3() accountkey.AccountKey {
	threshold := uint(2)

	k1, err := crypto.HexToECDSA("98275a145bc1726eb0445433088f5f882f8a4a9499135239cfb4040e78991dab")
	if err != nil {
		panic(err)
	}
	k2, err := crypto.HexToECDSA("c64f2cd1196e2a1791365b00c4bc07ab8f047b73152e4617c6ed06ac221a4b0c")
	if err != nil {
		panic(err)
	}
	k3, err := crypto.HexToECDSA("ed580f5bd71a2ee4dae5cb43e331b7d0318596e561e6add7844271ed94156b20")
	if err != nil {
		panic(err)
	}

	keys := accountkey.WeightedPublicKeys{
		accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k1.PublicKey)),
		accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k2.PublicKey)),
		accountkey.NewWeightedPublicKey(1, (*accountkey.PublicKeySerializable)(&k3.PublicKey)),
	}
	txKey := accountkey.NewAccountKeyWeightedMultiSigWithValues(threshold, keys)

	return accountkey.NewAccountKeyRoleBasedWithValues(accountkey.AccountKeyRoleBased{txKey, txKey, txKey})
}

func genNewAccountCreationRoleBasedMultisig3(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	addr := common.BytesToAddress(genRandomHash().Bytes())
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountCreation, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:         from.Nonce,
		types.TxValueKeyTo:            addr,
		types.TxValueKeyAmount:        amount,
		types.TxValueKeyGasLimit:      gasLimit,
		types.TxValueKeyGasPrice:      gasPrice,
		types.TxValueKeyFrom:          from.Addr,
		types.TxValueKeyHumanReadable: false,
		types.TxValueKeyAccountKey:    genAccountKeyRoleBasedMultisig3(),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}
func genNewFeeDelegatedValueTransfer(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransfer, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    from.Nonce,
		types.TxValueKeyTo:       to.Addr,
		types.TxValueKeyAmount:   amount,
		types.TxValueKeyGasLimit: gasLimit,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     from.Addr,
		types.TxValueKeyFeePayer: from.Addr,
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	err = tx.SignFeePayerWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genNewFeeDelegatedValueTransferWithRatio(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx, err := types.NewTransactionWithMap(types.TxTypeFeeDelegatedValueTransferWithRatio, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:              from.Nonce,
		types.TxValueKeyTo:                 to.Addr,
		types.TxValueKeyAmount:             amount,
		types.TxValueKeyGasLimit:           gasLimit,
		types.TxValueKeyGasPrice:           gasPrice,
		types.TxValueKeyFrom:               from.Addr,
		types.TxValueKeyFeePayer:           from.Addr,
		types.TxValueKeyFeeRatioOfFeePayer: types.FeeRatio(30),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	err = tx.SignFeePayerWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genLegacySmartContractExecution(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	data := common.Hex2Bytes("6353586b000000000000000000000000bc5951f055a85f41a3b62fd6f68ab7de76d299b2")
	tx := types.NewTransaction(from.Nonce, to.Addr, amount, gasLimit, gasPrice, data)
	err := tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genNewSmartContractExecution(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	amount := big.NewInt(100)
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx, err := types.NewTransactionWithMap(types.TxTypeSmartContractExecution, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    from.Nonce,
		types.TxValueKeyTo:       to.Addr,
		types.TxValueKeyAmount:   amount,
		types.TxValueKeyGasLimit: gasLimit,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     from.Addr,
		// An abi-packed bytes calling "reward" of contracts/reward/contract/KlaytnReward.sol with an address "bc5951f055a85f41a3b62fd6f68ab7de76d299b2".
		types.TxValueKeyData: common.Hex2Bytes("6353586b000000000000000000000000bc5951f055a85f41a3b62fd6f68ab7de76d299b2"),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func genNewAccountUpdateAccountKeyPublic(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	k, _ := crypto.GenerateKey()
	tx, err := types.NewTransactionWithMap(types.TxTypeAccountUpdate, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:      from.Nonce,
		types.TxValueKeyGasLimit:   gasLimit,
		types.TxValueKeyGasPrice:   gasPrice,
		types.TxValueKeyFrom:       from.Addr,
		types.TxValueKeyAccountKey: accountkey.NewAccountKeyPublicWithValue(&k.PublicKey),
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	from.Keys = []*ecdsa.PrivateKey{k}
	from.AccKey = accountkey.NewAccountKeyPublicWithValue(&k.PublicKey)

	return tx
}

func genNewCancel(signer types.Signer, from *TestAccountType, to *TestAccountType) *types.Transaction {
	gasPrice := new(big.Int).SetUint64(25 * params.Ston)
	tx, err := types.NewTransactionWithMap(types.TxTypeCancel, map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:    from.Nonce,
		types.TxValueKeyGasLimit: gasLimit,
		types.TxValueKeyGasPrice: gasPrice,
		types.TxValueKeyFrom:     from.Addr,
	})

	if err != nil {
		panic(err)
	}

	err = tx.SignWithKeys(signer, from.Keys)
	if err != nil {
		panic(err)
	}

	return tx
}

func getFunctionName(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func isProfileEnabled() bool {
	return os.Getenv("PROFILE") != ""
}
