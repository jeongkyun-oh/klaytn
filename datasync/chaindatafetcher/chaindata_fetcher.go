// Copyright 2020 The klaytn Authors
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

package chaindatafetcher

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/klaytn/klaytn/api"
	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/vm"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/datasync/chaindatafetcher/kas"
	cfTypes "github.com/klaytn/klaytn/datasync/chaindatafetcher/types"
	"github.com/klaytn/klaytn/event"
	"github.com/klaytn/klaytn/log"
	"github.com/klaytn/klaytn/networks/p2p"
	"github.com/klaytn/klaytn/networks/rpc"
	"github.com/klaytn/klaytn/node"
	"github.com/klaytn/klaytn/node/cn"
	"github.com/rcrowley/go-metrics"
)

var logger = log.NewModuleLogger(log.ChainDataFetcher)

//go:generate mockgen -destination=./mocks/blockchain_mock.go -package=mocks github.com/klaytn/klaytn/datasync/chaindatafetcher BlockChain
type BlockChain interface {
	SubscribeChainEvent(ch chan<- blockchain.ChainEvent) event.Subscription
	CurrentHeader() *types.Header
	GetBlockByNumber(number uint64) *types.Block
	GetReceiptsByBlockHash(blockHash common.Hash) types.Receipts
}

type ChainDataFetcher struct {
	config *ChainDataFetcherConfig

	blockchain BlockChain
	debugAPI   *cn.PrivateDebugAPI

	chainCh  chan blockchain.ChainEvent
	chainSub event.Subscription

	reqCh  chan *cfTypes.Request // TODO-ChainDataFetcher add logic to insert new requests from APIs to this channel
	stopCh chan struct{}

	numHandlers int

	checkpointMu  sync.RWMutex
	checkpoint    int64
	checkpointMap map[int64]struct{}

	wg sync.WaitGroup

	repo Repository

	fetchingStarted      bool
	fetchingStopCh       chan struct{}
	fetchingWg           sync.WaitGroup
	rangeFetchingStarted bool
	rangeFetchingStopCh  chan struct{}
	rangeFetchingWg      sync.WaitGroup
}

func NewChainDataFetcher(ctx *node.ServiceContext, cfg *ChainDataFetcherConfig) (*ChainDataFetcher, error) {
	var (
		repo Repository
		err  error
	)
	switch cfg.Mode {
	case ModeKAS:
		repo, err = kas.NewRepository(cfg.KasConfig)
		if err != nil {
			logger.Error("Failed to create new Repository", "err", err, "user", cfg.KasConfig.DBUser, "host", cfg.KasConfig.DBHost, "port", cfg.KasConfig.DBPort, "name", cfg.KasConfig.DBName, "cacheUrl", cfg.KasConfig.CacheInvalidationURL, "x-chain-id", cfg.KasConfig.XChainId)
			return nil, err
		}
	case ModeKafka:
		// TODO-ChainDataFetcher implement new repository for kafka
		panic("implement me")
	default:
		logger.Crit("the chaindatafetcher mode is not supported", "mode", cfg.Mode)
	}
	checkpoint, err := repo.ReadCheckpoint()
	if err != nil {
		logger.Error("Failed to get checkpoint", "err", err)
		return nil, err
	}
	return &ChainDataFetcher{
		config:        cfg,
		chainCh:       make(chan blockchain.ChainEvent, cfg.BlockChannelSize),
		reqCh:         make(chan *cfTypes.Request, cfg.JobChannelSize),
		stopCh:        make(chan struct{}),
		numHandlers:   cfg.NumHandlers,
		checkpoint:    checkpoint,
		checkpointMap: make(map[int64]struct{}),
		repo:          repo,
	}, nil
}

func (f *ChainDataFetcher) Protocols() []p2p.Protocol {
	return []p2p.Protocol{}
}

func (f *ChainDataFetcher) APIs() []rpc.API {
	return []rpc.API{
		{
			Namespace: "chaindatafetcher",
			Version:   "1.0",
			Service:   NewPublicChainDataFetcherAPI(f),
			Public:    true,
		},
	}
}

func (f *ChainDataFetcher) Start(server p2p.Server) error {
	// launch multiple goroutines to handle new blocks
	for i := 0; i < f.numHandlers; i++ {
		go f.handleRequest()
	}

	if !f.config.NoDefaultStart {
		if err := f.startFetching(); err != nil {
			logger.Error("start fetching is failed", "err", err)
			return err
		}
	}
	logger.Info("chaindata fetcher is started", "numHandlers", f.numHandlers)
	return nil
}

func (f *ChainDataFetcher) Stop() error {
	f.stopFetching()
	f.stopRangeFetching()
	logger.Info("wait for all goroutines to be terminated...", "numGoroutines", f.config.NumHandlers)
	close(f.stopCh)
	f.wg.Wait()
	logger.Info("chaindata fetcher is stopped")
	return nil
}

func (f *ChainDataFetcher) sendRequests(startBlock, endBlock uint64, reqType cfTypes.RequestType, shouldUpdateCheckpoint bool, stopCh chan struct{}) {
	logger.Info("sending requests is started", "startBlock", startBlock, "endBlock", endBlock)
	for i := startBlock; i <= endBlock; i++ {
		select {
		case <-stopCh:
			logger.Info("stopped making requests", "startBlock", startBlock, "endBlock", endBlock, "stoppedBlock", i)
			return
		case f.reqCh <- cfTypes.NewRequest(reqType, shouldUpdateCheckpoint, i):
		}
	}
	logger.Info("sending requests is finished", "startBlock", startBlock, "endBlock", endBlock)
}

func (f *ChainDataFetcher) startFetching() error {
	if f.fetchingStarted {
		return errors.New("fetching is already started")
	}
	f.fetchingStarted = true

	// subscribe chain event in order to handle new blocks.
	f.chainSub = f.blockchain.SubscribeChainEvent(f.chainCh)
	checkpoint := uint64(f.checkpoint)
	currentBlock := f.blockchain.CurrentHeader().Number.Uint64()

	f.fetchingStopCh = make(chan struct{})
	f.fetchingWg.Add(1)

	// lanuch a goroutine to handle from checkpoint to the head block.
	go func() {
		defer f.fetchingWg.Done()
		f.sendRequests(uint64(f.checkpoint), currentBlock, cfTypes.RequestTypeAll, true, f.fetchingStopCh)
	}()
	logger.Info("fetching is started", "startedCheckpoint", checkpoint, "currentBlock", currentBlock)
	return nil
}

func (f *ChainDataFetcher) stopFetching() error {
	if !f.fetchingStarted {
		return errors.New("fetching is not running")
	}

	f.chainSub.Unsubscribe()
	close(f.fetchingStopCh)
	f.fetchingWg.Wait()
	f.fetchingStarted = false
	logger.Info("fetching is stopped")
	return nil
}

func (f *ChainDataFetcher) startRangeFetching(startBlock, endBlock uint64, reqType cfTypes.RequestType) error {
	if f.rangeFetchingStarted {
		return errors.New("range fetching is already started")
	}
	f.rangeFetchingStarted = true

	f.rangeFetchingStopCh = make(chan struct{})
	f.rangeFetchingWg.Add(1)
	go func() {
		defer f.rangeFetchingWg.Done()
		f.sendRequests(startBlock, endBlock, reqType, false, f.rangeFetchingStopCh)
		f.rangeFetchingStarted = false
	}()
	logger.Info("range fetching is started", "startBlock", startBlock, "endBlock", endBlock)
	return nil
}

func (f *ChainDataFetcher) stopRangeFetching() error {
	if !f.rangeFetchingStarted {
		return errors.New("range fetching is not running")
	}
	close(f.rangeFetchingStopCh)
	f.rangeFetchingWg.Wait()
	f.rangeFetchingStarted = false
	logger.Info("range fetching is stopped")
	return nil
}

func (f *ChainDataFetcher) makeChainEvent(blockNumber uint64) (blockchain.ChainEvent, error) {
	var logs []*types.Log
	block := f.blockchain.GetBlockByNumber(blockNumber)
	receipts := f.blockchain.GetReceiptsByBlockHash(block.Hash())
	for _, r := range receipts {
		logs = append(logs, r.Logs...)
	}
	var internalTraces []*vm.InternalTxTrace
	if block.Transactions().Len() > 0 {
		fct := "fastCallTracer"
		timeout := "24h"
		results, err := f.debugAPI.TraceBlockByNumber(context.Background(), rpc.BlockNumber(block.Number().Int64()), &cn.TraceConfig{
			Tracer:  &fct,
			Timeout: &timeout,
		})
		if err != nil {
			traceAPIErrorCounter.Inc(1)
			logger.Error("Failed to call trace block by number", "err", err, "blockNumber", block.NumberU64())
			return blockchain.ChainEvent{}, err
		}
		for _, r := range results {
			if r.Result != nil {
				internalTraces = append(internalTraces, r.Result.(*vm.InternalTxTrace))
			} else {
				traceAPIErrorCounter.Inc(1)
				logger.Error("the trace result is nil", "err", r.Error, "blockNumber", blockNumber)
				internalTraces = append(internalTraces, &vm.InternalTxTrace{Value: "0x0", Calls: []*vm.InternalTxTrace{}})
			}
		}
	}

	return blockchain.ChainEvent{
		Block:            block,
		Hash:             block.Hash(),
		Receipts:         receipts,
		Logs:             logs,
		InternalTxTraces: internalTraces,
	}, nil
}

func (f *ChainDataFetcher) Components() []interface{} {
	return nil
}

func (f *ChainDataFetcher) SetComponents(components []interface{}) {
	for _, component := range components {
		switch v := component.(type) {
		case *blockchain.BlockChain:
			f.blockchain = v
		case []rpc.API:
			for _, a := range v {
				switch s := a.Service.(type) {
				case *api.PublicBlockChainAPI:
					f.repo.SetComponent(s)
				case *cn.PrivateDebugAPI:
					f.debugAPI = s
				}
			}
		}
	}
}

func (f *ChainDataFetcher) handleRequestByType(reqType cfTypes.RequestType, shouldUpdateCheckpoint bool, ev blockchain.ChainEvent) {
	now := time.Now()
	// TODO-ChainDataFetcher parallelize handling data
	for i := cfTypes.RequestType(1); i < cfTypes.RequestTypeLength; i = i << 1 {
		if cfTypes.CheckRequestType(reqType, i) {
			f.updateGauge(f.retryFunc(f.repo.HandleChainEvent))(ev, reqType)
		}
	}
	elapsed := time.Since(now)
	totalInsertionTimeGauge.Update(elapsed.Milliseconds())

	if shouldUpdateCheckpoint {
		f.updateCheckpoint(ev.Block.Number().Int64())
	}
	handledBlockNumberGauge.Update(ev.Block.Number().Int64())
}

func (f *ChainDataFetcher) handleRequest() {
	f.wg.Add(1)
	defer f.wg.Done()
	for {
		select {
		case <-f.stopCh:
			logger.Info("handleRequest is stopped")
			return
		case ev := <-f.chainCh:
			numChainEventGauge.Update(int64(len(f.chainCh)))
			f.handleRequestByType(cfTypes.RequestTypeAll, true, ev)
		case req := <-f.reqCh:
			numRequestsGauge.Update(int64(len(f.reqCh)))
			ev, err := f.makeChainEvent(req.BlockNumber)
			if err != nil {
				// TODO-ChainDataFetcher handle error
				logger.Error("making chain event is failed", "err", err)
				break
			}
			f.handleRequestByType(req.ReqType, req.ShouldUpdateCheckpoint, ev)
		}
	}
}

func (f *ChainDataFetcher) updateCheckpoint(num int64) error {
	f.checkpointMu.Lock()
	defer f.checkpointMu.Unlock()
	f.checkpointMap[num] = struct{}{}

	updated := false
	newCheckpoint := f.checkpoint
	for {
		if _, ok := f.checkpointMap[newCheckpoint]; !ok {
			break
		}
		delete(f.checkpointMap, newCheckpoint)
		newCheckpoint++
		updated = true
	}

	if updated {
		f.checkpoint = newCheckpoint
		checkpointGauge.Update(f.checkpoint)
		return f.repo.WriteCheckpoint(newCheckpoint)
	}
	return nil
}

func getTimeGauge(reqType cfTypes.RequestType) (metrics.Gauge, error) {
	switch reqType {
	case cfTypes.RequestTypeTransaction:
		return txsInsertionTimeGauge, nil
	case cfTypes.RequestTypeTokenTransfer:
		return tokenTransfersInsertionTimeGauge, nil
	case cfTypes.RequestTypeContract:
		return contractsInsertionTimeGauge, nil
	case cfTypes.RequestTypeTrace:
		return tracesInsertionTimeGauge, nil
	default:
		return nil, fmt.Errorf("the request type is not supported. type: %v", reqType)
	}
}

func (f *ChainDataFetcher) updateGauge(insert HandleChainEventFn) HandleChainEventFn {
	return func(chainEvent blockchain.ChainEvent, reqType cfTypes.RequestType) error {
		now := time.Now()
		if err := insert(chainEvent, reqType); err != nil {
			return err
		}
		elapsed := time.Since(now)
		if gauge, err := getTimeGauge(reqType); err != nil {
			logger.Error("getTimeGauge is failed", "err", err, "type", reqType)
		} else {
			gauge.Update(elapsed.Milliseconds())
		}
		return nil
	}
}

func getRetryGauge(reqType cfTypes.RequestType) (metrics.Gauge, error) {
	switch reqType {
	case cfTypes.RequestTypeTransaction:
		return txsInsertionRetryGauge, nil
	case cfTypes.RequestTypeTokenTransfer:
		return tokenTransfersInsertionRetryGauge, nil
	case cfTypes.RequestTypeContract:
		return contractsInsertionRetryGauge, nil
	case cfTypes.RequestTypeTrace:
		return tracesInsertionRetryGauge, nil
	default:
		return nil, fmt.Errorf("the request type is not supported. type: %v", reqType)
	}
}

func (f *ChainDataFetcher) retryFunc(insert HandleChainEventFn) HandleChainEventFn {
	return func(event blockchain.ChainEvent, reqType cfTypes.RequestType) error {
		i := 0
		for err := insert(event, reqType); err != nil; err = insert(event, reqType) {
			select {
			case <-f.stopCh:
				return nil
			default:
				i++
				if gauge, err := getRetryGauge(reqType); err != nil {
					logger.Error("getRetryGauge is failed", "err", err, "type", reqType)
				} else {
					gauge.Update(int64(i))
					logger.Warn("retrying...", "blockNumber", event.Block.NumberU64(), "retryCount", i)
				}
				time.Sleep(DBInsertRetryInterval)
			}
		}
		return nil
	}
}

func (f *ChainDataFetcher) status() string {
	return fmt.Sprintf("{fetching: %v, rangeFetching: %v}", f.fetchingStarted, f.rangeFetchingStarted)
}
