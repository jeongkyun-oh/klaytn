// Copyright 2018 The klaytn Authors
// Copyright 2017 The go-ethereum Authors
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
// This file is derived from quorum/consensus/istanbul/validator/default.go (2018/06/04).
// Modified and improved for the klaytn development.

package validator

import (
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/consensus"
	"github.com/ground-x/klaytn/consensus/istanbul"
	"github.com/ground-x/klaytn/contracts/reward"
	"github.com/ground-x/klaytn/params"
	"math"
	"math/big"
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type weightedValidator struct {
	address common.Address

	rewardAddress common.Address
	votingPower   uint64 // TODO-Klaytn-Issue1336 This should be updated for governance implementation
	weight        int
}

func (val *weightedValidator) Address() common.Address {
	return val.address
}

func (val *weightedValidator) String() string {
	return val.Address().String()
}

func (val *weightedValidator) Equal(val2 *weightedValidator) bool {
	return val.address == val2.address
}

func (val *weightedValidator) Hash() int64 {
	return val.address.Hash().Big().Int64()
}

func (val *weightedValidator) RewardAddress() common.Address {
	return val.rewardAddress
}

func (val *weightedValidator) VotingPower() uint64 {
	return val.votingPower
}

func (val *weightedValidator) Weight() int {
	return val.weight
}

func newWeightedValidator(addr common.Address, reward common.Address, votingpower uint64, weight int) istanbul.Validator {
	return &weightedValidator{
		address:       addr,
		rewardAddress: reward,
		votingPower:   votingpower,
		weight:        weight,
	}
}

type weightedCouncil struct {
	subSize    uint64
	validators istanbul.Validators
	policy     istanbul.ProposerPolicy

	proposer    istanbul.Validator
	validatorMu sync.RWMutex
	selector    istanbul.ProposalSelector

	proposers         []istanbul.Validator
	proposersBlockNum uint64 // block number when proposers is determined

	stakingInfo *reward.StakingInfo

	blockNum uint64 // block number when council is determined
}

func RecoverWeightedCouncilProposer(valSet istanbul.ValidatorSet, proposerAddrs []common.Address) {
	weightedCouncil, ok := valSet.(*weightedCouncil)
	if !ok {
		logger.Error("Not weightedCouncil type. Return without recovering.")
		return
	}

	proposers := []istanbul.Validator{}

	for i, proposerAddr := range proposerAddrs {
		_, val := weightedCouncil.GetByAddress(proposerAddr)
		if val == nil {
			logger.Error("Proposer is not available now.", "proposer address", proposerAddr)
		}
		proposers = append(proposers, val)

		// TODO-Klaytn-Issue1166 Disable Trace log later
		logger.Trace("RecoverWeightedCouncilProposer() proposers", "i", i, "address", val.Address().String())
	}
	weightedCouncil.proposers = proposers
}

func NewWeightedCouncil(addrs []common.Address, rewards []common.Address, votingPowers []uint64, weights []int, policy istanbul.ProposerPolicy, committeeSize uint64, blockNum uint64, proposersBlockNum uint64, chain consensus.ChainReader) *weightedCouncil {

	if policy != istanbul.WeightedRandom {
		logger.Error("unsupported proposer policy for weighted council", "policy", policy)
		return nil
	}

	valSet := &weightedCouncil{}
	valSet.subSize = committeeSize
	valSet.policy = policy

	// prepare rewards if necessary
	if rewards == nil {
		rewards = make([]common.Address, len(addrs))
		for i := range addrs {
			rewards[i] = common.Address{}
		}
	}

	// prepare weights if necessary
	if weights == nil {
		// initialize with 0 weight.
		weights = make([]int, len(addrs))
	}

	// prepare votingPowers if necessary
	if votingPowers == nil {
		votingPowers = make([]uint64, len(addrs))
		if chain == nil {
			logger.Crit("Requires chain to initialize voting powers.")
		}

		//stateDB, err := chain.State()
		//if err != nil {
		//	logger.Crit("Failed to get statedb from chain.")
		//}

		for i := range addrs {
			// TODO-Klaytn-TokenEconomy: Use default value until the formula to calculate votingpower released
			votingPowers[i] = 1000
			//staking := stateDB.GetBalance(addr)
			//if staking.Cmp(common.Big0) == 0 {
			//	votingPowers[i] = 1
			//} else {
			//	votingPowers[i] = 2
			//}
		}
	}

	if len(addrs) != len(rewards) ||
		len(addrs) != len(votingPowers) ||
		len(addrs) != len(weights) {
		logger.Error("incomplete information for weighted council", "num addrs", len(addrs), "num rewards", len(rewards), "num votingPowers", len(votingPowers), "num weights", len(weights))
		return nil
	}

	// init validators
	valSet.validators = make([]istanbul.Validator, len(addrs))
	for i, addr := range addrs {
		valSet.validators[i] = newWeightedValidator(addr, rewards[i], votingPowers[i], weights[i])
	}

	// sort validator
	sort.Sort(valSet.validators)

	// init proposer
	if valSet.Size() > 0 {
		valSet.proposer = valSet.GetByIndex(0)
	}
	valSet.selector = weightedRandomProposer

	valSet.blockNum = blockNum
	valSet.proposers = make([]istanbul.Validator, len(addrs))
	copy(valSet.proposers, valSet.validators)
	valSet.proposersBlockNum = proposersBlockNum

	logger.Trace("Allocate new weightedCouncil", "weightedCouncil", valSet)

	return valSet
}

func GetWeightedCouncilData(valSet istanbul.ValidatorSet) (rewardAddrs []common.Address, votingPowers []uint64, weights []int, proposers []common.Address, proposersBlockNum uint64) {

	weightedCouncil, ok := valSet.(*weightedCouncil)
	if !ok {
		logger.Error("not weightedCouncil type.")
		return
	}

	if weightedCouncil.Policy() == istanbul.WeightedRandom {
		numVals := len(weightedCouncil.validators)
		rewardAddrs = make([]common.Address, numVals)
		votingPowers = make([]uint64, numVals)
		weights = make([]int, numVals)
		for i, val := range weightedCouncil.List() {
			weightedVal := val.(*weightedValidator)
			rewardAddrs[i] = weightedVal.rewardAddress
			votingPowers[i] = weightedVal.votingPower
			weights[i] = weightedVal.weight
		}

		proposers = make([]common.Address, len(weightedCouncil.proposers))
		for i, proposer := range weightedCouncil.proposers {
			proposers[i] = proposer.Address()
		}
		proposersBlockNum = weightedCouncil.proposersBlockNum
	} else {
		logger.Error("invalid proposer policy for weightedCouncil")
	}
	return
}

func weightedRandomProposer(valSet istanbul.ValidatorSet, lastProposer common.Address, round uint64) istanbul.Validator {
	if valSet.Size() == 0 {
		return nil
	}

	weightedCouncil, ok := valSet.(*weightedCouncil)
	if !ok {
		logger.Error("weightedRandomProposer() Not weightedCouncil type.")
		return nil
	}

	numProposers := len(weightedCouncil.proposers)
	if numProposers == 0 {
		logger.Error("weightedRandomProposer() No available proposers.")
		return nil
	}

	// At Refresh(), proposers is already randomly shuffled considering weights.
	// So let's just round robin this array
	blockNum := weightedCouncil.blockNum
	picker := (blockNum + round - params.CalcProposerBlockNumber(blockNum)) % uint64(numProposers)
	proposer := weightedCouncil.proposers[picker]

	// Enable below more detailed log when debugging
	// logger.Trace("Select a proposer using weighted random", "proposer", proposer.String(), "picker", picker, "blockNum of council", blockNum, "round", round, "blockNum of proposers updated", weightedCouncil.proposersBlockNum, "number of proposers", numProposers, "all proposers", weightedCouncil.proposers)

	return proposer
}

func (valSet *weightedCouncil) Size() uint64 {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	return uint64(len(valSet.validators))
}

func (valSet *weightedCouncil) SubGroupSize() uint64 {
	return valSet.subSize
}

func (valSet *weightedCouncil) SetSubGroupSize(size uint64) {
	valSet.subSize = size
}

func (valSet *weightedCouncil) List() []istanbul.Validator {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	return valSet.validators
}

func (valSet *weightedCouncil) SubList(prevHash common.Hash) []istanbul.Validator {
	return valSet.SubListWithProposer(prevHash, valSet.GetProposer().Address())
}

func (valSet *weightedCouncil) SubListWithProposer(prevHash common.Hash, proposer common.Address) []istanbul.Validator {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()

	if uint64(len(valSet.validators)) <= valSet.subSize {
		// logger.Trace("Choose all validators", "prevHash", prevHash, "proposer", proposer, "committee", valSet.validators)
		return valSet.validators
	}

	hashstring := strings.TrimPrefix(prevHash.Hex(), "0x")
	if len(hashstring) > 15 {
		hashstring = hashstring[:15]
	}

	seed, err := strconv.ParseInt(hashstring, 16, 64)
	if err != nil {
		logger.Error("input", "hash", prevHash.Hex())
		logger.Error("fail to make sub-list of validators", "seed", seed, "err", err)
		return valSet.validators
	}

	// shuffle
	committee := make([]istanbul.Validator, valSet.subSize)
	_, proposerValidator := valSet.GetByAddress(proposer)
	if proposerValidator == nil {
		logger.Error("fail to make sub-list of validators, because proposer is invalid", "address of proposer", proposer)
		return valSet.validators
	}
	committee[0] = proposerValidator

	// next proposer
	// TODO how to sync next proposer (how to get exact next proposer ?)
	committee[1] = valSet.selector(valSet, committee[0].Address(), uint64(0))

	proposerIdx, _ := valSet.GetByAddress(committee[0].Address())
	nextproposerIdx, _ := valSet.GetByAddress(committee[1].Address())

	// TODO-Klaytn-RemoveLater remove this check code if the implementation is stable.
	if proposerIdx < 0 || nextproposerIdx < 0 {
		vals := "["
		for _, v := range valSet.validators {
			vals += fmt.Sprintf("%s,", v.Address().Hex())
		}
		vals += "]"
		logger.Error("current proposer or next proposer not found in Council", "proposerIdx", proposerIdx, "nextproposerIdx", nextproposerIdx, "proposer", committee[0].Address().Hex(),
			"nextproposer", committee[1].Address().Hex(), "validators", vals)
	}

	if proposerIdx == nextproposerIdx {
		logger.Error("fail to make propser", "current proposer idx", proposerIdx, "next idx", nextproposerIdx)
	}

	limit := len(valSet.validators)
	picker := rand.New(rand.NewSource(seed))

	pickSize := limit - 2
	indexs := make([]int, pickSize)
	idx := 0
	for i := 0; i < limit; i++ {
		if i != proposerIdx && i != nextproposerIdx {
			indexs[idx] = i
			idx++
		}
	}
	for i := 0; i < pickSize; i++ {
		randIndex := picker.Intn(pickSize)
		indexs[i], indexs[randIndex] = indexs[randIndex], indexs[i]
	}

	for i := uint64(0); i < valSet.subSize-2; i++ {
		committee[i+2] = valSet.validators[indexs[i]]
	}

	if prevHash.Hex() == "0x0000000000000000000000000000000000000000000000000000000000000000" {
		logger.Error("### subList", "prevHash", prevHash.Hex())
	}

	logger.Error("New committee", "prevHash", prevHash, "proposer", proposer, "committee", valSet.validators)
	return committee
}

func (valSet *weightedCouncil) IsSubSet() bool {
	// TODO-Klaytn-RemoveLater We don't use this interface anymore. Eventually let's remove this function from ValidatorSet interface.
	return valSet.Size() > valSet.subSize
}

func (valSet *weightedCouncil) GetByIndex(i uint64) istanbul.Validator {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	if i < uint64(valSet.Size()) {
		return valSet.validators[i]
	}
	return nil
}

func (valSet *weightedCouncil) GetByAddress(addr common.Address) (int, istanbul.Validator) {
	for i, val := range valSet.List() {
		if addr == val.Address() {
			return i, val
		}
	}
	return -1, nil
}

func (valSet *weightedCouncil) GetProposer() istanbul.Validator {
	//logger.Trace("GetProposer()", "proposer", valSet.proposer)
	return valSet.proposer
}

func (valSet *weightedCouncil) IsProposer(address common.Address) bool {
	_, val := valSet.GetByAddress(address)
	return reflect.DeepEqual(valSet.GetProposer(), val)
}

func (valSet *weightedCouncil) CalcProposer(lastProposer common.Address, round uint64) {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()

	newProposer := valSet.selector(valSet, lastProposer, round)

	logger.Debug("Update a proposer", "old", valSet.proposer, "new", newProposer, "last proposer", lastProposer.String(), "round", round, "blockNum of council", valSet.blockNum, "blockNum of proposers", valSet.proposersBlockNum)
	valSet.proposer = newProposer
}

func (valSet *weightedCouncil) AddValidator(address common.Address) bool {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()
	for _, v := range valSet.validators {
		if v.Address() == address {
			return false
		}
	}

	// TODO-Klaytn-Issue1336 Update for governance implementation. How to determine initial value for rewardAddress and votingPower ?
	valSet.validators = append(valSet.validators, newWeightedValidator(address, common.Address{}, 0, 0))

	// sort validator
	sort.Sort(valSet.validators)
	return true
}

// removeValidatorFromProposers makes new candidate proposers by removing a validator with given address from existing proposers.
func (valSet *weightedCouncil) removeValidatorFromProposers(address common.Address) {
	newProposers := make([]istanbul.Validator, 0, len(valSet.proposers))

	for _, v := range valSet.proposers {
		if v.Address() != address {
			newProposers = append(newProposers, v)
		}
	}
	logger.Trace("Invalidate a validator from proposers", "num proposers(before)", len(valSet.proposers), "num proposers(after)", len(newProposers))

	valSet.proposers = newProposers
}

func (valSet *weightedCouncil) RemoveValidator(address common.Address) bool {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()

	for i, v := range valSet.validators {
		if v.Address() == address {
			valSet.validators = append(valSet.validators[:i], valSet.validators[i+1:]...)
			valSet.removeValidatorFromProposers(address)
			return true
		}
	}
	return false
}

func (valSet *weightedCouncil) ReplaceValidators(vals []istanbul.Validator) bool {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()

	valSet.validators = istanbul.Validators(make([]istanbul.Validator, len(vals)))
	copy(valSet.validators, istanbul.Validators(vals))
	return true
}

func (valSet *weightedCouncil) GetValidators() []istanbul.Validator {
	return valSet.validators
}

func (valSet *weightedCouncil) Copy() istanbul.ValidatorSet {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()

	var newWeightedCouncil = weightedCouncil{
		subSize:           valSet.subSize,
		policy:            valSet.policy,
		proposer:          valSet.proposer,
		selector:          valSet.selector,
		stakingInfo:       valSet.stakingInfo,
		proposersBlockNum: valSet.proposersBlockNum,
		blockNum:          valSet.blockNum,
	}
	newWeightedCouncil.validators = make([]istanbul.Validator, len(valSet.validators))
	copy(newWeightedCouncil.validators, valSet.validators)

	newWeightedCouncil.proposers = make([]istanbul.Validator, len(valSet.proposers))
	copy(newWeightedCouncil.proposers, valSet.proposers)

	return &newWeightedCouncil
}

func (valSet *weightedCouncil) F() int {
	if valSet.Size() > valSet.subSize {
		return int(math.Ceil(float64(valSet.subSize)/3)) - 1
	} else {
		return int(math.Ceil(float64(valSet.Size())/3)) - 1
	}
}

func (valSet *weightedCouncil) Policy() istanbul.ProposerPolicy { return valSet.policy }

func (valSet *weightedCouncil) Refresh(hash common.Hash, blockNum uint64) error {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()

	if valSet.proposersBlockNum == blockNum {
		// already refreshed
		return nil
	}

	// TODO-Klaytn-Issue1166 Disable trace logs below after all implementation is merged.
	if len(valSet.validators) == 0 {
		return errors.New("No validator")
	}

	hashString := strings.TrimPrefix(hash.Hex(), "0x")
	if len(hashString) > 15 {
		hashString = hashString[:15]
	}
	seed, err := strconv.ParseInt(hashString, 16, 64)
	if err != nil {
		return err
	}

	newStakingInfo := reward.GetStakingInfoFromStakingCache(blockNum)
	logger.Debug("Update staking to calculate new proposers", "blockNum", blockNum, "old stakingInfo", valSet.stakingInfo, "new stakingInfo", newStakingInfo)
	valSet.stakingInfo = newStakingInfo

	// TODO-Klaytn-Issue1166 Update weightedValidator information with staking info if available
	if valSet.stakingInfo != nil {
		logger.Info("Refresh with staking info", "stakingInfo", valSet.stakingInfo)
		// (1) Update rewardAddress
		// (2) Calculate total staking amount
		totalStaking := big.NewInt(0)
		for valIdx, val := range valSet.validators {
			i := valSet.stakingInfo.GetIndexByNodeId(val.Address())
			if i != -1 {
				val.(*weightedValidator).rewardAddress = valSet.stakingInfo.CouncilRewardAddrs[i]
				totalStaking.Add(totalStaking, valSet.stakingInfo.CouncilStakingAmounts[i])
			} else {
				val.(*weightedValidator).rewardAddress = common.Address{}
			}
			logger.Trace("Refresh updates rewardAddr of validator", "index", valIdx, "validator", val.(*weightedValidator), "rewardAddr", val.RewardAddress().String())
		}

		// TODO-Klaytn-Issue1400 one of exception cases
		if totalStaking.Cmp(common.Big0) > 0 {
			// update weight
			tmp := big.NewInt(0)
			tmp100 := big.NewInt(100)
			for valIdx, val := range valSet.validators {
				i := valSet.stakingInfo.GetIndexByNodeId(val.Address())
				if i != -1 {
					stakingAmount := valSet.stakingInfo.CouncilStakingAmounts[i]
					weight := int(tmp.Div(tmp.Mul(stakingAmount, tmp100), totalStaking).Int64()) // No overflow occurs here.
					val.(*weightedValidator).weight = weight
				} else {
					val.(*weightedValidator).weight = 0
				}
				logger.Trace("Refresh updates weight of validator", "index", valIdx, "validator", val, "weight", val.Weight())
			}
		} else {
			for i, val := range valSet.validators {
				val.(*weightedValidator).weight = 0
				logger.Trace("Refresh updates weight of validator to 0 due to staking value is 0", "index", i, "validator", val, "weight", val.Weight())
			}
		}
	} else {
		logger.Info("Refresh without staking info", "stakingInfo", valSet.stakingInfo)
	}

	candidateVals := []istanbul.Validator{}
	for _, val := range valSet.validators {
		weight := val.Weight()
		for i := 0; i < weight; i++ {
			candidateVals = append(candidateVals, val)
		}
	}

	if len(candidateVals) == 0 {
		// All validators has zero weight. Let's use all validators as candidate proposers.
		candidateVals = valSet.validators
		logger.Trace("Refresh uses all validators as candidate proposers, because all weight is zero.", "candidateVals", candidateVals)
	}

	proposers := make([]istanbul.Validator, len(candidateVals))

	limit := len(candidateVals)
	picker := rand.New(rand.NewSource(seed))

	indexs := make([]int, limit)
	idx := 0
	for i := 0; i < limit; i++ {
		indexs[idx] = i
		idx++
	}

	// shuffle
	for i := 0; i < limit; i++ {
		randIndex := picker.Intn(limit)
		indexs[i], indexs[randIndex] = indexs[randIndex], indexs[i]
	}

	for i := 0; i < limit; i++ {
		proposers[i] = candidateVals[indexs[i]]
		// Below log is too verbose. Use is only when debugging.
		// logger.Trace("Refresh calculates new proposers", "i", i, "proposers[i]", proposers[i].String())
	}

	valSet.proposers = proposers
	valSet.proposersBlockNum = blockNum

	logger.Info("Refresh done.", "blockNum", blockNum, "valSet", valSet, "new proposers", valSet.proposers)

	return nil
}

func (valSet *weightedCouncil) SetBlockNum(blockNum uint64) {
	valSet.blockNum = blockNum
}

func (valSet *weightedCouncil) Proposers() []istanbul.Validator {
	return valSet.proposers
}

func (valSet *weightedCouncil) TotalVotingPower() uint64 {
	sum := uint64(0)
	for _, v := range valSet.List() {
		sum += v.VotingPower()
	}
	return sum
}
