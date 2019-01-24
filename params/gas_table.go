// Copyright 2018 The klaytn Authors
// Copyright 2015 The go-ethereum Authors
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
// This file is derived from params/gas_table.go (2018/06/04).
// Modified and improved for the klaytn development.

package params

// GasTable organizes gas prices for different Klaytn phases.
type GasTable struct {
	ExtcodeSize uint64
	ExtcodeCopy uint64
	Balance     uint64
	SLoad       uint64
	Calls       uint64
	Suicide     uint64

	ExpByte uint64

	// CreateBySuicide occurs when the
	// refunded account is one that does

	// not exist. This logic is similar
	// to call. May be left nil. Nil means
	// not charged.
	CreateBySuicide uint64
}

// Variables containing gas prices for different Klaytn phases.
var (
	// TODO-Klaytn-RemoveLater Remove GasTableHomestead
	// GasTableHomestead contain the gas prices for
	// the homestead phase.
	//GasTableHomestead = GasTable{
	//	ExtcodeSize: 20,
	//	ExtcodeCopy: 20,
	//	Balance:     20,
	//	SLoad:       50,
	//	Calls:       40,
	//	Suicide:     0,
	//	ExpByte:     10,
	//}

	// TODO-Klaytn-RemoveLater Remove GasTableEIP150
	// GasTableHomestead contain the gas re-prices for
	// the homestead phase.
	//GasTableEIP150 = GasTable{
	//	ExtcodeSize: 700,
	//	ExtcodeCopy: 700,
	//	Balance:     400,
	//	SLoad:       200,
	//	Calls:       700,
	//	Suicide:     5000,
	//	ExpByte:     10,
	//
	//	CreateBySuicide: 25000,
	//}

	// GasTableEIP158 contain the gas re-prices for
	// the EIP15* phase.
	GasTableEIP158 = GasTable{
		ExtcodeSize: 700,  // G_extcode
		ExtcodeCopy: 700,  // G_extcode
		Balance:     400,  // G_balance
		SLoad:       200,  // G_sload
		Calls:       700,  // G_call
		Suicide:     5000, // G_selfdestruct
		ExpByte:     50,   // G_expbyte

		CreateBySuicide: 25000, // G_newaccount
	}
)
