// Copyright 2018 The go-klaytn Authors
// This file is part of the go-klaytn library.
//
// The go-klaytn library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-klaytn library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-klaytn library. If not, see <http://www.gnu.org/licenses/>.

package common

import (
	"errors"
	"github.com/ground-x/klaytn/log"
	"github.com/hashicorp/golang-lru"
	"math"
)

type CacheType int

const (
	LRUCacheType CacheType = iota
	LRUShardCacheType
	ARCChacheType
)

//it's set by flag
var DefaultCacheType CacheType = LRUCacheType
var CacheScale int = 100 // cache size = preset size * CacheScale / 100
var logger = log.NewModuleLogger(log.Common)

type CacheKey interface {
	getShardIndex(shardMask int) int
}

type Cache interface {
	Add(key CacheKey, value interface{}) (evicted bool)
	Get(key CacheKey) (value interface{}, ok bool)
	Contains(key CacheKey) bool
	Purge()
}

type lruCache struct {
	lru *lru.Cache
}

func (cache *lruCache) Add(key CacheKey, value interface{}) (evicted bool) {
	return cache.lru.Add(key, value)
}

func (cache *lruCache) Get(key CacheKey) (value interface{}, ok bool) {
	value, ok = cache.lru.Get(key)
	return
}

func (cache *lruCache) Contains(key CacheKey) bool {
	return cache.lru.Contains(key)
}

func (cache *lruCache) Purge() {
	cache.lru.Purge()
}

func (cache *lruCache) Keys() []interface{} {
	return cache.lru.Keys()
}

func (cache *lruCache) Peek(key CacheKey) (value interface{}, ok bool) {
	return cache.lru.Peek(key)
}

func (cache *lruCache) Remove(key CacheKey) {
	cache.lru.Remove(key)
}

func (cache *lruCache) Len() int {
	return cache.lru.Len()
}

type arcCache struct {
	arc *lru.ARCCache
}

func (cache *arcCache) Add(key CacheKey, value interface{}) (evicted bool) {
	cache.arc.Add(key, value)
	//TODO-GX: need to be removed or should be added according to usage of evicted flag
	return true
}

func (cache *arcCache) Get(key CacheKey) (value interface{}, ok bool) {
	return cache.arc.Get(key)
}

func (cache *arcCache) Contains(key CacheKey) bool {
	return cache.arc.Contains(key)
}

func (cache *arcCache) Purge() {
	cache.arc.Purge()
}

func (cache *arcCache) Keys() []interface{} {
	return cache.arc.Keys()
}

func (cache *arcCache) Peek(key CacheKey) (value interface{}, ok bool) {
	return cache.arc.Peek(key)
}

func (cache *arcCache) Remove(key CacheKey) {
	cache.arc.Remove(key)
}

func (cache *arcCache) Len() int {
	return cache.arc.Len()
}

type lruShardCache struct {
	shards         []*lru.Cache
	shardIndexMask int
}

func (cache *lruShardCache) Add(key CacheKey, val interface{}) (evicted bool) {
	shardIndex := key.getShardIndex(cache.shardIndexMask)
	return cache.shards[shardIndex].Add(key, val)
}

func (cache *lruShardCache) Get(key CacheKey) (value interface{}, ok bool) {
	shardIndex := key.getShardIndex(cache.shardIndexMask)
	return cache.shards[shardIndex].Get(key)
}

func (cache *lruShardCache) Contains(key CacheKey) bool {
	shardIndex := key.getShardIndex(cache.shardIndexMask)
	return cache.shards[shardIndex].Contains(key)
}

func (cache *lruShardCache) Purge() {
	for _, shard := range cache.shards {
		s := shard
		go s.Purge()
	}
}

func NewCache(config CacheConfiger) (Cache, error) {
	if config == nil {
		return nil, errors.New("cache config is nil")
	}
	return config.newCache()
}

type CacheConfiger interface {
	newCache() (Cache, error)
}

type LRUConfig struct {
	CacheSize int
}

func (c LRUConfig) newCache() (Cache, error) {
	cacheSize := c.CacheSize * CacheScale / 100
	lru, err := lru.New(cacheSize)
	return &lruCache{lru}, err
}

type LRUShardConfig struct {
	CacheSize int
	NumShards int
}

const (
	minShardSize = 10
	minNumShards = 2
)

//If key is not common.Hash nor common.Address then you should set numShard 1 or use LRU Cache
//The number of shards is readjusted to meet the minimum shard size.
func (c LRUShardConfig) newCache() (Cache, error) {
	cacheSize := c.CacheSize * CacheScale / 100

	if cacheSize < 1 {
		logger.Error("Negative Cache Size Error", "Cache Size", cacheSize, "Cache Scale", CacheScale)
		return nil, errors.New("Must provide a positive size ")
	}

	numShards := c.makeNumShardsPowOf2()

	if c.NumShards != numShards {
		logger.Warn("numShards is ", "Expected", c.NumShards, "Actual", numShards)
	}
	if cacheSize%numShards != 0 {
		logger.Warn("Cache size is ", "Expected", cacheSize, "Actual", cacheSize-(cacheSize%numShards))
	}

	lruShard := &lruShardCache{shards: make([]*lru.Cache, numShards), shardIndexMask: numShards - 1}
	shardsSize := cacheSize / numShards
	var err error
	for i := 0; i < numShards; i++ {
		lruShard.shards[i], err = lru.NewWithEvict(shardsSize, nil)

		if err != nil {
			return nil, err
		}
	}
	return lruShard, nil
}

func (c LRUShardConfig) makeNumShardsPowOf2() int {
	maxNumShards := float64(c.CacheSize * CacheScale / 100 / minShardSize)
	numShards := int(math.Min(float64(c.NumShards), maxNumShards))

	preNumShards := minNumShards
	for numShards > minNumShards {
		preNumShards = numShards
		numShards = numShards & (numShards - 1)
	}

	return preNumShards
}

type ARCConfig struct {
	CacheSize int
}

func (c ARCConfig) newCache() (Cache, error) {
	arc, err := lru.NewARC(c.CacheSize)
	return &arcCache{arc}, err
}
