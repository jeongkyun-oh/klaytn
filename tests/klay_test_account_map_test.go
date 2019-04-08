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
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/state"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/contracts/reward/contract"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/kerrors"
	"math/big"
)

////////////////////////////////////////////////////////////////////////////////
// AddressBalanceMap
////////////////////////////////////////////////////////////////////////////////
type AccountInfo struct {
	balance *big.Int
	nonce   uint64
}

type AccountMap struct {
	m        map[common.Address]*AccountInfo
	coinbase common.Address
}

func NewAccountMap() *AccountMap {
	return &AccountMap{
		m: make(map[common.Address]*AccountInfo),
	}
}

func (a *AccountMap) AddBalance(addr common.Address, v *big.Int) {
	if acc, ok := a.m[addr]; ok {
		acc.balance.Add(acc.balance, v)
	} else {
		// create an account
		a.Set(addr, v, 0)
	}
}

func (a *AccountMap) SubBalance(addr common.Address, v *big.Int) error {
	if acc, ok := a.m[addr]; ok {
		acc.balance.Sub(acc.balance, v)
	} else {
		return fmt.Errorf("trying to subtract balance from an uninitiailzed address (%s)", addr.Hex())
	}

	return nil
}

func (a *AccountMap) GetNonce(addr common.Address) (uint64, error) {
	if acc, ok := a.m[addr]; ok {
		return acc.nonce, nil
	}

	return 0, fmt.Errorf("trying to get nonce from an uninitialized address (%s)", addr.Hex())
}

func (a *AccountMap) IncNonce(addr common.Address) {
	if acc, ok := a.m[addr]; ok {
		acc.nonce++
	}
}

func (a *AccountMap) Set(addr common.Address, v *big.Int, nonce uint64) {
	a.m[addr] = &AccountInfo{new(big.Int).Set(v), nonce}
}

func (a *AccountMap) Initialize(bcdata *BCData) error {
	statedb, err := bcdata.bc.State()
	if err != nil {
		return err
	}

	// NOTE-Klaytn-Issue973 Developing Klaytn token economy
	// Add predefined accounts related to reward mechanism
	rewardContractAddr := common.HexToAddress(contract.RewardContractAddress)
	kirContractAddr := common.HexToAddress(contract.KIRContractAddress)
	pocContractAddr := common.HexToAddress(contract.PoCContractAddress)
	addrs := append(bcdata.addrs, &rewardContractAddr, &kirContractAddr, &pocContractAddr)

	for _, addr := range addrs {
		a.Set(*addr, statedb.GetBalance(*addr), statedb.GetNonce(*addr))
	}

	a.coinbase = *bcdata.addrs[0]

	return nil
}

func (a *AccountMap) Update(txs types.Transactions, signer types.Signer, picker types.AccountKeyPicker, currentBlockNumber uint64) error {
	for _, tx := range txs {
		to := tx.To()
		v := tx.Value()

		from, gasFrom, err := types.ValidateSender(signer, tx, picker, currentBlockNumber)
		if err != nil {
			return err
		}

		feePayer, gasFeePayer, err := types.ValidateFeePayer(signer, tx, picker, currentBlockNumber)
		if err != nil {
			return err
		}

		if to == nil {
			nonce, err := a.GetNonce(from)
			if err != nil {
				return err
			}
			codeHash := crypto.Keccak256Hash(tx.Data())
			addr := crypto.CreateAddress(from, nonce, codeHash)
			to = &addr
		}

		a.AddBalance(*to, v)
		a.SubBalance(from, v)

		// TODO-Klaytn: This gas fee calculation is correct only if the transaction is a value transfer transaction.
		// Calculate the correct transaction fee by checking the corresponding receipt.
		intrinsicGas, err := tx.IntrinsicGas(currentBlockNumber)
		if err != nil {
			return err
		}

		intrinsicGas += gasFrom + gasFeePayer

		fee := new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(intrinsicGas))

		feeRatio := tx.FeeRatio()
		switch {
		case feeRatio == types.MaxFeeRatio:
			a.SubBalance(feePayer, fee)

		case feeRatio < types.MaxFeeRatio:
			feeByFeePayer, feeBySender := types.CalcFeeWithRatio(feeRatio, fee)
			a.SubBalance(feePayer, feeByFeePayer)
			a.SubBalance(from, feeBySender)

		default:
			// feeRatio > types.MaxFeeRatio
			return kerrors.ErrMaxFeeRatioExceeded
		}

		a.AddBalance(a.coinbase, fee)

		a.IncNonce(from)
	}

	return nil
}

func (a *AccountMap) Verify(statedb *state.StateDB) error {
	for addr, acc := range a.m {
		if acc.nonce != statedb.GetNonce(addr) {
			return errors.New(fmt.Sprintf("[%s] nonce is different!! statedb(%d) != accountMap(%d).\n",
				addr.Hex(), statedb.GetNonce(addr), acc.nonce))
		}

		if acc.balance.Cmp(statedb.GetBalance(addr)) != 0 {
			return errors.New(fmt.Sprintf("[%s] balance is different!! statedb(%s) != accountMap(%s).\n",
				addr.Hex(), statedb.GetBalance(addr).String(), acc.balance.String()))
		}
	}

	return nil
}
