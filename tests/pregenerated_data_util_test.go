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
	"bufio"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/blockchain/vm"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/consensus/istanbul"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/governance"
	"github.com/ground-x/klaytn/node"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/storage/database"
	"github.com/ground-x/klaytn/work"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"io/ioutil"
	"math"
	"math/big"
	"os"
	"path"
	"strconv"
	"sync"

	istanbulBackend "github.com/ground-x/klaytn/consensus/istanbul/backend"
)

const (
	numValidatorsForTest = 4

	addressDirectory    = "addrs"
	privateKeyDirectory = "privatekeys"

	addressFilePrefix    = "addrs_"
	privateKeyFilePrefix = "privateKeys_"

	chainDataDir = "chaindata"
)

var totalTxs = 0

// getDataDirName returns a name of directory from the given parameters.
func getDataDirName(numFilesToGenerate int, ldbOption *opt.Options) string {
	dataDirectory := fmt.Sprintf("testdata%v", numFilesToGenerate)

	if ldbOption == nil {
		return dataDirectory
	}

	dataDirectory += fmt.Sprintf("NoSyncIs%s", strconv.FormatBool(ldbOption.NoSync))

	// Below codes can be used if necessary.
	//dataDirectory += fmt.Sprintf("_BlockCacheCapacity%vMB", ldbOption.BlockCacheCapacity / opt.MiB)
	//dataDirectory += fmt.Sprintf("_CompactionTableSize%vMB", ldbOption.CompactionTableSize / opt.MiB)
	//dataDirectory += fmt.Sprintf("_CompactionTableSizeMultiplier%v", int(ldbOption.CompactionTableSizeMultiplier))

	return dataDirectory
}

// writeToFile writes addresses and private keys to designated directories with given fileNum.
// Addresses are stored in a file like `addrs_0` and private keys are stored in a file like `privateKeys_0`.
func writeToFile(addrs []*common.Address, privKeys []*ecdsa.PrivateKey, fileNum int, dir string) {
	_ = os.Mkdir(path.Join(dir, addressDirectory), os.ModePerm)
	_ = os.Mkdir(path.Join(dir, privateKeyDirectory), os.ModePerm)

	addrsFile, err := os.Create(path.Join(dir, addressDirectory, addressFilePrefix+strconv.Itoa(fileNum)))
	if err != nil {
		panic(err)
	}

	privateKeysFile, err := os.Create(path.Join(dir, privateKeyDirectory, privateKeyFilePrefix+strconv.Itoa(fileNum)))
	if err != nil {
		panic(err)
	}

	wg := sync.WaitGroup{}

	wg.Add(2)

	syncSize := len(addrs) / 2

	go func() {
		for i, b := range addrs {
			addrsFile.WriteString(b.String() + "\n")
			if (i+1)%syncSize == 0 {
				addrsFile.Sync()
			}
		}

		addrsFile.Close()
		wg.Done()
	}()

	go func() {
		for i, key := range privKeys {
			privateKeysFile.WriteString(hex.EncodeToString(crypto.FromECDSA(key)) + "\n")
			if (i+1)%syncSize == 0 {
				privateKeysFile.Sync()
			}
		}

		privateKeysFile.Close()
		wg.Done()
	}()

	wg.Wait()
}

func readAddrsFromFile(dir string, num int) ([]*common.Address, error) {
	var addrs []*common.Address

	addrsFile, err := os.Open(path.Join(dir, addressDirectory, addressFilePrefix+strconv.Itoa(num)))
	if err != nil {
		return nil, err
	}

	defer addrsFile.Close()

	scanner := bufio.NewScanner(addrsFile)
	for scanner.Scan() {
		keyStr := scanner.Text()
		addr := common.HexToAddress(keyStr)
		addrs = append(addrs, &addr)
	}

	return addrs, nil
}

func readPrivateKeysFromFile(dir string, num int) ([]*ecdsa.PrivateKey, error) {
	var privKeys []*ecdsa.PrivateKey
	privateKeysFile, err := os.Open(path.Join(dir, privateKeyDirectory, privateKeyFilePrefix+strconv.Itoa(num)))
	if err != nil {
		return nil, err
	}

	defer privateKeysFile.Close()

	scanner := bufio.NewScanner(privateKeysFile)
	for scanner.Scan() {
		keyStr := scanner.Text()

		key, err := hex.DecodeString(keyStr)
		if err != nil {
			return nil, fmt.Errorf("%v", err)
		}

		if pk, err := crypto.ToECDSA(key); err != nil {
			return nil, fmt.Errorf("%v", err)
		} else {
			privKeys = append(privKeys, pk)
		}
	}

	return privKeys, nil
}

func readAddrsAndPrivateKeysFromFile(dir string, num int) ([]*common.Address, []*ecdsa.PrivateKey, error) {
	addrs, err := readAddrsFromFile(dir, num)
	if err != nil {
		return nil, nil, err
	}

	privateKeys, err := readPrivateKeysFromFile(dir, num)
	if err != nil {
		return nil, nil, err
	}

	return addrs, privateKeys, nil
}

// makeAddrsFromFile extracts the address stored in file by numAccounts.
func makeAddrsFromFile(numAccounts int, testDataDir string, indexPicker func(int, int) int) ([]*common.Address, error) {
	addrs := make([]*common.Address, 0, numAccounts)

	files, err := ioutil.ReadDir(path.Join(testDataDir, addressDirectory))
	if err != nil {
		return nil, err
	}

	numFiles := len(files)
	remain := numAccounts
	for i := 1; remain > 0; i++ {
		fileIndex := indexPicker(i, numFiles)

		fmt.Println("Read addresses", "fileIndex", fileIndex)
		// Read recipient addresses from file.
		addrsPerFile, err := readAddrsFromFile(testDataDir, fileIndex)

		if err != nil {
			return nil, err
		}

		partSize := int(math.Min(float64(len(addrsPerFile)), float64(remain)))
		addrs = append(addrs, addrsPerFile[:partSize]...)
		remain -= partSize
	}

	return addrs, nil
}

// makeAddrsAndPrivKeysFromFile extracts the address and private key stored in file by numAccounts.
func makeAddrsAndPrivKeysFromFile(numAccounts int, fileDir string) ([]*common.Address, []*ecdsa.PrivateKey, error) {
	addrs := make([]*common.Address, 0, numAccounts)
	privKeys := make([]*ecdsa.PrivateKey, 0, numAccounts)

	remain := numAccounts
	fileIndex := 0
	for remain > 0 {
		// Read addresses and private keys from file.
		addrsPerFile, privKeysPerFile, err := readAddrsAndPrivateKeysFromFile(fileDir, fileIndex)

		if err != nil {
			return nil, nil, err
		}

		partSize := int(math.Min(float64(len(addrsPerFile)), float64(remain)))
		addrs = append(addrs, addrsPerFile[:partSize]...)
		privKeys = append(privKeys, privKeysPerFile[:partSize]...)
		remain -= partSize
		fileIndex++
	}

	return addrs, privKeys, nil
}

// generateGovernaceDataForTest returns *governance.Governance for test.
func generateGovernaceDataForTest() *governance.Governance {
	return governance.NewGovernance(&params.ChainConfig{
		ChainID:       big.NewInt(2018),
		UnitPrice:     25000000000,
		DeriveShaImpl: 0,
		Istanbul: &params.IstanbulConfig{
			Epoch:          istanbul.DefaultConfig.Epoch,
			ProposerPolicy: uint64(istanbul.DefaultConfig.ProposerPolicy),
			SubGroupSize:   istanbul.DefaultConfig.SubGroupSize,
		},
		Governance: governance.GetDefaultGovernanceConfig(params.UseIstanbul),
	})
}

// generateDefaultLevelDBOption returns default LevelDB option for pre-generated tests.
func generateDefaultLevelDBOption() *opt.Options {
	return &opt.Options{WriteBuffer: 256 * opt.MiB, CompactionTableSize: 4 * opt.MiB, CompactionTableSizeMultiplier: 2}
}

// generateDefaultDBConfig returns default database.DBConfig for pre-generated tests.
func generateDefaultDBConfig() *database.DBConfig {
	return &database.DBConfig{Partitioned: true, ParallelDBWrite: true}
}

// getValidatorAddrsAndKeys returns the first `numValidators` addresses and private keys
// for validators.
func getValidatorAddrsAndKeys(addrs []*common.Address, privateKeys []*ecdsa.PrivateKey, numValidators int) ([]common.Address, []*ecdsa.PrivateKey) {
	validatorAddresses := make([]common.Address, numValidators)
	validatorPrivateKeys := make([]*ecdsa.PrivateKey, numValidators)

	for i := 0; i < numValidators; i++ {
		validatorPrivateKeys[i] = privateKeys[i]
		validatorAddresses[i] = *addrs[i]
	}

	return validatorAddresses, validatorPrivateKeys
}

// GenABlockWithTxPoolWithoutAccountMap basically does the same thing with GenABlockWithTxPool,
// however, it does not accept AccountMap which validates the outcome with stateDB.
// This is to remove the overhead of AccountMap management.
func (bcdata *BCData) GenABlockWithTxPoolWithoutAccountMap(txPool *blockchain.TxPool) error {
	signer := types.MakeSigner(bcdata.bc.Config(), bcdata.bc.CurrentHeader().Number)

	pending, err := txPool.Pending()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return errEmptyPending
	}

	// TODO-Klaytn-Issue136 gasPrice
	pooltxs := types.NewTransactionsByPriceAndNonce(signer, pending)

	// Set the block header
	header, err := bcdata.prepareHeader()
	if err != nil {
		return err
	}

	stateDB, err := bcdata.bc.TryGetCachedStateDB(bcdata.bc.CurrentBlock().Root())
	if err != nil {
		return err
	}

	gp := new(blockchain.GasPool)
	gp = gp.AddGas(GasLimit)
	task := work.NewTask(bcdata.bc.Config(), signer, stateDB, gp, header)
	task.ApplyTransactions(pooltxs, bcdata.bc, *bcdata.rewardBase)
	newtxs := task.Transactions()
	receipts := task.Receipts()

	if len(newtxs) == 0 {
		return errEmptyPending
	}

	// Finalize the block.
	b, err := bcdata.engine.Finalize(bcdata.bc, header, stateDB, newtxs, receipts)
	if err != nil {
		return err
	}

	// Seal the block.
	b, err = sealBlock(b, bcdata.validatorPrivKeys)
	if err != nil {
		return err
	}

	// Insert the block into the blockchain.
	if n, err := bcdata.bc.InsertChain(types.Blocks{b}); err != nil {
		return fmt.Errorf("err = %s, n = %d\n", err, n)
	}

	totalTxs += len(newtxs)
	fmt.Println("blockNum", b.NumberU64(), "numTxs", len(newtxs), "totalTxs", totalTxs)

	return nil
}

// NewBCDataForPreGeneratedTest returns a new BCData pointer constructed either 1) from the scratch or 2) from the existing data.
func NewBCDataForPreGeneratedTest(testDataDir string, tc *preGeneratedTC) (*BCData, error) {
	totalTxs = 0

	if numValidatorsForTest > tc.numTotalSenders {
		return nil, errors.New("numTotalSenders should be bigger numValidatorsForTest")
	}

	// Remove test data directory if 1) exists and and 2) generating test.
	if _, err := os.Stat(testDataDir); err == nil && tc.isGenerateTest {
		os.RemoveAll(testDataDir)
	}

	// Remove transactions.rlp if exists
	if _, err := os.Stat(transactionsJournalFilename); err == nil {
		os.RemoveAll(transactionsJournalFilename)
	}

	////////////////////////////////////////////////////////////////////////////////
	// Create a database
	tc.dbc.Dir = path.Join(testDataDir, chainDataDir)
	fmt.Println("DBDir", tc.dbc.Dir)
	chainDB, err := database.NewLevelDBManagerForTest(tc.dbc, tc.levelDBOption)
	if err != nil {
		return nil, err
	}

	////////////////////////////////////////////////////////////////////////////////
	// Create a governance
	gov := generateGovernaceDataForTest()

	////////////////////////////////////////////////////////////////////////////////
	// Prepare sender addresses and private keys
	// 1) If generating test, create accounts and private keys as many as numTotalSenders
	// 2) If executing test, load accounts and private keys from file as many as numTotalSenders
	var addrs []*common.Address
	var privKeys []*ecdsa.PrivateKey
	if tc.isGenerateTest {
		addrs, privKeys, err = createAccounts(tc.numTotalSenders)
		writeToFile(addrs, privKeys, 0, testDataDir)
	} else {
		addrs, privKeys, err = makeAddrsAndPrivKeysFromFile(tc.numTotalSenders, testDataDir)
	}

	if err != nil {
		return nil, err
	}

	////////////////////////////////////////////////////////////////////////////////
	// Set the genesis address
	genesisAddr := *addrs[0]

	////////////////////////////////////////////////////////////////////////////////
	// Use the first `numValidatorsForTest` accounts as validators
	validatorAddresses, validatorPrivKeys := getValidatorAddrsAndKeys(addrs, privKeys, numValidatorsForTest)

	////////////////////////////////////////////////////////////////////////////////
	// Setup istanbul consensus backend
	engine := istanbulBackend.New(genesisAddr, istanbul.DefaultConfig, validatorPrivKeys[0], chainDB, gov, node.CONSENSUSNODE)

	////////////////////////////////////////////////////////////////////////////////
	// Make a BlockChain
	// 1) If generating test, call initBlockChain
	// 2) If executing test, call blockchain.NewBlockChain
	var bc *blockchain.BlockChain
	if tc.isGenerateTest {
		bc, err = initBlockChain(chainDB, tc.cacheConfig, addrs, validatorAddresses, engine)
	} else {
		chainConfig, err := getChainConfig(chainDB)
		if err != nil {
			return nil, err
		}
		bc, err = blockchain.NewBlockChain(chainDB, tc.cacheConfig, chainConfig, engine, vm.Config{})
	}

	if err != nil {
		return nil, err
	}

	return &BCData{bc, addrs, privKeys, chainDB,
		&genesisAddr, validatorAddresses,
		validatorPrivKeys, engine}, nil
}

// getChainConfig returns chain config from chainDB.
func getChainConfig(chainDB database.DBManager) (*params.ChainConfig, error) {
	stored := chainDB.ReadBlockByNumber(0)
	if stored == nil {
		return nil, errors.New("chainDB.ReadBlockByNumber(0) == nil")
	}

	chainConfig := chainDB.ReadChainConfig(stored.Hash())
	if chainConfig == nil {
		return nil, errors.New("chainConfig == nil")
	}

	return chainConfig, nil
}
