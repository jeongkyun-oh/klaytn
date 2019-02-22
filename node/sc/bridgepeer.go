// Copyright 2018 The klaytn Authors
// Copyright 2015 The go-ethereum Authors
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
// This file is derived from eth/peer.go (2018/06/04).
// Modified and improved for the klaytn development.

package sc

import (
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/networks/p2p"
	"github.com/ground-x/klaytn/networks/p2p/discover"
	"github.com/ground-x/klaytn/ser/rlp"
	"gopkg.in/fatih/set.v0"
	"math/big"
	"sync"
	"time"
)

var (
	errClosed            = errors.New("peer set is closed")
	errAlreadyRegistered = errors.New("peer is already registered")
	errNotRegistered     = errors.New("peer is not registered")
)

const (
	maxKnownTxs    = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks = 1024  // Maximum block hashes to keep in the known list (prevent DOS)

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 128

	// maxQueuedProps is the maximum number of block propagations to queue up before
	// dropping broadcasts. There's not much point in queueing stale blocks, so a few
	// that might cover uncles should be enough.
	maxQueuedProps = 4

	// maxQueuedAnns is the maximum number of block announcements to queue up before
	// dropping broadcasts. Similarly to block propagations, there's no point to queue
	// above some healthy uncle limit, so use that.
	maxQueuedAnns = 4

	handshakeTimeout = 5 * time.Second
)

// BridgePeerInfo represents a short summary of the Klaytn Bridge sub-protocol metadata known
// about a connected peer.
type BridgePeerInfo struct {
	Version int    `json:"version"` // Klaytn Bridge protocol version negotiated
	Head    string `json:"head"`    // SHA3 hash of the peer's best owned block
}

// propEvent is a block propagation, waiting for its turn in the broadcast queue.
type propEvent struct {
	block *types.Block
	td    *big.Int
}

type PeerSetManager interface {
	BridgePeerSet() *bridgePeerSet
}

type BridgePeer interface {
	// Close signals the broadcast goroutine to terminate.
	Close()

	// Info gathers and returns a collection of metadata known about a peer.
	Info() *BridgePeerInfo

	Head() (hash common.Hash, td *big.Int)

	// AddToKnownTxs adds a transaction to knownTxs for the peer, ensuring that it
	// will never be propagated to this particular peer.
	AddToKnownTxs(hash common.Hash)

	// Send writes an RLP-encoded message with the given code.
	// data should have been encoded as an RLP list.
	Send(msgcode uint64, data interface{}) error

	// SendNodeDataRLP sends a batch of arbitrary internal data, corresponding to the
	// hashes requested.
	SendNodeData(data [][]byte) error

	// SendReceiptsRLP sends a batch of transaction receipts, corresponding to the
	// ones requested from an already RLP encoded format.
	SendReceiptsRLP(receipts []rlp.RawValue) error

	// Handshake executes the klaytn protocol handshake, negotiating version number,
	// network IDs, difficulties, head, genesis blocks, and onChildChain(if the node is on child chain for the peer)
	// and returning if the peer on the same chain or not and error.
	Handshake(network uint64, chainID, td *big.Int, head common.Hash, genesis common.Hash) error

	// ConnType returns the conntype of the peer.
	ConnType() p2p.ConnType

	// GetID returns the id of the peer.
	GetID() string

	// GetP2PPeerID returns the id of the p2p.Peer.
	GetP2PPeerID() discover.NodeID

	// GetChainID returns the chain id of the peer.
	GetChainID() *big.Int

	// GetAddr returns the address of the peer.
	GetAddr() common.Address

	// SetAddr sets the address of the peer.
	SetAddr(addr common.Address)

	// GetVersion returns the version of the peer.
	GetVersion() int

	// GetKnownBlocks returns the knownBlocks of the peer.
	GetKnownBlocks() *set.Set

	// GetKnownTxs returns the knownBlocks of the peer.
	GetKnownTxs() *set.Set

	// GetP2PPeer returns the p2p.
	GetP2PPeer() *p2p.Peer

	// GetRW returns the MsgReadWriter of the peer.
	GetRW() p2p.MsgReadWriter

	// Handle is the callback invoked to manage the life cycle of a Klaytn Peer. When
	// this function terminates, the Peer is disconnected.
	Handle(bn *MainBridge) error

	// SendServiceChainTxs sends child chain tx data to from child chain to parent chain.
	SendServiceChainTxs(txs types.Transactions) error

	// SendServiceChainInfoRequest sends a parentChainInfo request from child chain to parent chain.
	SendServiceChainInfoRequest(addr *common.Address) error

	// SendServiceChainInfoResponse sends a parentChainInfo from parent chain to child chain.
	// parentChainInfo includes nonce of an account and gasPrice in the parent chain.
	SendServiceChainInfoResponse(pcInfo *parentChainInfo) error

	// SendServiceChainReceiptRequest sends a receipt request from child chain to parent chain.
	SendServiceChainReceiptRequest(txHashes []common.Hash) error

	// SendServiceChainReceiptResponse sends a receipt as a response to request from child chain.
	SendServiceChainReceiptResponse(receipts []*types.ReceiptForStorage) error
}

// baseBridgePeer is a common data structure used by implementation of Peer.
type baseBridgePeer struct {
	id string

	addr common.Address

	*p2p.Peer
	rw p2p.MsgReadWriter

	version  int         // Protocol version negotiated
	forkDrop *time.Timer // Timed connection dropper if forks aren't validated in time

	head common.Hash
	td   *big.Int
	lock sync.RWMutex

	knownTxs    *set.Set                  // Set of transaction hashes known to be known by this peer
	knownBlocks *set.Set                  // Set of block hashes known to be known by this peer
	queuedTxs   chan []*types.Transaction // Queue of transactions to broadcast to the peer
	queuedProps chan *propEvent           // Queue of blocks to broadcast to the peer
	queuedAnns  chan *types.Block         // Queue of blocks to announce to the peer
	term        chan struct{}             // Termination channel to stop the broadcaster

	chainID *big.Int // A child chain must know parent chain's ChainID to sign a transaction.
}

// newPeer returns new Peer interface.
func newBridgePeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) BridgePeer {
	id := p.ID()

	return &singleChannelPeer{
		baseBridgePeer: &baseBridgePeer{
			Peer:        p,
			rw:          rw,
			version:     version,
			id:          fmt.Sprintf("%x", id[:8]),
			knownTxs:    set.New(),
			knownBlocks: set.New(),
			queuedTxs:   make(chan []*types.Transaction, maxQueuedTxs),
			queuedProps: make(chan *propEvent, maxQueuedProps),
			queuedAnns:  make(chan *types.Block, maxQueuedAnns),
			term:        make(chan struct{}),
		},
	}
}

// Broadcast is a write loop that multiplexes block propagations, announcements
// and transaction broadcasts into the remote peer. The goal is to have an async
// writer that does not lock up node internals.
func (p *baseBridgePeer) Broadcast() {
	for {
		select {
		case txs := <-p.queuedTxs:
			if err := p.SendTransactions(txs); err != nil {
				logger.Error("fail to SendTransactions", "err", err)
				continue
				//return
			}
			p.Log().Trace("Broadcast transactions", "count", len(txs))

		case prop := <-p.queuedProps:
			if err := p.SendNewBlock(prop.block, prop.td); err != nil {
				logger.Error("fail to SendNewBlock", "err", err)
				continue
				//return
			}
			p.Log().Trace("Propagated block", "number", prop.block.Number(), "hash", prop.block.Hash(), "td", prop.td)

		//case block := <-p.queuedAnns:
		//	if err := p.SendNewBlockHashes([]common.Hash{block.Hash()}, []uint64{block.NumberU64()}); err != nil {
		//		logger.Error("fail to SendNewBlockHashes", "err", err)
		//		continue
		//		//return
		//	}
		//	p.Log().Trace("Announced block", "number", block.Number(), "hash", block.Hash())

		case <-p.term:
			p.Log().Debug("Peer broadcast loop end")
			return
		}
	}
}

// Close signals the broadcast goroutine to terminate.
func (p *baseBridgePeer) Close() {
	close(p.term)
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *baseBridgePeer) Info() *BridgePeerInfo {
	hash, _ := p.Head()

	return &BridgePeerInfo{
		Version: p.version,
		Head:    hash.Hex(),
	}
}

// Head retrieves a copy of the current head hash and total difficulty of the
// peer.
func (p *baseBridgePeer) Head() (hash common.Hash, td *big.Int) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(hash[:], p.head[:])
	return hash, new(big.Int).Set(p.td)
}

// SetHead updates the head hash and total difficulty of the peer.
func (p *baseBridgePeer) SetHead(hash common.Hash, td *big.Int) {
	p.lock.Lock()
	defer p.lock.Unlock()

	copy(p.head[:], hash[:])
	p.td.Set(td)
}

// AddToKnownBlocks adds a block to knownBlocks for the peer, ensuring that the block will
// never be propagated to this particular peer.
func (p *baseBridgePeer) AddToKnownBlocks(hash common.Hash) {
	if !p.knownBlocks.Has(hash) {
		// If we reached the memory allowance, drop a previously known block hash
		for p.knownBlocks.Size() >= maxKnownBlocks {
			p.knownBlocks.Pop()
		}
		p.knownBlocks.Add(hash)
	}
}

// AddToKnownTxs adds a transaction to knownTxs for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *baseBridgePeer) AddToKnownTxs(hash common.Hash) {
	if !p.knownTxs.Has(hash) {
		// If we reached the memory allowance, drop a previously known transaction hash
		for p.knownTxs.Size() >= maxKnownTxs {
			p.knownTxs.Pop()
		}
		p.knownTxs.Add(hash)
	}
}

// Send writes an RLP-encoded message with the given code.
// data should have been encoded as an RLP list.
func (p *baseBridgePeer) Send(msgcode uint64, data interface{}) error {
	return p2p.Send(p.rw, msgcode, data)
}

// SendTransactions sends transactions to the peer and includes the hashes
// in its transaction hash set for future reference.
func (p *baseBridgePeer) SendTransactions(txs types.Transactions) error {
	for _, tx := range txs {
		p.AddToKnownTxs(tx.Hash())
	}
	return p2p.Send(p.rw, TxMsg, txs)
}

// ReSendTransactions sends txs to a peer in order to prevent the txs from missing.
func (p *baseBridgePeer) ReSendTransactions(txs types.Transactions) error {
	return p2p.Send(p.rw, TxMsg, txs)
}

func (p *baseBridgePeer) AsyncSendTransactions(txs []*types.Transaction) {
	select {
	case p.queuedTxs <- txs:
		for _, tx := range txs {
			p.AddToKnownTxs(tx.Hash())
		}
	default:
		p.Log().Debug("Dropping transaction propagation", "count", len(txs))
	}
}

// AsyncSendNewBlockHash queues the availability of a block for propagation to a
// remote peer. If the peer's broadcast queue is full, the event is silently
// dropped.
func (p *baseBridgePeer) AsyncSendNewBlockHash(block *types.Block) {
	select {
	case p.queuedAnns <- block:
		p.knownBlocks.Add(block.Hash())
	default:
		p.Log().Debug("Dropping block announcement", "number", block.NumberU64(), "hash", block.Hash())
	}
}

// SendNewBlock propagates an entire block to a remote peer.
func (p *baseBridgePeer) SendNewBlock(block *types.Block, td *big.Int) error {
	p.knownBlocks.Add(block.Hash())
	return p2p.Send(p.rw, NewBlockMsg, []interface{}{block, td})
}

// AsyncSendNewBlock queues an entire block for propagation to a remote peer. If
// the peer's broadcast queue is full, the event is silently dropped.
func (p *baseBridgePeer) AsyncSendNewBlock(block *types.Block, td *big.Int) {
	select {
	case p.queuedProps <- &propEvent{block: block, td: td}:
		p.knownBlocks.Add(block.Hash())
	default:
		p.Log().Debug("Dropping block propagation", "number", block.NumberU64(), "hash", block.Hash())
	}
}

// SendBlockHeaders sends a batch of block headers to the remote peer.
func (p *baseBridgePeer) SendBlockHeaders(headers []*types.Header) error {
	return p2p.Send(p.rw, BlockHeadersMsg, headers)
}

// SendBlockBodiesRLP sends a batch of block contents to the remote peer from
// an already RLP encoded format.
func (p *baseBridgePeer) SendBlockBodiesRLP(bodies []rlp.RawValue) error {
	return p2p.Send(p.rw, BlockBodiesMsg, bodies)
}

// SendNodeDataRLP sends a batch of arbitrary internal data, corresponding to the
// hashes requested.
func (p *baseBridgePeer) SendNodeData(data [][]byte) error {
	return p2p.Send(p.rw, NodeDataMsg, data)
}

// SendReceiptsRLP sends a batch of transaction receipts, corresponding to the
// ones requested from an already RLP encoded format.
func (p *baseBridgePeer) SendReceiptsRLP(receipts []rlp.RawValue) error {
	return p2p.Send(p.rw, ReceiptsMsg, receipts)
}

// RequestBodies fetches a batch of blocks' bodies corresponding to the hashes
// specified.
func (p *baseBridgePeer) RequestBodies(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of block bodies", "count", len(hashes))
	return p2p.Send(p.rw, BlockBodiesRequestMsg, hashes)
}

// RequestNodeData fetches a batch of arbitrary data from a node's known state
// data, corresponding to the specified hashes.
func (p *baseBridgePeer) RequestNodeData(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of state data", "count", len(hashes))
	return p2p.Send(p.rw, NodeDataRequestMsg, hashes)
}

// RequestReceipts fetches a batch of transaction receipts from a remote node.
func (p *baseBridgePeer) RequestReceipts(hashes []common.Hash) error {
	p.Log().Debug("Fetching batch of receipts", "count", len(hashes))
	return p2p.Send(p.rw, ReceiptsRequestMsg, hashes)
}

func (p *baseBridgePeer) SendServiceChainTxs(txs types.Transactions) error {
	return p2p.Send(p.rw, ServiceChainTxsMsg, txs)
}

func (p *baseBridgePeer) SendServiceChainInfoRequest(addr *common.Address) error {
	return p2p.Send(p.rw, ServiceChainParentChainInfoRequestMsg, addr)
}

func (p *baseBridgePeer) SendServiceChainInfoResponse(pcInfo *parentChainInfo) error {
	return p2p.Send(p.rw, ServiceChainParentChainInfoResponseMsg, pcInfo)
}

func (p *baseBridgePeer) SendServiceChainReceiptRequest(txHashes []common.Hash) error {
	return p2p.Send(p.rw, ServiceChainReceiptRequestMsg, txHashes)
}

func (p *baseBridgePeer) SendServiceChainReceiptResponse(receipts []*types.ReceiptForStorage) error {
	return p2p.Send(p.rw, ServiceChainReceiptResponseMsg, receipts)
}

// Handshake executes the klaytn protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *baseBridgePeer) Handshake(network uint64, chainID, td *big.Int, head common.Hash, genesis common.Hash) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status statusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, StatusMsg, &statusData{
			ProtocolVersion: uint32(p.version),
			NetworkId:       network,
			TD:              td,
			CurrentBlock:    head,
			GenesisBlock:    genesis,
			ChainID:         chainID,
		})
	}()
	go func() {
		e := p.readStatus(network, &status, genesis)
		if e != nil {
			errc <- e
			return
		}
		errc <- e
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return err
			}
		case <-timeout.C:
			return p2p.DiscReadTimeout
		}
	}
	p.td, p.head, p.chainID = status.TD, status.CurrentBlock, status.ChainID
	return nil
}

func (p *baseBridgePeer) readStatus(network uint64, status *statusData, genesis common.Hash) error {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != StatusMsg {
		return errResp(ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, StatusMsg)
	}
	if msg.Size > ProtocolMaxMsgSize {
		return errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
	}
	// Decode the handshake and make sure everything matches
	if err := msg.Decode(&status); err != nil {
		return errResp(ErrDecode, "msg %v: %v", msg, err)
	}
	//if status.GenesisBlock != genesis {
	//	return errResp(ErrGenesisBlockMismatch, "%x (!= %x)", status.GenesisBlock[:8], genesis[:8])
	//}
	if status.NetworkId != network {
		return errResp(ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	if int(status.ProtocolVersion) != p.version {
		return errResp(ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	return nil
}

// String implements fmt.Stringer.
func (p *baseBridgePeer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("klay/%2d", p.version),
	)
}

// ConnType returns the conntype of the peer.
func (p *baseBridgePeer) ConnType() p2p.ConnType {
	return p.Peer.ConnType()
}

// GetID returns the id of the peer.
func (p *baseBridgePeer) GetID() string {
	return p.id
}

// GetP2PPeerID returns the id of the p2p.Peer.
func (p *baseBridgePeer) GetP2PPeerID() discover.NodeID {
	return p.Peer.ID()
}

// GetChainID returns the chain id of the peer.
func (p *baseBridgePeer) GetChainID() *big.Int {
	return p.chainID
}

// GetAddr returns the address of the peer.
func (p *baseBridgePeer) GetAddr() common.Address {
	return p.addr
}

// SetAddr sets the address of the peer.
func (p *baseBridgePeer) SetAddr(addr common.Address) {
	p.addr = addr
}

// GetVersion returns the version of the peer.
func (p *baseBridgePeer) GetVersion() int {
	return p.version
}

// GetKnownBlocks returns the knownBlocks of the peer.
func (p *baseBridgePeer) GetKnownBlocks() *set.Set {
	return p.knownBlocks
}

// GetKnownTxs returns the knownBlocks of the peer.
func (p *baseBridgePeer) GetKnownTxs() *set.Set {
	return p.knownTxs
}

// GetP2PPeer returns the p2p.Peer.
func (p *baseBridgePeer) GetP2PPeer() *p2p.Peer {
	return p.Peer
}

// GetRW returns the MsgReadWriter of the peer.
func (p *baseBridgePeer) GetRW() p2p.MsgReadWriter {
	return p.rw
}

// Handle is the callback invoked to manage the life cycle of a Klaytn Peer. When
// this function terminates, the Peer is disconnected.
func (p *baseBridgePeer) Handle(bn *MainBridge) error {
	return bn.handle(p)
}

// singleChannelPeer is a peer that uses a single channel.
type singleChannelPeer struct {
	*baseBridgePeer
}

// bridgePeerSet represents the collection of active peers currently participating in
// the Klaytn sub-protocol.
type bridgePeerSet struct {
	peers  map[string]BridgePeer
	lock   sync.RWMutex
	closed bool
}

// newBridgePeerSet creates a new peer set to track the active participants.
func newBridgePeerSet() *bridgePeerSet {
	peerSet := &bridgePeerSet{
		peers: make(map[string]BridgePeer),
	}

	return peerSet
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known.
func (ps *bridgePeerSet) Register(p BridgePeer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.GetID()]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.GetID()] = p

	return nil
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *bridgePeerSet) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.Close()

	return nil
}

// istanbul BFT
func (ps *bridgePeerSet) Peers() map[string]BridgePeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	set := make(map[string]BridgePeer)
	for id, p := range ps.peers {
		set[id] = p
	}
	return set
}

// Peer retrieves the registered peer with the given id.
func (ps *bridgePeerSet) Peer(id string) BridgePeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len returns if the current number of peers in the set.
func (ps *bridgePeerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// PeersWithoutBlock retrieves a list of peers that do not have a given block in
// their set of known hashes.
func (ps *bridgePeerSet) PeersWithoutBlock(hash common.Hash) []BridgePeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]BridgePeer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.GetKnownBlocks().Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *bridgePeerSet) PeersWithoutTx(hash common.Hash) []BridgePeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]BridgePeer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.GetKnownTxs().Has(hash) {
			list = append(list, p)
		}
	}
	return list
}

// BestPeer retrieves the known peer with the currently highest total difficulty.
func (ps *bridgePeerSet) BestPeer() BridgePeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var (
		bestPeer BridgePeer
		bestTd   *big.Int
	)
	for _, p := range ps.peers {
		if _, td := p.Head(); bestPeer == nil || td.Cmp(bestTd) > 0 {
			bestPeer, bestTd = p, td
		}
	}
	return bestPeer
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *bridgePeerSet) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.GetP2PPeer().Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}