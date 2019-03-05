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
	votingPower   float64 // TODO-Klaytn-Issue1336 This should be updated for governance implementation
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

func (val *weightedValidator) VotingPower() float64 {
	return val.votingPower
}

func (val *weightedValidator) Weight() int {
	return val.weight
}

func newWeightedValidator(addr common.Address, reward common.Address, votingpower float64) istanbul.Validator {
	return &weightedValidator{
		address:       addr,
		rewardAddress: reward,
		votingPower:   votingpower,
	}
}

type weightedCouncil struct {
	subSize int

	validators istanbul.Validators
	policy     istanbul.ProposerPolicy

	proposer    istanbul.Validator
	validatorMu sync.RWMutex
	selector    istanbul.ProposalSelector

	proposers         []istanbul.Validator
	proposersBlockNum uint64 // block number when proposers is determined

	stakingInfo *common.StakingInfo
	stakings    []*big.Int

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

func NewWeightedCouncil(addrs []common.Address, rewards []common.Address, votingPowers []float64, weights []int, policy istanbul.ProposerPolicy, committeeSize int, blockNum uint64, proposersBlockNum uint64, chain consensus.ChainReader) *weightedCouncil {
	// TODO-Klaytn-Issue1166 Disable Trace log later
	valSet := &weightedCouncil{}

	valSet.subSize = committeeSize
	valSet.policy = policy

	// init validators
	valSet.validators = make([]istanbul.Validator, len(addrs))
	logger.Trace("NewWeightedCouncil() params", "addrs", addrs, "rewards", rewards, "votingPowers", votingPowers, "weights", weights, "policy", policy)

	// init rewards if necessary
	if rewards == nil {
		rewards = make([]common.Address, len(addrs))
		for i := range addrs {
			rewards[i] = common.Address{}
		}
	}

	// init votingPowers if necessary
	if votingPowers == nil {
		votingPowers = make([]float64, len(addrs))
		if chain == nil {
			logger.Crit("Requires chain to initialize voting powers.")
		}

		//stateDB, err := chain.State()
		//if err != nil {
		//	logger.Crit("Failed to get statedb from chain.")
		//}

		for i := range addrs {
			// TODO-Klaytn-TokenEconomy: Use default value until the formula to calculate votingpower released
			votingPowers[i] = 1
			//staking := stateDB.GetBalance(addr)
			//if staking.Cmp(common.Big0) == 0 {
			//	votingPowers[i] = 1
			//} else {
			//	votingPowers[i] = 2
			//}
		}
	}

	for i, addr := range addrs {
		valSet.validators[i] = newWeightedValidator(addr, rewards[i], votingPowers[i])
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

	logger.Trace("NewWeightedCouncil() New weightedCouncil", "weightedCouncil", valSet)

	return valSet
}

func GetWeightedCouncilData(valSet istanbul.ValidatorSet) (rewardAddrs []common.Address, votingPowers []float64, weights []int, proposers []common.Address, proposersBlockNum uint64) {
	// TODO-Klaytn-Issue1166 Disable Trace log later

	weightedCouncil, ok := valSet.(*weightedCouncil)
	if !ok {
		logger.Error("GetWeightedCouncilData() Not weightedCouncil type.")
		return
	}

	if weightedCouncil.Policy() == istanbul.WeightedRandom {
		numVals := len(weightedCouncil.validators)
		rewardAddrs = make([]common.Address, numVals)
		votingPowers = make([]float64, numVals)
		weights = make([]int, numVals)
		for i, val := range weightedCouncil.List() {
			weightedVal := val.(*weightedValidator)
			rewardAddrs[i] = weightedVal.rewardAddress
			votingPowers[i] = weightedVal.votingPower
			weights[i] = weightedVal.weight
			logger.Trace("GetWeightedCouncilData()", "i", i, "rewardAddr", rewardAddrs[i], "votingPower", votingPowers[i], "weight", weights[i])
		}

		proposers = make([]common.Address, len(weightedCouncil.proposers))
		for i, proposer := range weightedCouncil.proposers {
			proposers[i] = proposer.Address()
			logger.Trace("GetWeightedCouncilData() proposers", "i", i, "addr", proposers[i])
		}
		proposersBlockNum = weightedCouncil.proposersBlockNum
	} else {
		logger.Error("GetWeightedCouncilData() WeightedCouncil with wrong proposer policy.")
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

	// TODO-Klaytn-Issue1166 proposers is already randomly shuffled considering weights.
	// So let's just round robin this array
	blockNum := weightedCouncil.blockNum
	picker := (blockNum + round - reward.CalcProposerBlockNumber(blockNum)) % uint64(numProposers)
	proposer := weightedCouncil.proposers[picker]

	// TODO-Klaytn-Issue1166 Disable Trace log later
	logger.Trace("Issue1166: weightedRandomProposer() returns", "proposer", proposer.String(), "weighedCouncil.blockNum", blockNum, "round", round, "picker", picker, "proposers", weightedCouncil.proposers)

	return proposer
}

func (valSet *weightedCouncil) Size() int {
	valSet.validatorMu.RLock()
	defer valSet.validatorMu.RUnlock()
	return len(valSet.validators)
}

func (valSet *weightedCouncil) SubGroupSize() int {
	return valSet.subSize
}

func (valSet *weightedCouncil) SetSubGroupSize(size int) {
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

	if len(valSet.validators) <= int(valSet.subSize) {
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
	committee[0] = New(proposer)

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

	for i := 0; i < int(valSet.subSize)-2; i++ {
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
	return valSet.Size() > int(valSet.subSize)
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

	logger.Debug("Update proposer", "old proposer", valSet.proposer, "new proposer", newProposer, "last proposer", lastProposer.String(), "round", round)
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
	valSet.validators = append(valSet.validators, newWeightedValidator(address, common.Address{}, 0))

	// sort validator
	sort.Sort(valSet.validators)
	return true
}

func (valSet *weightedCouncil) RemoveValidator(address common.Address) bool {
	valSet.validatorMu.Lock()
	defer valSet.validatorMu.Unlock()

	for i, v := range valSet.validators {
		if v.Address() == address {
			valSet.validators = append(valSet.validators[:i], valSet.validators[i+1:]...)
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
		subSize:  valSet.subSize,
		policy:   valSet.policy,
		proposer: valSet.proposer,
		selector: valSet.selector,
		// stakingInfo:       valSet.stakingInfo, // TODO-Klaytn-Issue1455 Enable this after StakingInfo is introduced
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
	if valSet.Size() > int(valSet.subSize) {
		return int(math.Ceil(float64(valSet.subSize)/3)) - 1
	} else {
		return int(math.Ceil(float64(valSet.Size())/3)) - 1
	}
}

func (valSet *weightedCouncil) Policy() istanbul.ProposerPolicy { return valSet.policy }

func (valSet *weightedCouncil) Refresh(prevHash common.Hash) error {
	// TODO-Klaytn-Issue1166 Disable trace logs below after all implementation is merged.
	if valSet.Size() == 0 {
		return errors.New("No validator")
	}

	hashString := strings.TrimPrefix(prevHash.Hex(), "0x")
	if len(hashString) > 15 {
		hashString = hashString[:15]
	}
	seed, err := strconv.ParseInt(hashString, 16, 64)
	if err != nil {
		logger.Trace("Parsing error", "prevHash", prevHash, "hashString", hashString, "seed", seed, "err", err)
		return err
	}

	// TODO-Klaytn-Issue1166 Update weightedValidator information with staking info if available
	if valSet.stakingInfo != nil {
		logger.Info("Refresh() - Let's use staking info", "stakingInfo", valSet.stakingInfo)
		// (1) Update rewardAddress
		// (2) Calculate total staking amount
		totalStaking := big.NewInt(0)
		for valIdx, val := range valSet.List() {
			i := valSet.stakingInfo.GetIndexByNodeId(val.Address())
			if i != -1 {
				val.(*weightedValidator).rewardAddress = valSet.stakingInfo.CouncilRewardAddrs[i]
				totalStaking.Add(totalStaking, valSet.stakingInfo.CouncilStakingAmounts[i])
			} else {
				val.(*weightedValidator).rewardAddress = common.Address{}
			}
			logger.Trace("Refresh() - Update rewardAddr with staking info", "Council index", valIdx, "validator", val.(*weightedValidator))
		}

		// TODO-Klaytn-Issue1400 one of exception cases
		if totalStaking.Cmp(common.Big0) > 0 {
			// update weight
			tmp := big.NewInt(0)
			tmp100 := big.NewInt(100)
			for _, val := range valSet.List() {
				i := valSet.stakingInfo.GetIndexByNodeId(val.Address())
				if i != -1 {
					stakingAmount := valSet.stakingInfo.CouncilStakingAmounts[i]
					weight := int(tmp.Div(tmp.Mul(stakingAmount, tmp100), totalStaking).Int64()) // No overflow occurs here.
					val.(*weightedValidator).weight = weight
					logger.Trace("Refresh() Update weight", "validator", val.(*weightedValidator), "weight", weight)
				} else {
					val.(*weightedValidator).weight = 0
					logger.Trace("Refresh() Set weight to 0, because of no staking info.", "validator", val.(*weightedValidator))
				}
			}
		} else {
			for i, val := range valSet.List() {
				val.(*weightedValidator).weight = 0
				logger.Trace("Refresh() Set weight to 0, because total staking value is 0.", "i", i, "validator", val.(*weightedValidator))
			}
		}
	} else {
		logger.Info("Issue1166-V3: Refresh() - No staking info", "stakingInfo", valSet.stakingInfo)
	}

	candidateVals := []istanbul.Validator{}
	for _, val := range valSet.List() {
		weight := val.Weight()
		for i := 0; i < weight; i++ {
			candidateVals = append(candidateVals, val)
		}
	}

	if len(candidateVals) == 0 {
		// No validator with weight found. Let's use all validators
		for _, val := range valSet.List() {
			candidateVals = append(candidateVals, val)
		}
		logger.Trace("Refresh() Use all validators, because there is no weight information", "candidateVals", candidateVals)
	} else {
		logger.Trace("Refresh() Candidate validators chosen with weights", "candidateVals", candidateVals)
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
		logger.Trace("Refresh() Calculated new proposers", "i", i, "proposers[i]", proposers[i].String())
	}

	valSet.proposers = proposers

	logger.Info("Refresh done.", "valSet", valSet, "new proposers", valSet.proposers)

	return nil
}

func (valSet *weightedCouncil) SetStakingInfo(stakingInfo *common.StakingInfo) error {
	valSet.stakingInfo = stakingInfo
	return nil
}
func (valSet *weightedCouncil) SetBlockNum(blockNum uint64) {
	valSet.blockNum = blockNum
}

func (valSet *weightedCouncil) Proposers() []istanbul.Validator {
	return valSet.proposers
}

func (valSet *weightedCouncil) TotalVotingPower() float64 {
	sum := float64(0.0)
	for _, v := range valSet.List() {
		sum += float64(v.VotingPower())
	}
	return sum
}
