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

package fork

const (
	// HardFork block numbers for Baobab
	FirstBaobabHardFork uint64 = 86400 * 37
)

var (
	// hardForkConfig is a global variable defined in params package.
	// This value will not be changed unless it is a test code.
	// The test code can override this value via `UpdateHardForkConfig`.
	hardForkConfig = &HardForkConfig{
		RoleBasedRLPFixBlockNumber: FirstBaobabHardFork,
	}
)

// HardForkConfig defines a block number for each hard fork feature.
type HardForkConfig struct {
	RoleBasedRLPFixBlockNumber uint64
}

// IsRoleBasedRLPFixEnabled returns true if the blockNumber is greater than or equal to
// the block number defined in hardForkConfig.
func IsRoleBasedRLPFixEnabled(blockNumber uint64) bool {
	return blockNumber >= hardForkConfig.RoleBasedRLPFixBlockNumber
}

// UpdateHardForkConfig sets values in HardForkConfig if it is not nil.
// NOTE: this is only for test code to give flexibility of the test code.
func UpdateHardForkConfig(h *HardForkConfig) {
	if h != nil {
		hardForkConfig = h
	}
}
