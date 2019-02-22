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
// This file is derived from eth/backend.go (2018/06/04).
// Modified and improved for the klaytn development.

package sc

import (
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/accounts"
	"github.com/ground-x/klaytn/api"
	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/blockchain/types"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/crypto"
	"github.com/ground-x/klaytn/event"
	"github.com/ground-x/klaytn/networks/p2p"
	"github.com/ground-x/klaytn/networks/p2p/discover"
	"github.com/ground-x/klaytn/networks/rpc"
	"github.com/ground-x/klaytn/node"
	"github.com/ground-x/klaytn/params"
	"github.com/ground-x/klaytn/ser/rlp"
	"github.com/ground-x/klaytn/storage/database"
	"math/big"
	"sync"
	"time"
)

const (
	forceSyncCycle = 10 * time.Second // Time interval to force syncs, even if few peers are available
)

// NodeInfo represents a short summary of the Ethereum sub-protocol metadata
// known about the host peer.
type SubBridgeInfo struct {
	Network    uint64              `json:"network"`    // Ethereum network ID (1=Frontier, 2=Morden, Ropsten=3, Rinkeby=4)
	Difficulty *big.Int            `json:"difficulty"` // Total difficulty of the host's blockchain
	Genesis    common.Hash         `json:"genesis"`    // SHA3 hash of the host's genesis block
	Config     *params.ChainConfig `json:"config"`     // Chain configuration for the fork rules
	Head       common.Hash         `json:"head"`       // SHA3 hash of the host's best owned block
}

// CN implements the Klaytn consensus node service.
type SubBridge struct {
	config *SCConfig

	// DB interfaces
	chainDB database.DBManager // Block chain database

	eventMux       *event.TypeMux
	accountManager *accounts.Manager

	gasPrice *big.Int

	networkId     uint64
	netRPCService *api.PublicNetAPI

	lock sync.RWMutex // Protects the variadic fields (klay.g. gas price and coinbase)

	bridgeServer p2p.Server
	ctx          *node.ServiceContext
	maxPeers     int

	APIBackend *SubBridgeAPI

	// channels for fetcher, syncer, txsyncLoop
	newPeerCh   chan BridgePeer
	quitSync    chan struct{}
	noMorePeers chan struct{}

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg   sync.WaitGroup
	pmwg sync.WaitGroup

	blockchain *blockchain.BlockChain
	txPool     *blockchain.TxPool

	// chain event
	chainHeadCh  chan blockchain.ChainHeadEvent
	chainHeadSub event.Subscription
	txCh         chan blockchain.NewTxsEvent
	txSub        event.Subscription

	peers        *bridgePeerSet
	handler      *SubBridgeHandler
	eventhandler *ChildChainEventHandler
	//fetcher    *fetcher.Fetcher
}

// New creates a new CN object (including the
// initialisation of the common CN object)
func NewSubBridge(ctx *node.ServiceContext, config *SCConfig) (*SubBridge, error) {
	chainDB, err := CreateDB(ctx, config, "subbridgedata")
	if err != nil {
		return nil, err
	}

	config.chainkey = config.ChainKey()

	//config.chainkey, err = crypto.GenerateKey()
	//pubkey := crypto.PubkeyToAddress(config.chainkey.PublicKey)
	//config.ChainAccountAddr = &pubkey
	//config.AnchoringPeriod = uint64(1)
	//if err != nil {
	//	return nil, err
	//}
	sc := &SubBridge{
		config:         config,
		chainDB:        chainDB,
		peers:          newBridgePeerSet(),
		newPeerCh:      make(chan BridgePeer),
		noMorePeers:    make(chan struct{}),
		eventMux:       ctx.EventMux,
		accountManager: ctx.AccountManager,
		networkId:      config.NetworkId,
		ctx:            ctx,
		chainHeadCh:    make(chan blockchain.ChainHeadEvent, chainHeadChanSize),
		txCh:           make(chan blockchain.NewTxsEvent, transactionChanSize),
		quitSync:       make(chan struct{}),
		maxPeers:       config.MaxPeer,
	}

	logger.Info("Initialising Klaytn-Bridge protocol", "network", config.NetworkId)

	bcVersion := chainDB.ReadDatabaseVersion()
	if bcVersion != blockchain.BlockChainVersion && bcVersion != 0 {
		return nil, fmt.Errorf("Blockchain DB version mismatch (%d / %d). Run klay upgradedb.\n", bcVersion, blockchain.BlockChainVersion)
	}
	chainDB.WriteDatabaseVersion(blockchain.BlockChainVersion)

	sc.APIBackend = &SubBridgeAPI{sc}
	sc.handler, err = NewSubBridgeHandler(sc)
	if err != nil {
		return nil, err
	}
	sc.eventhandler, err = NewChildChainEventHandler(sc)
	if err != nil {
		return nil, err
	}

	return sc, nil
}

// implement PeerSetManager
func (sb *SubBridge) BridgePeerSet() *bridgePeerSet {
	return sb.peers
}

func (mb *SubBridge) GetEventHadler() ChainEventHandler {
	return mb.eventhandler
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *SubBridge) APIs() []rpc.API {
	// Append all the local APIs and return
	return []rpc.API{
		{
			Namespace: "bridge",
			Version:   "1.0",
			Service:   s.APIBackend,
			Public:    true,
		},
		{
			Namespace: "bridge",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		},
	}
}

func (s *SubBridge) AccountManager() *accounts.Manager { return s.accountManager }
func (s *SubBridge) EventMux() *event.TypeMux          { return s.eventMux }
func (s *SubBridge) ChainDB() database.DBManager       { return s.chainDB }
func (s *SubBridge) IsListening() bool                 { return true } // Always listening
func (s *SubBridge) ProtocolVersion() int              { return int(s.SCProtocol().Versions[0]) }
func (s *SubBridge) NetVersion() uint64                { return s.networkId }

func (s *SubBridge) Components() []interface{} {
	return nil
}

func (sc *SubBridge) SetComponents(components []interface{}) {
	for _, component := range components {
		switch v := component.(type) {
		case *blockchain.BlockChain:
			sc.blockchain = v
			// event from core-service
			sc.chainHeadSub = sc.blockchain.SubscribeChainHeadEvent(sc.chainHeadCh)

		case *blockchain.TxPool:
			sc.txPool = v
			// event from core-service
			sc.txSub = sc.txPool.SubscribeNewTxsEvent(sc.txCh)
		}
	}

	sc.pmwg.Add(1)
	go sc.loop()
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *SubBridge) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (s *SubBridge) SCProtocol() SCProtocol {
	return SCProtocol{
		Name:     "servicechain",
		Versions: []uint{1},
		Lengths:  []uint64{20},
	}
}

// NodeInfo retrieves some protocol metadata about the running host node.
func (pm *SubBridge) NodeInfo() *MainBridgeInfo {
	currentBlock := pm.blockchain.CurrentBlock()
	return &MainBridgeInfo{
		Network:    pm.networkId,
		Difficulty: pm.blockchain.GetTd(currentBlock.Hash(), currentBlock.NumberU64()),
		Genesis:    pm.blockchain.Genesis().Hash(),
		Config:     pm.blockchain.Config(),
		Head:       currentBlock.Hash(),
	}
}

// getChainID returns the current chain id.
func (pm *SubBridge) getChainID() *big.Int {
	return pm.blockchain.Config().ChainID
}

// Start implements node.Service, starting all internal goroutines needed by the
// Klaytn protocol implementation.
func (s *SubBridge) Start(srvr p2p.Server) error {

	serverConfig := p2p.Config{}
	serverConfig.PrivateKey = s.ctx.NodeKey()
	serverConfig.Name = s.ctx.NodeType().String()
	serverConfig.Logger = logger
	serverConfig.ListenAddr = s.config.BridgePort
	serverConfig.MaxPeers = s.maxPeers
	serverConfig.NoDiscovery = true
	serverConfig.EnableMultiChannelServer = false

	// connect to mainbridge as outbound
	serverConfig.StaticNodes = s.config.MainBridges()

	p2pServer := p2p.NewServer(serverConfig)

	s.bridgeServer = p2pServer

	scprotocols := make([]p2p.Protocol, 0, len(s.SCProtocol().Versions))
	for i, version := range s.SCProtocol().Versions {
		// Compatible; initialise the sub-protocol
		version := version
		scprotocols = append(scprotocols, p2p.Protocol{
			Name:    s.SCProtocol().Name,
			Version: version,
			Length:  s.SCProtocol().Lengths[i],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := s.newPeer(int(version), p, rw)
				pubKey, _ := p.ID().Pubkey()
				addr := crypto.PubkeyToAddress(*pubKey)
				peer.SetAddr(addr)
				select {
				case s.newPeerCh <- peer:
					s.wg.Add(1)
					defer s.wg.Done()
					return s.handle(peer)
				case <-s.quitSync:
					return p2p.DiscQuitting
				}
			},
			NodeInfo: func() interface{} {
				return s.NodeInfo()
			},
			PeerInfo: func(id discover.NodeID) interface{} {
				if p := s.peers.Peer(fmt.Sprintf("%x", id[:8])); p != nil {
					return p.Info()
				}
				return nil
			},
		})
	}
	s.bridgeServer.AddProtocols(scprotocols)

	if err := p2pServer.Start(); err != nil {
		return errors.New("fail to bridgeserver start")
	}

	// Start the RPC service
	s.netRPCService = api.NewPublicNetAPI(s.bridgeServer, s.NetVersion())

	// Figure out a max peers count based on the server limits
	//s.maxPeers = s.bridgeServer.MaxPeers()
	//validator := func(header *types.Header) error {
	//	return nil
	//}
	//heighter := func() uint64 {
	//	return s.blockchain.CurrentBlock().NumberU64()
	//}
	//inserter := func(blocks types.Blocks) (int, error) {
	//	return 0, nil
	//}
	//s.fetcher = fetcher.New(s.GetBlockByHash, validator, s.BroadcastBlock, heighter, inserter, s.removePeer)

	go s.syncer()

	return nil
}

//func (pm *SubBridge) GetBlockByHash(hash common.Hash) *types.Block {
//	return pm.blockchain.GetBlockByHash(hash)
//}
//
//func (pm *SubBridge) BroadcastBlock(block *types.Block, propagate bool) {
//	// do nothing
//}

func (pm *SubBridge) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) BridgePeer {
	return newBridgePeer(pv, p, rw)
}

// genUnsignedServiceChainTx generates an unsigned transaction, which type is TxTypeChainDataAnchoring.
// Nonce of account used for service chain transaction will be increased after the signing.
func (scpm *SubBridge) genUnsignedServiceChainTx(block *types.Block) (*types.Transaction, error) {
	chainHashes := types.NewChainHashes(block)
	encodedCCTxData, err := rlp.EncodeToBytes(chainHashes)
	if err != nil {
		return nil, err
	}

	values := map[types.TxValueKeyType]interface{}{
		types.TxValueKeyNonce:        scpm.getChainAccountNonce(), // chain account nonce will be increased after signing a transaction.
		types.TxValueKeyFrom:         *scpm.GetChainAccountAddr(),
		types.TxValueKeyTo:           *scpm.GetChainAccountAddr(),
		types.TxValueKeyAmount:       new(big.Int).SetUint64(0),
		types.TxValueKeyGasLimit:     uint64(999999999998), // TODO-Klaytn-ServiceChain should define proper gas limit
		types.TxValueKeyGasPrice:     new(big.Int).SetUint64(scpm.getRemoteGasPrice()),
		types.TxValueKeyAnchoredData: encodedCCTxData,
	}

	if tx, err := types.NewTransactionWithMap(types.TxTypeChainDataAnchoring, values); err != nil {
		return nil, err
	} else {
		return tx, nil
	}
}

// getChainAccountNonce returns the chain account nonce of chain account address.
func (scpm *SubBridge) getChainAccountNonce() uint64 {
	return uint64(0)
}

// GetChainAccountAddr returns a pointer of a hex address of an account used for service chain.
// If given as a parameter, it will use it. If not given, it will use the address of the public key
// derived from chainKey.
func (scpm *SubBridge) GetChainAccountAddr() *common.Address {
	return &common.Address{}
}

func (scpm *SubBridge) getRemoteGasPrice() uint64 {
	return uint64(0)
}

func (pm *SubBridge) handle(p BridgePeer) error {
	// Ignore maxPeers if this is a trusted peer
	if pm.peers.Len() >= pm.maxPeers && !p.GetP2PPeer().Info().Networks[p2p.ConnDefault].Trusted {
		return p2p.DiscTooManyPeers
	}
	p.GetP2PPeer().Log().Debug("klaytn peer connected", "name", p.GetP2PPeer().Name())

	// Execute the handshake
	var (
		genesis = pm.blockchain.Genesis()
		head    = pm.blockchain.CurrentHeader()
		hash    = head.Hash()
		number  = head.Number.Uint64()
		td      = pm.blockchain.GetTd(hash, number)
	)

	err := p.Handshake(pm.networkId, pm.getChainID(), td, hash, genesis.Hash())
	if err != nil {
		p.GetP2PPeer().Log().Debug("klaytn peer handshake failed", "err", err)
		fmt.Println(err)
		return err
	}

	// Register the peer locally
	if err := pm.peers.Register(p); err != nil {
		// if starting node with unlock account, can't register peer until finish unlock
		p.GetP2PPeer().Log().Info("klaytn peer registration failed", "err", err)
		fmt.Println(err)
		return err
	}
	defer pm.removePeer(p.GetID())

	pm.eventhandler.RegisterNewPeer(p)

	p.GetP2PPeer().Log().Info("Added a P2P Peer", "peerID", p.GetP2PPeerID())

	//pubKey, err := p.GetP2PPeerID().Pubkey()
	//if err != nil {
	//	return err
	//}
	//addr := crypto.PubkeyToAddress(*pubKey)

	// main loop. handle incoming messages.
	for {
		if err := pm.handleMsg(p); err != nil {
			p.GetP2PPeer().Log().Debug("klaytn message handling failed", "err", err)
			return err
		}
	}
}

func (sc *SubBridge) loop() {
	defer sc.pmwg.Done()

	report := time.NewTicker(1 * time.Second)
	defer report.Stop()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent
		case ev := <-sc.chainHeadCh:
			if ev.Block != nil {
				sc.eventhandler.HandleChainHeadEvent(ev.Block)
			} else {
				logger.Error("subbridge block event is nil")
			}
		case ev := <-sc.txCh:
			if ev.Txs != nil {
				sc.eventhandler.HandleTxsEvent(ev.Txs)
			} else {
				logger.Error("subbridge tx event is nil")
			}
		case <-report.C:
			// report status
		}
	}
}

func (pm *SubBridge) removePeer(id string) {
	// Short circuit if the peer was already removed
	peer := pm.peers.Peer(id)
	if peer == nil {
		return
	}
	logger.Debug("Removing klaytn peer", "peer", id)

	if err := pm.peers.Unregister(id); err != nil {
		logger.Error("Peer removal failed", "peer", id, "err", err)
	}
	// Hard disconnect at the networking layer
	if peer != nil {
		peer.GetP2PPeer().Disconnect(p2p.DiscUselessPeer)
	}
}

// handleMsg is invoked whenever an inbound message is received from a remote
// peer. The remote connection is torn down upon returning any error.
func (pm *SubBridge) handleMsg(p BridgePeer) error {
	//Below message size checking is done by handle().
	//Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.GetRW().ReadMsg()
	if err != nil {
		p.GetP2PPeer().Log().Debug("ProtocolManager failed to read msg", "err", err)
		return err
	}
	if msg.Size > ProtocolMaxMsgSize {
		err := errResp(ErrMsgTooLarge, "%v > %v", msg.Size, ProtocolMaxMsgSize)
		p.GetP2PPeer().Log().Debug("ProtocolManager over max msg size", "err", err)
		return err
	}
	defer msg.Discard()

	return pm.handler.HandleMainMsg(p, msg)
}

func (pm *SubBridge) syncer() {
	// Start and ensure cleanup of sync mechanisms
	//pm.fetcher.Start()
	//defer pm.fetcher.Stop()
	//defer pm.downloader.Terminate()

	// Wait for different events to fire synchronisation operations
	forceSync := time.NewTicker(forceSyncCycle)
	defer forceSync.Stop()

	for {
		select {
		case peer := <-pm.newPeerCh:
			go pm.synchronise(peer)

		case <-forceSync.C:
			// Force a sync even if not enough peers are present
			go pm.synchronise(pm.peers.BestPeer())

		case <-pm.noMorePeers:
			return
		}
	}
}

func (pm *SubBridge) synchronise(peer BridgePeer) {
	// @TODO Klaytn ServiceChain Sync
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Klaytn protocol.
func (s *SubBridge) Stop() error {

	close(s.quitSync)

	s.chainHeadSub.Unsubscribe()
	s.txSub.Unsubscribe()
	s.eventMux.Stop()
	s.chainDB.Close()

	s.bridgeServer.Stop()

	return nil
}