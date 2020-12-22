// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package cn

import (
	"math/big"
	"time"

	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/common"
	"github.com/klaytn/klaytn/common/hexutil"
	"github.com/klaytn/klaytn/consensus/istanbul"
	"github.com/klaytn/klaytn/datasync/downloader"
	"github.com/klaytn/klaytn/node/cn/gasprice"
	"github.com/klaytn/klaytn/storage/database"
	"github.com/klaytn/klaytn/storage/statedb"
)

var _ = (*configMarshaling)(nil)

// MarshalTOML marshals as TOML.
func (c Config) MarshalTOML() (interface{}, error) {
	type Config struct {
		Genesis                 *blockchain.Genesis `toml:",omitempty"`
		NetworkId               uint64
		SyncMode                downloader.SyncMode
		NoPruning               bool
		WorkerDisable           bool
		DownloaderDisable       bool
		FetcherDisable          bool
		ParentOperatorAddr      *common.Address `toml:",omitempty"`
		AnchoringPeriod         uint64
		SentChainTxsLimit       uint64
		OverwriteGenesis        bool
		DBType                  database.DBType
		SkipBcVersionCheck      bool `toml:"-"`
		SingleDB                bool
		NumStateTrieShards      uint
		LevelDBCompression      database.LevelDBCompressionType
		LevelDBBufferPool       bool
		LevelDBCacheSize        int
		DynamoDBConfig          database.DynamoDBConfig
		TrieCacheSize           int
		TrieTimeout             time.Duration
		TrieBlockInterval       uint
		TriesInMemory           uint64
		SenderTxHashIndexing    bool
		ParallelDBWrite         bool
		TrieNodeCacheConfig     statedb.TrieNodeCacheConfig
		ServiceChainSigner      common.Address `toml:",omitempty"`
		ExtraData               hexutil.Bytes  `toml:",omitempty"`
		GasPrice                *big.Int
		Rewardbase              common.Address `toml:",omitempty"`
		TxPool                  blockchain.TxPoolConfig
		GPO                     gasprice.Config
		EnablePreimageRecording bool
		EnableInternalTxTracing bool
		Istanbul                istanbul.Config
		DocRoot                 string `toml:"-"`
		WsEndpoint              string `toml:",omitempty"`
		TxResendInterval        uint64
		TxResendCount           int
		TxResendUseLegacy       bool
		NoAccountCreation       bool
		IsPrivate               bool
		AutoRestartFlag         bool
		RestartTimeOutFlag      time.Duration
		DaemonPathFlag          string
	}
	var enc Config
	enc.Genesis = c.Genesis
	enc.NetworkId = c.NetworkId
	enc.SyncMode = c.SyncMode
	enc.NoPruning = c.NoPruning
	enc.WorkerDisable = c.WorkerDisable
	enc.DownloaderDisable = c.DownloaderDisable
	enc.FetcherDisable = c.FetcherDisable
	enc.ParentOperatorAddr = c.ParentOperatorAddr
	enc.AnchoringPeriod = c.AnchoringPeriod
	enc.SentChainTxsLimit = c.SentChainTxsLimit
	enc.OverwriteGenesis = c.OverwriteGenesis
	enc.DBType = c.DBType
	enc.SkipBcVersionCheck = c.SkipBcVersionCheck
	enc.SingleDB = c.SingleDB
	enc.NumStateTrieShards = c.NumStateTrieShards
	enc.LevelDBCompression = c.LevelDBCompression
	enc.LevelDBBufferPool = c.LevelDBBufferPool
	enc.LevelDBCacheSize = c.LevelDBCacheSize
	enc.DynamoDBConfig = c.DynamoDBConfig
	enc.TrieCacheSize = c.TrieCacheSize
	enc.TrieTimeout = c.TrieTimeout
	enc.TrieBlockInterval = c.TrieBlockInterval
	enc.TriesInMemory = c.TriesInMemory
	enc.SenderTxHashIndexing = c.SenderTxHashIndexing
	enc.ParallelDBWrite = c.ParallelDBWrite
	enc.TrieNodeCacheConfig = c.TrieNodeCacheConfig
	enc.ServiceChainSigner = c.ServiceChainSigner
	enc.ExtraData = c.ExtraData
	enc.GasPrice = c.GasPrice
	enc.Rewardbase = c.Rewardbase
	enc.TxPool = c.TxPool
	enc.GPO = c.GPO
	enc.EnablePreimageRecording = c.EnablePreimageRecording
	enc.EnableInternalTxTracing = c.EnableInternalTxTracing
	enc.Istanbul = c.Istanbul
	enc.DocRoot = c.DocRoot
	enc.WsEndpoint = c.WsEndpoint
	enc.TxResendInterval = c.TxResendInterval
	enc.TxResendCount = c.TxResendCount
	enc.TxResendUseLegacy = c.TxResendUseLegacy
	enc.NoAccountCreation = c.NoAccountCreation
	enc.IsPrivate = c.IsPrivate
	enc.AutoRestartFlag = c.AutoRestartFlag
	enc.RestartTimeOutFlag = c.RestartTimeOutFlag
	enc.DaemonPathFlag = c.DaemonPathFlag
	return &enc, nil
}

// UnmarshalTOML unmarshals from TOML.
func (c *Config) UnmarshalTOML(unmarshal func(interface{}) error) error {
	type Config struct {
		Genesis                 *blockchain.Genesis `toml:",omitempty"`
		NetworkId               *uint64
		SyncMode                *downloader.SyncMode
		NoPruning               *bool
		WorkerDisable           *bool
		DownloaderDisable       *bool
		FetcherDisable          *bool
		ParentOperatorAddr      *common.Address `toml:",omitempty"`
		AnchoringPeriod         *uint64
		SentChainTxsLimit       *uint64
		OverwriteGenesis        *bool
		DBType                  *database.DBType
		SkipBcVersionCheck      *bool `toml:"-"`
		SingleDB                *bool
		NumStateTrieShards      *uint
		LevelDBCompression      *database.LevelDBCompressionType
		LevelDBBufferPool       *bool
		LevelDBCacheSize        *int
		DynamoDBConfig          *database.DynamoDBConfig
		TrieCacheSize           *int
		TrieTimeout             *time.Duration
		TrieBlockInterval       *uint
		TriesInMemory           *uint64
		SenderTxHashIndexing    *bool
		ParallelDBWrite         *bool
		TrieNodeCacheConfig     *statedb.TrieNodeCacheConfig
		ServiceChainSigner      *common.Address `toml:",omitempty"`
		ExtraData               *hexutil.Bytes  `toml:",omitempty"`
		GasPrice                *big.Int
		Rewardbase              *common.Address `toml:",omitempty"`
		TxPool                  *blockchain.TxPoolConfig
		GPO                     *gasprice.Config
		EnablePreimageRecording *bool
		EnableInternalTxTracing *bool
		Istanbul                *istanbul.Config
		DocRoot                 *string `toml:"-"`
		WsEndpoint              *string `toml:",omitempty"`
		TxResendInterval        *uint64
		TxResendCount           *int
		TxResendUseLegacy       *bool
		NoAccountCreation       *bool
		IsPrivate               *bool
		AutoRestartFlag         *bool
		RestartTimeOutFlag      *time.Duration
		DaemonPathFlag          *string
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
	if dec.WorkerDisable != nil {
		c.WorkerDisable = *dec.WorkerDisable
	}
	if dec.DownloaderDisable != nil {
		c.DownloaderDisable = *dec.DownloaderDisable
	}
	if dec.FetcherDisable != nil {
		c.FetcherDisable = *dec.FetcherDisable
	}
	if dec.ParentOperatorAddr != nil {
		c.ParentOperatorAddr = dec.ParentOperatorAddr
	}
	if dec.AnchoringPeriod != nil {
		c.AnchoringPeriod = *dec.AnchoringPeriod
	}
	if dec.SentChainTxsLimit != nil {
		c.SentChainTxsLimit = *dec.SentChainTxsLimit
	}
	if dec.OverwriteGenesis != nil {
		c.OverwriteGenesis = *dec.OverwriteGenesis
	}
	if dec.DBType != nil {
		c.DBType = *dec.DBType
	}
	if dec.SkipBcVersionCheck != nil {
		c.SkipBcVersionCheck = *dec.SkipBcVersionCheck
	}
	if dec.SingleDB != nil {
		c.SingleDB = *dec.SingleDB
	}
	if dec.NumStateTrieShards != nil {
		c.NumStateTrieShards = *dec.NumStateTrieShards
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
	if dec.DynamoDBConfig != nil {
		c.DynamoDBConfig = *dec.DynamoDBConfig
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
	if dec.TriesInMemory != nil {
		c.TriesInMemory = *dec.TriesInMemory
	}
	if dec.SenderTxHashIndexing != nil {
		c.SenderTxHashIndexing = *dec.SenderTxHashIndexing
	}
	if dec.ParallelDBWrite != nil {
		c.ParallelDBWrite = *dec.ParallelDBWrite
	}
	if dec.TrieNodeCacheConfig != nil {
		c.TrieNodeCacheConfig = *dec.TrieNodeCacheConfig
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
	if dec.EnableInternalTxTracing != nil {
		c.EnableInternalTxTracing = *dec.EnableInternalTxTracing
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
	if dec.TxResendInterval != nil {
		c.TxResendInterval = *dec.TxResendInterval
	}
	if dec.TxResendCount != nil {
		c.TxResendCount = *dec.TxResendCount
	}
	if dec.TxResendUseLegacy != nil {
		c.TxResendUseLegacy = *dec.TxResendUseLegacy
	}
	if dec.NoAccountCreation != nil {
		c.NoAccountCreation = *dec.NoAccountCreation
	}
	if dec.IsPrivate != nil {
		c.IsPrivate = *dec.IsPrivate
	}
	if dec.AutoRestartFlag != nil {
		c.AutoRestartFlag = *dec.AutoRestartFlag
	}
	if dec.RestartTimeOutFlag != nil {
		c.RestartTimeOutFlag = *dec.RestartTimeOutFlag
	}
	if dec.DaemonPathFlag != nil {
		c.DaemonPathFlag = *dec.DaemonPathFlag
	}
	return nil
}
