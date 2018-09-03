package core

import (
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/consensus/istanbul"
	"time"
	"github.com/ground-x/go-gxplatform/blockchain/types"
)

func (c *core) sendPreprepare(request *istanbul.Request) {
	logger := c.logger.New("state", c.state)

	// If I'm the proposer and I have the same sequence with the proposal
	if c.current.Sequence().Cmp(request.Proposal.Number()) == 0 && c.isProposer() {
		curView := c.currentView()

		if c.enabledRN && c.backend.CurrentBlock().NumberU64() % 10 == 0 {
			// ranger node
			proof := &types.Proof{
				Solver:       c.backend.Address(),
				BlockNumber:  c.backend.CurrentBlock().Number(),
				Nonce: 	      c.backend.CurrentBlock().Nonce(),
			}

			proofpreprepare, err := Encode(&istanbul.ProofPreprepare{
				View:     curView,
				Proposal: request.Proposal,
				Proof:    proof,

			})
			if err != nil {
				logger.Error("Failed to encode", "view", curView)
				return
			}

			c.broadcast(&message{
				Hash: request.Proposal.ParentHash(),
				Code: msgProofPreprepare,
				Msg:  proofpreprepare,
			})

			// ranger node
			//targets := make(map[common.Address]bool)
			//// exclude validator nodes, only send ranger nodes
			//for _, addr := range c.backend.GetPeers() {
			//	var notval = true
			//	for _, val := range c.valSet.List() {
			//		if addr == val.Address() {
			//			notval = false
			//		}
			//	}
			//	if notval {
			//		targets[addr] = true
			//	}
			//}
			go c.backend.GossipProof(*proof)

		} else {

			preprepare, err := Encode(&istanbul.Preprepare{
				View:     curView,
				Proposal: request.Proposal,
			})
			if err != nil {
				logger.Error("Failed to encode", "view", curView)
				return
			}

			c.broadcast(&message{
				Hash: request.Proposal.ParentHash(),
				Code: msgPreprepare,
				Msg:  preprepare,
			})
		}
	}
}

func (c *core) handleProofPrepare(msg *message, src istanbul.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)

	var proofprepare *istanbul.ProofPreprepare
	err := msg.Decode(&proofprepare)
	if err != nil {
		return errFailedDecodePreprepare
	}

	preprepare, err := Encode(&istanbul.Preprepare{
		View:     proofprepare.View,
		Proposal: proofprepare.Proposal,
	})
	if err != nil {
		logger.Error("Failed to encode", "view", proofprepare.View)
		return err
	}

	err = c.handlePreprepare(&message{
		Hash: msg.Hash,
		Code: msgPreprepare,
		Msg:  preprepare,
	},src)
	if err != nil {
		return err
	}

	// ranger node
	//targets := make(map[common.Address]bool)
	//// exclude validator nodes, only send ranger nodes
	//for _, addr := range c.backend.GetPeers() {
	//	var notval = true
	//	for _, val := range c.valSet.List() {
	//		if addr == val.Address() {
	//			notval = false
	//		}
	//	}
	//	if notval {
	//		targets[addr] = true
	//	}
	//}
	go c.backend.GossipProof(*proofprepare.Proof)

	return nil
}

func (c *core) handlePreprepare(msg *message, src istanbul.Validator) error {
	logger := c.logger.New("from", src, "state", c.state)

	// Decode PRE-PREPARE
	var preprepare *istanbul.Preprepare
	err := msg.Decode(&preprepare)
	if err != nil {
		return errFailedDecodePreprepare
	}

	// Ensure we have the same view with the PRE-PREPARE message
	// If it is old message, see if we need to broadcast COMMIT
	if err := c.checkMessage(msgPreprepare, preprepare.View); err != nil {
		if err == errOldMessage {
			// Get validator set for the given proposal
			valSet := c.backend.ParentValidators(preprepare.Proposal).Copy()
			previousProposer := c.backend.GetProposer(preprepare.Proposal.Number().Uint64() - 1)
			valSet.CalcProposer(previousProposer, preprepare.View.Round.Uint64())
			// Broadcast COMMIT if it is an existing block
			// 1. The proposer needs to be a proposer matches the given (Sequence + Round)
			// 2. The given block must exist
			if valSet.IsProposer(src.Address()) && c.backend.HasPropsal(preprepare.Proposal.Hash(), preprepare.Proposal.Number()) {
				c.sendCommitForOldBlock(preprepare.View, preprepare.Proposal.Hash(), preprepare.Proposal.ParentHash())
				return nil
			}
		}
		return err
	}

	// Check if the message comes from current proposer
	if !c.valSet.IsProposer(src.Address()) {
		logger.Warn("Ignore preprepare messages from non-proposer")
		return errNotFromProposer
	}

	// Verify the proposal we received
	if duration, err := c.backend.Verify(preprepare.Proposal); err != nil {
		logger.Warn("Failed to verify proposal", "err", err, "duration", duration)
		// if it's a future block, we will handle it again after the duration
		if err == consensus.ErrFutureBlock {
			c.stopFuturePreprepareTimer()
			c.futurePreprepareTimer = time.AfterFunc(duration, func() {
				c.sendEvent(backlogEvent{
					src: src.Address(),
					msg: msg,
					Hash: msg.Hash,
				})
			})
		} else {
			c.sendNextRoundChange()
		}
		return err
	}

	// Here is about to accept the PRE-PREPARE
	if c.state == StateAcceptRequest {
		// Send ROUND CHANGE if the locked proposal and the received proposal are different
		if c.current.IsHashLocked() {
			if preprepare.Proposal.Hash() == c.current.GetLockedHash() {
				// Broadcast COMMIT and enters Prepared state directly
				c.acceptPreprepare(preprepare)
				c.setState(StatePrepared)
				c.sendCommit()
			} else {
				// Send round change
				c.sendNextRoundChange()
			}
		} else {
			// Either
			//   1. the locked proposal and the received proposal match
			//   2. we have no locked proposal
			c.acceptPreprepare(preprepare)
			c.setState(StatePreprepared)
			c.sendPrepare()
		}
	}

	return nil
}

func (c *core) acceptPreprepare(preprepare *istanbul.Preprepare) {
	c.consensusTimestamp = time.Now()
	c.current.SetPreprepare(preprepare)
}
