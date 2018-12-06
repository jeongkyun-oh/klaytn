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

package params

import (
	"math/big"
	"time"
)

var (
	TargetGasLimit = GenesisGasLimit // The artificial target
)

const (
	// Fee schedule parameters

	CallValueTransferGas  uint64 = 9000  // Paid for CALL when the value transfer is non-zero.                  // G_callvalue
	CallNewAccountGas     uint64 = 25000 // Paid for CALL when the destination address didn't exist prior.      // G_newaccount
	TxGas                 uint64 = 21000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions. // G_transaction
	TxGasContractCreation uint64 = 53000 // Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions. // G_transaction + G_create
	TxDataZeroGas         uint64 = 4     // Per byte of data attached to a transaction that equals zero. NOTE: Not payable on data of calls between transactions. // G_txdatazero
	QuadCoeffDiv          uint64 = 512   // Divisor for the quadratic particle of the memory cost equation.
	SstoreSetGas          uint64 = 20000 // Once per SLOAD operation.                                           // G_sset
	LogDataGas            uint64 = 8     // Per byte in a LOG* operation's data.                                // G_logdata
	CallStipend           uint64 = 2300  // Free gas given at beginning of call.                                // G_callstipend
	Sha3Gas               uint64 = 30    // Once per SHA3 operation.                                                 // G_sha3
	Sha3WordGas           uint64 = 6     // Once per word of the SHA3 operation's data.                              // G_sha3word
	SstoreResetGas        uint64 = 5000  // Once per SSTORE operation if the zeroness changes from zero.             // G_sreset
	SstoreClearGas        uint64 = 5000  // Once per SSTORE operation if the zeroness doesn't change.                // G_sreset
	SstoreRefundGas       uint64 = 15000 // Once per SSTORE operation if the zeroness changes to zero.               // R_sclear
	JumpdestGas           uint64 = 1     // Refunded gas, once per SSTORE operation if the zeroness changes to zero. // G_jumpdest
	CreateDataGas         uint64 = 200   // Paid per byte for a CREATE operation to succeed in placing code into state. // G_codedeposit
	LogGas                uint64 = 375   // Per LOG* operation.                                                          // G_log
	CopyGas               uint64 = 3     // Partial payment for COPY operations, multiplied by words copied, rounded up. // G_copy
	CreateGas             uint64 = 32000 // Once per CREATE operation & contract-creation transaction.               // G_create
	SuicideRefundGas      uint64 = 24000 // Refunded following a suicide operation.                                  // R_selfdestruct
	MemoryGas             uint64 = 3     // Times the address of the (highest referenced byte in memory + 1). NOTE: referencing happens on read, write and in instructions such as RETURN and CALL. // G_memory
	LogTopicGas           uint64 = 375   // Multiplied by the * of the LOG*, per LOG transaction. e.g. LOG0 incurs 0 * c_txLogTopicGas, LOG4 incurs 4 * c_txLogTopicGas.   // G_logtopic
	TxDataNonZeroGas      uint64 = 68    // Per byte of data attached to a transaction that is not equal to zero. NOTE: Not payable on data of calls between transactions. // G_txdatanonzero

	// Precompiled contract gas prices

	EcrecoverGas            uint64 = 3000   // Elliptic curve sender recovery gas price
	Sha256BaseGas           uint64 = 60     // Base price for a SHA256 operation
	Sha256PerWordGas        uint64 = 12     // Per-word price for a SHA256 operation
	Ripemd160BaseGas        uint64 = 600    // Base price for a RIPEMD160 operation
	Ripemd160PerWordGas     uint64 = 120    // Per-word price for a RIPEMD160 operation
	IdentityBaseGas         uint64 = 15     // Base price for a data copy operation
	IdentityPerWordGas      uint64 = 3      // Per-work price for a data copy operation
	ModExpQuadCoeffDiv      uint64 = 20     // Divisor for the quadratic particle of the big int modular exponentiation
	Bn256AddGas             uint64 = 500    // Gas needed for an elliptic curve addition
	Bn256ScalarMulGas       uint64 = 40000  // Gas needed for an elliptic curve scalar multiplication
	Bn256PairingBaseGas     uint64 = 100000 // Base price for an elliptic curve pairing check
	Bn256PairingPerPointGas uint64 = 80000  // Per-point price for an elliptic curve pairing check

	GasLimitBoundDivisor uint64 = 1024    // The bound divisor of the gas limit, used in update calculations.
	MinGasLimit          uint64 = 5000    // Minimum the gas limit may ever be.
	GenesisGasLimit      uint64 = 4712388 // Gas limit of the Genesis block.

	MaximumExtraDataSize uint64 = 32 // Maximum size extra data may be after Genesis.

	EpochDuration   uint64 = 30000 // Duration between proof-of-work epochs.
	CallCreateDepth uint64 = 1024  // Maximum depth of call/create stack.
	StackLimit      uint64 = 1024  // Maximum size of VM stack allowed.

	MaxCodeSize = 24576 // Maximum bytecode to permit for a contract

	// istanbul BFT
	BFTMaximumExtraDataSize uint64 = 65 // Maximum size extra data may be after Genesis.
)

var (
	DifficultyBoundDivisor = big.NewInt(2048)   // The bound divisor of the difficulty, used in the update calculations.
	GenesisDifficulty      = big.NewInt(131072) // Difficulty of the Genesis block.
	MinimumDifficulty      = big.NewInt(131072) // The minimum that the difficulty may ever be.
	DurationLimit          = big.NewInt(13)     // The decision boundary on the blocktime duration used to determine whether difficulty should go up or not.
)

// Parameters for execution time limit
var (
	// TODO-GX Determine more practical values through actual running experience
	TotalTimeLimit = 250 * time.Millisecond // Execution time limit for all txs in a block
	OpcodeCntLimit = uint64(3000000)        // Opcode count limit for tx
)

// istanbul BFT
func GetMaximumExtraDataSize(isBFT bool) uint64 {
	if isBFT {
		return BFTMaximumExtraDataSize
	} else {
		return MaximumExtraDataSize
	}
}
