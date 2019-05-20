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
// This file is derived from quorum/consensus/istanbul/backend/snapshot.go (2018/06/04).
// Modified and improved for the klaytn development.

package backend

import (
	"bytes"
	"encoding/json"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/consensus/istanbul"
	"github.com/ground-x/klaytn/consensus/istanbul/validator"
	"github.com/ground-x/klaytn/governance"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/storage/database"
)

const (
	dbKeySnapshotPrefix = "istanbul-snapshot"
)

// Snapshot is the state of the authorization voting at a given point in time.
type Snapshot struct {
	Epoch         uint64                // The number of blocks after which to checkpoint and reset the pending votes
	Number        uint64                // Block number where the snapshot was created
	Hash          common.Hash           // Block hash where the snapshot was created
	ValSet        istanbul.ValidatorSet // Set of authorized validators at this moment
	Policy        uint64
	CommitteeSize uint64
}

func getGovernanceValue(gov *governance.Governance) (epoch uint64, policy uint64, committeeSize uint64) {
	if r := gov.GetGovernanceValue("istanbul.epoch"); r != nil {
		epoch = r.(uint64)
	} else {
		epoch = params.DefaultEpoch
	}

	if r := gov.GetGovernanceValue("istanbul.policy"); r != nil {
		policy = r.(uint64)
	} else {
		policy = params.DefaultProposerPolicy
	}

	if r := gov.GetGovernanceValue("istanbul.committeesize"); r != nil {
		committeeSize = r.(uint64)
	} else {
		committeeSize = params.DefaultSubGroupSize
	}
	return
}

// newSnapshot create a new snapshot with the specified startup parameters. This
// method does not initialize the set of recent validators, so only ever use if for
// the genesis block.
func newSnapshot(gov *governance.Governance, number uint64, hash common.Hash, valSet istanbul.ValidatorSet, chainConfig *params.ChainConfig) *Snapshot {
	epoch, policy, committeeSize := getGovernanceValue(gov)

	snap := &Snapshot{
		Epoch:         epoch,
		Number:        number,
		Hash:          hash,
		ValSet:        valSet,
		Policy:        policy,
		CommitteeSize: committeeSize,
	}
	return snap
}

// loadSnapshot loads an existing snapshot from the database.
func loadSnapshot(db database.DBManager, hash common.Hash) (*Snapshot, error) {
	blob, err := db.ReadIstanbulSnapshot(hash)
	if err != nil {
		return nil, err
	}
	snap := new(Snapshot)
	if err := json.Unmarshal(blob, snap); err != nil {
		return nil, err
	}
	return snap, nil
}

// store inserts the snapshot into the database.
func (s *Snapshot) store(db database.DBManager) error {
	blob, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return db.WriteIstanbulSnapshot(s.Hash, blob)
}

// copy creates a deep copy of the snapshot, though not the individual votes.
func (s *Snapshot) copy() *Snapshot {
	cpy := &Snapshot{
		Epoch:         s.Epoch,
		Number:        s.Number,
		Hash:          s.Hash,
		ValSet:        s.ValSet.Copy(),
		Policy:        s.Policy,
		CommitteeSize: s.CommitteeSize,
	}
	return cpy
}

// checkVote return whether it's a valid vote
func (s *Snapshot) checkVote(address common.Address, authorize bool) bool {
	_, validator := s.ValSet.GetByAddress(address)
	return (validator != nil && !authorize) || (validator == nil && authorize)
}

// apply creates a new authorization snapshot by applying the given headers to
// the original one.
func (s *Snapshot) apply(headers []*types.Header, gov *governance.Governance, addr common.Address, epoch uint64) (*Snapshot, error) {
	// Allow passing in no headers for cleaner code
	if len(headers) == 0 {
		return s, nil
	}
	// Sanity check that the headers can be applied
	for i := 0; i < len(headers)-1; i++ {
		if headers[i+1].Number.Uint64() != headers[i].Number.Uint64()+1 {
			return nil, errInvalidVotingChain
		}
	}
	if headers[0].Number.Uint64() != s.Number+1 {
		return nil, errInvalidVotingChain
	}

	// Copy values which might be changed by governance vote
	s.Epoch, s.Policy, s.CommitteeSize = getGovernanceValue(gov)

	// Iterate through the headers and create a new snapshot
	snap := s.copy()

	for _, header := range headers {
		// Remove any votes on checkpoint blocks
		number := header.Number.Uint64()

		// Resolve the authorization key and check against validators
		validator, err := ecrecover(header)
		if err != nil {
			return nil, err
		}
		if _, v := snap.ValSet.GetByAddress(validator); v == nil {
			return nil, errUnauthorized
		}

		snap.ValSet = gov.HandleGovernanceVote(snap.ValSet, header, validator, addr)

		if number%s.Epoch == 0 {
			if len(header.Governance) > 0 {
				go gov.UpdateGovernance(number, header.Governance)
			}
			gov.UpdateCurrentGovernance(number)
			gov.ClearVotes(number)
		}
	}
	snap.Number += uint64(len(headers))
	snap.Hash = headers[len(headers)-1].Hash()

	if snap.ValSet.Policy() == istanbul.WeightedRandom {
		// TODO-Klaytn-Issue1166 We have to update block number of ValSet too.
		snap.ValSet.SetBlockNum(snap.Number)
	}

	gov.SetTotalVotingPower(snap.ValSet.TotalVotingPower())
	gov.SetMyVotingPower(snap.getMyVotingPower(addr))

	return snap, nil
}

func (s *Snapshot) getMyVotingPower(addr common.Address) uint64 {
	for _, a := range s.ValSet.List() {
		if a.Address() == addr {
			return a.VotingPower()
		}
	}
	return 0
}

// validators retrieves the list of authorized validators in ascending order.
func (s *Snapshot) validators() []common.Address {
	validators := make([]common.Address, 0, s.ValSet.Size())
	for _, validator := range s.ValSet.List() {
		validators = append(validators, validator.Address())
	}
	return sortValidatorArray(validators)
}

func (s *Snapshot) committee(prevHash common.Hash, view *istanbul.View) []common.Address {
	committeeList := s.ValSet.SubList(prevHash, view)

	committee := make([]common.Address, 0, len(committeeList))
	for _, v := range committeeList {
		committee = append(committee, v.Address())
	}
	return committee
}

func sortValidatorArray(validators []common.Address) []common.Address {
	for i := 0; i < len(validators); i++ {
		for j := i + 1; j < len(validators); j++ {
			if bytes.Compare(validators[i][:], validators[j][:]) > 0 {
				validators[i], validators[j] = validators[j], validators[i]
			}
		}
	}
	return validators
}

type snapshotJSON struct {
	Epoch  uint64      `json:"epoch"`
	Number uint64      `json:"number"`
	Hash   common.Hash `json:"hash"`

	// for validator set
	Validators   []common.Address        `json:"validators"`
	Policy       istanbul.ProposerPolicy `json:"policy"`
	SubGroupSize uint64                  `json:"subgroupsize"`

	// for weighted validator
	RewardAddrs       []common.Address `json:"rewardAddrs"`
	VotingPowers      []uint64         `json:"votingPower"`
	Weights           []int            `json:"weight"`
	Proposers         []common.Address `json:"proposers"`
	ProposersBlockNum uint64           `json:"proposersBlockNum"`
}

func (s *Snapshot) toJSONStruct() *snapshotJSON {
	var rewardAddrs []common.Address
	var votingPowers []uint64
	var weights []int
	var proposers []common.Address
	var proposersBlockNum uint64

	// TODO-Klaytn-Issue1166 For weightedCouncil
	if s.ValSet.Policy() == istanbul.WeightedRandom {
		rewardAddrs, votingPowers, weights, proposers, proposersBlockNum = validator.GetWeightedCouncilData(s.ValSet)
	}

	return &snapshotJSON{
		Epoch:             s.Epoch,
		Number:            s.Number,
		Hash:              s.Hash,
		Validators:        s.validators(),
		Policy:            istanbul.ProposerPolicy(s.Policy),
		SubGroupSize:      s.CommitteeSize,
		RewardAddrs:       rewardAddrs,
		VotingPowers:      votingPowers,
		Weights:           weights,
		Proposers:         proposers,
		ProposersBlockNum: proposersBlockNum,
	}
}

// Unmarshal from a json byte array
func (s *Snapshot) UnmarshalJSON(b []byte) error {
	var j snapshotJSON
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}

	s.Epoch = j.Epoch
	s.Number = j.Number
	s.Hash = j.Hash

	// TODO-Klaytn-Issue1166 For weightedCouncil
	if j.Policy == istanbul.WeightedRandom {
		s.ValSet = validator.NewWeightedCouncil(j.Validators, j.RewardAddrs, j.VotingPowers, j.Weights, j.Policy, j.SubGroupSize, j.Number, j.ProposersBlockNum, nil)
		validator.RecoverWeightedCouncilProposer(s.ValSet, j.Proposers)
	} else {
		s.ValSet = validator.NewSubSet(j.Validators, j.Policy, j.SubGroupSize)
	}
	return nil
}

// Marshal to a json byte array
func (s *Snapshot) MarshalJSON() ([]byte, error) {
	j := s.toJSONStruct()
	return json.Marshal(j)
}
