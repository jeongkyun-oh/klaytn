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
	"github.com/ground-x/klaytn/storage/database"
	"math/big"
	"sync"
	"time"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize   = 10
	chainLogChanSize    = 100
	transactionChanSize = 500
)

// NodeInfo represents a short summary of the Ethereum sub-protocol metadata
// known about the host peer.
type MainBridgeInfo struct {
	Network    uint64              `json:"network"`    // Ethereum network ID (1=Frontier, 2=Morden, Ropsten=3, Rinkeby=4)
	Difficulty *big.Int            `json:"difficulty"` // Total difficulty of the host's blockchain
	Genesis    common.Hash         `json:"genesis"`    // SHA3 hash of the host's genesis block
	Config     *params.ChainConfig `json:"config"`     // Chain configuration for the fork rules
	Head       common.Hash         `json:"head"`       // SHA3 hash of the host's best owned block
}

// CN implements the Klaytn consensus node service.
type MainBridge struct {
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

	APIBackend *MainBridgeAPI

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

	chainHeadCh  chan blockchain.ChainHeadEvent
	chainHeadSub event.Subscription
	logsCh       chan []*types.Log
	logsSub      event.Subscription
	txCh         chan blockchain.NewTxsEvent
	txSub        event.Subscription

	peers        *bridgePeerSet
	handler      *MainBridgeHandler
	eventhandler *MainChainEventHandler
}

// New creates a new CN object (including the
// initialisation of the common CN object)
func NewMainBridge(ctx *node.ServiceContext, config *SCConfig) (*MainBridge, error) {
	config.ChildChainIndexing = true
	chainDB := CreateDB(ctx, config, "scchaindata")

	if config.chainkey != nil || config.ChainAccountAddr != nil {
		logger.Warn("MainBridge doesn't need a chain account. Chain account is only for SubBridge.")
	}

	sc := &MainBridge{
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
		logsCh:         make(chan []*types.Log, chainLogChanSize),
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

	sc.APIBackend = &MainBridgeAPI{sc}

	var err error
	sc.handler, err = NewMainBridgeHandler(sc.config, sc)
	if err != nil {
		return nil, err
	}
	sc.eventhandler, err = NewMainChainEventHandler(sc, sc.handler)
	if err != nil {
		return nil, err
	}

	return sc, nil
}

// CreateDB creates the chain database.
func CreateDB(ctx *node.ServiceContext, config *SCConfig, name string) database.DBManager {
	// OpenFilesLimit and LevelDBCacheSize are used by minimum value.
	dbc := &database.DBConfig{Dir: name, DBType: database.LevelDB, ChildChainIndexing: config.ChildChainIndexing}
	return ctx.OpenDatabase(dbc)
}

// implement PeerSetManager
func (mb *MainBridge) BridgePeerSet() *bridgePeerSet {
	return mb.peers
}

// APIs returns the collection of RPC services the ethereum package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *MainBridge) APIs() []rpc.API {
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

func (s *MainBridge) AccountManager() *accounts.Manager { return s.accountManager }
func (s *MainBridge) EventMux() *event.TypeMux          { return s.eventMux }
func (s *MainBridge) ChainDB() database.DBManager       { return s.chainDB }
func (s *MainBridge) IsListening() bool                 { return true } // Always listening
func (s *MainBridge) ProtocolVersion() int              { return int(s.SCProtocol().Versions[0]) }
func (s *MainBridge) NetVersion() uint64                { return s.networkId }

func (s *MainBridge) Components() []interface{} {
	return nil
}

func (sc *MainBridge) SetComponents(components []interface{}) {
	for _, component := range components {
		switch v := component.(type) {
		case *blockchain.BlockChain:
			sc.blockchain = v
			// event from core-service
			sc.chainHeadSub = sc.blockchain.SubscribeChainHeadEvent(sc.chainHeadCh)
			sc.logsSub = sc.blockchain.SubscribeLogsEvent(sc.logsCh)
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
func (s *MainBridge) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (s *MainBridge) SCProtocol() SCProtocol {
	return SCProtocol{
		Name:     SCProtocolName,
		Versions: SCProtocolVersion,
		Lengths:  SCProtocolLength,
	}
}

// NodeInfo retrieves some protocol metadata about the running host node.
func (pm *MainBridge) NodeInfo() *MainBridgeInfo {
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
func (pm *MainBridge) getChainID() *big.Int {
	return pm.blockchain.Config().ChainID
}

// Start implements node.Service, starting all internal goroutines needed by the
// Klaytn protocol implementation.
func (s *MainBridge) Start(srvr p2p.Server) error {

	serverConfig := p2p.Config{}
	serverConfig.PrivateKey = s.ctx.NodeKey()
	serverConfig.Name = s.ctx.NodeType().String()
	serverConfig.Logger = logger
	serverConfig.ListenAddr = s.config.BridgePort
	serverConfig.MaxPeers = s.maxPeers
	serverConfig.NoDiscovery = true
	serverConfig.EnableMultiChannelServer = false

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

	go s.syncer()

	return nil
}

func (pm *MainBridge) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) BridgePeer {
	return newBridgePeer(pv, p, rw)
}

func (pm *MainBridge) handle(p BridgePeer) error {
	// Ignore maxPeers if this is a trusted peer
	if pm.peers.Len() >= pm.maxPeers && !p.GetP2PPeer().Info().Networks[p2p.ConnDefault].Trusted {
		return p2p.DiscTooManyPeers
	}
	p.GetP2PPeer().Log().Debug("klaytn peer connected", "name", p.GetP2PPeer().Name())

	// Execute the handshake
	var (
		head   = pm.blockchain.CurrentHeader()
		hash   = head.Hash()
		number = head.Number.Uint64()
		td     = pm.blockchain.GetTd(hash, number)
	)

	err := p.Handshake(pm.networkId, pm.getChainID(), td, hash)
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

	pm.handler.RegisterNewPeer(p)

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

func (sc *MainBridge) loop() {
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
				logger.Error("mainbridge block event is nil")
			}
		// Handle NewTxsEvent
		case ev := <-sc.txCh:
			if ev.Txs != nil {
				sc.eventhandler.HandleTxsEvent(ev.Txs)
			} else {
				logger.Error("mainbridge tx event is nil")
			}
		// Handle ChainLogsEvent
		case logs := <-sc.logsCh:
			sc.eventhandler.HandleLogsEvent(logs)
		case <-report.C:
			// report status
		case err := <-sc.chainHeadSub.Err():
			if err != nil {
				logger.Error("mainbridge block subscription ", "err", err)
			}
			return
		case err := <-sc.txSub.Err():
			if err != nil {
				logger.Error("mainbridge tx subscription ", "err", err)
			}
			return
		case err := <-sc.logsSub.Err():
			if err != nil {
				logger.Error("mainbridge log subscription ", "err", err)
			}
			return
		}
	}
}

func (pm *MainBridge) removePeer(id string) {
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
func (pm *MainBridge) handleMsg(p BridgePeer) error {
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

	return pm.handler.HandleSubMsg(p, msg)
}

func (pm *MainBridge) syncer() {
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

func (pm *MainBridge) synchronise(peer BridgePeer) {
	// @TODO Klaytn ServiceChain Sync
}

// Stop implements node.Service, terminating all internal goroutines used by the
// Klaytn protocol.
func (s *MainBridge) Stop() error {

	close(s.quitSync)

	s.chainHeadSub.Unsubscribe()
	s.txSub.Unsubscribe()
	s.logsSub.Unsubscribe()
	s.eventMux.Stop()
	s.chainDB.Close()

	s.bridgeServer.Stop()

	return nil
}

func errResp(code errCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}
