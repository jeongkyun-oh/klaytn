package work

import (
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/networks/p2p"
	"github.com/ground-x/go-gxplatform/node"
	"sync"
	"sync/atomic"
)

type CpuAgent struct {
	mu sync.Mutex

	workCh        chan *Task
	stop          chan struct{}
	quitCurrentOp chan struct{}
	returnCh      chan<- *Result

	chain  consensus.ChainReader
	engine consensus.Engine

	isMining int32 // isMining indicates whether the agent is currently mining

	nodetype p2p.ConnType
}

func NewCpuAgent(chain consensus.ChainReader, engine consensus.Engine, nodetype p2p.ConnType) *CpuAgent {
	miner := &CpuAgent{
		chain:    chain,
		engine:   engine,
		stop:     make(chan struct{}, 1),
		workCh:   make(chan *Task, 1),
		nodetype: nodetype,
	}
	return miner
}

func (self *CpuAgent) Work() chan<- *Task            { return self.workCh }
func (self *CpuAgent) SetReturnCh(ch chan<- *Result) { self.returnCh = ch }

func (self *CpuAgent) Stop() {
	if !atomic.CompareAndSwapInt32(&self.isMining, 1, 0) {
		return // agent already stopped
	}
	self.stop <- struct{}{}
done:
	// Empty work channel
	for {
		select {
		case <-self.workCh:
		default:
			break done
		}
	}
}

func (self *CpuAgent) Start() {
	if !atomic.CompareAndSwapInt32(&self.isMining, 0, 1) {
		return // agent already started
	}
	go self.update()
}

func (self *CpuAgent) update() {
out:
	for {
		select {
		case work := <-self.workCh:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
			}
			self.quitCurrentOp = make(chan struct{})
			go self.mine(work, self.quitCurrentOp)
			self.mu.Unlock()
		case <-self.stop:
			self.mu.Lock()
			if self.quitCurrentOp != nil {
				close(self.quitCurrentOp)
				self.quitCurrentOp = nil
			}
			self.mu.Unlock()
			break out
		}
	}
}

func (self *CpuAgent) mine(work *Task, stop <-chan struct{}) {
	// TODO-KLAYTN drop or missing tx and remove mining on BN, RN, GN
	if self.nodetype != node.CONSENSUSNODE {
		self.returnCh <- &Result{work, nil}
		return
	}

	if result, err := self.engine.Seal(self.chain, work.Block, stop); result != nil {
		logger.Info("Successfully sealed new block", "number", result.Number(), "hash", result.Hash())

		self.returnCh <- &Result{work, result}
	} else {
		if err != nil {
			logger.Warn("Block sealing failed", "err", err)
		}
		self.returnCh <- nil
	}
}

func (self *CpuAgent) GetHashRate() int64 {
	if pow, ok := self.engine.(consensus.PoW); ok {
		return int64(pow.Hashrate())
	}
	return 0
}
