package consensus

import (
	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/core/types"
)

// Constants to match up protocol versions and messages
const (
	Gxp62 = 62
	Gxp63 = 63

	// istanbul msg-code for ranger node
	PoRMsg     = 0x12
	PoRSendMsg = 0x13
)

var (
	GxpProtocol = Protocol{
		Name:     "gxp",
		Versions: []uint{Gxp63, Gxp62},
		Lengths:  []uint64{17, 8},
	}
)

// Protocol defines the protocol of the consensus
type Protocol struct {
	// Official short name of the protocol used during capability negotiation.
	Name string
	// Supported versions of the eth protocol (first is primary).
	Versions []uint
	// Number of implemented message corresponding to different protocol versions.
	Lengths []uint64
}

// istanbul BFT
// Broadcaster defines the interface to enqueue blocks to fetcher and find peer
type Broadcaster interface {
	// Enqueue add a block into fetcher queue
	Enqueue(id string, block *types.Block)
	// FindPeers retrives peers by addresses
	FindPeers(map[common.Address]bool) map[common.Address]Peer

    GetPeers() []common.Address
}

// Peer defines the interface to communicate with peer
type Peer interface {
	// Send sends the message to this peer
	Send(msgcode uint64, data interface{}) error
}
