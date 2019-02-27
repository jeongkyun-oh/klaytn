// Copyright 2018 The klaytn Authors
// Copyright 2014 The go-ethereum Authors
// This file is part of go-ethereum.
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
// This file is derived from eth/protocol.go (2018/06/04).
// Modified and improved for the klaytn development.

package cn

import (
	"fmt"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/event"
	"github.com/ground-x/klaytn/ser/rlp"
	"io"
	"math/big"
)

// Constants to match up protocol versions and messages
const (
	klay62 = 62
	klay63 = 63
)

// ProtocolName is the official short name of the protocol used during capability negotiation.
var ProtocolName = "klay"

// ProtocolVersions are the upported versions of the klay protocol (first is primary).
var ProtocolVersions = []uint{klay63, klay62}

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{17, 8}

const ProtocolMaxMsgSize = 10 * 1024 * 1024 // Maximum cap on the size of a protocol message

// klaytn protocol message codes
// TODO-Klaytn-Issue751 Protocol message should be refactored. Present code is not used.
const (
	// Protocol messages belonging to klay/62
	StatusMsg                              = 0x00
	NewBlockHashesMsg                      = 0x01
	BlockHeaderFetchRequestMsg             = 0x02
	BlockHeaderFetchResponseMsg            = 0x03
	BlockBodiesFetchRequestMsg             = 0x04
	BlockBodiesFetchResponseMsg            = 0x05
	TxMsg                                  = 0x06
	BlockHeadersRequestMsg                 = 0x07
	BlockHeadersMsg                        = 0x08
	BlockBodiesRequestMsg                  = 0x09
	BlockBodiesMsg                         = 0x0a
	NewBlockMsg                            = 0x0b
	ServiceChainTxsMsg                     = 0x0c
	ServiceChainReceiptResponseMsg         = 0x0d
	ServiceChainReceiptRequestMsg          = 0x0e
	ServiceChainParentChainInfoResponseMsg = 0x0f
	ServiceChainParentChainInfoRequestMsg  = 0x10

	// Protocol messages belonging to klay/63
	NodeDataRequestMsg = 0x11
	NodeDataMsg        = 0x12
	ReceiptsRequestMsg = 0x13
	ReceiptsMsg        = 0x14
)

type errCode int

const (
	ErrMsgTooLarge = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
	ErrInvalidPeerHierarchy
	ErrUnexpectedTxType
	ErrFailedToGetStateDB
)

func (e errCode) String() string {
	return errorToString[int(e)]
}

// XXX change once legacy code is out
var errorToString = map[int]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
	ErrInvalidPeerHierarchy:    "InvalidPeerHierarchy",
	ErrUnexpectedTxType:        "Unexpected tx type",
	ErrFailedToGetStateDB:      "Failed to get stateDB",
}

type txPool interface {
	// AddRemotes should add the given transactions to the pool.
	AddRemotes([]*types.Transaction) []error

	// Pending should return pending transactions.
	// The slice should be modifiable by the caller.
	Pending() (map[common.Address]types.Transactions, error)

	// SubscribeNewTxsEvent should return an event subscription of
	// NewTxsEvent and send events to the given channel.
	SubscribeNewTxsEvent(chan<- blockchain.NewTxsEvent) event.Subscription
}

// statusData is the network packet for the status message.
type statusData struct {
	ProtocolVersion uint32
	NetworkId       uint64
	TD              *big.Int
	CurrentBlock    common.Hash
	GenesisBlock    common.Hash
	ChainID         *big.Int // A child chain must know parent chain's ChainID to sign a transaction.
	OnChildChain    bool     // OnChildChain presents if the peer is on child chain or not(same chain or parent chain).
}

// newBlockHashesData is the network packet for the block announcements.
type newBlockHashesData []struct {
	Hash   common.Hash // Hash of one particular block being announced
	Number uint64      // Number of one particular block being announced
}

// getBlockHeadersData represents a block header query.
type getBlockHeadersData struct {
	Origin  hashOrNumber // Block from which to retrieve headers
	Amount  uint64       // Maximum number of headers to retrieve
	Skip    uint64       // Blocks to skip between consecutive headers
	Reverse bool         // Query direction (false = rising towards latest, true = falling towards genesis)
}

// hashOrNumber is a combined field for specifying an origin block.
type hashOrNumber struct {
	Hash   common.Hash // Block hash from which to retrieve headers (excludes Number)
	Number uint64      // Block hash from which to retrieve headers (excludes Hash)
}

// EncodeRLP is a specialized encoder for hashOrNumber to encode only one of the
// two contained union fields.
func (hn *hashOrNumber) EncodeRLP(w io.Writer) error {
	if hn.Hash == (common.Hash{}) {
		return rlp.Encode(w, hn.Number)
	}
	if hn.Number != 0 {
		return fmt.Errorf("both origin hash (%x) and number (%d) provided", hn.Hash, hn.Number)
	}
	return rlp.Encode(w, hn.Hash)
}

// DecodeRLP is a specialized decoder for hashOrNumber to decode the contents
// into either a block hash or a block number.
func (hn *hashOrNumber) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	origin, err := s.Raw()
	if err == nil {
		switch {
		case size == 32:
			err = rlp.DecodeBytes(origin, &hn.Hash)
		case size <= 8:
			err = rlp.DecodeBytes(origin, &hn.Number)
		default:
			err = fmt.Errorf("invalid input size %d for origin", size)
		}
	}
	return err
}

// newBlockData is the network packet for the block propagation message.
type newBlockData struct {
	Block *types.Block
	TD    *big.Int
}

// blockBody represents the data content of a single block.
type blockBody struct {
	Transactions []*types.Transaction // Transactions contained within a block
	Uncles       []*types.Header      // Uncles contained within a block
}

// blockBodiesData is the network packet for block content distribution.
type blockBodiesData []*blockBody
