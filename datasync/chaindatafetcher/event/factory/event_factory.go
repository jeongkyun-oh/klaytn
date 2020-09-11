package factory

//import (
//	"github.com/ground-x/wallet-api/comm/event/sns"
//	"github.com/ground-x/wallet-api/common/deftype"
//	"github.com/ground-x/wallet-api/common/defvar"
//)
//
//func ConfigListen(conf config.Config) {
//	switch conf.Event.Type {
//	case defvar.EventTypeSNS:
//		event.NewEventBroker = sns.NewSNS
//
//	case defvar.EventTypeKafka:
//		event.NewEventBroker = func() deftype.EventBroker {
//			return kafka.New(conf.Event.GroupID, conf.Event.Brokers, conf.Event.Replicas)
//		}
//
//	}
//}
