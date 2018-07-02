package ranger

import (
	"math/big"
	"github.com/ground-x/go-gxplatform/core/types"
	"github.com/ground-x/go-gxplatform/common"
	"github.com/ground-x/go-gxplatform/rpc"
	"github.com/ground-x/go-gxplatform/core/state"
	"github.com/ground-x/go-gxplatform/consensus"
	"github.com/ground-x/go-gxplatform/event"
	"github.com/ground-x/go-gxplatform/core"
	"github.com/ground-x/go-gxplatform/log"
	"github.com/ground-x/go-gxplatform/consensus/istanbul"
	"github.com/ground-x/go-gxplatform/crypto/sha3"
	"github.com/ground-x/go-gxplatform/rlp"
	"github.com/hashicorp/golang-lru"
	"github.com/ground-x/go-gxplatform/p2p"
	"sync"
)

type RangerEngine struct {
	coreMu    sync.RWMutex

	proofFeed   *event.Feed
}

var (
	inmemoryAddresses  = 20 // Number of recent addresses from ecrecover
	recentAddresses, _ = lru.NewARC(inmemoryAddresses)
)

func(re *RangerEngine) Author(header *types.Header) (common.Address, error) {
	log.Info("RangeEngine.Author","header",header.Hash())
	return ecrecover(header)
}

// ecrecover extracts the GXP account address from a signed header.
func ecrecover(header *types.Header) (common.Address, error) {
	hash := header.Hash()
	if addr, ok := recentAddresses.Get(hash); ok {
		return addr.(common.Address), nil
	}

	// Retrieve the signature from the header extra-data
	istanbulExtra, err := types.ExtractIstanbulExtra(header)
	if err != nil {
		return common.Address{}, err
	}

	addr, err := istanbul.GetSignatureAddress(sigHash(header).Bytes(), istanbulExtra.Seal)
	if err != nil {
		return addr, err
	}
	recentAddresses.Add(hash, addr)
	return addr, nil
}

func sigHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewKeccak256()

	// Clean seal is required for calculating proposer seal.
	rlp.Encode(hasher, types.IstanbulFilteredHeader(header, false))
	hasher.Sum(hash[:0])
	return hash
}

func(re *RangerEngine) VerifyHeader(chain consensus.ChainReader, header *types.Header, seal bool) error {
	log.Info("RangeEngine.VerifyHeader") // ,"header",header.Hash())
	return nil
}

func(re *RangerEngine) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	abort, results := make(chan struct{}), make(chan error, len(headers))
	for i := 0; i < len(headers); i++ {
		results <- nil
	}
	return abort , results
}

func(re *RangerEngine) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	log.Debug("RangeEngine.VerifyUncles") // ,"num",block.Number(),"hash",block.Hash())
	return nil
}

func(re *RangerEngine) VerifySeal(chain consensus.ChainReader, header *types.Header) error {
	log.Debug("RangeEngine.VerifySeal") // ,"num",header.Number,"hash",header.Hash())
	return nil
}

func(re *RangerEngine) Prepare(chain consensus.ChainReader, header *types.Header) error {
	log.Debug("RangeEngine.Prepare") // ,"num",header.Number,"hash",header.Hash())
	return nil
}

func(re *RangerEngine) Finalize(chain consensus.ChainReader, header *types.Header, state *state.StateDB, txs []*types.Transaction,
		uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	log.Debug("RangeEngine.Finalize") //,"num",header.Number,"hash",header.Hash())
	return &types.Block{}, nil
}

func(re *RangerEngine) Seal(chain consensus.ChainReader, block *types.Block, stop <-chan struct{}) (*types.Block, error) {
	log.Debug("RangeEngine.Seal") //,"num",block.Number(),"hash",block.Hash())
	return &types.Block{}, nil
}

func(re *RangerEngine) CalcDifficulty(chain consensus.ChainReader, time uint64, parent *types.Header) *big.Int {
	log.Debug("RangeEngine.CalcDifficulty")
	return common.Big0
}

func(re *RangerEngine) APIs(chain consensus.ChainReader) []rpc.API {
	return []rpc.API{}
}

func(re *RangerEngine) Protocol() consensus.Protocol {
	return consensus.Protocol{
		Name:     "istanbul",
		Versions: []uint{64},
		Lengths:  []uint64{19},
	}
}

// NewChainHead handles a new head block comes
func(re *RangerEngine) NewChainHead() error {
	return nil
}

// HandleMsg handles a message from peer
func(re *RangerEngine) HandleMsg(address common.Address, msg p2p.Msg) (bool, error) {

	re.coreMu.Lock()
	defer re.coreMu.Unlock()

	if msg.Code == consensus.PoRMsg {

		////var proof types.Proof
		//dataOut := make([]byte, msg.Size)
		//if err := msg.Decode(&dataOut); err != nil {
		//	log.Error("Invalid []byte RLP", "err", err)
		//	return false, nil
		//}
		//
		////var proof types.Proof
		//proof := new(types.Proof)
		//if err := rlp.DecodeBytes(dataOut, &proof); err != nil {
		//	log.Error("Invalid proof RLP", "err", err)
		//	return false, nil
		//}

		//var proof types.Proof
		proof := new(types.Proof)
		if err := msg.Decode(&proof); err != nil {
			log.Error("Invalid proof RLP", "err", err)
			return false, nil
		}

        re.proofFeed.Send(NewProofEvent{proof})

		return true, nil
	}

	return false, nil
}

// SetBroadcaster sets the broadcaster to send message to peers
func(re *RangerEngine) SetBroadcaster(broadcaster consensus.Broadcaster) {
}

type RangeTxPool struct {

}

func(re *RangeTxPool) AddRemotes([]*types.Transaction) []error {
	log.Debug("RangeTxPool.AddRemotes")
	return nil
}

func(re *RangeTxPool) Pending() (map[common.Address]types.Transactions, error) {
	log.Debug("RangeTxPool.Pending")
	return map[common.Address]types.Transactions{}, nil
}

func(re *RangeTxPool) SubscribeNewTxsEvent(newtxch chan<- core.NewTxsEvent) event.Subscription {
	log.Debug("RangeTxPool.SubscribeNewTxsEvent")
	return nil
}