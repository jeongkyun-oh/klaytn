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
package tests

import (
	"bytes"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/blockchain/vm"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/profile"
	"github.com/ground-x/klaytn/consensus"
	"github.com/ground-x/klaytn/consensus/istanbul"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/crypto/sha3"
	"github.com/ground-x/klaytn/node"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"github.com/ground-x/klaytn/storage/database"
	"github.com/ground-x/klaytn/work"
	"math/big"
	"os"
	"time"

	istanbulBackend "github.com/ground-x/klaytn/consensus/istanbul/backend"
	istanbulCore "github.com/ground-x/klaytn/consensus/istanbul/core"
)

const transactionsJournalFilename = "transactions.rlp"

// If you don't want to remove 'chaindata', set removeChaindataOnExit = false
const removeChaindataOnExit = true

const GasLimit uint64 = 1000000000000000000

var (
	errEmptyPending = errors.New("pending is empty")
)

type BCData struct {
	bc                 *blockchain.BlockChain
	addrs              []*common.Address
	privKeys           []*ecdsa.PrivateKey
	db                 database.DBManager
	rewardBase         *common.Address
	validatorAddresses []common.Address
	validatorPrivKeys  []*ecdsa.PrivateKey
	engine             consensus.Istanbul
}

var dir = "chaindata"

func NewBCData(maxAccounts, numValidators int) (*BCData, error) {
	conf := node.DefaultConfig

	// Remove leveldb dir if exists
	if _, err := os.Stat(dir); err == nil {
		os.RemoveAll(dir)
	}

	// Remove transactions.rlp if exists
	if _, err := os.Stat(transactionsJournalFilename); err == nil {
		os.RemoveAll(transactionsJournalFilename)
	}

	////////////////////////////////////////////////////////////////////////////////
	// Create a database
	chainDb, err := NewDatabase(dir, database.LEVELDB)
	if err != nil {
		return nil, err
	}
	////////////////////////////////////////////////////////////////////////////////
	// Create accounts as many as maxAccounts
	if numValidators > maxAccounts {
		return nil, errors.New("maxAccounts should be bigger numValidators!!")
	}
	addrs, privKeys, err := createAccounts(maxAccounts)
	if err != nil {
		return nil, err
	}

	////////////////////////////////////////////////////////////////////////////////
	// Set the genesis address
	genesisAddr := *addrs[0]

	////////////////////////////////////////////////////////////////////////////////
	// Use first 4 accounts as vaildators
	validatorPrivKeys := make([]*ecdsa.PrivateKey, numValidators)
	validatorAddresses := make([]common.Address, numValidators)
	for i := 0; i < numValidators; i++ {
		validatorPrivKeys[i] = privKeys[i]
		validatorAddresses[i] = *addrs[i]
	}

	////////////////////////////////////////////////////////////////////////////////
	// Setup istanbul consensus backend
	engine := istanbulBackend.New(genesisAddr, genesisAddr, istanbul.DefaultConfig, validatorPrivKeys[0], chainDb)

	////////////////////////////////////////////////////////////////////////////////
	// Make a blockchain
	bc, err := initBlockchain(&conf, chainDb, addrs, validatorAddresses, engine)
	if err != nil {
		return nil, err
	}

	return &BCData{bc, addrs, privKeys, chainDb,
		&genesisAddr, validatorAddresses,
		validatorPrivKeys, engine}, nil
}

func (bcdata *BCData) Shutdown() {
	bcdata.bc.Stop()

	bcdata.db.Close()
	// Remove leveldb dir which was created for this test.
	if removeChaindataOnExit {
		os.RemoveAll(dir)
		os.RemoveAll(transactionsJournalFilename)
	}
}

func (bcdata *BCData) prepareHeader() (*types.Header, error) {
	tstart := time.Now()
	parent := bcdata.bc.CurrentBlock()

	tstamp := tstart.Unix()
	if parent.Time().Cmp(new(big.Int).SetInt64(tstamp)) >= 0 {
		tstamp = parent.Time().Int64() + 1
	}
	// this will ensure we're not going off too far in the future
	if now := time.Now().Unix(); tstamp > now {
		wait := time.Duration(tstamp-now) * time.Second
		time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Coinbase:   common.Address{},
		Number:     num.Add(num, common.Big1),
		GasLimit:   blockchain.CalcGasLimit(parent),
		Time:       big.NewInt(tstamp),
	}

	if err := bcdata.engine.Prepare(bcdata.bc, header); err != nil {
		return nil, errors.New(fmt.Sprintf("Failed to prepare header for mining %s.\n", err))
	}

	return header, nil
}

func (bcdata *BCData) MineABlock(transactions types.Transactions, signer types.Signer, prof *profile.Profiler) (*types.Block, error) {
	// Set the block header
	start := time.Now()
	header, err := bcdata.prepareHeader()
	if err != nil {
		return nil, err
	}
	prof.Profile("mine_prepareHeader", time.Now().Sub(start))

	statedb, err := bcdata.bc.State()
	if err != nil {
		return nil, err
	}

	// Group transactions by the sender address
	start = time.Now()
	txs := make(map[common.Address]types.Transactions)
	for _, tx := range transactions {
		acc, err := types.Sender(signer, tx)
		if err != nil {
			return nil, err
		}
		txs[acc] = append(txs[acc], tx)
	}
	prof.Profile("mine_groupTransactions", time.Now().Sub(start))

	// Create a transaction set where transactions are sorted by price and nonce
	start = time.Now()
	txset := types.NewTransactionsByPriceAndNonce(signer, txs) // TODO-GX-issue136 gasPrice
	prof.Profile("mine_NewTransactionsByPriceAndNonce", time.Now().Sub(start))

	// Apply the set of transactions
	start = time.Now()
	gp := new(blockchain.GasPool)
	gp = gp.AddGas(GasLimit)
	task := work.NewTask(bcdata.bc.Config(), signer, statedb, gp, header)
	task.ApplyTransactions(txset, bcdata.bc, *bcdata.rewardBase)
	newtxs := task.Transactions()
	receipts := task.Receipts()
	prof.Profile("mine_ApplyTransactions", time.Now().Sub(start))

	// Finalize the block
	start = time.Now()
	b, err := bcdata.engine.Finalize(bcdata.bc, header, statedb, newtxs, []*types.Header{}, receipts)
	if err != nil {
		return nil, err
	}
	prof.Profile("mine_finalize_block", time.Now().Sub(start))

	////////////////////////////////////////////////////////////////////////////////

	start = time.Now()
	b, err = sealBlock(b, bcdata.validatorPrivKeys)
	if err != nil {
		return nil, err
	}
	prof.Profile("mine_seal_block", time.Now().Sub(start))

	return b, nil
}

func (bcdata *BCData) GenABlock(accountMap *AccountMap, opt *testOption,
	numTransactions int, prof *profile.Profiler) error {
	// Make a set of transactions
	start := time.Now()
	signer := types.MakeSigner(bcdata.bc.Config(), bcdata.bc.CurrentHeader().Number)
	transactions, err := opt.makeTransactions(bcdata, accountMap, signer, numTransactions, nil, opt.txdata)
	if err != nil {
		return err
	}
	prof.Profile("main_makeTransactions", time.Now().Sub(start))

	return bcdata.GenABlockWithTransactions(accountMap, transactions, prof)
}

func (bcdata *BCData) GenABlockWithTxpool(accountMap *AccountMap, txpool *blockchain.TxPool,
	prof *profile.Profiler) error {
	signer := types.MakeSigner(bcdata.bc.Config(), bcdata.bc.CurrentHeader().Number)

	pending, err := txpool.Pending()
	if err != nil {
		return err
	}
	if len(pending) == 0 {
		return errEmptyPending
	}
	pooltxs := types.NewTransactionsByPriceAndNonce(signer, pending) // TODO-GX-issue136 gasPrice

	// Set the block header
	start := time.Now()
	header, err := bcdata.prepareHeader()
	if err != nil {
		return err
	}
	prof.Profile("mine_prepareHeader", time.Now().Sub(start))

	statedb, err := bcdata.bc.State()
	if err != nil {
		return err
	}

	start = time.Now()
	gp := new(blockchain.GasPool)
	gp = gp.AddGas(GasLimit)
	task := work.NewTask(bcdata.bc.Config(), signer, statedb, gp, header)
	task.ApplyTransactions(pooltxs, bcdata.bc, *bcdata.rewardBase)
	newtxs := task.Transactions()
	receipts := task.Receipts()
	prof.Profile("mine_ApplyTransactions", time.Now().Sub(start))

	// Finalize the block
	start = time.Now()
	b, err := bcdata.engine.Finalize(bcdata.bc, header, statedb, newtxs, []*types.Header{}, receipts)
	if err != nil {
		return err
	}
	prof.Profile("mine_finalize_block", time.Now().Sub(start))

	start = time.Now()
	b, err = sealBlock(b, bcdata.validatorPrivKeys)
	if err != nil {
		return err
	}
	prof.Profile("mine_seal_block", time.Now().Sub(start))

	// Update accountMap
	start = time.Now()
	if err := accountMap.Update(newtxs, signer); err != nil {
		return err
	}
	prof.Profile("main_update_accountMap", time.Now().Sub(start))

	// Insert the block into the blockchain
	start = time.Now()
	if n, err := bcdata.bc.InsertChain(types.Blocks{b}); err != nil {
		return fmt.Errorf("err = %s, n = %d\n", err, n)
	}
	prof.Profile("main_insert_blockchain", time.Now().Sub(start))

	// Apply reward
	start = time.Now()
	rewardAddr := *bcdata.rewardBase
	accountMap.AddBalance(rewardAddr, big.NewInt(params.KLAY))
	prof.Profile("main_apply_reward", time.Now().Sub(start))

	// Verification with accountMap
	start = time.Now()
	statedbNew, err := bcdata.bc.State()
	if err != nil {
		return err
	}
	if err := accountMap.Verify(statedbNew); err != nil {
		return err
	}
	prof.Profile("main_verification", time.Now().Sub(start))

	return nil
}

func (bcdata *BCData) GenABlockWithTransactions(accountMap *AccountMap, transactions types.Transactions,
	prof *profile.Profiler) error {

	signer := types.MakeSigner(bcdata.bc.Config(), bcdata.bc.CurrentHeader().Number)

	// Update accountMap
	start := time.Now()
	if err := accountMap.Update(transactions, signer); err != nil {
		return err
	}
	prof.Profile("main_update_accountMap", time.Now().Sub(start))

	// Mine a block!
	start = time.Now()
	b, err := bcdata.MineABlock(transactions, signer, prof)
	if err != nil {
		return err
	}
	prof.Profile("main_mineABlock", time.Now().Sub(start))

	// Insert the block into the blockchain
	start = time.Now()
	if n, err := bcdata.bc.InsertChain(types.Blocks{b}); err != nil {
		return fmt.Errorf("err = %s, n = %d\n", err, n)
	}
	prof.Profile("main_insert_blockchain", time.Now().Sub(start))

	// Apply reward
	start = time.Now()
	rewardAddr := *bcdata.rewardBase
	accountMap.AddBalance(rewardAddr, big.NewInt(params.KLAY))
	prof.Profile("main_apply_reward", time.Now().Sub(start))

	// Verification with accountMap
	start = time.Now()
	statedb, err := bcdata.bc.State()
	if err != nil {
		return err
	}
	if err := accountMap.Verify(statedb); err != nil {
		return err
	}
	prof.Profile("main_verification", time.Now().Sub(start))

	return nil
}

////////////////////////////////////////////////////////////////////////////////
func NewDatabase(dir, dbType string) (db database.DBManager, err error) {
	if dir == "" {
		return database.NewMemoryDBManager(), nil
	} else {
		return database.NewDBManager(dir, dbType, false, 16, 16)
	}
}

// Copied from consensus/istanbul/backend/engine.go
func prepareIstanbulExtra(validators []common.Address) ([]byte, error) {
	var buf bytes.Buffer

	buf.Write(bytes.Repeat([]byte{0x0}, types.IstanbulExtraVanity))

	ist := &types.IstanbulExtra{
		Validators:    validators,
		Seal:          []byte{},
		CommittedSeal: [][]byte{},
	}

	payload, err := rlp.EncodeToBytes(&ist)
	if err != nil {
		return nil, err
	}
	return append(buf.Bytes(), payload...), nil
}

func initBlockchain(conf *node.Config, db database.DBManager, coinbaseAddrs []*common.Address, validators []common.Address,
	engine consensus.Engine) (*blockchain.BlockChain, error) {

	extraData, err := prepareIstanbulExtra(validators)

	genesis := blockchain.DefaultGenesisBlock()
	genesis.Coinbase = *coinbaseAddrs[0]
	genesis.Config = Forks["Byzantium"]
	genesis.GasLimit = GasLimit
	genesis.ExtraData = extraData
	genesis.Nonce = 0
	genesis.Mixhash = types.IstanbulDigest
	genesis.Difficulty = big.NewInt(1)

	alloc := make(blockchain.GenesisAlloc)
	for _, a := range coinbaseAddrs {
		alloc[*a] = blockchain.GenesisAccount{Balance: big.NewInt(params.KLAY)}
	}

	genesis.Alloc = alloc

	chainConfig, _, err := blockchain.SetupGenesisBlock(db, genesis)
	if _, ok := err.(*params.ConfigCompatError); err != nil && !ok {
		return nil, err
	}

	chain, err := blockchain.NewBlockChain(db, nil, chainConfig, engine, vm.Config{})
	if err != nil {
		return nil, err
	}

	return chain, nil
}

func createAccounts(numAccounts int) ([]*common.Address, []*ecdsa.PrivateKey, error) {
	accs := make([]*common.Address, numAccounts)
	privKeys := make([]*ecdsa.PrivateKey, numAccounts)

	for i := 0; i < numAccounts; i++ {
		k, err := crypto.GenerateKey()
		if err != nil {
			return nil, nil, err
		}
		keyAddr := crypto.PubkeyToAddress(k.PublicKey)

		accs[i] = &keyAddr
		privKeys[i] = k
	}

	return accs, privKeys, nil
}

// Copied from consensus/istanbul/backend/engine.go
func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewKeccak256()

	// Clean seal is required for calculating proposer seal.
	rlp.Encode(hasher, types.IstanbulFilteredHeader(header, false))
	hasher.Sum(hash[:0])
	return hash
}

// writeSeal writes the extra-data field of the given header with the given seals.
// Copied from consensus/istanbul/backend/engine.go
func writeSeal(h *types.Header, seal []byte) error {
	if len(seal)%types.IstanbulExtraSeal != 0 {
		return errors.New("invalid signature")
	}

	istanbulExtra, err := types.ExtractIstanbulExtra(h)
	if err != nil {
		return err
	}

	istanbulExtra.Seal = seal
	payload, err := rlp.EncodeToBytes(&istanbulExtra)
	if err != nil {
		return err
	}

	h.Extra = append(h.Extra[:types.IstanbulExtraVanity], payload...)
	return nil
}

// writeCommittedSeals writes the extra-data field of a block header with given committed seals.
// Copied from consensus/istanbul/backend/engine.go
func writeCommittedSeals(h *types.Header, committedSeals [][]byte) error {
	errInvalidCommittedSeals := errors.New("invalid committed seals")

	if len(committedSeals) == 0 {
		return errInvalidCommittedSeals
	}

	for _, seal := range committedSeals {
		if len(seal) != types.IstanbulExtraSeal {
			return errInvalidCommittedSeals
		}
	}

	istanbulExtra, err := types.ExtractIstanbulExtra(h)
	if err != nil {
		return err
	}

	istanbulExtra.CommittedSeal = make([][]byte, len(committedSeals))
	copy(istanbulExtra.CommittedSeal, committedSeals)

	payload, err := rlp.EncodeToBytes(&istanbulExtra)
	if err != nil {
		return err
	}

	h.Extra = append(h.Extra[:types.IstanbulExtraVanity], payload...)
	return nil
}

// sign implements istanbul.backend.Sign
// Copied from consensus/istanbul/backend/backend.go
func sign(data []byte, privkey *ecdsa.PrivateKey) ([]byte, error) {
	hashData := crypto.Keccak256([]byte(data))
	return crypto.Sign(hashData, privkey)
}

func makeCommittedSeal(h *types.Header, privKeys []*ecdsa.PrivateKey) ([][]byte, error) {
	committedSeals := make([][]byte, 0, 3)

	for i := 1; i < 4; i++ {
		seal := istanbulCore.PrepareCommittedSeal(h.Hash())
		committedSeal, err := sign(seal, privKeys[i])
		if err != nil {
			return nil, err
		}
		committedSeals = append(committedSeals, committedSeal)
	}

	return committedSeals, nil
}

func sealBlock(b *types.Block, privKeys []*ecdsa.PrivateKey) (*types.Block, error) {
	header := b.Header()

	seal, err := sign(sigHash(header).Bytes(), privKeys[0])
	if err != nil {
		return nil, err
	}

	err = writeSeal(header, seal)
	if err != nil {
		return nil, err
	}

	committedSeals, err := makeCommittedSeal(header, privKeys)
	if err != nil {
		return nil, err
	}

	err = writeCommittedSeals(header, committedSeals)
	if err != nil {
		return nil, err
	}

	return b.WithSeal(header), nil
}
