package common

import (
	"context"
	"time"

	"github.com/klaytn/klaytn/blockchain"
)

type SubscribeType string

type EventBroker interface {
	Repository
	Publish(topic string, msg interface{}) error
	Subscribe(ctx context.Context, topic string, typ SubscribeType, arg interface{}) error
	CreateTopic(topic string) (Topic, error)
	DeleteTopic(arn string) error
	ListTopics() ([]Topic, error)
	Done()
}

//type Key struct {
//	Name  string
//	Value interface{}
//}
//
//type CreateItem struct {
//	Keys []Key
//	Doc  interface{}
//}
//
//type DeleteItem struct {
//	Keys []Key
//}

type Topic struct {
	Name string
	ARN  string
}

type IKey interface {
	Key() string
}

const DBInsertRetryInterval = 500 * time.Millisecond

//go:generate mockgen -destination=./mocks/repository_mock.go -package=mocks github.com/klaytn/klaytn/datasync/chaindatafetcher Repository
type Repository interface {
	InsertTransactions(event blockchain.ChainEvent) error
	InsertTokenTransfers(event blockchain.ChainEvent) error
	InsertTraceResults(event blockchain.ChainEvent) error
	InsertContracts(event blockchain.ChainEvent) error

	ReadCheckpoint() (int64, error)
	WriteCheckpoint(checkpoint int64) error

	SetComponent(component interface{})
}
