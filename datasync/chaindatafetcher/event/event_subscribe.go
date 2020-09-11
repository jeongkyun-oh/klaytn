package event

type EventSubscribe struct {
	topic string
}

func (r *EventSubscribe) Subscribe() string {
	return r.topic
}

func Subscribe(topic string) *EventSubscribe {
	return &EventSubscribe{
		topic: topic,
	}
}
