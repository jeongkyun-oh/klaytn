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

package sc

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"github.com/ground-x/klaytn/accounts/abi/bind"
	"github.com/ground-x/klaytn/common"
	bridgecontract "github.com/ground-x/klaytn/contracts/bridge"
	"github.com/ground-x/klaytn/event"
	"github.com/ground-x/klaytn/ser/rlp"
	"io"
	"math/big"
	"path"
	"time"
)

const (
	TokenEventChanSize = 30
	BridgeAddrJournal  = "bridge_addrs.rlp"
)

const (
	KLAY uint8 = iota
	TOKEN
	NFT
)

// RequestValueTransfer Event from SmartContract
type TokenReceivedEvent struct {
	TokenType    uint8
	ContractAddr common.Address
	TokenAddr    common.Address
	From         common.Address
	To           common.Address
	Amount       *big.Int // Amount is UID in NFT
	RequestNonce uint64
}

// TokenWithdraw Event from SmartContract
type TokenTransferEvent struct {
	TokenType    uint8
	ContractAddr common.Address
	TokenAddr    common.Address
	Owner        common.Address
	Amount       *big.Int // Amount is UID in NFT
	HandleNonce  uint64
}

// BridgeJournal has two types. When a single address is inserted, the Paired is disabled.
// In this case, only one of the LocalAddress or RemoteAddress is filled with the address.
// If two address in a pair is inserted, the Pared is enabled.
type BridgeJournal struct {
	LocalAddress  common.Address `json:"localAddress"`
	RemoteAddress common.Address `json:"remoteAddress"`
	Paired        bool           `json:"paired"`
}

type BridgeInfo struct {
	bridge         *bridgecontract.Bridge
	onServiceChain bool
	subscribed     bool
}

// DecodeRLP decodes the Klaytn
func (b *BridgeJournal) DecodeRLP(s *rlp.Stream) error {
	var elem struct {
		LocalAddress  common.Address
		RemoteAddress common.Address
		Paired        bool
	}
	if err := s.Decode(&elem); err != nil {
		return err
	}
	b.LocalAddress, b.RemoteAddress, b.Paired = elem.LocalAddress, elem.RemoteAddress, elem.Paired
	return nil
}

// EncodeRLP serializes b into the Klaytn RLP BridgeJournal format.
func (b *BridgeJournal) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, []interface{}{
		b.LocalAddress,
		b.RemoteAddress,
		b.Paired,
	})
}

// BridgeManager manages Bridge SmartContracts
// for value transfer between service chain and parent chain
type BridgeManager struct {
	subBridge *SubBridge

	receivedEvents map[common.Address]event.Subscription
	withdrawEvents map[common.Address]event.Subscription
	bridges        map[common.Address]*BridgeInfo

	tokenReceived event.Feed
	tokenWithdraw event.Feed

	scope event.SubscriptionScope

	journal *bridgeAddrJournal
}

func NewBridgeManager(main *SubBridge) (*BridgeManager, error) {
	bridgeAddrJournal := newBridgeAddrJournal(path.Join(main.config.DataDir, BridgeAddrJournal), main.config)

	bridgeManager := &BridgeManager{
		subBridge:      main,
		receivedEvents: make(map[common.Address]event.Subscription),
		withdrawEvents: make(map[common.Address]event.Subscription),
		bridges:        make(map[common.Address]*BridgeInfo),
		journal:        bridgeAddrJournal,
	}

	logger.Info("Load Bridge Address from JournalFiles ", "path", bridgeManager.journal.path)
	bridgeManager.journal.cache = []*BridgeJournal{}

	if err := bridgeManager.journal.load(func(gwjournal BridgeJournal) error {
		logger.Info("Load Bridge Address from JournalFiles ",
			"local address", gwjournal.LocalAddress.Hex(), "remote address", gwjournal.RemoteAddress.Hex())
		bridgeManager.journal.cache = append(bridgeManager.journal.cache, &gwjournal)
		return nil
	}); err != nil {
		logger.Error("fail to load bridge address", "err", err)
	}

	if err := bridgeManager.journal.rotate(bridgeManager.GetAllBridge()); err != nil {
		logger.Error("fail to rotate bridge journal", "err", err)
	}

	return bridgeManager, nil
}

// SubscribeTokenReceived registers a subscription of TokenReceivedEvent.
func (bm *BridgeManager) SubscribeTokenReceived(ch chan<- TokenReceivedEvent) event.Subscription {
	return bm.scope.Track(bm.tokenReceived.Subscribe(ch))
}

// SubscribeTokenWithDraw registers a subscription of TokenTransferEvent.
func (bm *BridgeManager) SubscribeTokenWithDraw(ch chan<- TokenTransferEvent) event.Subscription {
	return bm.scope.Track(bm.tokenWithdraw.Subscribe(ch))
}

// GetAllBridge returns a journal cache while removing unnecessary address pair.
func (bm *BridgeManager) GetAllBridge() []*BridgeJournal {
	gwjs := []*BridgeJournal{}

	for _, journal := range bm.journal.cache {
		if journal.Paired {
			bridgeInfo, ok := bm.bridges[journal.LocalAddress]
			if ok && !bridgeInfo.subscribed {
				continue
			}
			if bm.subBridge.AddressManager() != nil {
				bm.subBridge.addressManager.DeleteBridge(journal.LocalAddress)
			}

			bridgeInfo, ok = bm.bridges[journal.RemoteAddress]
			if ok && !bridgeInfo.subscribed {
				continue
			}
			if bm.subBridge.AddressManager() != nil {
				bm.subBridge.addressManager.DeleteBridge(journal.RemoteAddress)
			}
		}
		gwjs = append(gwjs, journal)
	}

	bm.journal.cache = gwjs

	return bm.journal.cache
}

// SetBridge stores the address and bridge pair with local/remote and subscription status.
func (bm *BridgeManager) SetBridge(addr common.Address, bridge *bridgecontract.Bridge, local bool, subscribed bool) {
	bm.bridges[addr] = &BridgeInfo{bridge, local, subscribed}
}

// LoadAllBridge reloads bridge and handles subscription by using the the journal cache.
func (bm *BridgeManager) LoadAllBridge() error {
	for _, journal := range bm.journal.cache {
		if journal.Paired {
			if bm.subBridge.AddressManager() == nil {
				return errors.New("address manager is not exist")
			}
			logger.Info("Add bridge pair in address manager")
			// Step 1: register bridge
			localBridge, err := bridgecontract.NewBridge(journal.LocalAddress, bm.subBridge.localBackend)
			if err != nil {
				return err
			}
			remoteBridge, err := bridgecontract.NewBridge(journal.RemoteAddress, bm.subBridge.remoteBackend)
			if err != nil {
				return err
			}
			bm.SetBridge(journal.LocalAddress, localBridge, true, false)
			bm.SetBridge(journal.RemoteAddress, remoteBridge, false, false)

			// Step 2: set address manager
			bm.subBridge.AddressManager().AddBridge(journal.LocalAddress, journal.RemoteAddress)

			// Step 3: subscribe event
			bm.subscribeEvent(journal.LocalAddress, localBridge)
			bm.subscribeEvent(journal.RemoteAddress, remoteBridge)

		} else {
			err := bm.loadBridge(journal.LocalAddress, bm.subBridge.localBackend, true, false)
			if err != nil {
				return err
			}
			err = bm.loadBridge(journal.RemoteAddress, bm.subBridge.remoteBackend, false, false)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadBridge creates new bridge contract for a given address and subscribes an event if needed.
func (bm *BridgeManager) loadBridge(addr common.Address, backend bind.ContractBackend, local bool, subscribed bool) error {
	var bridgeInfo *BridgeInfo

	defer func() {
		if bridgeInfo != nil && subscribed && !bm.bridges[addr].subscribed {
			logger.Info("bridge subscription is enabled by journal", "address", addr)
			bm.subscribeEvent(addr, bridgeInfo.bridge)
		}
	}()

	bridgeInfo = bm.bridges[addr]
	if bridgeInfo != nil {
		return nil
	}

	bridge, err := bridgecontract.NewBridge(addr, backend)
	if err != nil {
		return err
	}
	logger.Info("bridge ", "address", addr)
	bm.SetBridge(addr, bridge, local, false)
	bridgeInfo = bm.bridges[addr]

	return nil
}

// Deploy Bridge SmartContract on same node or remote node
func (bm *BridgeManager) DeployBridge(backend bind.ContractBackend, local bool) (common.Address, error) {
	if local {
		addr, bridge, err := bm.deployBridge(bm.subBridge.getChainID(), big.NewInt((int64)(bm.subBridge.handler.getNodeAccountNonce())), bm.subBridge.handler.nodeKey, backend, bm.subBridge.txPool.GasPrice())
		if err != nil {
			logger.Error("fail to deploy bridge", "err", err)
			return common.Address{}, err
		}
		bm.SetBridge(addr, bridge, local, false)
		bm.journal.insert(addr, common.Address{}, false)

		return addr, err
	} else {
		bm.subBridge.handler.LockChainAccount()
		defer bm.subBridge.handler.UnLockChainAccount()
		addr, bridge, err := bm.deployBridge(bm.subBridge.handler.parentChainID, big.NewInt((int64)(bm.subBridge.handler.getChainAccountNonce())), bm.subBridge.handler.chainKey, backend, new(big.Int).SetUint64(bm.subBridge.handler.remoteGasPrice))
		if err != nil {
			logger.Error("fail to deploy bridge", "err", err)
			return common.Address{}, err
		}
		bm.SetBridge(addr, bridge, local, false)
		bm.journal.insert(common.Address{}, addr, false)
		bm.subBridge.handler.addChainAccountNonce(1)
		return addr, err
	}
}

// DeployBridge handles actual smart contract deployment.
// To create contract, the chain ID, nonce, account key, private key, contract binding and gas price are used.
// The deployed contract address, transaction are returned. An error is also returned if any.
func (bm *BridgeManager) deployBridge(chainID *big.Int, nonce *big.Int, accountKey *ecdsa.PrivateKey, backend bind.ContractBackend, gasPrice *big.Int) (common.Address, *bridgecontract.Bridge, error) {
	// TODO-Klaytn change config
	if accountKey == nil {
		// Only for unit test
		return common.Address{}, nil, errors.New("nil accountKey")
	}

	auth := MakeTransactOpts(accountKey, nonce, chainID, gasPrice)

	addr, tx, contract, err := bridgecontract.DeployBridge(auth, backend, true)
	if err != nil {
		logger.Error("Failed to deploy contract.", "err", err)
		return common.Address{}, nil, err
	}
	logger.Info("Bridge is deploying...", "addr", addr, "txHash", tx.Hash().String())

	back, ok := backend.(bind.DeployBackend)
	if !ok {
		logger.Warn("DeployBacked type assertion is failed. Skip WaitDeployed.")
		return addr, contract, nil
	}

	timeoutContext, cancelTimeout := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelTimeout()

	addr, err = bind.WaitDeployed(timeoutContext, back, tx)
	if err != nil {
		logger.Error("Failed to WaitDeployed.", "err", err, "txHash", tx.Hash().String())
		return common.Address{}, nil, err
	}
	logger.Info("Bridge is deployed.", "addr", addr, "txHash", tx.Hash().String())
	return addr, contract, nil
}

// SubscribeEvent registers a subscription of BridgeERC20Received and BridgeTokenWithdrawn
func (bm *BridgeManager) SubscribeEvent(addr common.Address) error {
	bridgeInfo, ok := bm.bridges[addr]
	if !ok {
		return fmt.Errorf("there is no bridge contract which address %v", addr)
	}
	bm.subscribeEvent(addr, bridgeInfo.bridge)

	return nil
}

// SubscribeEvent sets watch logs and creates a goroutine loop to handle event messages.
func (bm *BridgeManager) subscribeEvent(addr common.Address, bridge *bridgecontract.Bridge) {
	tokenReceivedCh := make(chan *bridgecontract.BridgeRequestValueTransfer, TokenEventChanSize)
	tokenWithdrawCh := make(chan *bridgecontract.BridgeHandleValueTransfer, TokenEventChanSize)

	receivedSub, err := bridge.WatchRequestValueTransfer(nil, tokenReceivedCh)
	if err != nil {
		logger.Error("Failed to pBridge.WatchERC20Received", "err", err)
	}
	bm.receivedEvents[addr] = receivedSub
	withdrawnSub, err := bridge.WatchHandleValueTransfer(nil, tokenWithdrawCh)
	if err != nil {
		logger.Error("Failed to pBridge.WatchTokenWithdrawn", "err", err)
	}
	bm.withdrawEvents[addr] = withdrawnSub
	bm.bridges[addr].subscribed = true

	go bm.loop(addr, tokenReceivedCh, tokenWithdrawCh, bm.scope.Track(receivedSub), bm.scope.Track(withdrawnSub))
}

// UnsubscribeEvent cancels the contract's watch logs and initializes the status.
func (bm *BridgeManager) unsubscribeEvent(addr common.Address) {
	receivedSub := bm.receivedEvents[addr]
	receivedSub.Unsubscribe()

	withdrawSub := bm.withdrawEvents[addr]
	withdrawSub.Unsubscribe()

	bm.bridges[addr].subscribed = false
}

// Loop handles subscribed event messages.
func (bm *BridgeManager) loop(
	addr common.Address,
	receivedCh <-chan *bridgecontract.BridgeRequestValueTransfer,
	withdrawCh <-chan *bridgecontract.BridgeHandleValueTransfer,
	receivedSub event.Subscription,
	withdrawSub event.Subscription) {

	defer receivedSub.Unsubscribe()
	defer withdrawSub.Unsubscribe()

	// TODO-klaytn change goroutine logic for performance
	for {
		select {
		case ev := <-receivedCh:
			receiveEvent := TokenReceivedEvent{
				TokenType:    ev.Kind,
				ContractAddr: addr,
				TokenAddr:    ev.ContractAddress,
				From:         ev.From,
				To:           ev.To,
				Amount:       ev.Amount,
				RequestNonce: ev.RequestNonce,
			}
			bm.tokenReceived.Send(receiveEvent)
		case ev := <-withdrawCh:
			withdrawEvent := TokenTransferEvent{
				TokenType:    ev.Kind,
				ContractAddr: addr,
				TokenAddr:    ev.ContractAddress,
				Owner:        ev.Owner,
				Amount:       ev.Value,
				HandleNonce:  ev.HandleNonce,
			}
			bm.tokenWithdraw.Send(withdrawEvent)
		case err := <-receivedSub.Err():
			logger.Info("Contract Event Loop Running Stop by receivedSub.Err()", "err", err)
			return
		case err := <-withdrawSub.Err():
			logger.Info("Contract Event Loop Running Stop by withdrawSub.Err()", "err", err)
			return
		}
	}
}

// Stop closes a subscribed event scope of the bridge manager.
func (bm *BridgeManager) Stop() {
	bm.scope.Close()
}
