package event

import (
	"github.com/klaytn/klaytn/datasync/chaindatafetcher/common"
	"github.com/klaytn/klaytn/datasync/chaindatafetcher/event/kafka"
)

var NewEventBroker = func() common.EventBroker {
	groupId := "dummy-id"
	brokers := []string{"kafka:9094"}
	replicas := int16(1)
	return kafka.New(groupId, brokers, replicas)
}
