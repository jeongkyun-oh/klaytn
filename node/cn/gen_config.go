// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package cn

import (
	"math/big"
	"time"

	"github.com/ground-x/klaytn/blockchain"
	"github.com/ground-x/klaytn/common"
	"github.com/ground-x/klaytn/common/hexutil"
	"github.com/ground-x/klaytn/consensus/istanbul"
	"github.com/ground-x/klaytn/datasync/downloader"
	"github.com/ground-x/klaytn/node/cn/gasprice"
	"github.com/ground-x/klaytn/storage/database"
)

var _ = (*configMarshaling)(nil)

// MarshalTOML marshals as TOML.
func (c Config) MarshalTOML() (interface{}, error) {
	type Config struct {
		Genesis                 *blockchain.Genesis `toml:",omitempty"`
		NetworkId               uint64
		SyncMode                downloader.SyncMode
		NoPruning               bool
		MainChainAccountAddr    *common.Address `toml:",omitempty"`
		AnchoringPeriod         uint64
		SentChainTxsLimit       uint64
		SkipBcVersionCheck      bool `toml:"-"`
		PartitionedDB           bool
		NumStateTriePartitions  uint
		LevelDBCompression      database.LevelDBCompressionType
		LevelDBBufferPool       bool
		LevelDBCacheSize        int
		TrieCacheSize           int
		TrieTimeout             time.Duration
		TrieBlockInterval       uint
		SenderTxHashIndexing    bool
		ChildChainIndexing      bool
		ParallelDBWrite         bool
		StateDBCaching          bool
		TxPoolStateCache        bool
		TrieCacheLimit          int
		ServiceChainSigner      common.Address `toml:",omitempty"`
		ExtraData               hexutil.Bytes  `toml:",omitempty"`
		GasPrice                *big.Int
		Rewardbase              common.Address `toml:",omitempty"`
		TxPool                  blockchain.TxPoolConfig
		GPO                     gasprice.Config
		EnablePreimageRecording bool
		Istanbul                istanbul.Config
		DocRoot                 string `toml:"-"`
		WsEndpoint              string `toml:",omitempty"`
	}
	var enc Config
	enc.Genesis = c.Genesis
	enc.NetworkId = c.NetworkId
	enc.SyncMode = c.SyncMode
	enc.NoPruning = c.NoPruning
	enc.MainChainAccountAddr = c.MainChainAccountAddr
	enc.AnchoringPeriod = c.AnchoringPeriod
	enc.SentChainTxsLimit = c.SentChainTxsLimit
	enc.SkipBcVersionCheck = c.SkipBcVersionCheck
	enc.PartitionedDB = c.PartitionedDB
	enc.NumStateTriePartitions = c.NumStateTriePartitions
	enc.LevelDBCompression = c.LevelDBCompression
	enc.LevelDBBufferPool = c.LevelDBBufferPool
	enc.LevelDBCacheSize = c.LevelDBCacheSize
	enc.TrieCacheSize = c.TrieCacheSize
	enc.TrieTimeout = c.TrieTimeout
	enc.TrieBlockInterval = c.TrieBlockInterval
	enc.SenderTxHashIndexing = c.SenderTxHashIndexing
	enc.ChildChainIndexing = c.ChildChainIndexing
	enc.ParallelDBWrite = c.ParallelDBWrite
	enc.StateDBCaching = c.StateDBCaching
	enc.TxPoolStateCache = c.TxPoolStateCache
	enc.TrieCacheLimit = c.TrieCacheLimit
	enc.ServiceChainSigner = c.ServiceChainSigner
	enc.ExtraData = c.ExtraData
	enc.GasPrice = c.GasPrice
	enc.Rewardbase = c.Rewardbase
	enc.TxPool = c.TxPool
	enc.GPO = c.GPO
	enc.EnablePreimageRecording = c.EnablePreimageRecording
	enc.Istanbul = c.Istanbul
	enc.DocRoot = c.DocRoot
	enc.WsEndpoint = c.WsEndpoint
	return &enc, nil
}

// UnmarshalTOML unmarshals from TOML.
func (c *Config) UnmarshalTOML(unmarshal func(interface{}) error) error {
	type Config struct {
		Genesis                 *blockchain.Genesis `toml:",omitempty"`
		NetworkId               *uint64
		SyncMode                *downloader.SyncMode
		NoPruning               *bool
		MainChainAccountAddr    *common.Address `toml:",omitempty"`
		AnchoringPeriod         *uint64
		SentChainTxsLimit       *uint64
		SkipBcVersionCheck      *bool `toml:"-"`
		PartitionedDB           *bool
		NumStateTriePartitions  *uint
		LevelDBCompression      *database.LevelDBCompressionType
		LevelDBBufferPool       *bool
		LevelDBCacheSize        *int
		TrieCacheSize           *int
		TrieTimeout             *time.Duration
		TrieBlockInterval       *uint
		SenderTxHashIndexing    *bool
		ChildChainIndexing      *bool
		ParallelDBWrite         *bool
		StateDBCaching          *bool
		TxPoolStateCache        *bool
		TrieCacheLimit          *int
		ServiceChainSigner      *common.Address `toml:",omitempty"`
		ExtraData               *hexutil.Bytes  `toml:",omitempty"`
		GasPrice                *big.Int
		Rewardbase              *common.Address `toml:",omitempty"`
		TxPool                  *blockchain.TxPoolConfig
		GPO                     *gasprice.Config
		EnablePreimageRecording *bool
		Istanbul                *istanbul.Config
		DocRoot                 *string `toml:"-"`
		WsEndpoint              *string `toml:",omitempty"`
	}
	var dec Config
	if err := unmarshal(&dec); err != nil {
		return err
	}
	if dec.Genesis != nil {
		c.Genesis = dec.Genesis
	}
	if dec.NetworkId != nil {
		c.NetworkId = *dec.NetworkId
	}
	if dec.SyncMode != nil {
		c.SyncMode = *dec.SyncMode
	}
	if dec.NoPruning != nil {
		c.NoPruning = *dec.NoPruning
	}
	if dec.MainChainAccountAddr != nil {
		c.MainChainAccountAddr = dec.MainChainAccountAddr
	}
	if dec.AnchoringPeriod != nil {
		c.AnchoringPeriod = *dec.AnchoringPeriod
	}
	if dec.SentChainTxsLimit != nil {
		c.SentChainTxsLimit = *dec.SentChainTxsLimit
	}
	if dec.SkipBcVersionCheck != nil {
		c.SkipBcVersionCheck = *dec.SkipBcVersionCheck
	}
	if dec.PartitionedDB != nil {
		c.PartitionedDB = *dec.PartitionedDB
	}
	if dec.NumStateTriePartitions != nil {
		c.NumStateTriePartitions = *dec.NumStateTriePartitions
	}
	if dec.LevelDBCompression != nil {
		c.LevelDBCompression = *dec.LevelDBCompression
	}
	if dec.LevelDBBufferPool != nil {
		c.LevelDBBufferPool = *dec.LevelDBBufferPool
	}
	if dec.LevelDBCacheSize != nil {
		c.LevelDBCacheSize = *dec.LevelDBCacheSize
	}
	if dec.TrieCacheSize != nil {
		c.TrieCacheSize = *dec.TrieCacheSize
	}
	if dec.TrieTimeout != nil {
		c.TrieTimeout = *dec.TrieTimeout
	}
	if dec.TrieBlockInterval != nil {
		c.TrieBlockInterval = *dec.TrieBlockInterval
	}
	if dec.SenderTxHashIndexing != nil {
		c.SenderTxHashIndexing = *dec.SenderTxHashIndexing
	}
	if dec.ChildChainIndexing != nil {
		c.ChildChainIndexing = *dec.ChildChainIndexing
	}
	if dec.ParallelDBWrite != nil {
		c.ParallelDBWrite = *dec.ParallelDBWrite
	}
	if dec.StateDBCaching != nil {
		c.StateDBCaching = *dec.StateDBCaching
	}
	if dec.TxPoolStateCache != nil {
		c.TxPoolStateCache = *dec.TxPoolStateCache
	}
	if dec.TrieCacheLimit != nil {
		c.TrieCacheLimit = *dec.TrieCacheLimit
	}
	if dec.ServiceChainSigner != nil {
		c.ServiceChainSigner = *dec.ServiceChainSigner
	}
	if dec.ExtraData != nil {
		c.ExtraData = *dec.ExtraData
	}
	if dec.GasPrice != nil {
		c.GasPrice = dec.GasPrice
	}
	if dec.Rewardbase != nil {
		c.Rewardbase = *dec.Rewardbase
	}
	if dec.TxPool != nil {
		c.TxPool = *dec.TxPool
	}
	if dec.GPO != nil {
		c.GPO = *dec.GPO
	}
	if dec.EnablePreimageRecording != nil {
		c.EnablePreimageRecording = *dec.EnablePreimageRecording
	}
	if dec.Istanbul != nil {
		c.Istanbul = *dec.Istanbul
	}
	if dec.DocRoot != nil {
		c.DocRoot = *dec.DocRoot
	}
	if dec.WsEndpoint != nil {
		c.WsEndpoint = *dec.WsEndpoint
	}
	return nil
}
