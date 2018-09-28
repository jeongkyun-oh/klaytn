package work

import (
	"fmt"
	"gopkg.in/fatih/set.v0"
	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/blockchain"
	"github.com/ground-x/go-gxplatform/blockchain/state"
	"github.com/ground-x/go-gxplatform/blockchain/types"
	"github.com/ground-x/go-gxplatform/blockchain/vm"
	"github.com/ground-x/go-gxplatform/event"
	"github.com/ground-x/go-gxplatform/storage/database"
	"github.com/ground-x/go-gxplatform/log"
	"github.com/ground-x/go-gxplatform/metrics"
	"github.com/ground-x/go-gxplatform/params"
	"math/big"
	"sync"
	"sync/atomic"
	"time"
	"github.com/ground-x/go-gxplatform/networks/p2p"
	"github.com/ground-x/go-gxplatform/node"
)

const (
	resultQueueSize  = 10
	miningLogAtDepth = 5

	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
	// chainSideChanSize is the size of channel listening to ChainSideEvent.
	chainSideChanSize = 10
)

var (
	// Metrics for miner
	timeLimitReachedCounter = metrics.NewRegisteredCounter("miner/timelimitreached", nil)
	tooLongTxCounter = metrics.NewRegisteredCounter("miner/toolongtx", nil)
)

// Agent can register themself with the worker
type Agent interface {
	Work() chan<- *Task
	SetReturnCh(chan<- *Result)
	Stop()
	Start()
	GetHashRate() int64
}

// Task is the workers current environment and holds
// all of the current state information
type Task struct {
	config *params.ChainConfig
	signer types.Signer

	stateMu   sync.RWMutex        // protects state
	state     *state.StateDB      // apply state changes here
	ancestors *set.Set            // ancestor set (used for checking uncle parent validity)
	family    *set.Set            // family set (used for checking uncle invalidity)
	uncles    *set.Set            // uncle set
	tcount    int                 // tx count in cycle
	gasPool   *blockchain.GasPool // available gas used to pack transactions // TODO-GX-issue136

	Block *types.Block // the new block

	header   *types.Header
	txs      []*types.Transaction
	receipts []*types.Receipt

	createdAt time.Time
}

type Result struct {
	Task  *Task
	Block *types.Block
}

// worker is the main object which takes care of applying messages to the new state
type worker struct {
	config *params.ChainConfig
	engine consensus.Engine

	mu sync.Mutex

	// update loop
	mux          *event.TypeMux
	txsCh        chan blockchain.NewTxsEvent
	txsSub       event.Subscription
	chainHeadCh  chan blockchain.ChainHeadEvent
	chainHeadSub event.Subscription
	chainSideCh  chan blockchain.ChainSideEvent
	chainSideSub event.Subscription
	wg           sync.WaitGroup

	agents map[Agent]struct{}
	recv   chan *Result

	gxp     Backend
	chain   *blockchain.BlockChain
	proc    blockchain.Validator
	chainDB database.DBManager

	coinbase common.Address
	extra    []byte

	currentMu sync.Mutex
	current   *Task

	snapshotMu    sync.RWMutex
	snapshotBlock *types.Block
	snapshotState *state.StateDB

	uncleMu        sync.Mutex
	possibleUncles map[common.Hash]*types.Block

	unconfirmed *unconfirmedBlocks // set of locally mined blocks pending canonicalness confirmations

	// atomic status counters
	mining int32
	atWork int32

	nodetype p2p.ConnType
}

func newWorker(config *params.ChainConfig, engine consensus.Engine, coinbase common.Address, gxp Backend, mux *event.TypeMux, nodetype p2p.ConnType) *worker {
	worker := &worker{
		config:         config,
		engine:         engine,
		gxp:            gxp,
		mux:            mux,
		txsCh:          make(chan blockchain.NewTxsEvent, txChanSize),
		chainHeadCh:    make(chan blockchain.ChainHeadEvent, chainHeadChanSize),
		chainSideCh:    make(chan blockchain.ChainSideEvent, chainSideChanSize),
		chainDB:        gxp.ChainDB(),
		recv:           make(chan *Result, resultQueueSize),
		chain:          gxp.BlockChain(),
		proc:           gxp.BlockChain().Validator(),
		possibleUncles: make(map[common.Hash]*types.Block),
		coinbase:       coinbase,
		agents:         make(map[Agent]struct{}),
		unconfirmed:    newUnconfirmedBlocks(gxp.BlockChain(), miningLogAtDepth),
		nodetype:       nodetype,
	}

	// istanbul BFT
	if _, ok := engine.(consensus.Istanbul); ok || !config.IsBFT {
		// Subscribe NewTxsEvent for tx pool
		worker.txsSub = gxp.TxPool().SubscribeNewTxsEvent(worker.txsCh)
		// Subscribe events for blockchain
		worker.chainHeadSub = gxp.BlockChain().SubscribeChainHeadEvent(worker.chainHeadCh)
		worker.chainSideSub = gxp.BlockChain().SubscribeChainSideEvent(worker.chainSideCh)
		go worker.update()

		go worker.wait()
		worker.commitNewWork()
	}

	return worker
}

func (self *worker) setCoinbase(addr common.Address) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.coinbase = addr
}

func (self *worker) setExtra(extra []byte) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.extra = extra
}

func (self *worker) pending() (*types.Block, *state.StateDB) {
	if atomic.LoadInt32(&self.mining) == 0 {
		// return a snapshot to avoid contention on currentMu mutex
		self.snapshotMu.RLock()
		defer self.snapshotMu.RUnlock()
		return self.snapshotBlock, self.snapshotState.Copy()
	}

	self.currentMu.Lock()
	defer self.currentMu.Unlock()
	self.current.stateMu.Lock()
	defer self.current.stateMu.Unlock()
	return self.current.Block, self.current.state.Copy()
}

func (self *worker) pendingBlock() *types.Block {
	if atomic.LoadInt32(&self.mining) == 0 {
		// return a snapshot to avoid contention on currentMu mutex
		self.snapshotMu.RLock()
		defer self.snapshotMu.RUnlock()
		return self.snapshotBlock
	}

	self.currentMu.Lock()
	defer self.currentMu.Unlock()
	return self.current.Block
}

func (self *worker) start() {
	self.mu.Lock()
	defer self.mu.Unlock()

	atomic.StoreInt32(&self.mining, 1)

	// istanbul BFT
	if istanbul, ok := self.engine.(consensus.Istanbul); ok {
		istanbul.Start(self.chain, self.chain.CurrentBlock, self.chain.HasBadBlock)
	}

	// spin up agents
	for agent := range self.agents {
		agent.Start()
	}
}

func (self *worker) stop() {
	self.wg.Wait()

	self.mu.Lock()
	defer self.mu.Unlock()
	if atomic.LoadInt32(&self.mining) == 1 {
		for agent := range self.agents {
			agent.Stop()
		}
	}

	// istanbul BFT
	if istanbul, ok := self.engine.(consensus.Istanbul); ok {
		istanbul.Stop()
	}

	atomic.StoreInt32(&self.mining, 0)
	atomic.StoreInt32(&self.atWork, 0)
}

func (self *worker) register(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	self.agents[agent] = struct{}{}
	agent.SetReturnCh(self.recv)
}

func (self *worker) unregister(agent Agent) {
	self.mu.Lock()
	defer self.mu.Unlock()
	delete(self.agents, agent)
	agent.Stop()
}

func (self *worker) handleTxsCh(quitByErr chan bool) {
	defer self.txsSub.Unsubscribe()

	for {
		select {
		// Handle NewTxsEvent
		case ev := <-self.txsCh:
			// Apply transactions to the pending state if we're not mining.
			//
			// Note all transactions received may not be continuous with transactions
			// already included in the current mining block. These transactions will
			// be automatically eliminated.
			if atomic.LoadInt32(&self.mining) == 0 {
				self.currentMu.Lock()
				self.current.stateMu.Lock()
				txs := make(map[common.Address]types.Transactions)
				for _, tx := range ev.Txs {
					acc, err := types.Sender(self.current.signer, tx)
					if err != nil {
						log.Error("fail to types.Sender ", err)
					}
					txs[acc] = append(txs[acc], tx)
				}
				txset := types.NewTransactionsByPriceAndNonce(self.current.signer, txs) // TODO-GX-issue136 gasPrice
				self.current.commitTransactions(self.mux, txset, self.chain, self.coinbase)
				self.updateSnapshot()
				self.current.stateMu.Unlock()
				self.currentMu.Unlock()
			} else {
				// If we're mining, but nothing is being processed, wake on new transactions
				if self.config.Clique != nil && self.config.Clique.Period == 0 {
					self.commitNewWork()
				}
			}

		case <-quitByErr:
			return
		}
	}
}

func (self *worker) update() {
	defer self.chainHeadSub.Unsubscribe()
	defer self.chainSideSub.Unsubscribe()

	quitByErr := make(chan bool, 1)
	go self.handleTxsCh(quitByErr)

	for {
		// A real event arrived, process interesting content
		select {
		// Handle ChainHeadEvent
		case <-self.chainHeadCh:
			// istanbul BFT
			if h, ok := self.engine.(consensus.Handler); ok {
				h.NewChainHead()
			}
			self.commitNewWork()

			// TODO-GX-issue264 If we are using istanbul BFT, then we always have a canonical chain.
			//         Later we may be able to refine below code.
			// Handle ChainSideEvent
		case ev := <-self.chainSideCh:
			self.uncleMu.Lock()
			self.possibleUncles[ev.Block.Hash()] = ev.Block
			self.uncleMu.Unlock()

			// System stopped
		case <-self.txsSub.Err():
			quitByErr <- true
			return
		case <-self.chainHeadSub.Err():
			quitByErr <- true
			return
		case <-self.chainSideSub.Err():
			quitByErr <- true
			return
		}
	}
}

func (self *worker) wait() {
	for {
		mustCommitNewWork := true
		for result := range self.recv {
			atomic.AddInt32(&self.atWork, -1)

			if result == nil {
				continue
			}

			// TODO-KLAYTN drop or missing tx
			if self.nodetype != node.CONSENSUSNODE {
				//if self.nodetype == node.RANGERNODE || self.nodetype == node.GENERALNODE {
					self.gxp.ReBroadcastTxs(result.Task.Transactions())
				//}
				continue
			}

			block := result.Block
			work := result.Task

			// Update the block hash in all logs since it is now available and not when the
			// receipt/log of individual transactions were created.
			for _, r := range work.receipts {
				for _, l := range r.Logs {
					l.BlockHash = block.Hash()
				}
			}
			work.stateMu.Lock()
			for _, log := range work.state.Logs() {
				log.BlockHash = block.Hash()
			}

			stat, err := self.chain.WriteBlockWithState(block, work.receipts, work.state)
			if err != nil {
				log.Error("Failed writing block to chain", "err", err)
				continue
			}
			work.stateMu.Unlock()

			// TODO-GX-issue264 If we are using istanbul BFT, then we always have a canonical chain.
			//         Later we may be able to refine below code.

			// check if canon block and write transactions
			if stat == blockchain.CanonStatTy {
				// implicit by posting ChainHeadEvent
				mustCommitNewWork = false
			}

			// Broadcast the block and announce chain insertion event
			self.mux.Post(blockchain.NewMinedBlockEvent{Block: block})

			var events []interface{}

			work.stateMu.RLock()
			logs   := work.state.Logs()
			work.stateMu.RUnlock()

			events = append(events, blockchain.ChainEvent{Block: block, Hash: block.Hash(), Logs: logs})
			if stat == blockchain.CanonStatTy {
				events = append(events, blockchain.ChainHeadEvent{Block: block})
			}
			self.chain.PostChainEvents(events, logs)

			// Insert the block into the set of pending ones to wait for confirmations
			self.unconfirmed.Insert(block.NumberU64(), block.Hash())

			// TODO-GX-issue264 If we are using istanbul BFT, then we always have a canonical chain.
			//         Later we may be able to refine below code.
			if mustCommitNewWork {
				self.commitNewWork()
			}
		}
	}
}

// push sends a new work task to currently live work agents.
func (self *worker) push(work *Task) {
	if atomic.LoadInt32(&self.mining) != 1 {
		return
	}
	for agent := range self.agents {
		atomic.AddInt32(&self.atWork, 1)
		if ch := agent.Work(); ch != nil {
			ch <- work
		}
	}
}

// makeCurrent creates a new environment for the current cycle.
func (self *worker) makeCurrent(parent *types.Block, header *types.Header) error {
	state, err := self.chain.StateAt(parent.Root())
	if err != nil {
		return err
	}
	work := NewTask(self.config, types.NewEIP155Signer(self.config.ChainID), state, nil, header)

	// when 08 is processed ancestors contain 07 (quick block)
	for _, ancestor := range self.chain.GetBlocksFromHash(parent.Hash(), 7) {
		for _, uncle := range ancestor.Uncles() {
			work.family.Add(uncle.Hash())
		}
		work.family.Add(ancestor.Hash())
		work.ancestors.Add(ancestor.Hash())
	}

	// Keep track of transactions which return errors so they can be removed
	work.tcount = 0
	self.current = work
	return nil
}

func (self *worker) commitNewWork() {
	// Check any fork transitions needed
	pending, err := self.gxp.TxPool().Pending()
	if err != nil {
		log.Error("Failed to fetch pending transactions", "err", err)
		return
	}

	self.mu.Lock()
	defer self.mu.Unlock()
	self.uncleMu.Lock()
	defer self.uncleMu.Unlock()
	self.currentMu.Lock()
	defer self.currentMu.Unlock()

	tstart := time.Now()
	parent := self.chain.CurrentBlock()

	// TODO-KLAYTN drop or missing tx
	tstamp := tstart.Unix()
	if parent.Time().Cmp(new(big.Int).SetInt64(tstamp)) >= 0 {
		//if self.nodetype == node.RANGERNODE || self.nodetype == node.GENERALNODE {
		//	tstamp = parent.Time().Int64() + 5
		//} else {
			tstamp = parent.Time().Int64() + 1
		//}
	}
	// this will ensure we're not going off too far in the future
	if now := time.Now().Unix(); tstamp > now+1 {
		wait := time.Duration(tstamp-now) * time.Second
		log.Info("Mining too far in the future", "wait", common.PrettyDuration(wait))
		time.Sleep(wait)
	}

	num := parent.Number()
	header := &types.Header{
		ParentHash: parent.Hash(),
		Number:     num.Add(num, common.Big1),
		GasLimit:   blockchain.CalcGasLimit(parent),
		Extra:      self.extra,
		Time:       big.NewInt(tstamp),
	}
	// Only set the coinbase if we are mining (avoid spurious block rewards)
	if atomic.LoadInt32(&self.mining) == 1 {
		header.Coinbase = self.coinbase
	}
	if err := self.engine.Prepare(self.chain, header); err != nil {
		log.Error("Failed to prepare header for mining", "err", err)
		return
	}
	// Could potentially happen if starting to mine in an odd state.
	err = self.makeCurrent(parent, header)
	if err != nil {
		log.Error("Failed to create mining context", "err", err)
		return
	}

	// Obtain current work's state lock after we receive new work assignment.
	self.current.stateMu.Lock()
	defer self.current.stateMu.Unlock()

	// Create the current work task
	work := self.current
	txs := types.NewTransactionsByPriceAndNonce(self.current.signer, pending) // TODO-GX-issue136 gasPrice
	work.commitTransactions(self.mux, txs, self.chain, self.coinbase)

	// compute uncles for the new block.
	var (
		uncles    []*types.Header
		badUncles []common.Hash
	)
	for hash, uncle := range self.possibleUncles {
		if len(uncles) == 2 {
			break
		}
		if err := self.commitUncle(work, uncle.Header()); err != nil {
			log.Trace("Bad uncle found and will be removed", "hash", hash)
			log.Trace(fmt.Sprint(uncle))

			badUncles = append(badUncles, hash)
		} else {
			log.Debug("Committing new uncle to block", "hash", hash)
			uncles = append(uncles, uncle.Header())
		}
	}
	for _, hash := range badUncles {
		delete(self.possibleUncles, hash)
	}
	// Create the new block to seal with the consensus engine
	if work.Block, err = self.engine.Finalize(self.chain, header, work.state, work.txs, uncles, work.receipts); err != nil {
		log.Error("Failed to finalize block for sealing", "err", err)
		return
	}
	// We only care about logging if we're actually mining.
	if atomic.LoadInt32(&self.mining) == 1 {
		log.Info("Commit new mining work", "number", work.Block.Number(), "txs", work.tcount, "uncles", len(uncles), "elapsed", common.PrettyDuration(time.Since(tstart)))
		self.unconfirmed.Shift(work.Block.NumberU64() - 1)
	}

	self.push(work)
	self.updateSnapshot()
}

func (self *worker) commitUncle(work *Task, uncle *types.Header) error {
	hash := uncle.Hash()
	if work.uncles.Has(hash) {
		return fmt.Errorf("uncle not unique")
	}
	if !work.ancestors.Has(uncle.ParentHash) {
		return fmt.Errorf("uncle's parent unknown (%x)", uncle.ParentHash[0:4])
	}
	if work.family.Has(hash) {
		return fmt.Errorf("uncle already in family (%x)", hash)
	}
	work.uncles.Add(uncle.Hash())
	return nil
}

func (self *worker) updateSnapshot() {
	self.snapshotMu.Lock()
	defer self.snapshotMu.Unlock()

	self.snapshotBlock = types.NewBlock(
		self.current.header,
		self.current.txs,
		nil,
		self.current.receipts,
	)
	self.snapshotState = self.current.state.Copy()
}

func (env *Task) commitTransactions(mux *event.TypeMux, txs *types.TransactionsByPriceAndNonce, bc *blockchain.BlockChain, coinbase common.Address) {
	if env.gasPool == nil {
		env.gasPool = new(blockchain.GasPool).AddGas(env.header.GasLimit)
	}

	coalescedLogs := env.ApplyTransactions(txs, bc, coinbase)

	if len(coalescedLogs) > 0 || env.tcount > 0 {
		// make a copy, the state caches the logs and these logs get "upgraded" from pending to mined
		// logs by filling in the block hash when the block was mined by the local miner. This can
		// cause a race condition if a log was "upgraded" before the PendingLogsEvent is processed.
		cpy := make([]*types.Log, len(coalescedLogs))
		for i, l := range coalescedLogs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l
		}
		go func(logs []*types.Log, tcount int) {
			if len(logs) > 0 {
				mux.Post(blockchain.PendingLogsEvent{Logs: logs})
			}
			if tcount > 0 {
				mux.Post(blockchain.PendingStateEvent{})
			}
		}(cpy, env.tcount)
	}
}

func (env *Task) ApplyTransactions(txs *types.TransactionsByPriceAndNonce, bc *blockchain.BlockChain, coinbase common.Address) []*types.Log {
	var coalescedLogs []*types.Log

	// Limit the execution time of all transactions in a block
	var abort int32 = 0       // To break the below commitTransaction for loop when timed out
	chDone := make(chan bool) // To stop the goroutine below when processing txs is completed

	// chEVM is used to notify the below goroutine of the running EVM so it can call evm.Cancel
	// when timed out.  We use a buffered channel to prevent the main EVM execution routine
	// from being blocked due to the channel communication.
	chEVM := make(chan *vm.EVM, 1)

	go func() {
		blockTimer := time.NewTimer(params.TotalTimeLimit)
		timeout := false
		var evm *vm.EVM

		for {
			select {
			case <-blockTimer.C:
				timeout = true
				atomic.StoreInt32(&abort, 1)

			case <-chDone:
				// Everything is done. Stop this goroutine.
				return

			case evm = <-chEVM:
			}

			if timeout && evm != nil {
				// The total time limit reached, thus we stop the currently running EVM.
				evm.Cancel(vm.CancelByTotalTimeLimit)
				evm = nil
			}
		}
	}()

	vmConfig := &vm.Config{
		JumpTable: vm.ConstantinopleInstructionSet,
		RunningEVM: chEVM,
		UseOpcodeCntLimit: true,
	}

	for atomic.LoadInt32(&abort) == 0 {
		// TODO-GX-issue136
		// If we don't have enough gas for any further transactions then we're done
		if env.gasPool.Gas() < params.TxGas {
			log.Trace("Not enough gas for further transactions", "have", env.gasPool, "want", params.TxGas)
			break
		}
		// Retrieve the next transaction and abort if all done
		tx := txs.Peek()
		if tx == nil {
			break
		}
		// Error may be ignored here. The error has already been checked
		// during transaction acceptance is the transaction pool.
		//
		// We use the eip155 signer regardless of the current hf.
		from, _ := types.Sender(env.signer, tx)

		// NOTE-GX Since Klaytn is always in EIP155, the below replay protection code is not needed.
		// TODO-GX Remove the code commented below.
		// Check whether the tx is replay protected. If we're not in the EIP155 hf
		// phase, start ignoring the sender until we do.
		//if tx.Protected() && !env.config.IsEIP155(env.header.Number) {
		//	log.Trace("Ignoring reply protected transaction", "hash", tx.Hash())
		//	//log.Error("#### worker.commitTransaction","tx.protected",tx.Protected(),"tx.hash",tx.Hash(),"nonce",tx.Nonce(),"to",tx.To())
		//	txs.Pop()
		//	continue
		//}
		// Start executing the transaction
		env.state.Prepare(tx.Hash(), common.Hash{}, env.tcount)

		err, logs := env.commitTransaction(tx, bc, coinbase, env.gasPool, vmConfig)
		switch err {
		// TODO-GX-issue136
		case blockchain.ErrGasLimitReached:
			// Pop the current out-of-gas transaction without shifting in the next from the account
			log.Trace("Gas limit exceeded for current block", "sender", from)
			txs.Pop()

		case blockchain.ErrNonceTooLow:
			// New head notification data race between the transaction pool and miner, shift
			log.Trace("Skipping transaction with low nonce", "sender", from, "nonce", tx.Nonce())
			txs.Shift()

		case blockchain.ErrNonceTooHigh:
			// Reorg notification data race between the transaction pool and miner, skip account =
			log.Trace("Skipping account with hight nonce", "sender", from, "nonce", tx.Nonce())
			txs.Pop()

		case vm.ErrTotalTimeLimitReached:
			log.Warn("Transaction aborted due to time limit", "hash", tx.Hash())
			timeLimitReachedCounter.Inc(1)
			if env.tcount == 0 {
				log.Error("Transaction is too long", "hash", tx.Hash())
				tooLongTxCounter.Inc(1)
			}
			break

		case nil:
			// Everything ok, collect the logs and shift in the next transaction from the same account
			coalescedLogs = append(coalescedLogs, logs...)
			env.tcount++
			txs.Shift()

		default:
			// Strange error, discard the transaction and get the next in line (note, the
			// nonce-too-high clause will prevent us from executing in vain).
			log.Error("Transaction failed, account skipped", "hash", tx.Hash(), "err", err)
			txs.Shift()
		}
	}

	// Stop the goroutine that has been handling the timer.
	chDone <- true

	return coalescedLogs
}

func (env *Task) commitTransaction(tx *types.Transaction, bc *blockchain.BlockChain, coinbase common.Address, gp *blockchain.GasPool, vmConfig *vm.Config) (error, []*types.Log) {
	snap := env.state.Snapshot()

	receipt, _, err := blockchain.ApplyTransaction(env.config, bc, &coinbase, gp, env.state, env.header, tx, &env.header.GasUsed, vmConfig)
	if err != nil {
		env.state.RevertToSnapshot(snap)
		return err, nil
	}
	env.txs = append(env.txs, tx)
	env.receipts = append(env.receipts, receipt)

	return nil, receipt.Logs
}

func NewTask(config *params.ChainConfig, signer types.Signer, statedb *state.StateDB, gasPool *blockchain.GasPool, header *types.Header) *Task {
	return &Task{
		config:    config,
		signer:    signer,
		state:     statedb,
		ancestors: set.New(),
		family:    set.New(),
		uncles:    set.New(),
		gasPool:   gasPool,
		header:    header,
		createdAt: time.Now(),
	}
}

func (env *Task) Transactions() []*types.Transaction { return env.txs }
func (env *Task) Receipts() []*types.Receipt         { return env.receipts }
