package event

type EventPublish struct {
	topic string
}

func (r *EventSubscribe) Publish() string {
	return r.topic
}

func Publish(topic string) *EventPublish {
	return &EventPublish{
		topic: topic,
	}
}
