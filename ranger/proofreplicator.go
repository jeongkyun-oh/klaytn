package ranger

import (
	"fmt"
	"math/big"
	"github.com/ground-x/go-gxplatform/core/types"
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/log"
	"github.com/ground-x/go-gxplatform/accounts"
	"github.com/ground-x/go-gxplatform/common"
)

func (rn *Ranger) proofReplication() error {

	if rn.cnClient == nil {
		return nil
	}

	coinbase, err := rn.Coinbase()
	if err != nil {
		log.Error("Cannot start proofReplication without coinbase", "err", err)
		return fmt.Errorf("coinbase missing: %v", err)
	}

	account := accounts.Account{Address: coinbase}
	wallet , err := rn.accountManager.Find(account)
	if err != nil {
		log.Error("find err","err",err)
		return err
	}

	to := common.Address{}
	amount := big.NewInt(0)
	gaslimit := uint64(18446744073709551614)
	gasprice := big.NewInt(0)
	data := []byte{}

	currentProof := types.Proof{common.Address{},big.NewInt(0),0}

	for {
		select {
		case msg := <-rn.proofCh:
			if !currentProof.Compare(*msg.proof) {
				currentProof = *msg.proof
				statedb, _ := rn.blockchain.State()
				nonce := statedb.GetNonce(account.Address)

				log.Error("receive msg", "num", msg.proof.BlockNumber, "proof.nonce", msg.proof.Nonce, "addr", msg.proof.Solver,"nonce",nonce)

				var chainID *big.Int
				tx, err := wallet.SignTx(account, types.NewTransaction(nonce, to, amount, gaslimit, gasprice, data), chainID)
				if err != nil {
					log.Error("fail to make signed transaction", "err", err)
					continue
				}

				// send tx directly
				//err := rn.cnClient.SendTransaction(context.Background(), tx)
				//if err != nil {
				//	log.Error("fail to send transaction", "err", err)
				//}

				if cached, ok := rn.peerCache.Get(msg.addr);ok {
					cpeer := *cached.(*consensus.Peer)
					err := cpeer.Send(consensus.PoRSendMsg,tx)
					if err != nil {
						log.Error("fail to send transaction", "err", err)
					}
				} else {
					m := make(map[common.Address]bool)
					m[msg.addr] = true
					peermap := rn.protocolManager.FindPeers(m)
					p := peermap[msg.addr]
					err = p.Send(consensus.PoRSendMsg,tx)
					if err != nil {
						log.Error("fail to send transaction", "err", err)
					}
					rn.peerCache.Add(msg.addr, &p)
				}
			}
		default:
		}
	}
}