package kafka

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/klaytn/klaytn/blockchain"
	"github.com/klaytn/klaytn/datasync/chaindatafetcher/common"

	"github.com/klaytn/klaytn/log"

	"github.com/Shopify/sarama"
	"github.com/hashicorp/go-uuid"
)

var kb *KafkaBroker
var once sync.Once
var logger = log.NewModuleLogger(log.ChainDataFetcher)

type KafkaBroker struct {
	producer sarama.AsyncProducer
	admin    sarama.ClusterAdmin
	brokers  []string
	handlers map[string]func(*sarama.ConsumerMessage) error
	consumer *Consumer
	replicas int16
}

func New(groupID string, brokerList []string, replicas int16) common.EventBroker {
	once.Do(func() {
		kb = &KafkaBroker{
			brokers:  brokerList,
			handlers: map[string]func(*sarama.ConsumerMessage) error{},
			replicas: replicas,
		}
		kb.newClusterAdmin()
		kb.newProducer()

		// TODO: context has to be passed by outside.
		kb.consumer = NewConsumer(context.Background(), kb.newConsumer(groupID))
	})

	return kb
}

func (r *KafkaBroker) Publish(topic string, msg interface{}) error {
	r.CreateTopic(topic)
	item := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(topic),
	}
	if v, ok := msg.(common.IKey); ok {
		item.Key = sarama.StringEncoder(v.Key())
	}
	logger.Info("debug msg for item and msg", "item", item, "msg", msg)
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	item.Value = sarama.StringEncoder(data)

	r.producer.Input() <- item

	return nil
}

func (r *KafkaBroker) Subscribe(ctx context.Context, topic string, typ common.SubscribeType, handler interface{}) error {
	r.CreateTopic(topic)
	h, ok := handler.(func(*sarama.ConsumerMessage) error)
	if !ok {
		return errors.New("unsupported type")
	}

	return r.consumer.Subscribe(topic, h)
}

func (r *KafkaBroker) CreateTopic(topic string) (common.Topic, error) {
	err := r.admin.CreateTopic(topic, &sarama.TopicDetail{
		NumPartitions:     10,
		ReplicationFactor: r.replicas,
	}, false)

	return common.Topic{Name: topic}, err
}

func (r *KafkaBroker) DeleteTopic(topic string) error {
	return r.admin.DeleteTopic(topic)
}

func (r *KafkaBroker) ListTopics() ([]common.Topic, error) {
	topics, err := r.admin.ListTopics()
	if err != nil {
		return nil, err
	}

	ret := []common.Topic{}
	for k := range topics {
		ret = append(ret, common.Topic{Name: k, ARN: k})
	}

	return ret, nil
}

func (r *KafkaBroker) Done() {}

func (r *KafkaBroker) newProducer() {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForLocal
	config.Producer.Compression = sarama.CompressionSnappy
	config.Producer.Flush.Frequency = 500 * time.Millisecond

	producer, err := sarama.NewAsyncProducer(r.brokers, config)
	if err != nil {
		logger.Crit("Failed to start Sarama producer", "err", err)
	}

	rand.Uint64()

	r.producer = producer
}

func (r *KafkaBroker) newConsumer(groupID string) sarama.ConsumerGroup {
	config := sarama.NewConfig()
	config.Version = sarama.MaxVersion
	config.Consumer.Group.Session.Timeout = 6 * time.Second
	config.Consumer.Group.Heartbeat.Interval = 2 * time.Second

	id, _ := uuid.GenerateUUID()
	config.ClientID = fmt.Sprintf("%s-%s", groupID, id)

	consumer, err := sarama.NewConsumerGroup(r.brokers, groupID, config)
	if err != nil {
		logger.Crit("NewConsumerGroup is failed", "err", err)
	}

	return consumer
}

func (r *KafkaBroker) newClusterAdmin() {
	config := sarama.NewConfig()
	config.Version = sarama.MaxVersion

	admin, err := sarama.NewClusterAdmin(r.brokers, config)
	if err != nil {
		logger.Crit("NewClusterAdmin is failed", "err", err)
	}
	r.admin = admin
}

func (r *KafkaBroker) InsertTransactions(event blockchain.ChainEvent) error {
	bytes, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return r.Publish("event", string(bytes))
}

func (r *KafkaBroker) InsertTokenTransfers(event blockchain.ChainEvent) error {
	return nil
}

func (r *KafkaBroker) InsertTraceResults(event blockchain.ChainEvent) error {
	return nil
}

func (r *KafkaBroker) InsertContracts(event blockchain.ChainEvent) error {
	return nil
}

func (r *KafkaBroker) ReadCheckpoint() (int64, error) {
	return 0, nil
}

func (r *KafkaBroker) WriteCheckpoint(checkpoint int64) error {
	return nil
}

func (r *KafkaBroker) SetComponent(component interface{}) {}
