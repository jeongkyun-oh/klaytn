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
	"github.com/klaytn/klaytn/api"
	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/blockchain/types"
	"github.com/klaytn/klaytn/blockchain/vm"
	"github.com/klaytn/klaytn/datasync/chaindatafetcher/kas"
	"github.com/klaytn/klaytn/event"
	"github.com/klaytn/klaytn/log"
	"github.com/klaytn/klaytn/networks/p2p"
	"github.com/klaytn/klaytn/networks/rpc"
	"github.com/klaytn/klaytn/node"
	"github.com/klaytn/klaytn/node/cn"
	"sync"
)

var logger = log.NewModuleLogger(log.ChainDataFetcher)

type ChainDataFetcher struct {
	config *ChainDataFetcherConfig

	blockchain    *blockchain.BlockChain
	blockchainAPI *api.PublicBlockChainAPI
	debugAPI      *cn.PrivateDebugAPI

	chainCh  chan blockchain.ChainEvent
	chainSub event.Subscription

	reqCh  chan *request // TODO-ChainDataFetcher add logic to insert new requests from APIs to this channel
	resCh  chan *response
	stopCh chan struct{}

	numHandlers int

	checkpoint    int64
	checkpointMap map[int64]struct{}

	wg sync.WaitGroup

	repo Repository

	started      bool
	rangeStarted bool
}

func NewChainDataFetcher(ctx *node.ServiceContext, cfg *ChainDataFetcherConfig) (*ChainDataFetcher, error) {
	repo, err := kas.NewRepository(cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)
	if err != nil {
		logger.Error("Failed to create new Repository", "err", err, "user", cfg.DBUser, "host", cfg.DBHost, "port", cfg.DBPort, "name", cfg.DBName)
		return nil, err
	}
	checkpoint, err := repo.ReadCheckpoint()
	if err != nil {
		logger.Error("Failed to get checkpoint", "err", err)
		return nil, err
	}
	return &ChainDataFetcher{
		config:        cfg,
		chainCh:       make(chan blockchain.ChainEvent, cfg.BlockChannelSize),
		reqCh:         make(chan *request, cfg.JobChannelSize),
		resCh:         make(chan *response, cfg.JobChannelSize),
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
			return err
		}
	}

	go f.reqLoop()
	go f.resLoop()

	return nil
}

func (f *ChainDataFetcher) Stop() error {
	if f.chainSub != nil {
		f.chainSub.Unsubscribe()
	}
	close(f.stopCh)

	logger.Info("wait for all goroutines to be terminated...")
	f.wg.Wait()
	logger.Info("terminated all goroutines for chaindatafetcher")
	f.started = false
	return nil
}

func (f *ChainDataFetcher) startFetching() error {
	if f.started {
		return errors.New("the chaindata fetcher is already started")
	}

	f.chainSub = f.blockchain.SubscribeChainEvent(f.chainCh)
	currentBlock := f.blockchain.CurrentHeader().Number.Uint64()
	if err := f.startRangeFetching(uint64(f.checkpoint), currentBlock, requestTypeAll); err != nil {
		return err
	}

	f.started = true
	return nil
}

func (f *ChainDataFetcher) stopFetching() error {
	if !f.started {
		return errors.New("the chaindata fetcher is not running")
	}

	f.chainSub.Unsubscribe()
	f.started = false
	return nil
}

func (f *ChainDataFetcher) startRangeFetching(start, end uint64, reqType requestType) error {
	if f.rangeStarted {
		return errors.New("the chaindata fetcher is already started with range")
	}
	f.rangeStarted = true
	defer func() { f.rangeStarted = false }()

	// TODO-ChainDataFetcher parallelize the following codes
	for i := start; i < end; i++ {
		e, err := f.makeChainEvent(i)
		if err != nil {
			return err
		}

		f.reqCh <- newRequest(reqType, e)
		// TODO-ChainDataFetcher add stop logic while processing the events.
	}
	return nil
}

func (f *ChainDataFetcher) stopRangeFetching() error {
	// TODO-ChainDataFetcher add logic for stopping
	return nil
}

// TODO-ChainDataFetcher push down this logic to handleRequest
func (f *ChainDataFetcher) makeChainEvent(blockNumber uint64) (blockchain.ChainEvent, error) {
	var logs []*types.Log
	block := f.blockchain.GetBlockByNumber(blockNumber)
	receipts := f.blockchain.GetReceiptsByBlockHash(block.Hash())
	for _, r := range receipts {
		logs = append(logs, r.Logs...)
	}
	fct := "fastCallTracer"
	results, err := f.debugAPI.TraceBlockByNumber(context.Background(), rpc.BlockNumber(block.Number().Int64()), &cn.TraceConfig{
		Tracer: &fct,
	})
	if err != nil {
		return blockchain.ChainEvent{}, err
	}
	var internalTraces []*vm.InternalTxTrace
	for _, r := range results {
		// TODO-ChainDataFetcher Assume that the input parameters are valid always.
		internalTraces = append(internalTraces, r.Result.(*vm.InternalTxTrace))
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
					f.blockchainAPI = s
				case *cn.PrivateDebugAPI:
					f.debugAPI = s
				}
			}
		}
	}
}

func (f *ChainDataFetcher) handleRequest() {
	f.wg.Add(1)
	defer f.wg.Done()
	for {
		select {
		case <-f.stopCh:
			logger.Info("handleRequest is stopped")
			return
		case req := <-f.reqCh:
			// TODO-ChainDataFetcher parallelize handling data
			if hasTransactions(req.reqType) {
				if err := f.repo.InsertTransactions(req.event); err != nil {
					f.resCh <- newResponse(req.reqType, req.event.Block.Number(), err)
				}
			}

			if hasTokenTransfers(req.reqType) {
				if err := f.repo.InsertTokenTransfers(req.event); err != nil {
					f.resCh <- newResponse(req.reqType, req.event.Block.Number(), err)
				}
			}

			if hasContracts(req.reqType) {
				if err := f.repo.InsertContracts(req.event); err != nil {
					f.resCh <- newResponse(req.reqType, req.event.Block.Number(), err)
				}
			}

			if hasTraces(req.reqType) {
				if err := f.repo.InsertTraceResults(req.event); err != nil {
					f.resCh <- newResponse(req.reqType, req.event.Block.Number(), err)
				}
			}
		}
	}
}

func (f *ChainDataFetcher) reqLoop() {
	f.wg.Add(1)
	defer f.wg.Done()
	for {
		select {
		case <-f.stopCh:
			logger.Info("stopped reqLoop for chaindatafetcher")
			return
		case ev := <-f.chainCh:
			f.reqCh <- newRequest(requestTypeAll, ev)
		}
	}
}

func (f *ChainDataFetcher) resLoop() {
	f.wg.Add(1)
	defer f.wg.Done()
	for {
		select {
		case <-f.stopCh:
			logger.Info("stopped resLoop for chaindatafetcher")
			return
		case res := <-f.resCh:
			if res.err != nil {
				logger.Error("db insertion is failed", "blockNumber", res.blockNumber, "reqType", res.reqType, "err", res.err)
				// TODO-ChainDataFetcher add retry logic when data insertion is failed
			} else {
				if err := f.updateCheckpoint(res.blockNumber.Int64()); err != nil {
					logger.Error("Failed to update checkpoint", "err", err, "checkpoint", res.blockNumber.Int64())
				}
				// TODO-ChainDataFetcher add retry logic when checkpoint insertion is failed
			}
		}
	}
}

func (f *ChainDataFetcher) updateCheckpoint(num int64) error {
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
		return f.repo.WriteCheckpoint(newCheckpoint)
	}
	return nil
}
