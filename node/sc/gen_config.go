// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package sc

import (
	"time"

	"github.com/ground-x/klaytn/common"
)

// MarshalTOML marshals as TOML.
func (s SCConfig) MarshalTOML() (interface{}, error) {
	type SCConfig struct {
		Name                    string `toml:"-"`
		EnabledBridge           bool
		IsMainBridge            bool
		DataDir                 string
		NetworkId               uint64
		SkipBcVersionCheck      bool `toml:"-"`
		DatabaseHandles         int  `toml:"-"`
		LevelDBCacheSize        int
		TrieCacheSize           int
		TrieTimeout             time.Duration
		TrieBlockInterval       uint
		ChildChainIndexing      bool
		BridgePort              string
		MaxPeer                 int
		MainChainAccountAddr    *common.Address `toml:",omitempty"`
		ServiceChainAccountAddr *common.Address `toml:",omitempty"`
		AnchoringPeriod         uint64
		SentChainTxsLimit       uint64
		ParentChainURL          string
		VTRecovery              bool
	}
	var enc SCConfig
	enc.Name = s.Name
	enc.EnabledBridge = s.EnabledBridge
	enc.IsMainBridge = s.IsMainBridge
	enc.DataDir = s.DataDir
	enc.NetworkId = s.NetworkId
	enc.SkipBcVersionCheck = s.SkipBcVersionCheck
	enc.DatabaseHandles = s.DatabaseHandles
	enc.LevelDBCacheSize = s.LevelDBCacheSize
	enc.TrieCacheSize = s.TrieCacheSize
	enc.TrieTimeout = s.TrieTimeout
	enc.TrieBlockInterval = s.TrieBlockInterval
	enc.ChildChainIndexing = s.ChildChainIndexing
	enc.BridgePort = s.BridgePort
	enc.MaxPeer = s.MaxPeer
	enc.MainChainAccountAddr = s.MainChainAccountAddr
	enc.ServiceChainAccountAddr = s.ServiceChainAccountAddr
	enc.AnchoringPeriod = s.AnchoringPeriod
	enc.SentChainTxsLimit = s.SentChainTxsLimit
	enc.ParentChainURL = s.ParentChainURL
	enc.VTRecovery = s.VTRecovery
	return &enc, nil
}

// UnmarshalTOML unmarshals from TOML.
func (s *SCConfig) UnmarshalTOML(unmarshal func(interface{}) error) error {
	type SCConfig struct {
		Name                    *string `toml:"-"`
		EnabledBridge           *bool
		IsMainBridge            *bool
		DataDir                 *string
		NetworkId               *uint64
		SkipBcVersionCheck      *bool `toml:"-"`
		DatabaseHandles         *int  `toml:"-"`
		LevelDBCacheSize        *int
		TrieCacheSize           *int
		TrieTimeout             *time.Duration
		TrieBlockInterval       *uint
		ChildChainIndexing      *bool
		BridgePort              *string
		MaxPeer                 *int
		MainChainAccountAddr    *common.Address `toml:",omitempty"`
		ServiceChainAccountAddr *common.Address `toml:",omitempty"`
		AnchoringPeriod         *uint64
		SentChainTxsLimit       *uint64
		ParentChainURL          *string
		VTRecovery              *bool
	}
	var dec SCConfig
	if err := unmarshal(&dec); err != nil {
		return err
	}
	if dec.Name != nil {
		s.Name = *dec.Name
	}
	if dec.EnabledBridge != nil {
		s.EnabledBridge = *dec.EnabledBridge
	}
	if dec.IsMainBridge != nil {
		s.IsMainBridge = *dec.IsMainBridge
	}
	if dec.DataDir != nil {
		s.DataDir = *dec.DataDir
	}
	if dec.NetworkId != nil {
		s.NetworkId = *dec.NetworkId
	}
	if dec.SkipBcVersionCheck != nil {
		s.SkipBcVersionCheck = *dec.SkipBcVersionCheck
	}
	if dec.DatabaseHandles != nil {
		s.DatabaseHandles = *dec.DatabaseHandles
	}
	if dec.LevelDBCacheSize != nil {
		s.LevelDBCacheSize = *dec.LevelDBCacheSize
	}
	if dec.TrieCacheSize != nil {
		s.TrieCacheSize = *dec.TrieCacheSize
	}
	if dec.TrieTimeout != nil {
		s.TrieTimeout = *dec.TrieTimeout
	}
	if dec.TrieBlockInterval != nil {
		s.TrieBlockInterval = *dec.TrieBlockInterval
	}
	if dec.ChildChainIndexing != nil {
		s.ChildChainIndexing = *dec.ChildChainIndexing
	}
	if dec.BridgePort != nil {
		s.BridgePort = *dec.BridgePort
	}
	if dec.MaxPeer != nil {
		s.MaxPeer = *dec.MaxPeer
	}
	if dec.MainChainAccountAddr != nil {
		s.MainChainAccountAddr = dec.MainChainAccountAddr
	}
	if dec.ServiceChainAccountAddr != nil {
		s.ServiceChainAccountAddr = dec.ServiceChainAccountAddr
	}
	if dec.AnchoringPeriod != nil {
		s.AnchoringPeriod = *dec.AnchoringPeriod
	}
	if dec.SentChainTxsLimit != nil {
		s.SentChainTxsLimit = *dec.SentChainTxsLimit
	}
	if dec.ParentChainURL != nil {
		s.ParentChainURL = *dec.ParentChainURL
	}
	if dec.VTRecovery != nil {
		s.VTRecovery = *dec.VTRecovery
	}
	return nil
}
