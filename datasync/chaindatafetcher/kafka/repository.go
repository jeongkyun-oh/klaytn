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

package kafka

import (
	"fmt"

	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/datasync/chaindatafetcher/types"
)

type repository struct {
	topicPrefix string
	blockchain  *blockchain.BlockChain
	broker      types.EventBroker
}

func NewRepository(config *KafkaConfig) *repository {
	broker := New(config.GroupID, config.BrokerList, config.Replicas, config.Partitions)
	return &repository{
		topicPrefix: config.TopicPrefix,
		broker:      broker,
	}
}

func (r *repository) SetComponent(component interface{}) {
	switch c := component.(type) {
	case *blockchain.BlockChain:
		r.blockchain = c
	}
}

func (r *repository) HandleChainEvent(event blockchain.ChainEvent, dataType types.RequestType) error {
	//var encoded []byte
	switch dataType {
	case types.RequestTypeBlockGroup:
		output, err := makeBlockGroupOutput(r.blockchain, event.Block, event.Receipts)
		if err != nil {
			return err
		}
		return r.broker.Publish(r.topicPrefix+"-blockgroup", output)
	case types.RequestTypeTraceGroup:
		return r.broker.Publish(r.topicPrefix+"-tracegroup", event.InternalTxTraces)
	default:
		return fmt.Errorf("not supported type. [blockNumber: %v, reqType: %v]", event.Block.NumberU64(), dataType)
	}
}
