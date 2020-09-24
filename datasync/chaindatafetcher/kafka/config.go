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

import "github.com/Shopify/sarama"

const (
	DefaultReplicas   = 1
	DefaultPartitions = 1
)

type KafkaConfig struct {
	SaramaConfig *sarama.Config // kafka client configurations.
	Brokers      []string       // Brokers is a list of broker URLs.
	Partitions   int32          // Partitions is the number of partitions of a topic.
	Replicas     int16          // Replicas is a replication factor of kafka settings. This is the number of the replicated partitions in the kafka cluster.
}

func GetDefaultKafkaConfig() *KafkaConfig {
	// TODO-ChainDataFetcher add more configuration if necessary
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Version = sarama.MaxVersion
	return &KafkaConfig{
		SaramaConfig: config,
		Partitions:   DefaultPartitions,
		Replicas:     DefaultReplicas,
	}
}
