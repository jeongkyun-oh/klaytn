// Copyright 2018 The klaytn Authors
// Copyright 2017 AMIS Technologies
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

package genesis

import (
	"encoding/json"
	"github.com/ground-x/klaytn/governance"
	"io/ioutil"
	"math/big"
	"path/filepath"
	"time"

	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	istcommon "github.com/ground-x/klaytn/cmd/istanbul/common"
	"github.com/ground-x/klaytn/consensus/istanbul"
	"github.com/ground-x/klaytn/params"
)

const (
	FileName       = "genesis.json"
	InitGasLimit   = 999999999999 // 4700000
	InitDifficulty = 1
)

func New(options ...Option) *blockchain.Genesis {
	genesis := &blockchain.Genesis{
		Timestamp:  uint64(time.Now().Unix()),
		GasLimit:   InitGasLimit,
		Difficulty: big.NewInt(InitDifficulty),
		Alloc:      make(blockchain.GenesisAlloc),
		Config: &params.ChainConfig{
			ChainID: big.NewInt(2018),
			Istanbul: &params.IstanbulConfig{
				ProposerPolicy: uint64(istanbul.DefaultConfig.ProposerPolicy),
				Epoch:          istanbul.DefaultConfig.Epoch,
				SubGroupSize:   istanbul.DefaultConfig.SubGroupSize,
			},
			UnitPrice:     0,
			DeriveShaImpl: 0,
			Governance:    governance.GetDefaultGovernanceConfig(params.UseIstanbul),
		},
		Mixhash: types.IstanbulDigest,
	}

	for _, opt := range options {
		opt(genesis)
	}

	return genesis
}

func NewClique(options ...Option) *blockchain.Genesis {
	genesis := &blockchain.Genesis{
		Timestamp:  uint64(time.Now().Unix()),
		GasLimit:   InitGasLimit,
		Difficulty: big.NewInt(InitDifficulty),
		Alloc:      make(blockchain.GenesisAlloc),
		Config: &params.ChainConfig{
			ChainID: big.NewInt(3000), // TODO-Klaytn Needs Optional chainID
			Clique: &params.CliqueConfig{
				Period: 1,
				Epoch:  30000,
			},
		},
	}

	for _, opt := range options {
		opt(genesis)
	}

	return genesis
}

func NewFileAt(dir string, options ...Option) string {
	genesis := New(options...)
	if err := Save(dir, genesis); err != nil {
		logger.Error("Failed to save genesis", "dir", dir, "err", err)
		return ""
	}

	return filepath.Join(dir, FileName)
}

func NewFile(options ...Option) string {
	dir, _ := istcommon.GenerateRandomDir()
	return NewFileAt(dir, options...)
}

func Save(dataDir string, genesis *blockchain.Genesis) error {
	filePath := filepath.Join(dataDir, FileName)

	var raw []byte
	var err error
	raw, err = json.Marshal(genesis)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filePath, raw, 0600)
}